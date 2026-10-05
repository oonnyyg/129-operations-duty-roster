package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func mustTime(t *testing.T, v string) time.Time {
	t.Helper()
	tm, err := time.Parse(timeLayout, v)
	if err != nil {
		t.Fatalf("bad test time %q: %v", v, err)
	}
	return tm.UTC()
}

func shift(id, start, end, qual string) Shift {
	st, _ := time.Parse(timeLayout, start)
	en, _ := time.Parse(timeLayout, end)
	return Shift{ID: id, Interval: Interval{Start: st.UTC(), End: en.UTC()}, Qualification: qual}
}

func person(id string, maxMinutes int, quals ...string) Person {
	p := Person{ID: id, Qualifications: map[string]bool{}, MaxMinutes: maxMinutes}
	for _, q := range quals {
		p.Qualifications[q] = true
	}
	return p
}

func assignmentMap(p *Problem, assign []int) map[string]string {
	m := map[string]string{}
	for si, pi := range assign {
		m[p.Shifts[si].ID] = p.People[pi].ID
	}
	return m
}

// Greedy first-fit in person-ID order fails here: giving s1 to "a" blocks
// s2, so the solver must backtrack and hand s1 to "b".
func TestSolveRequiresBacktracking(t *testing.T) {
	p := &Problem{
		People: []Person{person("a", 600, "x", "y"), person("b", 600, "x")},
		Shifts: []Shift{
			shift("s1", "2026-10-03T08:00Z", "2026-10-03T09:00Z", "x"),
			shift("s2", "2026-10-03T08:30Z", "2026-10-03T09:30Z", "y"),
		},
		Fixed: map[string]string{},
	}
	if err := validateProblem(p); err != nil {
		t.Fatal(err)
	}
	assign, ok := solve(p)
	if !ok {
		t.Fatal("expected a feasible roster")
	}
	got := assignmentMap(p, assign)
	if got["s1"] != "b" || got["s2"] != "a" {
		t.Fatalf("unexpected assignment: %v", got)
	}
	if err := checkPlan(p, got); err != nil {
		t.Fatalf("solution failed verification: %v", err)
	}
}

// The rest gap is inclusive: exactly minRestMinutes between shifts is fine,
// one minute less is not.
func TestRestBoundary(t *testing.T) {
	base := Problem{
		MinRestMinutes: 60,
		People:         []Person{person("a", 600, "x")},
		Fixed:          map[string]string{},
	}
	okCase := base
	okCase.Shifts = []Shift{
		shift("s1", "2026-10-03T08:00Z", "2026-10-03T09:00Z", "x"),
		shift("s2", "2026-10-03T10:00Z", "2026-10-03T11:00Z", "x"),
	}
	if _, ok := solve(&okCase); !ok {
		t.Fatal("gap of exactly minRestMinutes should be feasible")
	}
	tightCase := base
	tightCase.Shifts = []Shift{
		shift("s1", "2026-10-03T08:00Z", "2026-10-03T09:00Z", "x"),
		shift("s2", "2026-10-03T09:59Z", "2026-10-03T11:00Z", "x"),
	}
	if _, ok := solve(&tightCase); ok {
		t.Fatal("gap below minRestMinutes should be infeasible")
	}
}

// Intervals are half-open: unavailability ending at shift start, and a shift
// ending exactly when another begins, do not conflict.
func TestHalfOpenIntervals(t *testing.T) {
	avail := person("a", 600, "x")
	avail.Unavailable = []Interval{{
		Start: mustTime(t, "2026-10-03T06:00Z"),
		End:   mustTime(t, "2026-10-03T08:00Z"),
	}}
	p := &Problem{
		People: []Person{avail},
		Shifts: []Shift{
			shift("s1", "2026-10-03T08:00Z", "2026-10-03T09:00Z", "x"),
			shift("s2", "2026-10-03T09:00Z", "2026-10-03T10:00Z", "x"),
		},
		Fixed: map[string]string{},
	}
	if _, ok := solve(p); !ok {
		t.Fatal("touching intervals should not conflict")
	}
}

// Objective: minimise the maximum per-person minutes, then break ties by
// the lexicographically smallest person-ID sequence in shift order.
func TestObjectiveMinMaxThenLexicographic(t *testing.T) {
	p := &Problem{
		People: []Person{person("a", 600, "x"), person("b", 600, "x")},
		Shifts: []Shift{
			shift("s1", "2026-10-03T08:00Z", "2026-10-03T10:00Z", "x"),
			shift("s2", "2026-10-03T12:00Z", "2026-10-03T14:00Z", "x"),
		},
		Fixed: map[string]string{},
	}
	assign, ok := solve(p)
	if !ok {
		t.Fatal("expected feasible roster")
	}
	got := assignmentMap(p, assign)
	// Splitting 120/120 (max 120) beats 240/0 (max 240); "a" then "b" is
	// the lexicographically smallest split.
	if got["s1"] != "a" || got["s2"] != "b" {
		t.Fatalf("unexpected assignment: %v", got)
	}
}

func TestFixedAssignmentsRespected(t *testing.T) {
	p := &Problem{
		People: []Person{person("a", 600, "x"), person("b", 600, "x")},
		Shifts: []Shift{
			shift("s1", "2026-10-03T08:00Z", "2026-10-03T09:00Z", "x"),
			shift("s2", "2026-10-03T12:00Z", "2026-10-03T13:00Z", "x"),
		},
		Fixed: map[string]string{"s1": "b"},
	}
	assign, ok := solve(p)
	if !ok {
		t.Fatal("expected feasible roster")
	}
	got := assignmentMap(p, assign)
	if got["s1"] != "b" {
		t.Fatalf("fixed assignment changed: %v", got)
	}
	// A fixed assignment that violates constraints makes the input infeasible.
	p.Fixed = map[string]string{"s1": "b", "s2": "b"}
	p.MinRestMinutes = 600
	if _, ok := solve(p); ok {
		t.Fatal("infeasible fixed assignments must report no solution")
	}
}

