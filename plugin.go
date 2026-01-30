// Package plasmactlchassis implements a launchr plugin with chassis management functionality
package plasmactlchassis

import (
	"context"
	"embed"

	"github.com/launchrctl/launchr"
	"github.com/launchrctl/launchr/pkg/action"

	"github.com/plasmash/plasmactl-chassis/actions/add"
	"github.com/plasmash/plasmactl-chassis/actions/list"
	"github.com/plasmash/plasmactl-chassis/actions/query"
	"github.com/plasmash/plasmactl-chassis/actions/remove"
	"github.com/plasmash/plasmactl-chassis/actions/rename"
	"github.com/plasmash/plasmactl-chassis/actions/show"
)

//go:embed actions/*/*.yaml
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
	listYaml, _ := actionYamlFS.ReadFile("actions/list/list.yaml")
	listAct := action.NewFromYAML("chassis:list", listYaml)
	listAct.SetRuntime(action.NewFnRuntime(func(_ context.Context, a *action.Action) error {
		input := a.Input()
		log, term := getLogger(a)

		section := ""
		if input.Arg("section") != nil {
			section = input.Arg("section").(string)
		}

		list := &list.List{
			Section: section,
			Tree:    input.Opt("tree").(bool),
		}
		list.SetLogger(log)
		list.SetTerm(term)
		return list.Execute()
	}))

	// chassis:show - Show chassis section details
	showYaml, _ := actionYamlFS.ReadFile("actions/show/show.yaml")
	showAct := action.NewFromYAML("chassis:show", showYaml)
	showAct.SetRuntime(action.NewFnRuntime(func(_ context.Context, a *action.Action) error {
		input := a.Input()
		log, term := getLogger(a)

		section := ""
		if input.Arg("section") != nil {
			section = input.Arg("section").(string)
		}

		show := &show.Show{
			Section:  section,
			Platform: input.Opt("platform").(string),
		}
		show.SetLogger(log)
		show.SetTerm(term)
		return show.Execute()
	}))

	// chassis:add - Add a chassis section
	addYaml, _ := actionYamlFS.ReadFile("actions/add/add.yaml")
	addAct := action.NewFromYAML("chassis:add", addYaml)
	addAct.SetRuntime(action.NewFnRuntime(func(_ context.Context, a *action.Action) error {
		input := a.Input()
		log, term := getLogger(a)

		add := &add.Add{
			Section: input.Arg("section").(string),
		}
		add.SetLogger(log)
		add.SetTerm(term)
		return add.Execute()
	}))

	// chassis:remove - Remove a chassis section
	removeYaml, _ := actionYamlFS.ReadFile("actions/remove/remove.yaml")
	removeAct := action.NewFromYAML("chassis:remove", removeYaml)
	removeAct.SetRuntime(action.NewFnRuntime(func(_ context.Context, a *action.Action) error {
		input := a.Input()
		log, term := getLogger(a)

		remove := &remove.Remove{
			Section: input.Arg("section").(string),
		}
		remove.SetLogger(log)
		remove.SetTerm(term)
		return remove.Execute()
	}))

	// chassis:rename - Rename a chassis section
	renameYaml, _ := actionYamlFS.ReadFile("actions/rename/rename.yaml")
	renameAct := action.NewFromYAML("chassis:rename", renameYaml)
	renameAct.SetRuntime(action.NewFnRuntime(func(_ context.Context, a *action.Action) error {
		input := a.Input()
		log, term := getLogger(a)

		ren := &rename.Rename{
			Old: input.Arg("old").(string),
			New: input.Arg("new").(string),
		}
		ren.SetLogger(log)
		ren.SetTerm(term)
		return ren.Execute()
	}))

	// chassis:query - Query chassis sections for a node or component
	queryYaml, _ := actionYamlFS.ReadFile("actions/query/query.yaml")
	queryAct := action.NewFromYAML("chassis:query", queryYaml)
	queryAct.SetRuntime(action.NewFnRuntime(func(_ context.Context, a *action.Action) error {
		input := a.Input()
		log, term := getLogger(a)

		q := &query.Query{
			Identifier: input.Arg("identifier").(string),
		}
		q.SetLogger(log)
		q.SetTerm(term)
		return q.Execute()
	}))

	return []*action.Action{
		listAct,
		showAct,
		addAct,
		removeAct,
		renameAct,
		queryAct,
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
