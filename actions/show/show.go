package show

import (
	"sort"

	"github.com/launchrctl/launchr/pkg/action"
	"github.com/plasmash/plasmactl-chassis/internal/chassis"
)

// Show implements the chassis:show command
type Show struct {
	action.WithLogger
	action.WithTerm

	Section  string
	Platform string
}

// Execute runs the show action
func (s *Show) Execute() error {
	c, err := chassis.Load(".")
	if err != nil {
		return err
	}

	if !c.Exists(s.Section) {
		s.Term().Error().Printfln("Section %q not found in chassis.yaml", s.Section)
		return nil
	}

	s.Term().Info().Printfln("Section: %s", s.Section)
	s.Term().Println()

	// Show allocated nodes
	nodesByPlatform, err := chassis.LoadNodesByPlatform(".")
	if err != nil {
		s.Log().Debug("Failed to load nodes", "error", err)
	}

	s.Term().Info().Println("Allocated nodes:")
	hasNodes := false

	// Get sorted platform names
	var platforms []string
	for platform := range nodesByPlatform {
		if s.Platform == "" || s.Platform == platform {
			platforms = append(platforms, platform)
		}
	}
	sort.Strings(platforms)

	for _, platform := range platforms {
		nodes := nodesByPlatform[platform]
		sectionNodes := chassis.NodesForSection(nodes, s.Section)
		for _, node := range sectionNodes {
			s.Term().Printfln("  [%s] %s", platform, node.Hostname)
			hasNodes = true
		}
	}

	if !hasNodes {
		s.Term().Printfln("  (none)")
	}

	s.Term().Println()

	// Show attached components
	attachments, err := chassis.LoadAttachments(".", s.Section)
	if err != nil {
		s.Log().Debug("Failed to load attachments", "error", err)
	}

	s.Term().Info().Println("Attached components:")
	if len(attachments) == 0 {
		s.Term().Printfln("  (none)")
	} else {
		for _, a := range attachments {
			s.Term().Printfln("  %s", a.Component)
		}
	}

	return nil
}
