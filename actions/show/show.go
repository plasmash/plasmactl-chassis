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

	// If section specified, validate it exists
	if s.Section != "" && !c.Exists(s.Section) {
		s.Term().Error().Printfln("Section %q not found in chassis.yaml", s.Section)
		return nil
	}

	// Load all nodes
	nodesByPlatform, err := chassis.LoadNodesByPlatform(".")
	if err != nil {
		s.Log().Debug("Failed to load nodes", "error", err)
	}

	// Get sections to show
	var sections []string
	if s.Section != "" {
		sections = c.FlattenWithPrefix(s.Section)
	} else {
		sections = c.Flatten()
	}

	// Collect allocations by section
	type sectionInfo struct {
		nodes       map[string][]string // platform -> hostnames
		attachments []chassis.Attachment
	}
	sectionData := make(map[string]*sectionInfo)

	for _, section := range sections {
		info := &sectionInfo{
			nodes: make(map[string][]string),
		}

		// Get sorted platform names
		var platforms []string
		for platform := range nodesByPlatform {
			if s.Platform == "" || s.Platform == platform {
				platforms = append(platforms, platform)
			}
		}
		sort.Strings(platforms)

		// Find nodes for this exact section (not children)
		for _, platform := range platforms {
			nodes := nodesByPlatform[platform]
			for _, node := range nodes {
				for _, nc := range node.Chassis {
					if nc == section {
						info.nodes[platform] = append(info.nodes[platform], node.Hostname)
						break
					}
				}
			}
		}

		// Find attachments for this exact section
		allAttachments, _ := chassis.LoadAttachments(".", section)
		for _, a := range allAttachments {
			if a.Section == section {
				info.attachments = append(info.attachments, a)
			}
		}

		// Only include sections with allocations or attachments
		if len(info.nodes) > 0 || len(info.attachments) > 0 {
			sectionData[section] = info
		}
	}

	if len(sectionData) == 0 {
		s.Term().Info().Println("No allocations or attachments found")
		return nil
	}

	// Display results
	for _, section := range sections {
		info, exists := sectionData[section]
		if !exists {
			continue
		}

		s.Term().Info().Printfln("%s", section)

		// Show nodes
		for platform, hostnames := range info.nodes {
			for _, hostname := range hostnames {
				s.Term().Printfln("  node: [%s] %s", platform, hostname)
			}
		}

		// Show attachments
		for _, a := range info.attachments {
			s.Term().Printfln("  component: %s", a.Component)
		}

		s.Term().Println()
	}

	return nil
}
