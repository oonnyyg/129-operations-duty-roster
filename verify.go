package main

import (
	"encoding/json"
	"fmt"
)

// checkPlan verifies a full assignment against every constraint:
// coverage, valid references, fixed assignments, qualification,
// unavailability, overlap, rest gap and per-person minute limits.
func checkPlan(p *Problem, assign map[string]string) error {
	if len(assign) != len(p.Shifts) {
		return fmt.Errorf("plan covers %d shifts, want %d", len(assign), len(p.Shifts))
	}
	taken := make([][]Interval, len(p.People))
	minutes := make([]int, len(p.People))
	for _, si := range p.sortedShifts() {
		shift := &p.Shifts[si]
		personID, ok := assign[shift.ID]
		if !ok {
			return fmt.Errorf("shift %q has no assignment", shift.ID)
		}
		pi := personIndex(p, personID)
		if pi < 0 {
			return fmt.Errorf("shift %q assigned to unknown person %q", shift.ID, personID)
		}
		if fixed, isFixed := p.Fixed[shift.ID]; isFixed && fixed != personID {
			return fmt.Errorf("shift %q is fixed to %q but assigned to %q", shift.ID, fixed, personID)
		}
		if !p.canTake(pi, si, taken[pi], minutes[pi], -1) {
			return fmt.Errorf("shift %q: person %q violates qualification, availability, rest or workload constraints", shift.ID, personID)
		}
		taken[pi] = append(taken[pi], shift.Interval)
		minutes[pi] += shift.Interval.minutes()
	}
	return nil
}

func decodePlan(data []byte) (map[string]string, error) {
	var plan planJSON
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("invalid plan JSON: %w", err)
	}
	assign := map[string]string{}
	for _, a := range plan.Assignments {
		if _, dup := assign[a.Shift]; dup {
			return nil, fmt.Errorf("plan assigns shift %q more than once", a.Shift)
		}
		assign[a.Shift] = a.Person
	}
	return assign, nil
}
