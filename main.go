package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

func jsonUnmarshal(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func jsonMarshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return jsonUnmarshal(data, v)
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if path == "-" {
		_, err = os.Stdout.Write(data)
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// PersonSummary reports one person's load.
type PersonSummary struct {
	PersonID string   `json:"person_id"`
	Minutes  int      `json:"minutes"`
	Shifts   []string `json:"shifts"`
}

// Solution is the output document for generate.
type Solution struct {
	Assignment Assignment      `json:"assignment"`
	Summary    []PersonSummary `json:"summary"`
}

func buildSolution(p *Problem, a Assignment) *Solution {
	totals := personTotals(p, a)
	byPerson := map[string][]string{}
	for _, shift := range sortedShifts(p.Shifts) {
		pid := a[shift.ID]
		byPerson[pid] = append(byPerson[pid], shift.ID)
	}
	sol := &Solution{Assignment: a}
	for _, person := range p.People {
		sol.Summary = append(sol.Summary, PersonSummary{
			PersonID: person.ID,
			Minutes:  totals[person.ID],
			Shifts:   byPerson[person.ID],
		})
	}
	return sol
}

func loadProblem(path string) (*Problem, error) {
	var p Problem
	if err := readJSON(path, &p); err != nil {
		return nil, fmt.Errorf("reading problem: %w", err)
	}
	if err := validate(&p); err != nil {
		return nil, fmt.Errorf("invalid problem: %w", err)
	}
	return &p, nil
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage:
  roster generate <problem.json> <out.json|->
  roster verify <problem.json> <assignment.json>
  roster swap <problem.json> <assignment.json> <shiftA> <shiftB> [out.json|-]`)
	os.Exit(2)
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	switch os.Args[1] {
	case "generate":
		err = cmdGenerate(os.Args[2:])
	case "verify":
		err = cmdVerify(os.Args[2:])
	case "swap":
		err = cmdSwap(os.Args[2:])
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func cmdGenerate(args []string) error {
	if len(args) != 2 {
		usage()
	}
	p, err := loadProblem(args[0])
	if err != nil {
		return err
	}
	a, ok := solve(p)
	if !ok {
		return fmt.Errorf("no feasible assignment exists")
	}
	return writeJSON(args[1], buildSolution(p, a))
}

func cmdVerify(args []string) error {
	if len(args) != 2 {
		usage()
	}
	p, err := loadProblem(args[0])
	if err != nil {
		return err
	}
	var a Assignment
	if err := readJSON(args[1], &a); err != nil {
		return fmt.Errorf("reading assignment: %w", err)
	}
	if err := verifyAssignment(p, a, true); err != nil {
		return fmt.Errorf("assignment invalid: %w", err)
	}
	fmt.Println("assignment valid")
	return nil
}

func cmdSwap(args []string) error {
	if len(args) != 4 && len(args) != 5 {
		usage()
	}
	p, err := loadProblem(args[0])
	if err != nil {
		return err
	}
	var a Assignment
	if err := readJSON(args[1], &a); err != nil {
		return fmt.Errorf("reading assignment: %w", err)
	}
	res, err := swapShifts(p, a, args[2], args[3])
	if err != nil {
		return err
	}
	out := "-"
	if len(args) == 5 {
		out = args[4]
	}
	return writeJSON(out, res)
}
