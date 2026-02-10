package remove

import (
	"fmt"

	"github.com/launchrctl/launchr/pkg/action"
	"github.com/plasmash/plasmactl-chassis/internal/chassis"
)

// Remove implements the chassis:remove command
type Remove struct {
	action.WithLogger
	action.WithTerm

	Chassis string
}

// Execute runs the remove action
func (r *Remove) Execute() error {
	c, err := chassis.Load(".")
	if err != nil {
		return err
	}

	if !c.Exists(r.Chassis) {
		r.Term().Error().Printfln("Chassis %q not found", r.Chassis)
		return nil
	}

	// Check for allocated nodes
	nodesByPlatform, err := chassis.LoadNodesByPlatform(".")
	if err != nil {
		r.Log().Debug("Failed to load nodes", "error", err)
	}

	var allocatedNodes []string
	for platform, nodes := range nodesByPlatform {
		for _, node := range chassis.NodesForChassis(nodes, r.Chassis) {
			allocatedNodes = append(allocatedNodes, fmt.Sprintf("[%s] %s", platform, node.Hostname))
		}
	}

	if len(allocatedNodes) > 0 {
		r.Term().Error().Printfln("Cannot remove chassis path %q: nodes are allocated", r.Chassis)
		r.Term().Println()
		r.Term().Info().Println("Allocated nodes:")
		for _, n := range allocatedNodes {
			r.Term().Printfln("  %s", n)
		}
		r.Term().Println()
		r.Term().Info().Println("Use node:allocate <hostname> <chassis>- to deallocate first")
		return nil
	}

	// Check for attached components
	attachments, err := chassis.LoadAttachments(".", r.Chassis)
	if err != nil {
		r.Log().Debug("Failed to load attachments", "error", err)
	}

	if len(attachments) > 0 {
		r.Term().Error().Printfln("Cannot remove chassis path %q: components are attached", r.Chassis)
		r.Term().Println()
		r.Term().Info().Println("Attached components:")
		for _, a := range attachments {
			r.Term().Printfln("  %s", a.Component)
		}
		r.Term().Println()
		r.Term().Info().Println("Use component:detach to detach first")
		return nil
	}

	// Safe to remove
	if err := c.Remove(r.Chassis); err != nil {
		return err
	}

	if err := c.Save("."); err != nil {
		return err
	}

	r.Term().Success().Printfln("Removed: %s", r.Chassis)
	return nil
}
