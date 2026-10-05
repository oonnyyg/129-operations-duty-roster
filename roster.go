package main

import (
	"errors"
	"fmt"
	"sort"
	"time"
)

// Interval is a half-open UTC time range [Start, End).
type Interval struct {
	Start time.Time
	End   time.Time
}

func (iv Interval) minutes() int {
	return int(iv.End.Sub(iv.Start) / time.Minute)
}

func overlaps(a, b Interval) bool {
	return a.Start.Before(b.End) && b.Start.Before(a.End)
}

type Person struct {
	ID             string
	Qualifications map[string]bool
	Unavailable    []Interval
	MaxMinutes     int
}

type Shift struct {
	ID            string
	Interval      Interval
	Qualification string
}

type Problem struct {
	MinRestMinutes int
	People         []Person
	Shifts         []Shift
	Fixed          map[string]string // shift ID -> person ID
}

func validateProblem(p *Problem) error {
	if p.MinRestMinutes < 0 {
		return errors.New("minRestMinutes must be a non-negative integer")
	}
	seen := map[string]bool{}
	for i := range p.People {
		pp := &p.People[i]
		if pp.ID == "" {
			return errors.New("person id must not be empty")
		}
		if seen[pp.ID] {
			return fmt.Errorf("duplicate person id %q", pp.ID)
		}
		seen[pp.ID] = true
		if pp.MaxMinutes < 0 {
			return fmt.Errorf("person %q: maxMinutes must be a non-negative integer", pp.ID)
		}
		for _, iv := range pp.Unavailable {
			if !iv.End.After(iv.Start) {
				return fmt.Errorf("person %q: unavailable interval end must be after start", pp.ID)
			}
		}
	}
	if len(p.People) > 8 {
		return fmt.Errorf("at most 8 people allowed, got %d", len(p.People))
	}
	if len(p.Shifts) > 12 {
		return fmt.Errorf("at most 12 shifts allowed, got %d", len(p.Shifts))
	}
	seen = map[string]bool{}
	for _, s := range p.Shifts {
		if s.ID == "" {
			return errors.New("shift id must not be empty")
		}
		if seen[s.ID] {
			return fmt.Errorf("duplicate shift id %q", s.ID)
		}
		seen[s.ID] = true
		if s.Qualification == "" {
			return fmt.Errorf("shift %q: qualification must not be empty", s.ID)
		}
		if !s.Interval.End.After(s.Interval.Start) {
			return fmt.Errorf("shift %q: end must be after start", s.ID)
		}
	}
	for shiftID, personID := range p.Fixed {
		if !seen[shiftID] {
			return fmt.Errorf("fixed assignment references unknown shift %q", shiftID)
		}
		if p.personByID(personID) == nil {
			return fmt.Errorf("fixed assignment references unknown person %q", personID)
		}
	}
	return nil
}

func (p *Problem) personByID(id string) *Person {
	for i := range p.People {
		if p.People[i].ID == id {
			return &p.People[i]
		}
	}
	return nil
}

func (p *Problem) shiftByID(id string) *Shift {
	for i := range p.Shifts {
		if p.Shifts[i].ID == id {
			return &p.Shifts[i]
		}
	}
	return nil
}

// sortedShifts returns shift indexes ordered by start time, then by ID.
func (p *Problem) sortedShifts() []int {
	idx := make([]int, len(p.Shifts))
	for i := range idx {
		idx[i] = i
	}
	sort.Slice(idx, func(a, b int) bool {
		sa, sb := p.Shifts[idx[a]], p.Shifts[idx[b]]
		if !sa.Interval.Start.Equal(sb.Interval.Start) {
			return sa.Interval.Start.Before(sb.Interval.Start)
		}
		return sa.ID < sb.ID
	})
	return idx
}

// canTake reports whether person may cover shift given their own
// qualification, unavailability, rest and workload limits. assigned maps
// shift index -> person index for already-decided shifts of this person.
func (p *Problem) canTake(personIdx, shiftIdx int, taken []Interval, usedMinutes, capMinutes int) bool {
	person := &p.People[personIdx]
	shift := &p.Shifts[shiftIdx]
	if !person.Qualifications[shift.Qualification] {
		return false
	}
	for _, iv := range person.Unavailable {
		if overlaps(iv, shift.Interval) {
			return false
		}
	}
	rest := time.Duration(p.MinRestMinutes) * time.Minute
	for _, t := range taken {
		if overlaps(t, shift.Interval) {
			return false
		}
		gap := shift.Interval.Start.Sub(t.End)
		if gap < 0 {
			gap = t.Start.Sub(shift.Interval.End)
		}
		if gap < rest {
			return false
		}
	}
	limit := person.MaxMinutes
	if capMinutes >= 0 && capMinutes < limit {
		limit = capMinutes
	}
	return usedMinutes+shift.Interval.minutes() <= limit
}
