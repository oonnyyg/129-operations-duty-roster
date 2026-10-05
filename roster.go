package main

import (
	"fmt"
	"sort"
	"time"
)

// Interval is a half-open time range [Start, End).
type Interval struct {
	Start time.Time
	End   time.Time
}

func (iv Interval) minutes() int {
	return int(iv.End.Sub(iv.Start).Minutes())
}

func overlaps(a, b Interval) bool {
	return a.Start.Before(b.End) && b.Start.Before(a.End)
}

// Person describes one team member.
type Person struct {
	ID            string     `json:"id"`
	Qualification string     `json:"qualification"`
	Unavailable   []Interval `json:"unavailable"`
	MaxMinutes    int        `json:"max_minutes"`
}

// Shift needs exactly one person holding Qualification.
type Shift struct {
	ID            string   `json:"id"`
	Interval      Interval `json:"interval"`
	Qualification string   `json:"qualification"`
}

// Config holds global limits.
type Config struct {
	MinRestMinutes int `json:"min_rest_minutes"`
}

// Problem is the JSON input document.
type Problem struct {
	People []Person `json:"people"`
	Shifts []Shift  `json:"shifts"`
	Config Config   `json:"config"`
	// Fixed maps shift ID to a pre-assigned person ID.
	Fixed map[string]string `json:"fixed"`
}

// Assignment maps shift ID to person ID.
type Assignment map[string]string

func parseTime(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid time %q: %w", s, err)
	}
	if t.Location() != time.UTC {
		return time.Time{}, fmt.Errorf("time %q is not UTC", s)
	}
	if t.Second() != 0 || t.Nanosecond() != 0 {
		return time.Time{}, fmt.Errorf("time %q is not minute-precise", s)
	}
	return t, nil
}

// intervalJSON is the wire form of Interval.
type intervalJSON struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

func (iv *Interval) UnmarshalJSON(data []byte) error {
	var raw intervalJSON
	if err := jsonUnmarshal(data, &raw); err != nil {
		return err
	}
	start, err := parseTime(raw.Start)
	if err != nil {
		return err
	}
	end, err := parseTime(raw.End)
	if err != nil {
		return err
	}
	if !end.After(start) {
		return fmt.Errorf("interval end %q must be after start %q", raw.End, raw.Start)
	}
	iv.Start, iv.End = start, end
	return nil
}

func (iv Interval) MarshalJSON() ([]byte, error) {
	return jsonMarshal(intervalJSON{
		Start: iv.Start.UTC().Format(time.RFC3339),
		End:   iv.End.UTC().Format(time.RFC3339),
	})
}

// sortedShifts returns shifts ordered by start time, then ID.
func sortedShifts(shifts []Shift) []Shift {
	out := make([]Shift, len(shifts))
	copy(out, shifts)
	sort.Slice(out, func(i, j int) bool {
		if !out[i].Interval.Start.Equal(out[j].Interval.Start) {
			return out[i].Interval.Start.Before(out[j].Interval.Start)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// validate checks structural rules of the problem.
func validate(p *Problem) error {
	if len(p.People) == 0 || len(p.People) > 8 {
		return fmt.Errorf("people count must be 1..8, got %d", len(p.People))
	}
	if len(p.Shifts) > 12 {
		return fmt.Errorf("shift count must be <= 12, got %d", len(p.Shifts))
	}
	if p.Config.MinRestMinutes < 0 {
		return fmt.Errorf("min_rest_minutes must be non-negative")
	}
	seen := map[string]bool{}
	for _, person := range p.People {
		if person.ID == "" {
			return fmt.Errorf("person id must not be empty")
		}
		if seen[person.ID] {
			return fmt.Errorf("duplicate person id %q", person.ID)
		}
		seen[person.ID] = true
		if person.MaxMinutes < 0 {
			return fmt.Errorf("person %q max_minutes must be non-negative", person.ID)
		}
	}
	seen = map[string]bool{}
	for _, shift := range p.Shifts {
		if shift.ID == "" {
			return fmt.Errorf("shift id must not be empty")
		}
		if seen[shift.ID] {
			return fmt.Errorf("duplicate shift id %q", shift.ID)
		}
		seen[shift.ID] = true
	}
	people := map[string]bool{}
	for _, person := range p.People {
		people[person.ID] = true
	}
	for shiftID, personID := range p.Fixed {
		if !seen[shiftID] {
			return fmt.Errorf("fixed assignment references unknown shift %q", shiftID)
		}
		if !people[personID] {
			return fmt.Errorf("fixed assignment references unknown person %q", personID)
		}
	}
	return nil
}
