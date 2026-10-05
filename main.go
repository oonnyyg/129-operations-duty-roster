package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
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
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage:
  roster generate -input problem.json -output plan.json
  roster verify   -input problem.json -plan plan.json
  roster swap     -input problem.json -plan plan.json -shift-a S1 -shift-b S2 -output newplan.json`)
}

func readFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return data, nil
}

func loadProblem(path string) (*Problem, error) {
	data, err := readFile(path)
	if err != nil {
		return nil, err
	}
	return decodeProblem(data)
}

func cmdGenerate(args []string) error {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	input := fs.String("input", "", "problem JSON file")
	output := fs.String("output", "", "output plan JSON file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *input == "" || *output == "" {
		return fmt.Errorf("both -input and -output are required")
	}
	p, err := loadProblem(*input)
	if err != nil {
		return err
	}
	assign, ok := solve(p)
	if !ok {
		return fmt.Errorf("no feasible roster exists for the given constraints")
	}
	plan := buildPlan(p, assign)
	data, err := encodePlan(plan)
	if err != nil {
		return err
	}
	if err := os.WriteFile(*output, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", *output, err)
	}
	fmt.Printf("wrote feasible roster to %s\n", *output)
	return nil
}

func cmdVerify(args []string) error {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	input := fs.String("input", "", "problem JSON file")
	planPath := fs.String("plan", "", "plan JSON file to check")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *input == "" || *planPath == "" {
		return fmt.Errorf("both -input and -plan are required")
	}
	p, err := loadProblem(*input)
	if err != nil {
		return err
	}
	data, err := readFile(*planPath)
	if err != nil {
		return err
	}
	assign, err := decodePlan(data)
	if err != nil {
		return err
	}
	if err := checkPlan(p, assign); err != nil {
		return fmt.Errorf("plan is not compliant: %w", err)
	}
	fmt.Println("plan is compliant")
	return nil
}

func cmdSwap(args []string) error {
	fs := flag.NewFlagSet("swap", flag.ContinueOnError)
	input := fs.String("input", "", "problem JSON file")
	planPath := fs.String("plan", "", "current plan JSON file")
	shiftA := fs.String("shift-a", "", "first shift ID")
	shiftB := fs.String("shift-b", "", "second shift ID")
	output := fs.String("output", "", "output plan JSON file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *input == "" || *planPath == "" || *output == "" || *shiftA == "" || *shiftB == "" {
		return fmt.Errorf("-input, -plan, -output, -shift-a and -shift-b are all required")
	}
	if *shiftA == *shiftB {
		return fmt.Errorf("shift-a and shift-b must differ")
	}
	p, err := loadProblem(*input)
	if err != nil {
		return err
	}
	data, err := readFile(*planPath)
	if err != nil {
		return err
	}
	assign, err := decodePlan(data)
	if err != nil {
		return err
	}
	if err := checkPlan(p, assign); err != nil {
		return fmt.Errorf("current plan is not compliant, refusing to swap: %w", err)
	}
	personA, ok := assign[*shiftA]
	if !ok {
		return fmt.Errorf("shift %q is not assigned in the plan", *shiftA)
	}
	personB, ok := assign[*shiftB]
	if !ok {
		return fmt.Errorf("shift %q is not assigned in the plan", *shiftB)
	}
	if personA == personB {
		return fmt.Errorf("shifts %q and %q are both assigned to %q; nothing to swap", *shiftA, *shiftB, personA)
	}
	assign[*shiftA], assign[*shiftB] = personB, personA
	if err := checkPlan(p, assign); err != nil {
		return fmt.Errorf("swap rejected, original plan unchanged: %w", err)
	}
	assignIdx := make([]int, len(p.Shifts))
	for i, s := range p.Shifts {
		assignIdx[i] = personIndex(p, assign[s.ID])
	}
	plan := buildPlan(p, assignIdx)
	out, err := encodePlan(plan)
	if err != nil {
		return err
	}
	if err := os.WriteFile(*output, out, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", *output, err)
	}
	fmt.Printf("swap accepted: shift %s %s -> %s, shift %s %s -> %s\n",
		*shiftA, personA, personB, *shiftB, personB, personA)
	fmt.Printf("wrote updated roster to %s\n", *output)
	return nil
}
