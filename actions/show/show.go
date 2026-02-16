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

// AllocationInfo represents a node allocation
type AllocationInfo struct {
	Node     string   `json:"node"`
	Platform string   `json:"platform"`
	Chassis  []string `json:"chassis"`
}

// DisplayName returns the node formatted as "hostname@platform".
func (a AllocationInfo) DisplayName() string {
	return a.Node + "@" + a.Platform
}

// AttachmentInfo represents a component attachment
type AttachmentInfo struct {
	Component string `json:"component"`
	Version   string `json:"version,omitempty"`
	Chassis   string `json:"chassis"`
}

// DisplayName returns the component formatted as "name@version".
func (a AttachmentInfo) DisplayName() string {
	return component.FormatDisplayName(a.Component, a.Version)
}

// ShowResult is the structured output for chassis:show
type ShowResult struct {
	Chassis     string           `json:"chassis,omitempty"`
	Allocations []AllocationInfo `json:"allocations,omitempty"`
	Attachments []AttachmentInfo `json:"attachments,omitempty"`
}

// Show implements the chassis:show command
type Show struct {
	action.WithLogger
	action.WithTerm

	Dir      string
	Chassis  string
	Platform string
	Kind     string // "allocations" or "attachments" to filter

	result *ShowResult
}

// Result returns the structured result for JSON output
func (s *Show) Result() any {
	return s.result
}

// Execute runs the show action
func (s *Show) Execute() error {
	c, err := chassis.Load(s.Dir)
	if err != nil {
		return err
	}

	// If chassis path specified, validate it exists
	if s.Chassis != "" && !c.Exists(s.Chassis) {
		return fmt.Errorf("chassis %q not found in chassis.yaml", s.Chassis)
	}

	showAllocations := s.Kind == "" || s.Kind == "allocations"
	showAttachments := s.Kind == "" || s.Kind == "attachments"

	// Load all nodes by platform
	nodesByPlatform, err := node.LoadByPlatform(s.Dir)
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
	components, err := component.LoadFromPlaybooks(s.Dir)
	if err != nil {
		s.Log().Debug("Failed to load components", "error", err)
	}

	// Build version map for quick lookup
	versionMap := make(map[string]string)
	for _, comp := range components {
		versionMap[comp.Name] = comp.Version
	}

	// Get attachments map (component → chassis paths)
	attachmentsMap := components.Attachments(c)

	// Collect component attachments for the chassis path
	type componentInfo struct {
		chassis   string
		component string
		version   string
	}
	var compInfos []componentInfo

	for compName, chassisPaths := range attachmentsMap {
		for _, chassisPath := range chassisPaths {
			// Check if chassis path matches query (exact match or descendant)
			if s.Chassis == "" || chassisPath == s.Chassis || chassis.IsDescendantOf(chassisPath, s.Chassis) {
				compInfos = append(compInfos, componentInfo{
					chassis:   chassisPath,
					component: compName,
					version:   versionMap[compName],
				})
			}
		}
	}

	// Sort components by chassis path, then component name
	sort.Slice(compInfos, func(i, j int) bool {
		if compInfos[i].chassis != compInfos[j].chassis {
			return compInfos[i].chassis < compInfos[j].chassis
		}
		return compInfos[i].component < compInfos[j].component
	})

	// Collect all node allocations (EFFECTIVE - after distribution)
	type nodeInfo struct {
		platform string
		node     string
		chassis  []string // effective chassis paths after distribution
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
			effectiveChassis := allocations[n.Hostname]

			// If chassis filter is specified, check if node is allocated to it
			if s.Chassis != "" {
				found := false
				for _, chassisPath := range effectiveChassis {
					if chassisPath == s.Chassis || chassis.IsDescendantOf(chassisPath, s.Chassis) {
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
				node:     n.Hostname,
				chassis:  effectiveChassis,
			})
		}
	}

	// Sort nodes by platform, then node
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].platform != nodes[j].platform {
			return nodes[i].platform < nodes[j].platform
		}
		return nodes[i].node < nodes[j].node
	})

	// Build result
	s.result = &ShowResult{
		Chassis: s.Chassis,
	}

	for _, n := range nodes {
		s.result.Allocations = append(s.result.Allocations, AllocationInfo{
			Node:     n.node,
			Platform: n.platform,
			Chassis:  n.chassis,
		})
	}

	for _, comp := range compInfos {
		s.result.Attachments = append(s.result.Attachments, AttachmentInfo{
			Component: comp.component,
			Version:   comp.version,
			Chassis:   comp.chassis,
		})
	}

	// Output
	hasAllocations := showAllocations && len(s.result.Allocations) > 0
	hasAttachments := showAttachments && len(s.result.Attachments) > 0

	if !hasAllocations && !hasAttachments {
		s.Term().Info().Println("No allocations or attachments found")
		return nil
	}

	if hasAllocations {
		s.Term().Info().Printfln("Allocations (%d nodes)", len(s.result.Allocations))
		for _, n := range s.result.Allocations {
			chassisStr := strings.Join(n.Chassis, ", ")
			if len(chassisStr) > 60 {
				chassisStr = chassisStr[:57] + "..."
			}
			s.Term().Printfln("  %s  [%s]", n.DisplayName(), chassisStr)
		}
	}

	if hasAttachments {
		s.Term().Info().Printfln("Attachments (%d components)", len(s.result.Attachments))
		for _, a := range s.result.Attachments {
			s.Term().Printfln("  %s  @ %s", a.DisplayName(), a.Chassis)
		}
	}

	return nil
}
