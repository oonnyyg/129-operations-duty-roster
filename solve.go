package main

// solver performs exhaustive backtracking with branch and bound.
// Objective: minimize the maximum per-person total minutes; ties are
// broken by the lexicographically smallest person-ID sequence over
// shifts ordered by (start, ID).
type solver struct {
	problem *Problem
	order   []*Shift // shifts sorted by (start, ID)
	people  []*Person
	byID    map[string]*Person

	assign []string   // assign[i] = person ID for order[i], "" if unset
	load   []int      // per-person index: minutes so far
	shifts [][]*Shift // per-person index: assigned shifts

	best    []string
	bestMax int
	found   bool
}

func newSolver(p *Problem) *solver {
	sorted := sortedShifts(p.Shifts)
	order := make([]*Shift, len(sorted))
	for i := range sorted {
		order[i] = &sorted[i]
	}
	people := make([]*Person, len(p.People))
	byID := map[string]*Person{}
	for i := range p.People {
		people[i] = &p.People[i]
		byID[p.People[i].ID] = &p.People[i]
	}
	s := &solver{
		problem: p,
		order:   order,
		people:  people,
		byID:    byID,
		assign:  make([]string, len(order)),
		load:    make([]int, len(people)),
		shifts:  make([][]*Shift, len(people)),
	}
	// Apply fixed assignments up front; infeasible fixed entries make
	// the whole problem infeasible.
	for i, shift := range order {
		if fixed, ok := p.Fixed[shift.ID]; ok {
			s.assign[i] = fixed
		}
	}
	return s
}

// personIdx returns the index of a person ID.
func (s *solver) personIdx(id string) int {
	for i, p := range s.people {
		if p.ID == id {
			return i
		}
	}
	return -1
}

// canPlace checks incremental constraints for putting shift on person.
func (s *solver) canPlace(pi int, shift *Shift) bool {
	person := s.people[pi]
	if !canCover(person, shift) {
		return false
	}
	if s.load[pi]+shift.Interval.minutes() > person.MaxMinutes {
		return false
	}
	for _, other := range s.shifts[pi] {
		if overlaps(other.Interval, shift.Interval) {
			return false
		}
		// Rest constraint in both directions.
		if !other.Interval.End.After(shift.Interval.Start) {
			if int(shift.Interval.Start.Sub(other.Interval.End).Minutes()) < s.problem.Config.MinRestMinutes {
				return false
			}
		}
		if !shift.Interval.End.After(other.Interval.Start) {
			if int(other.Interval.Start.Sub(shift.Interval.End).Minutes()) < s.problem.Config.MinRestMinutes {
				return false
			}
		}
	}
	return true
}

func (s *solver) place(pi int, shift *Shift) {
	s.load[pi] += shift.Interval.minutes()
	s.shifts[pi] = append(s.shifts[pi], shift)
}

func (s *solver) unplace(pi int, shift *Shift) {
	s.load[pi] -= shift.Interval.minutes()
	s.shifts[pi] = s.shifts[pi][:len(s.shifts[pi])-1]
}

func (s *solver) currentMax() int {
	m := 0
	for _, l := range s.load {
		if l > m {
			m = l
		}
	}
	return m
}

// lexLessPrefix reports whether the current partial assignment is
// lexicographically smaller than best over the assigned prefix.
// Returns false (prune) if it is already greater.
func (s *solver) prefixExceedsBest(depth int) bool {
	if !s.found {
		return false
	}
	for i := 0; i < len(s.order); i++ {
		cur := s.assign[i]
		if cur == "" {
			return false // remaining could still be smaller-or-equal
		}
		if cur != s.best[i] {
			return cur > s.best[i]
		}
	}
	return false
}

func (s *solver) search(depth int) {
	if depth == len(s.order) {
		m := s.currentMax()
		if !s.found || m < s.bestMax || (m == s.bestMax && s.lexSmaller()) {
			s.found = true
			s.bestMax = m
			s.best = append([]string(nil), s.assign...)
		}
		return
	}
	if s.found {
		cur := s.currentMax()
		if cur > s.bestMax {
			return
		}
		// Only prune on the lexicographic prefix when the max load can
		// no longer improve (loads never decrease during search).
		if cur == s.bestMax && s.prefixExceedsBest(depth) {
			return
		}
	}
	shift := s.order[depth]
	if s.assign[depth] != "" {
		pi := s.personIdx(s.assign[depth])
		if !s.canPlace(pi, shift) {
			return // infeasible fixed assignment
		}
		s.place(pi, shift)
		s.search(depth + 1)
		s.unplace(pi, shift)
		return
	}
	for pi, person := range s.people {
		if !s.canPlace(pi, shift) {
			continue
		}
		s.assign[depth] = person.ID
		s.place(pi, shift)
		s.search(depth + 1)
		s.unplace(pi, shift)
		s.assign[depth] = ""
	}
}

// lexSmaller compares current assignment to best.
func (s *solver) lexSmaller() bool {
	for i := range s.assign {
		if s.assign[i] != s.best[i] {
			return s.assign[i] < s.best[i]
		}
	}
	return false
}

// solve finds the optimal assignment or reports infeasibility.
func solve(p *Problem) (Assignment, bool) {
	s := newSolver(p)
	s.search(0)
	if !s.found {
		return nil, false
	}
	out := Assignment{}
	for i, shift := range s.order {
		out[shift.ID] = s.best[i]
	}
	return out, true
}
