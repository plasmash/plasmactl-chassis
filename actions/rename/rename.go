package rename

import (
	"fmt"

	"github.com/launchrctl/launchr/pkg/action"
	"github.com/plasmash/plasmactl-chassis/internal/chassis"
)

// Rename implements the chassis:rename command
type Rename struct {
	action.WithLogger
	action.WithTerm

	Old string
	New string
}

// Execute runs the rename action
func (r *Rename) Execute() error {
	c, err := chassis.Load(".")
	if err != nil {
		return err
	}

	if !c.Exists(r.Old) {
		r.Term().Error().Printfln("Chassis %q does not exist", r.Old)
		return nil
	}

	if c.Exists(r.New) {
		r.Term().Error().Printfln("Chassis %q already exists", r.New)
		return nil
	}

	// Rename in chassis.yaml
	if err := c.Rename(r.Old, r.New); err != nil {
		r.Term().Error().Printfln("Failed to rename chassis path: %s", err)
		return nil
	}

	if err := c.Save("."); err != nil {
		return err
	}

	// Update attachments
	updatedAttachments, err := chassis.UpdateAttachments(".", r.Old, r.New)
	if err != nil {
		r.Term().Warning().Printfln("Chassis renamed but failed to update attachments: %s", err)
	}

	// Update allocations
	updatedAllocations, err := chassis.UpdateAllocations(".", r.Old, r.New)
	if err != nil {
		r.Term().Warning().Printfln("Chassis renamed but failed to update allocations: %s", err)
	}

	r.Term().Success().Printfln("Renamed: %s → %s", r.Old, r.New)
	if len(updatedAttachments) > 0 {
		r.Term().Info().Println("Updated attachments:")
		for _, p := range updatedAttachments {
			fmt.Printf("  - %s\n", p)
		}
	}
	if len(updatedAllocations) > 0 {
		r.Term().Info().Println("Updated allocations:")
		for _, p := range updatedAllocations {
			fmt.Printf("  - %s\n", p)
		}
	}

	return nil
}
