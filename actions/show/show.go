package show

import (
	"fmt"
	"sort"
	"strings"

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

	// Collect all attachments for the section (and children)
	type componentInfo struct {
		section   string
		component string
	}
	var components []componentInfo

	// Determine query section - use root if not specified
	sectionToQuery := s.Section
	if sectionToQuery == "" {
		// Find root from chassis (first segment of first path)
		roots := c.Flatten()
		if len(roots) > 0 {
			parts := strings.SplitN(roots[0], ".", 2)
			sectionToQuery = parts[0]
		}
	}

	if sectionToQuery != "" {
		attachments, _ := chassis.LoadAttachments(".", sectionToQuery)
		for _, a := range attachments {
			components = append(components, componentInfo{
				section:   a.Section,
				component: a.Component,
			})
		}
	}

	// Sort components by section, then component name
	sort.Slice(components, func(i, j int) bool {
		if components[i].section != components[j].section {
			return components[i].section < components[j].section
		}
		return components[i].component < components[j].component
	})

	// Collect all nodes
	type nodeInfo struct {
		platform string
		hostname string
	}
	var nodes []nodeInfo

	// Get sorted platform names
	var platforms []string
	for platform := range nodesByPlatform {
		if s.Platform == "" || s.Platform == platform {
			platforms = append(platforms, platform)
		}
	}
	sort.Strings(platforms)

	for _, platform := range platforms {
		platformNodes := nodesByPlatform[platform]
		matchingNodes := chassis.NodesForSection(platformNodes, s.Section)
		if s.Section == "" {
			// No section filter - include all nodes
			matchingNodes = platformNodes
		}
		for _, node := range matchingNodes {
			nodes = append(nodes, nodeInfo{
				platform: platform,
				hostname: node.Hostname,
			})
		}
	}

	// Sort nodes by platform, then hostname
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].platform != nodes[j].platform {
			return nodes[i].platform < nodes[j].platform
		}
		return nodes[i].hostname < nodes[j].hostname
	})

	// Output
	if len(components) == 0 && len(nodes) == 0 {
		s.Term().Info().Println("No allocations or attachments found")
		return nil
	}

	for _, node := range nodes {
		fmt.Printf("node: %s [%s]\n", node.hostname, node.platform)
	}

	for _, comp := range components {
		fmt.Printf("component: %s [%s]\n", comp.component, comp.section)
	}

	return nil
}
