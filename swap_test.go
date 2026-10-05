package main

import "testing"

func swapProblem() *Problem {
	return &Problem{
		People: []Person{
			{ID: "A", Qualification: "medic", MaxMinutes: 600},
			{ID: "B", Qualification: "driver", MaxMinutes: 600},
		},
		Shifts: []Shift{
			{ID: "s1", Interval: iv(0, 0, 2, 0), Qualification: "medic"},
			{ID: "s2", Interval: iv(3, 0, 5, 0), Qualification: "driver"},
		},
	}
}

// TestSwapQualificationMismatch ensures a swap that would put a person
// on a shift they are not qualified for is rejected and the original
// assignment is untouched.
func TestSwapQualificationMismatch(t *testing.T) {
	p := swapProblem()
	orig := Assignment{"s1": "A", "s2": "B"}
	res, err := swapShifts(p, orig, "s1", "s2")
	if err == nil {
		t.Fatal("expected swap to be rejected")
	}
	if res != nil {
		t.Fatal("expected no result on failed swap")
	}
	if orig["s1"] != "A" || orig["s2"] != "B" {
		t.Fatalf("original assignment mutated: %v", orig)
	}
}

// TestSwapSuccess exchanges two qualified people.
func TestSwapSuccess(t *testing.T) {
	p := &Problem{
		People: []Person{
			{ID: "A", Qualification: "q", MaxMinutes: 600},
			{ID: "B", Qualification: "q", MaxMinutes: 600},
		},
		Shifts: []Shift{
			{ID: "s1", Interval: iv(0, 0, 2, 0), Qualification: "q"},
			{ID: "s2", Interval: iv(3, 0, 5, 0), Qualification: "q"},
		},
	}
	res, err := swapShifts(p, Assignment{"s1": "A", "s2": "B"}, "s1", "s2")
	if err != nil {
		t.Fatalf("unexpected rejection: %v", err)
	}
	if res.Assignment["s1"] != "B" || res.Assignment["s2"] != "A" {
		t.Fatalf("unexpected result: %v", res.Assignment)
	}
	if len(res.Changes) != 2 {
		t.Fatalf("expected 2 change notes, got %v", res.Changes)
	}
}

// TestSwapViolatesFixed ensures fixed assignments block swaps.
func TestSwapViolatesFixed(t *testing.T) {
	p := &Problem{
		People: []Person{
			{ID: "A", Qualification: "q", MaxMinutes: 600},
			{ID: "B", Qualification: "q", MaxMinutes: 600},
		},
		Shifts: []Shift{
			{ID: "s1", Interval: iv(0, 0, 2, 0), Qualification: "q"},
			{ID: "s2", Interval: iv(3, 0, 5, 0), Qualification: "q"},
		},
		Fixed: map[string]string{"s1": "A"},
	}
	_, err := swapShifts(p, Assignment{"s1": "A", "s2": "B"}, "s1", "s2")
	if err == nil {
		t.Fatal("expected swap violating fixed assignment to be rejected")
	}
}

// TestSwapRestViolation ensures the full rest rule is re-checked, not
// just overlap of the two swapped shifts.
func TestSwapRestViolation(t *testing.T) {
	p := &Problem{
		People: []Person{
			{ID: "A", Qualification: "q", MaxMinutes: 600},
			{ID: "B", Qualification: "q", MaxMinutes: 600},
		},
		Shifts: []Shift{
			{ID: "s1", Interval: iv(0, 0, 1, 0), Qualification: "q"},
			{ID: "s2", Interval: iv(2, 0, 3, 0), Qualification: "q"},
			{ID: "s3", Interval: iv(3, 30, 4, 0), Qualification: "q"},
		},
		Config: Config{MinRestMinutes: 90},
	}
	// A holds s1,s3 (rest 150 min ok); B holds s2. Swapping s2/s3 gives
	// B s3 and A s1,s2 -> rest 60 min < 90.
	orig := Assignment{"s1": "A", "s2": "B", "s3": "A"}
	if err := verifyAssignment(p, orig, true); err != nil {
		t.Fatalf("setup assignment invalid: %v", err)
	}
	if _, err := swapShifts(p, orig, "s2", "s3"); err == nil {
		t.Fatal("expected rest violation to reject swap")
	}
}

// TestSolveFixedPreserved ensures generation keeps fixed assignments.
func TestSolveFixedPreserved(t *testing.T) {
	p := &Problem{
		People: []Person{
			{ID: "A", Qualification: "q", MaxMinutes: 600},
			{ID: "B", Qualification: "q", MaxMinutes: 600},
		},
		Shifts: []Shift{
			{ID: "s1", Interval: iv(0, 0, 1, 0), Qualification: "q"},
			{ID: "s2", Interval: iv(2, 0, 3, 0), Qualification: "q"},
		},
		Fixed: map[string]string{"s2": "B"},
	}
	a, ok := solve(p)
	if !ok {
		t.Fatal("expected feasible")
	}
	if a["s2"] != "B" {
		t.Fatalf("fixed assignment changed: %v", a)
	}
}

// TestSolveLexTieBreak picks the lexicographically smallest person-ID
// sequence among equal max-load solutions.
func TestSolveLexTieBreak(t *testing.T) {
	p := &Problem{
		People: []Person{
			{ID: "B", Qualification: "q", MaxMinutes: 600},
			{ID: "A", Qualification: "q", MaxMinutes: 600},
		},
		Shifts: []Shift{
			{ID: "s1", Interval: iv(0, 0, 1, 0), Qualification: "q"},
			{ID: "s2", Interval: iv(0, 0, 1, 0), Qualification: "q"},
		},
	}
	a, ok := solve(p)
	if !ok {
		t.Fatal("expected feasible")
	}
	// Both people get 60 minutes either way; (A,B) beats (B,A).
	if a["s1"] != "A" || a["s2"] != "B" {
		t.Fatalf("expected lexicographically smallest sequence, got %v", a)
	}
}
