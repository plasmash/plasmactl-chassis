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
	DryRun             bool     `json:"dry_run,omitempty"`
	UpdatedAttachments []string `json:"updated_attachments,omitempty"`
	UpdatedAllocations []string `json:"updated_allocations,omitempty"`
}

// Rename implements the chassis:rename command
type Rename struct {
	action.WithLogger
	action.WithTerm

	Dir    string
	Old    string
	New    string
	DryRun bool

	result *RenameResult
}

// Result returns the structured result for JSON output.
func (r *Rename) Result() any {
	return r.result
}

// Execute runs the rename action
func (r *Rename) Execute() error {
	c, err := chassis.Load(r.Dir)
	if err != nil {
		return err
	}

	if !c.Exists(r.Old) {
		return fmt.Errorf("chassis %q does not exist", r.Old)
	}

	if c.Exists(r.New) {
		return fmt.Errorf("chassis %q already exists", r.New)
	}

	if r.DryRun {
		return r.executeDryRun()
	}

	// Rename in chassis.yaml
	if err := c.Rename(r.Old, r.New); err != nil {
		return fmt.Errorf("failed to rename chassis path: %w", err)
	}

	if err := c.Save(r.Dir); err != nil {
		return err
	}

	// Update attachments
	updatedAttachments, err := chassis.UpdateAttachments(r.Dir, r.Old, r.New)
	if err != nil {
		r.Term().Warning().Printfln("Chassis renamed but failed to update attachments: %s", err)
	}

	// Update allocations
	updatedAllocations, err := chassis.UpdateAllocations(r.Dir, r.Old, r.New)
	if err != nil {
		r.Term().Warning().Printfln("Chassis renamed but failed to update allocations: %s", err)
	}

	r.result = &RenameResult{
		Old:                r.Old,
		New:                r.New,
		UpdatedAttachments: updatedAttachments,
		UpdatedAllocations: updatedAllocations,
	}

	r.Term().Success().Printfln("Renamed: %s -> %s", r.Old, r.New)
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

// executeDryRun shows what would change without modifying any files.
func (r *Rename) executeDryRun() error {
	r.Term().Info().Println("[dry-run] No changes will be made")
	r.Term().Printfln("  chassis.yaml: %s -> %s", r.Old, r.New)

	// Find affected attachment files
	attachments, err := chassis.LoadAttachments(r.Dir, r.Old)
	if err != nil {
		r.Log().Debug("Failed to load attachments", "error", err)
	}

	seen := make(map[string]bool)
	var affectedPlaybooks []string
	for _, a := range attachments {
		if !seen[a.Playbook] {
			seen[a.Playbook] = true
			affectedPlaybooks = append(affectedPlaybooks, a.Playbook)
		}
	}

	// Find affected allocation files
	nodesByPlatform, err := chassis.LoadNodesByPlatform(r.Dir)
	if err != nil {
		r.Log().Debug("Failed to load nodes", "error", err)
	}

	var affectedNodeFiles []string
	for platform, nodes := range nodesByPlatform {
		for _, n := range chassis.NodesForChassis(nodes, r.Old) {
			affectedNodeFiles = append(affectedNodeFiles, fmt.Sprintf("inst/%s/nodes/%s.yaml", platform, n.Hostname))
		}
	}

	if len(affectedPlaybooks) > 0 {
		r.Term().Info().Println("Would update attachments:")
		for _, p := range affectedPlaybooks {
			r.Term().Printfln("  - %s", p)
		}
	}
	if len(affectedNodeFiles) > 0 {
		r.Term().Info().Println("Would update allocations:")
		for _, p := range affectedNodeFiles {
			r.Term().Printfln("  - %s", p)
		}
	}

	r.result = &RenameResult{
		Old:                r.Old,
		New:                r.New,
		DryRun:             true,
		UpdatedAttachments: affectedPlaybooks,
		UpdatedAllocations: affectedNodeFiles,
	}

	return nil
}
