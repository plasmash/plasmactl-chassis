package show

import (
	"fmt"
	"sort"
	"strings"

	"github.com/launchrctl/launchr/pkg/action"
	"github.com/plasmash/plasmactl-chassis/pkg/chassis"
	"github.com/plasmash/plasmactl-component/pkg/component"
	"github.com/plasmash/plasmactl-node/pkg/node"
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

	// Load all nodes by platform
	nodesByPlatform, err := node.LoadByPlatform(".")
	if err != nil {
		s.Log().Debug("Failed to load nodes", "error", err)
	}

	// Filter by platform if specified
	if s.Platform != "" {
		filtered := make(map[string]node.Nodes)
		if nodes, ok := nodesByPlatform[s.Platform]; ok {
			filtered[s.Platform] = nodes
		}
		nodesByPlatform = filtered
	}

	// Load components from playbooks
	components, err := component.LoadFromPlaybooks(".")
	if err != nil {
		s.Log().Debug("Failed to load components", "error", err)
	}

	// Get attachments map (component → sections)
	attachmentsMap := components.Attachments(c)

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

	// Collect component attachments for the section
	type componentInfo struct {
		section   string
		component string
	}
	var compInfos []componentInfo

	for compName, sections := range attachmentsMap {
		for _, sect := range sections {
			// Check if section matches query (exact match or descendant)
			if sectionToQuery == "" || sect == sectionToQuery || chassis.IsDescendantOf(sect, sectionToQuery) {
				compInfos = append(compInfos, componentInfo{
					section:   sect,
					component: compName,
				})
			}
		}
	}

	// Sort components by section, then component name
	sort.Slice(compInfos, func(i, j int) bool {
		if compInfos[i].section != compInfos[j].section {
			return compInfos[i].section < compInfos[j].section
		}
		return compInfos[i].component < compInfos[j].component
	})

	// Collect all node allocations (EFFECTIVE - after distribution)
	type nodeInfo struct {
		platform string
		hostname string
		sections []string // effective sections after distribution
	}
	var nodes []nodeInfo

	// Get sorted platform names
	var platforms []string
	for platform := range nodesByPlatform {
		platforms = append(platforms, platform)
	}
	sort.Strings(platforms)

	for _, platform := range platforms {
		platformNodes := nodesByPlatform[platform]

		// Compute effective allocations for all nodes in this platform
		allocations := platformNodes.Allocations(c)

		for _, n := range platformNodes {
			effectiveSections := allocations[n.Hostname]

			// If section filter is specified, check if node is allocated to it
			if s.Section != "" {
				found := false
				for _, sect := range effectiveSections {
					if sect == s.Section || chassis.IsDescendantOf(sect, s.Section) {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}

			nodes = append(nodes, nodeInfo{
				platform: platform,
				hostname: n.Hostname,
				sections: effectiveSections,
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
	if len(compInfos) == 0 && len(nodes) == 0 {
		s.Term().Info().Println("No allocations or attachments found")
		return nil
	}

	if len(nodes) > 0 {
		s.Term().Info().Printfln("Allocations (%d nodes)", len(nodes))
		for _, n := range nodes {
			// Show node with section count - use chassis:query for details
			fmt.Printf("node\t%s\t%s\t(%d sections)\n", n.hostname, n.platform, len(n.sections))
		}
	}

	if len(compInfos) > 0 {
		if len(nodes) > 0 {
			fmt.Println()
		}
		s.Term().Info().Printfln("Attachments (%d components)", len(compInfos))
		for _, comp := range compInfos {
			fmt.Printf("component\t%s\t%s\n", comp.component, comp.section)
		}
	}

	return nil
}
