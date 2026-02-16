package add

import (
	"fmt"

	"github.com/launchrctl/launchr/pkg/action"
	"github.com/plasmash/plasmactl-chassis/internal/chassis"
)

// AddResult is the structured result of chassis:add.
type AddResult struct {
	Chassis string `json:"chassis"`
}

// Add implements the chassis:add command
type Add struct {
	action.WithLogger
	action.WithTerm

	Dir     string
	Chassis string
	Force   bool

	result *AddResult
}

// Result returns the structured result for JSON output.
func (a *Add) Result() any {
	return a.result
}

// Execute runs the add action
func (a *Add) Execute() error {
	c, err := chassis.Load(a.Dir)
	if err != nil {
		return err
	}

	if a.Force && c.Exists(a.Chassis) {
		a.result = &AddResult{Chassis: a.Chassis}
		a.Term().Info().Printfln("Already exists: %s", a.Chassis)
		return nil
	}

	if err := c.Add(a.Chassis); err != nil {
		return fmt.Errorf("failed to add chassis path: %w", err)
	}

	if err := c.Save(a.Dir); err != nil {
		return err
	}

	a.result = &AddResult{Chassis: a.Chassis}
	a.Term().Success().Printfln("Added: %s", a.Chassis)
	return nil
}
