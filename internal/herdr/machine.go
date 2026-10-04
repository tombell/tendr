package herdr

import (
	"context"
	"encoding/json"
	"fmt"
)

type Machine struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Target  string `json:"target"`
	Enabled bool   `json:"enabled"`
}

func (c Client) ListMachines(ctx context.Context) ([]Machine, error) {
	output, err := c.run(ctx, "", "machine", "list", "--json")
	if err != nil {
		return nil, fmt.Errorf("list Herdr machines: %w", err)
	}
	var machines []Machine
	if err := json.Unmarshal(output, &machines); err != nil {
		return nil, fmt.Errorf("parse Herdr machines: %w", err)
	}
	return machines, nil
}

func (c Client) ResolveMachine(ctx context.Context, selector string) (Machine, error) {
	machines, err := c.ListMachines(ctx)
	if err != nil {
		return Machine{}, err
	}
	// Herdr gives exact profile IDs priority over case-sensitive labels.
	for _, machine := range machines {
		if machine.ID == selector {
			return enabledMachine(machine, selector)
		}
	}
	var matched *Machine
	for index := range machines {
		if machines[index].Label != selector {
			continue
		}
		if matched != nil {
			return Machine{}, fmt.Errorf("machine label %q is ambiguous; use its profile ID", selector)
		}
		matched = &machines[index]
	}
	if matched == nil {
		return Machine{}, fmt.Errorf("unknown machine %q; use `herdr machine list`", selector)
	}
	return enabledMachine(*matched, selector)
}

func enabledMachine(machine Machine, selector string) (Machine, error) {
	if !machine.Enabled {
		return Machine{}, fmt.Errorf("machine %q is disabled", selector)
	}
	if machine.Target == "" {
		return Machine{}, fmt.Errorf("machine %q has no SSH target", selector)
	}
	return machine, nil
}
