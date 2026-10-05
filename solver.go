package main

import "sort"

// solverState tracks per-person booked intervals and minutes during search.
type solverState struct {
	taken   [][]Interval
	minutes []int
	assign  []int // shift index -> person index, -1 undecided
}

func newSolverState(p *Problem) *solverState {
	assign := make([]int, len(p.Shifts))
	for i := range assign {
		assign[i] = -1
	}
	return &solverState{
		taken:   make([][]Interval, len(p.People)),
		minutes: make([]int, len(p.People)),
		assign:  assign,
	}
}

func (s *solverState) place(p *Problem, shiftIdx, personIdx int) {
	s.assign[shiftIdx] = personIdx
	s.taken[personIdx] = append(s.taken[personIdx], p.Shifts[shiftIdx].Interval)
	s.minutes[personIdx] += p.Shifts[shiftIdx].Interval.minutes()
}

func (s *solverState) undo(p *Problem, shiftIdx, personIdx int) {
	s.assign[shiftIdx] = -1
	s.taken[personIdx] = s.taken[personIdx][:len(s.taken[personIdx])-1]
	s.minutes[personIdx] -= p.Shifts[shiftIdx].Interval.minutes()
}

// feasible reports whether a full assignment exists under the per-person
// minute cap (-1 means no extra cap). Fixed assignments are honoured.
func feasible(p *Problem, order []int, capMinutes int) bool {
	st := newSolverState(p)
	// Apply fixed assignments first; they must themselves be legal.
	for _, si := range order {
		personID, ok := p.Fixed[p.Shifts[si].ID]
		if !ok {
			continue
		}
		pi := personIndex(p, personID)
		if !p.canTake(pi, si, st.taken[pi], st.minutes[pi], capMinutes) {
			return false
		}
		st.place(p, si, pi)
	}
	var search func(pos int) bool
	search = func(pos int) bool {
		if pos == len(order) {
			return true
		}
		si := order[pos]
		if st.assign[si] >= 0 {
			return search(pos + 1)
		}
		for pi := range p.People {
			if p.canTake(pi, si, st.taken[pi], st.minutes[pi], capMinutes) {
				st.place(p, si, pi)
				if search(pos + 1) {
					return true
				}
				st.undo(p, si, pi)
			}
		}
		return false
	}
	return search(0)
}

func personIndex(p *Problem, id string) int {
	for i := range p.People {
		if p.People[i].ID == id {
			return i
		}
	}
	return -1
}

// solve returns the assignment (shift index -> person index) that first
// minimises the maximum per-person total minutes, then yields the
// lexicographically smallest person-ID sequence in shift order.
// ok is false when no feasible assignment exists.
func solve(p *Problem) (assign []int, ok bool) {
	order := p.sortedShifts()
	if !feasible(p, order, -1) {
		return nil, false
	}
	// Phase 1: smallest achievable cap on the maximum per-person minutes.
	total := 0
	for _, s := range p.Shifts {
		total += s.Interval.minutes()
	}
	lo, hi := 0, total
	for lo < hi {
		mid := (lo + hi) / 2
		if feasible(p, order, mid) {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	cap := lo
	// Phase 2: lexicographically smallest person-ID sequence under the cap.
	// People are tried in ID order, so the first complete assignment found
	// by depth-first search is the lexicographic minimum.
	peopleOrder := make([]int, len(p.People))
	for i := range peopleOrder {
		peopleOrder[i] = i
	}
	sort.Slice(peopleOrder, func(a, b int) bool {
		return p.People[peopleOrder[a]].ID < p.People[peopleOrder[b]].ID
	})
	st := newSolverState(p)
	for _, si := range order {
		if personID, okF := p.Fixed[p.Shifts[si].ID]; okF {
			st.place(p, si, personIndex(p, personID))
		}
	}
	var search func(pos int) bool
	search = func(pos int) bool {
		if pos == len(order) {
			return true
		}
		si := order[pos]
		if st.assign[si] >= 0 {
			return search(pos + 1)
		}
		for _, pi := range peopleOrder {
			if p.canTake(pi, si, st.taken[pi], st.minutes[pi], cap) {
				st.place(p, si, pi)
				if search(pos + 1) {
					return true
				}
				st.undo(p, si, pi)
			}
		}
		return false
	}
	if !search(0) {
		return nil, false
	}
	return st.assign, true
}
