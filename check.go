package main

import (
	"fmt"
	"sort"
)

// personIndex maps person ID to Person.
func personIndex(p *Problem) map[string]*Person {
	idx := make(map[string]*Person, len(p.People))
	for i := range p.People {
		idx[p.People[i].ID] = &p.People[i]
	}
	return idx
}

// shiftIndex maps shift ID to Shift.
func shiftIndex(p *Problem) map[string]*Shift {
	idx := make(map[string]*Shift, len(p.Shifts))
	for i := range p.Shifts {
		idx[p.Shifts[i].ID] = &p.Shifts[i]
	}
	return idx
}

// canCover reports whether person may take shift in isolation:
// qualification match and no overlap with unavailable intervals.
func canCover(person *Person, shift *Shift) bool {
	if person.Qualification != shift.Qualification {
		return false
	}
	for _, un := range person.Unavailable {
		if overlaps(un, shift.Interval) {
			return false
		}
	}
	return true
}

// checkPersonShifts validates one person's ordered shift list for
// overlap, minimum rest, and total duration limit.
func checkPersonShifts(person *Person, shifts []*Shift, minRest int) error {
	sort.Slice(shifts, func(i, j int) bool {
		if !shifts[i].Interval.Start.Equal(shifts[j].Interval.Start) {
			return shifts[i].Interval.Start.Before(shifts[j].Interval.Start)
		}
		return shifts[i].ID < shifts[j].ID
	})
	total := 0
	for i, shift := range shifts {
		total += shift.Interval.minutes()
		if i > 0 {
			prev := shifts[i-1]
			if prev.Interval.End.After(shift.Interval.Start) {
				return fmt.Errorf("person %q has overlapping shifts %q and %q", person.ID, prev.ID, shift.ID)
			}
			gap := int(shift.Interval.Start.Sub(prev.Interval.End).Minutes())
			if gap < minRest {
				return fmt.Errorf("person %q rest between %q and %q is %d min, below minimum %d",
					person.ID, prev.ID, shift.ID, gap, minRest)
			}
		}
	}
	if total > person.MaxMinutes {
		return fmt.Errorf("person %q total %d min exceeds limit %d", person.ID, total, person.MaxMinutes)
	}
	return nil
}

// verifyAssignment checks a complete assignment against every rule,
// including that fixed assignments are preserved when checkFixed is true.
func verifyAssignment(p *Problem, a Assignment, checkFixed bool) error {
	people := personIndex(p)
	shifts := shiftIndex(p)
	if len(a) != len(p.Shifts) {
		return fmt.Errorf("assignment covers %d shifts, expected %d", len(a), len(p.Shifts))
	}
	byPerson := map[string][]*Shift{}
	for shiftID, personID := range a {
		shift, ok := shifts[shiftID]
		if !ok {
			return fmt.Errorf("assignment references unknown shift %q", shiftID)
		}
		person, ok := people[personID]
		if !ok {
			return fmt.Errorf("assignment references unknown person %q", personID)
		}
		if !canCover(person, shift) {
			return fmt.Errorf("person %q cannot cover shift %q (qualification or availability)", personID, shiftID)
		}
		if checkFixed {
			if fixed, ok := p.Fixed[shiftID]; ok && fixed != personID {
				return fmt.Errorf("shift %q is fixed to person %q, got %q", shiftID, fixed, personID)
			}
		}
		byPerson[personID] = append(byPerson[personID], shift)
	}
	for _, person := range p.People {
		if err := checkPersonShifts(people[person.ID], byPerson[person.ID], p.Config.MinRestMinutes); err != nil {
			return err
		}
	}
	return nil
}

// personTotals computes total minutes per person ID.
func personTotals(p *Problem, a Assignment) map[string]int {
	shifts := shiftIndex(p)
	totals := map[string]int{}
	for shiftID, personID := range a {
		totals[personID] += shifts[shiftID].Interval.minutes()
	}
	return totals
}
