package main

import (
	"testing"
	"time"
)

func at(hour, minute int) time.Time {
	return time.Date(2026, 1, 1, hour, minute, 0, 0, time.UTC)
}

func iv(sh, sm, eh, em int) Interval {
	return Interval{Start: at(sh, sm), End: at(eh, em)}
}

// TestSolveRequiresBacktracking builds a case where assigning the first
// shift to the first feasible person (A) dead-ends: s2 can then take
// nobody. The solver must backtrack and give s1 to B instead.
func TestSolveRequiresBacktracking(t *testing.T) {
	p := &Problem{
		People: []Person{
			{ID: "A", Qualification: "q", MaxMinutes: 120},
			{ID: "B", Qualification: "q", MaxMinutes: 120,
				Unavailable: []Interval{iv(1, 0, 1, 30)}},
		},
		Shifts: []Shift{
			{ID: "s1", Interval: iv(0, 0, 1, 0), Qualification: "q"},
			{ID: "s2", Interval: iv(0, 30, 1, 30), Qualification: "q"},
			{ID: "s3", Interval: iv(1, 30, 2, 30), Qualification: "q"},
		},
	}
	a, ok := solve(p)
	if !ok {
		t.Fatal("expected feasible solution")
	}
	want := Assignment{"s1": "B", "s2": "A", "s3": "A"}
	for k, v := range want {
		if a[k] != v {
			t.Fatalf("shift %s: got %q, want %q (full: %v)", k, a[k], v, a)
		}
	}
	if err := verifyAssignment(p, a, true); err != nil {
		t.Fatalf("solver output failed verification: %v", err)
	}
}

// TestSolveInfeasible expects a clear infeasibility result.
func TestSolveInfeasible(t *testing.T) {
	p := &Problem{
		People: []Person{
			{ID: "A", Qualification: "q", MaxMinutes: 30},
		},
		Shifts: []Shift{
			{ID: "s1", Interval: iv(0, 0, 1, 0), Qualification: "q"},
		},
	}
	if _, ok := solve(p); ok {
		t.Fatal("expected infeasible, got a solution")
	}
}

// TestRestBoundary checks that a gap equal to min_rest is allowed and
// one minute less is rejected.
func TestRestBoundary(t *testing.T) {
	mk := func(gapStart int) *Problem {
		return &Problem{
			People: []Person{
				{ID: "A", Qualification: "q", MaxMinutes: 240},
			},
			Shifts: []Shift{
				{ID: "s1", Interval: iv(0, 0, 1, 0), Qualification: "q"},
				{ID: "s2", Interval: iv(0, gapStart, 3, 0), Qualification: "q"},
			},
			Config: Config{MinRestMinutes: 60},
		}
	}
	okCase := mk(120) // s1 ends 01:00, s2 starts 02:00 -> exactly 60 min rest
	a, ok := solve(okCase)
	if !ok {
		t.Fatal("exact minimum rest should be feasible")
	}
	if a["s1"] != "A" || a["s2"] != "A" {
		t.Fatalf("expected both shifts on A, got %v", a)
	}
	badCase := mk(119) // 59 min rest -> infeasible with one person
	if _, ok := solve(badCase); ok {
		t.Fatal("rest below minimum should be infeasible")
	}
	err := verifyAssignment(okCase, Assignment{"s1": "A", "s2": "A"}, false)
	if err != nil {
		t.Fatalf("verify rejected boundary-valid assignment: %v", err)
	}
	err = verifyAssignment(badCase, Assignment{"s1": "A", "s2": "A"}, false)
	if err == nil {
		t.Fatal("verify accepted assignment violating minimum rest")
	}
}
