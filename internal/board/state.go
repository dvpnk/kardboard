package board

import (
	"slices"
	"strings"
)

// The state of a card is never stored. It follows from the card, its
// column and its activity, so everyone who reads the board sees the same.

// States of a card. A card in none of them has the state "".
const (
	// StateWaiting: an agent asked a question, and no person has written
	// since.
	StateWaiting = "waiting"
	// StateQueued: it is an agent's turn, see State.
	StateQueued = "queued"
)

// States lists the states a card can be in.
var States = []string{StateQueued, StateWaiting}

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
//   - waiting: an agent asked a question, and no person has written since;
//   - queued: the card belongs to an agent, its column has `agent`, and
//     every card under depends_on is done.
//
// A person is anyone who is not one of the board's agents. An agent's work
// in a column ends by moving the card on or by asking; a card it leaves
// where it is stays queued.
func (b *Board) State(c *Card) CardState {
	entries := b.Activity(c.ID)
	question := -1
	for i, e := range entries {
		switch {
		case e.Kind == KindQuestion && b.IsAgent(e.Author):
			question = i
		case !b.IsAgent(e.Author):
			question = -1 // a person wrote: whatever was asked is answered
		}
	}
	if question >= 0 {
		return CardState{State: StateWaiting, WaitingFor: slices.Clone(entries[question].To)}
	}
	col, ok := b.Config.Column(c.Column)
	if ok && col.Agent != nil && len(b.AgentsOf(c)) > 0 && b.dependenciesDone(c) {
		return CardState{State: StateQueued}
	}
	return CardState{}
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
