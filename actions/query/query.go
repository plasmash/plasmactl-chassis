package query

import (
	"sort"

	"github.com/launchrctl/launchr/pkg/action"
	"github.com/plasmash/plasmactl-chassis/pkg/chassis"
	"github.com/plasmash/plasmactl-component/pkg/component"
	"github.com/plasmash/plasmactl-node/pkg/node"
)

// QueryResult is the structured output for chassis:query
type QueryResult struct {
	Paths []string `json:"paths"`
}

// Query implements the chassis:query command
type Query struct {
	action.WithLogger
	action.WithTerm

	Identifier string
	Kind       string // "node" or "component" to skip auto-detection

	result QueryResult
}

// Execute runs the query action
func (q *Query) Execute() error {
	// Load chassis for distribution computation
	c, err := chassis.Load(".")
	if err != nil {
		return err
	}

	var chassisPaths []string

	// Search based on kind or auto-detect
	searchNode := q.Kind == "" || q.Kind == "node"
	searchComponent := q.Kind == "" || q.Kind == "component"

	// Search in nodes (allocations with distribution)
	if searchNode {
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
					chassisPaths = append(chassisPaths, allocations[n.Hostname]...)
				}
			}
		}
	}

	// Search in attachments (components)
	if searchComponent && len(chassisPaths) == 0 {
		components, err := component.LoadFromPlaybooks(".")
		if err != nil {
			q.Log().Debug("Failed to load components", "error", err)
		}

		attachmentsMap := components.Attachments(c)
		if attached, ok := attachmentsMap[q.Identifier]; ok {
			chassisPaths = append(chassisPaths, attached...)
		}
	}

	if len(chassisPaths) == 0 {
		q.Term().Warning().Printfln("No allocation or attachment found for %q", q.Identifier)
		return nil
	}

	// Remove duplicates and sort
	seen := make(map[string]bool)
	unique := []string{}
	for _, p := range chassisPaths {
		if !seen[p] {
			seen[p] = true
			unique = append(unique, p)
		}
	}
	sort.Strings(unique)

	q.result.Paths = unique

	for _, s := range unique {
		q.Term().Printfln("%s", s)
	}

	return nil
}

// Result returns the structured result for JSON output
func (q *Query) Result() any {
	return q.result
}
