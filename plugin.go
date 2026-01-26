// Package plasmactlchassis implements a launchr plugin with chassis management functionality
package plasmactlchassis

import (
	"context"
	"embed"

	"github.com/launchrctl/launchr"
	"github.com/launchrctl/launchr/pkg/action"

	caction "github.com/plasmash/plasmactl-chassis/action"
)

//go:embed action/*.yaml
var actionYamlFS embed.FS

func init() {
	launchr.RegisterPlugin(&Plugin{})
}

// Plugin is [launchr.Plugin] plugin providing chassis management functionality.
type Plugin struct {
	cfg launchr.Config
}

// PluginInfo implements [launchr.Plugin] interface.
func (p *Plugin) PluginInfo() launchr.PluginInfo {
	return launchr.PluginInfo{
		Weight: 10,
	}
}

// OnAppInit implements [launchr.Plugin] interface.
func (p *Plugin) OnAppInit(app launchr.App) error {
	app.Services().Get(&p.cfg)
	return nil
}

// DiscoverActions implements [launchr.ActionDiscoveryPlugin] interface.
func (p *Plugin) DiscoverActions(_ context.Context) ([]*action.Action, error) {
	// chassis:list - List chassis sections
	listYaml, _ := actionYamlFS.ReadFile("action/list.yaml")
	listAct := action.NewFromYAML("chassis:list", listYaml)
	listAct.SetRuntime(action.NewFnRuntime(func(_ context.Context, a *action.Action) error {
		input := a.Input()
		log, term := getLogger(a)

		section := ""
		if input.Arg("section") != nil {
			section = input.Arg("section").(string)
		}

		list := &caction.List{
			Section: section,
			Tree:    input.Opt("tree").(bool),
		}
		list.SetLogger(log)
		list.SetTerm(term)
		return list.Execute()
	}))

	// chassis:show - Show chassis section details
	showYaml, _ := actionYamlFS.ReadFile("action/show.yaml")
	showAct := action.NewFromYAML("chassis:show", showYaml)
	showAct.SetRuntime(action.NewFnRuntime(func(_ context.Context, a *action.Action) error {
		input := a.Input()
		log, term := getLogger(a)

		show := &caction.Show{
			Section:  input.Arg("section").(string),
			Platform: input.Opt("platform").(string),
		}
		show.SetLogger(log)
		show.SetTerm(term)
		return show.Execute()
	}))

	// chassis:add - Add a chassis section
	addYaml, _ := actionYamlFS.ReadFile("action/add.yaml")
	addAct := action.NewFromYAML("chassis:add", addYaml)
	addAct.SetRuntime(action.NewFnRuntime(func(_ context.Context, a *action.Action) error {
		input := a.Input()
		log, term := getLogger(a)

		add := &caction.Add{
			Section: input.Arg("section").(string),
		}
		add.SetLogger(log)
		add.SetTerm(term)
		return add.Execute()
	}))

	// chassis:remove - Remove a chassis section
	removeYaml, _ := actionYamlFS.ReadFile("action/remove.yaml")
	removeAct := action.NewFromYAML("chassis:remove", removeYaml)
	removeAct.SetRuntime(action.NewFnRuntime(func(_ context.Context, a *action.Action) error {
		input := a.Input()
		log, term := getLogger(a)

		remove := &caction.Remove{
			Section: input.Arg("section").(string),
		}
		remove.SetLogger(log)
		remove.SetTerm(term)
		return remove.Execute()
	}))

	return []*action.Action{
		listAct,
		showAct,
		addAct,
		removeAct,
	}, nil
}

func getLogger(a *action.Action) (*launchr.Logger, *launchr.Terminal) {
	log := launchr.Log()
	if rt, ok := a.Runtime().(action.RuntimeLoggerAware); ok {
		log = rt.LogWith()
	}

	term := launchr.Term()
	if rt, ok := a.Runtime().(action.RuntimeTermAware); ok {
		term = rt.Term()
	}

	return log, term
}
