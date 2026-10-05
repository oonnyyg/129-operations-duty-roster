package main

import (
	"encoding/json"
	"fmt"
	"time"
)

const timeLayout = "2006-01-02T15:04Z07:00"

type intervalJSON struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type personJSON struct {
	ID             string         `json:"id"`
	Qualifications []string       `json:"qualifications"`
	Unavailable    []intervalJSON `json:"unavailable"`
	MaxMinutes     int            `json:"maxMinutes"`
}

type shiftJSON struct {
	ID            string `json:"id"`
	Start         string `json:"start"`
	End           string `json:"end"`
	Qualification string `json:"qualification"`
}

type fixedJSON struct {
	Shift  string `json:"shift"`
	Person string `json:"person"`
}

type inputJSON struct {
	MinRestMinutes int          `json:"minRestMinutes"`
	People         []personJSON `json:"people"`
	Shifts         []shiftJSON  `json:"shifts"`
	Fixed          []fixedJSON  `json:"fixed"`
}

func parseTime(field, value string) (time.Time, error) {
	t, err := time.Parse(timeLayout, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s: invalid time %q (want UTC minute precision, e.g. 2026-10-03T08:00Z)", field, value)
	}
	return t.UTC(), nil
}

func parseInterval(field string, iv intervalJSON) (Interval, error) {
	start, err := parseTime(field+".start", iv.Start)
	if err != nil {
		return Interval{}, err
	}
	end, err := parseTime(field+".end", iv.End)
	if err != nil {
		return Interval{}, err
	}
	return Interval{Start: start, End: end}, nil
}

func decodeProblem(data []byte) (*Problem, error) {
	var in inputJSON
	if err := json.Unmarshal(data, &in); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	p := &Problem{MinRestMinutes: in.MinRestMinutes, Fixed: map[string]string{}}
	for _, pj := range in.People {
		person := Person{
			ID:             pj.ID,
			Qualifications: map[string]bool{},
			MaxMinutes:     pj.MaxMinutes,
		}
		for _, q := range pj.Qualifications {
			person.Qualifications[q] = true
		}
		for _, uj := range pj.Unavailable {
			iv, err := parseInterval("person "+pj.ID+" unavailable", uj)
			if err != nil {
				return nil, err
			}
			person.Unavailable = append(person.Unavailable, iv)
		}
		p.People = append(p.People, person)
	}
	for _, sj := range in.Shifts {
		iv, err := parseInterval("shift "+sj.ID, intervalJSON{Start: sj.Start, End: sj.End})
		if err != nil {
			return nil, err
		}
		p.Shifts = append(p.Shifts, Shift{ID: sj.ID, Interval: iv, Qualification: sj.Qualification})
	}
	for _, fj := range in.Fixed {
		if _, dup := p.Fixed[fj.Shift]; dup {
			return nil, fmt.Errorf("duplicate fixed assignment for shift %q", fj.Shift)
		}
		p.Fixed[fj.Shift] = fj.Person
	}
	if err := validateProblem(p); err != nil {
		return nil, err
	}
	return p, nil
}

type assignmentJSON struct {
	Shift  string `json:"shift"`
	Person string `json:"person"`
}

type personSummaryJSON struct {
	Person       string   `json:"person"`
	TotalMinutes int      `json:"totalMinutes"`
	Shifts       []string `json:"shifts"`
}

type planJSON struct {
	Assignments []assignmentJSON    `json:"assignments"`
	Summary     []personSummaryJSON `json:"summary"`
}

// buildPlan renders an assignment as a plan with per-person summary.
// Shifts are listed in sorted order (start, then ID).
func buildPlan(p *Problem, assign []int) planJSON {
	order := p.sortedShifts()
	plan := planJSON{
		Assignments: []assignmentJSON{},
		Summary:     []personSummaryJSON{},
	}
	minutes := make([]int, len(p.People))
	shifts := make([][]string, len(p.People))
	for _, si := range order {
		pi := assign[si]
		plan.Assignments = append(plan.Assignments, assignmentJSON{
			Shift:  p.Shifts[si].ID,
			Person: p.People[pi].ID,
		})
		minutes[pi] += p.Shifts[si].Interval.minutes()
		shifts[pi] = append(shifts[pi], p.Shifts[si].ID)
	}
	for i, person := range p.People {
		if shifts[i] == nil {
			shifts[i] = []string{}
		}
		plan.Summary = append(plan.Summary, personSummaryJSON{
			Person:       person.ID,
			TotalMinutes: minutes[i],
			Shifts:       shifts[i],
		})
	}
	return plan
}

func encodePlan(plan planJSON) ([]byte, error) {
	return json.MarshalIndent(plan, "", "  ")
}
