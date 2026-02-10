package add

import (
	"github.com/launchrctl/launchr/pkg/action"
	"github.com/plasmash/plasmactl-chassis/internal/chassis"
)

// Add implements the chassis:add command
type Add struct {
	action.WithLogger
	action.WithTerm

	Chassis string
}

// Execute runs the add action
func (a *Add) Execute() error {
	c, err := chassis.Load(".")
	if err != nil {
		return err
	}

	if err := c.Add(a.Chassis); err != nil {
		a.Term().Error().Printfln("Failed to add chassis path: %s", err)
		return nil
	}

	if err := c.Save("."); err != nil {
		return err
	}

	a.Term().Success().Printfln("Added: %s", a.Chassis)
	return nil
}