func TestSwapQualificationMismatchRejected(t *testing.T) {
	p := &Problem{
		People: []Person{person("a", 600, "x"), person("b", 600, "y")},
		Shifts: []Shift{
			shift("s1", "2026-10-03T08:00Z", "2026-10-03T09:00Z", "x"),
			shift("s2", "2026-10-03T12:00Z", "2026-10-03T13:00Z", "y"),
		},
		Fixed: map[string]string{},
	}
	current := map[string]string{"s1": "a", "s2": "b"}
	if err := checkPlan(p, current); err != nil {
		t.Fatalf("baseline plan should be compliant: %v", err)
	}
	swapped := map[string]string{"s1": "b", "s2": "a"}
	if err := checkPlan(p, swapped); err == nil {
		t.Fatal("swap giving an unqualified person a shift must be rejected")
	}
}

func TestSwapFixedAssignmentRejected(t *testing.T) {
	p := &Problem{
		People: []Person{person("a", 600, "x"), person("b", 600, "x")},
		Shifts: []Shift{
			shift("s1", "2026-10-03T08:00Z", "2026-10-03T09:00Z", "x"),
			shift("s2", "2026-10-03T12:00Z", "2026-10-03T13:00Z", "x"),
		},
		Fixed: map[string]string{"s1": "a"},
	}
	swapped := map[string]string{"s1": "b", "s2": "a"}
	if err := checkPlan(p, swapped); err == nil {
		t.Fatal("swap moving a fixed assignment must be rejected")
	}
}

// End-to-end swap: a compliant swap writes a new plan; a non-compliant swap
// leaves the original plan file untouched.
func TestSwapCommandEndToEnd(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "input.json")
	planPath := filepath.Join(dir, "plan.json")
	outPath := filepath.Join(dir, "out.json")
	input := `{
	  "minRestMinutes": 60,
	  "people": [
	    {"id": "a", "qualifications": ["x"], "maxMinutes": 600},
	    {"id": "b", "qualifications": ["x"], "maxMinutes": 600}
	  ],
	  "shifts": [
	    {"id": "s1", "start": "2026-10-03T08:00Z", "end": "2026-10-03T09:00Z", "qualification": "x"},
	    {"id": "s2", "start": "2026-10-03T12:00Z", "end": "2026-10-03T13:00Z", "qualification": "x"}
	  ]
	}`
	if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cmdGenerate([]string{"-input", inputPath, "-output", planPath}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	original, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := cmdVerify([]string{"-input", inputPath, "-plan", planPath}); err != nil {
		t.Fatalf("verify: %v", err)
	}
	// Compliant swap succeeds and mentions both people.
	if err := cmdSwap([]string{"-input", inputPath, "-plan", planPath,
		"-shift-a", "s1", "-shift-b", "s2", "-output", outPath}); err != nil {
		t.Fatalf("swap: %v", err)
	}
	if err := cmdVerify([]string{"-input", inputPath, "-plan", outPath}); err != nil {
		t.Fatalf("swapped plan should verify: %v", err)
	}
	out, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "\"person\": \"b\"") {
		t.Fatalf("swapped plan missing reassignment: %s", out)
	}
	// A swap that would violate the rest constraint is rejected. Person "a"
	// also holds s3 (20:00-21:00); taking s2 (12:00-13:00) would leave only
	// 420 minutes of rest, below the 600-minute minimum.
	tight := `{
	  "minRestMinutes": 600,
	  "people": [
	    {"id": "a", "qualifications": ["x"], "maxMinutes": 600},
	    {"id": "b", "qualifications": ["x"], "maxMinutes": 600}
	  ],
	  "shifts": [
	    {"id": "s1", "start": "2026-10-03T08:00Z", "end": "2026-10-03T09:00Z", "qualification": "x"},
	    {"id": "s2", "start": "2026-10-03T12:00Z", "end": "2026-10-03T13:00Z", "qualification": "x"},
	    {"id": "s3", "start": "2026-10-03T20:00Z", "end": "2026-10-03T21:00Z", "qualification": "x"}
	  ]
	}`
	if err := os.WriteFile(inputPath, []byte(tight), 0o644); err != nil {
		t.Fatal(err)
	}
	restPlan := `{"assignments": [
	  {"shift": "s1", "person": "a"},
	  {"shift": "s2", "person": "b"},
	  {"shift": "s3", "person": "a"}
	]}`
	if err := os.WriteFile(planPath, []byte(restPlan), 0o644); err != nil {
		t.Fatal(err)
	}
	original, err = os.ReadFile(planPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := cmdSwap([]string{"-input", inputPath, "-plan", planPath,
		"-shift-a", "s1", "-shift-b", "s2", "-output", outPath}); err == nil {
		t.Fatal("swap violating rest for one person must be rejected")
	}
	after, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(original) {
		t.Fatal("rejected swap must leave the original plan unchanged")
	}
}

func TestInfeasibleProblemReported(t *testing.T) {
	p := &Problem{
		People: []Person{person("a", 30, "x")},
		Shifts: []Shift{
			shift("s1", "2026-10-03T08:00Z", "2026-10-03T09:00Z", "x"),
		},
		Fixed: map[string]string{},
	}
	if _, ok := solve(p); ok {
		t.Fatal("workload cap exceeded: expected infeasible")
	}
}
