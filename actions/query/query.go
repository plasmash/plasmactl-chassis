package query

import (
	"fmt"
	"sort"

	"github.com/launchrctl/launchr/pkg/action"
	"github.com/plasmash/plasmactl-chassis/internal/chassis"
)

// Query implements the chassis:query command
type Query struct {
	action.WithLogger
	action.WithTerm

	Identifier string
}

// Execute runs the query action
func (q *Query) Execute() error {
	var sections []string

	// Search in nodes (allocations)
	nodesByPlatform, err := chassis.LoadNodesByPlatform(".")
	if err != nil {
		q.Log().Debug("Failed to load nodes", "error", err)
	}

	for _, nodes := range nodesByPlatform {
		for _, node := range nodes {
			if node.Hostname == q.Identifier {
				sections = append(sections, node.Chassis...)
			}
		}
	}

	// Search in attachments (components)
	if len(sections) == 0 {
		// Load chassis to get root
		c, err := chassis.Load(".")
		if err != nil {
			return err
		}

		// Find root from chassis
		roots := c.Flatten()
		if len(roots) > 0 {
			// Get the root (first segment)
			root := roots[0]
			for i, ch := range roots[0] {
				if ch == '.' {
					root = roots[0][:i]
					break
				}
			}

			attachments, _ := chassis.LoadAttachments(".", root)
			for _, a := range attachments {
				if a.Component == q.Identifier {
					sections = append(sections, a.Section)
				}
			}
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
