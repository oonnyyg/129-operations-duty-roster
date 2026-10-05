package main

import "fmt"

// SwapResult describes a successful exchange.
type SwapResult struct {
	Assignment Assignment `json:"assignment"`
	Changes    []string   `json:"changes"`
}

// swapShifts exchanges the assignees of two shifts and re-verifies
// every constraint, including fixed assignments. On any failure the
// original assignment is returned unchanged with an error.
func swapShifts(p *Problem, current Assignment, shiftAID, shiftBID string) (*SwapResult, error) {
	if err := verifyAssignment(p, current, true); err != nil {
		return nil, fmt.Errorf("current assignment is invalid: %w", err)
	}
	shifts := shiftIndex(p)
	if _, ok := shifts[shiftAID]; !ok {
		return nil, fmt.Errorf("unknown shift %q", shiftAID)
	}
	if _, ok := shifts[shiftBID]; !ok {
		return nil, fmt.Errorf("unknown shift %q", shiftBID)
	}
	if shiftAID == shiftBID {
		return nil, fmt.Errorf("cannot swap shift %q with itself", shiftAID)
	}
	personA, ok := current[shiftAID]
	if !ok {
		return nil, fmt.Errorf("shift %q is unassigned", shiftAID)
	}
	personB, ok := current[shiftBID]
	if !ok {
		return nil, fmt.Errorf("shift %q is unassigned", shiftBID)
	}
	if personA == personB {
		return nil, fmt.Errorf("both shifts are held by %q; nothing to swap", personA)
	}
	next := Assignment{}
	for k, v := range current {
		next[k] = v
	}
	next[shiftAID] = personB
	next[shiftBID] = personA
	if err := verifyAssignment(p, next, true); err != nil {
		return nil, fmt.Errorf("swap rejected, original kept: %w", err)
	}
	return &SwapResult{
		Assignment: next,
		Changes: []string{
			fmt.Sprintf("shift %s: %s -> %s", shiftAID, personA, personB),
			fmt.Sprintf("shift %s: %s -> %s", shiftBID, personB, personA),
		},
	}, nil
}
