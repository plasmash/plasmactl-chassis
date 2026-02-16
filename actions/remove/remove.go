package remove

import (
	"fmt"
	"strings"

	"github.com/launchrctl/launchr/pkg/action"
	"github.com/plasmash/plasmactl-chassis/internal/chassis"
	"github.com/plasmash/plasmactl-node/pkg/node"
)

// RemoveResult is the structured result of chassis:remove.
type RemoveResult struct {
	Chassis            string   `json:"chassis"`
	DryRun             bool     `json:"dry_run,omitempty"`
	AllocatedNodes     []string `json:"allocated_nodes,omitempty"`
	AttachedComponents []string `json:"attached_components,omitempty"`
}

// Remove implements the chassis:remove command
type Remove struct {
	action.WithLogger
	action.WithTerm

	Dir     string
	Chassis string
	DryRun  bool

	result *RemoveResult
}

// Result returns the structured result for JSON output.
func (r *Remove) Result() any {
	return r.result
}

// Execute runs the remove action
func (r *Remove) Execute() error {
	c, err := chassis.Load(r.Dir)
	if err != nil {
		return err
	}

	if !c.Exists(r.Chassis) {
		return fmt.Errorf("chassis %q not found", r.Chassis)
	}

	// Check for allocated nodes using distributed allocations
	nodesByPlatform, err := node.LoadByPlatform(r.Dir)
	if err != nil {
		r.Log().Debug("Failed to load nodes", "error", err)
	}

	var allocatedNodes []string
	for _, nodes := range nodesByPlatform {
		allocations := nodes.Allocations(c.Chassis)
		for _, n := range nodes {
			for _, cp := range allocations[n.Hostname] {
				if cp == r.Chassis || strings.HasPrefix(cp, r.Chassis+".") {
					allocatedNodes = append(allocatedNodes, n.DisplayName())
					break
				}
			}
		}
	}

	// Check for attached components
	attachments, err := chassis.LoadAttachments(r.Dir, r.Chassis)
	if err != nil {
		r.Log().Debug("Failed to load attachments", "error", err)
	}

	var attachedComponents []string
	for _, a := range attachments {
		attachedComponents = append(attachedComponents, a.Component)
	}

	// Dry-run: report what would block removal
	if r.DryRun {
		r.result = &RemoveResult{
			Chassis:            r.Chassis,
			DryRun:             true,
			AllocatedNodes:     allocatedNodes,
			AttachedComponents: attachedComponents,
		}

		r.Term().Info().Println("[dry-run] No changes will be made")
		if len(allocatedNodes) > 0 {
			r.Term().Info().Println("Allocated nodes:")
			for _, n := range allocatedNodes {
				r.Term().Printfln("  %s", n)
			}
		}
		if len(attachedComponents) > 0 {
			r.Term().Info().Println("Attached components:")
			for _, comp := range attachedComponents {
				r.Term().Printfln("  %s", comp)
			}
		}
		if len(allocatedNodes) == 0 && len(attachedComponents) == 0 {
			r.Term().Success().Printfln("Safe to remove: %s", r.Chassis)
		}
		return nil
	}

	// Check blockers
	if len(allocatedNodes) > 0 {
		r.Term().Info().Println("Allocated nodes:")
		for _, n := range allocatedNodes {
			r.Term().Printfln("  %s", n)
		}
		return fmt.Errorf("cannot remove chassis %q: %d node(s) are allocated (deallocate them first)", r.Chassis, len(allocatedNodes))
	}

	if len(attachedComponents) > 0 {
		r.Term().Info().Println("Attached components:")
		for _, comp := range attachedComponents {
			r.Term().Printfln("  %s", comp)
		}
		return fmt.Errorf("cannot remove chassis %q: %d component(s) are attached (detach them first)", r.Chassis, len(attachedComponents))
	}

	// Safe to remove
	if err := c.Remove(r.Chassis); err != nil {
		return err
	}

	if err := c.Save(r.Dir); err != nil {
		return err
	}

	r.result = &RemoveResult{Chassis: r.Chassis}
	r.Term().Success().Printfln("Removed: %s", r.Chassis)
	return nil
}
