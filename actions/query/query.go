package query

import (
	"fmt"
	"sort"

	"github.com/launchrctl/launchr/pkg/action"
	"github.com/plasmash/plasmactl-chassis/pkg/chassis"
	"github.com/plasmash/plasmactl-component/pkg/component"
	"github.com/plasmash/plasmactl-node/pkg/node"
)

// Query implements the chassis:query command
type Query struct {
	action.WithLogger
	action.WithTerm

	Identifier string
}

// Execute runs the query action
func (q *Query) Execute() error {
	// Load chassis for distribution computation
	c, err := chassis.Load(".")
	if err != nil {
		return err
	}

	var sections []string

	// Search in nodes (allocations with distribution)
	nodesByPlatform, err := node.LoadByPlatform(".")
	if err != nil {
		q.Log().Debug("Failed to load nodes", "error", err)
	}

	for _, nodes := range nodesByPlatform {
		// Compute effective allocations for all nodes in this platform
		allocations := nodes.Allocations(c)

		for _, n := range nodes {
			if n.Hostname == q.Identifier {
				// Use effective allocations (after distribution)
				sections = append(sections, allocations[n.Hostname]...)
			}
		}
	}

	// Search in attachments (components)
	if len(sections) == 0 {
		components, err := component.LoadFromPlaybooks(".")
		if err != nil {
			q.Log().Debug("Failed to load components", "error", err)
		}

		attachmentsMap := components.Attachments(c)
		if attachedSections, ok := attachmentsMap[q.Identifier]; ok {
			sections = append(sections, attachedSections...)
		}
	}

	if len(sections) == 0 {
		q.Term().Warning().Printfln("No allocation or attachment found for %q", q.Identifier)
		return nil
	}

	// Remove duplicates and sort
	seen := make(map[string]bool)
	unique := []string{}
	for _, s := range sections {
		if !seen[s] {
			seen[s] = true
			unique = append(unique, s)
		}
	}
	sort.Strings(unique)

	for _, s := range unique {
		fmt.Println(s)
	}

	return nil
}
