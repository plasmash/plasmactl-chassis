package rename

import (
	"fmt"

	"github.com/launchrctl/launchr/pkg/action"
	"github.com/plasmash/plasmactl-chassis/internal/chassis"
)

// RenameResult is the structured result of chassis:rename.
type RenameResult struct {
	Old                string   `json:"old"`
	New                string   `json:"new"`
	UpdatedAttachments []string `json:"updated_attachments,omitempty"`
	UpdatedAllocations []string `json:"updated_allocations,omitempty"`
}

// Rename implements the chassis:rename command
type Rename struct {
	action.WithLogger
	action.WithTerm

	Old string
	New string

	result *RenameResult
}

// Result returns the structured result for JSON output.
func (r *Rename) Result() any {
	return r.result
}

// Execute runs the rename action
func (r *Rename) Execute() error {
	c, err := chassis.Load(".")
	if err != nil {
		return err
	}

	if !c.Exists(r.Old) {
		return fmt.Errorf("chassis %q does not exist", r.Old)
	}

	if c.Exists(r.New) {
		return fmt.Errorf("chassis %q already exists", r.New)
	}

	// Rename in chassis.yaml
	if err := c.Rename(r.Old, r.New); err != nil {
		return fmt.Errorf("failed to rename chassis path: %w", err)
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

	r.result = &RenameResult{
		Old:                r.Old,
		New:                r.New,
		UpdatedAttachments: updatedAttachments,
		UpdatedAllocations: updatedAllocations,
	}

	r.Term().Success().Printfln("Renamed: %s → %s", r.Old, r.New)
	if len(updatedAttachments) > 0 {
		r.Term().Info().Println("Updated attachments:")
		for _, p := range updatedAttachments {
			r.Term().Printfln("  - %s", p)
		}
	}
	if len(updatedAllocations) > 0 {
		r.Term().Info().Println("Updated allocations:")
		for _, p := range updatedAllocations {
			r.Term().Printfln("  - %s", p)
		}
	}

	return nil
}
