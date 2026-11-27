package board

import (
	"slices"
	"strings"
)

// The state of a card is never stored. It follows from the card, its
// column and its activity, so everyone who reads the board sees the same.

// States of a card. A card in none of them has the state "".
const (
	// StateRunning: the last run entry is `run started`.
	StateRunning = "running"
	// StateWaiting: a question is open. The last run entry is `run paused`,
	// or an agent asked after the last run, and no person has written since.
	StateWaiting = "waiting"
	// StateFailed: the last run entry is `run failed` and no person has
	// written since.
	StateFailed = "failed"
	// StateQueued: a run can start, see State.
	StateQueued = "queued"
)

// CardState is the state of a card and, while it waits, whom for.
type CardState struct {
	State string
	// WaitingFor names whom the open question is for. Empty while waiting
	// means: whoever sees it.
	WaitingFor []string
}

// IsAgent reports whether person is one of the board's agents.
func (b *Board) IsAgent(person string) bool {
	return b.Config != nil && b.HasPerson(b.Config.Agents, person)
}

// AgentsOf returns the agents a card is assigned to.
func (b *Board) AgentsOf(c *Card) []string {
	var out []string
	for _, p := range c.Assignees() {
		if b.IsAgent(p) {
			out = append(out, p)
		}
	}
	return out
}

// State works out the state of a card, in this order:
//
//   - running: the last run entry is `run started`;
//   - waiting: the last run entry is `run paused`, or an agent asked a
//     question after the last run entry, and no person has written since;
//   - failed: the last run entry is `run failed` and no person has written
//     since;
//   - queued: the card belongs to an agent, its column has `agent`, every
//     card under depends_on is done, and there is no run yet, a person has
//     written since the last run, or the last run was in another column.
//
// A person is anyone who is not one of the board's agents.
func (b *Board) State(c *Card) CardState {
	entries := b.Activity(c.ID)
	lastRun := -1
	for i, e := range entries {
		if IsRun(e.Kind) {
			lastRun = i
		}
	}
	personAfter := func(i int) bool {
		return slices.ContainsFunc(entries[i+1:], func(e Entry) bool { return !IsRun(e.Kind) && !b.IsAgent(e.Author) })
	}
	// The question that is open, if one is: the last an agent asked.
	question := -1
	for i, e := range entries {
		if e.Kind == KindQuestion && b.IsAgent(e.Author) {
			question = i
		}
	}

	if lastRun >= 0 {
		switch entries[lastRun].Kind {
		case KindRunStarted:
			return CardState{State: StateRunning}
		case KindRunPaused:
			if !personAfter(lastRun) {
				return b.waiting(entries, question)
			}
		}
	}
	if question > lastRun && !personAfter(question) {
		return b.waiting(entries, question)
	}
	if lastRun >= 0 && entries[lastRun].Kind == KindRunFailed && !personAfter(lastRun) {
		return CardState{State: StateFailed}
	}
	if b.queued(c, entries, lastRun, personAfter) {
		return CardState{State: StateQueued}
	}
	return CardState{}
}

func (b *Board) waiting(entries []Entry, question int) CardState {
	st := CardState{State: StateWaiting}
	if question >= 0 {
		st.WaitingFor = slices.Clone(entries[question].To)
	}
	return st
}

func (b *Board) queued(c *Card, entries []Entry, lastRun int, personAfter func(int) bool) bool {
	col, ok := b.Config.Column(c.Column)
	if !ok || col.Agent == nil || len(b.AgentsOf(c)) == 0 || !b.dependenciesDone(c) {
		return false
	}
	if lastRun < 0 || personAfter(lastRun) {
		return true
	}
	return runColumn(entries, lastRun) != c.Column
}

// runColumn returns the column of the run whose entry is at index i: its
// own `column` value, else that of the run's earlier entries.
func runColumn(entries []Entry, i int) string {
	for ; i >= 0; i-- {
		if e := entries[i]; IsRun(e.Kind) && e.Values["column"] != "" {
			return e.Values["column"]
		}
	}
	return ""
}

// dependenciesDone reports whether every card under depends_on is in a
// column with done. A card on another board is not checked here and does
// not hold the card back; a card that does not exist does.
func (b *Board) dependenciesDone(c *Card) bool {
	deps, _ := c.Fields["depends_on"].([]string)
	for _, ref := range deps {
		if strings.Contains(ref, "/") && !strings.HasPrefix(ref, b.Config.ID+"/") {
			continue
		}
		dep, ok := b.Card(strings.TrimPrefix(ref, b.Config.ID+"/"))
		if !ok {
			return false
		}
		if col, ok := b.Config.Column(dep.Column); !ok || !col.Done {
			return false
		}
	}
	return true
}
