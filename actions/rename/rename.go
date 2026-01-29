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
		r.Term().Error().Printfln("Section %q does not exist", r.Old)
		return nil
	}

	if c.Exists(r.New) {
		r.Term().Error().Printfln("Section %q already exists", r.New)
		return nil
	}

	// Rename in chassis.yaml
	if err := c.Rename(r.Old, r.New); err != nil {
		r.Term().Error().Printfln("Failed to rename section: %s", err)
		return nil
	}

	if err := c.Save("."); err != nil {
		return err
	}

	// Update playbook references
	updated, err := chassis.UpdatePlaybookReferences(".", r.Old, r.New)
	if err != nil {
		r.Term().Warning().Printfln("Chassis renamed but failed to update playbooks: %s", err)
		return nil
	}

	r.Term().Success().Printfln("Renamed: %s → %s", r.Old, r.New)
	if len(updated) > 0 {
		r.Term().Info().Println("Updated playbooks:")
		for _, p := range updated {
			fmt.Printf("  - %s\n", p)
		}
	}

	return nil
}
