package board

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// The functions in this file turn the named actions of the command line
// (take, done, assign, unassign) into a Change. The web UI uses the same
// functions, so a click there does exactly what the command does.

// assignment builds the "assignee=..." value for a list of people. An empty
// list removes the field.
func assignment(people []string) string {
	return FieldAssignee + "=" + strings.Join(people, ",")
}

// assigneesOf returns who the card is assigned to now. An unknown card has
// nobody; the change itself reports that the card does not exist.
func (b *Board) assigneesOf(id string) []string {
	if c, ok := b.Card(id); ok {
		return c.Assignees()
	}
	return nil
}

// with returns people followed by those of more that are not in it yet.
// Two addresses the .mailmap joins count as one person.
func (b *Board) with(people []string, more ...string) []string {
	out := slices.Clone(people)
	for _, p := range more {
		if !b.HasPerson(out, p) {
			out = append(out, p)
		}
	}
	return out
}

// TakeChange assigns the card to the user and moves it to the column for
// work in progress. With add, the user joins the people already on the card
// instead of replacing them. A board without such a column only assigns.
func (b *Board) TakeChange(id, user string, add bool) Change {
	people := []string{user}
	if add {
		people = b.with(b.assigneesOf(id), user)
	}
	ch := Change{User: user, Set: []string{assignment(people)}}
	if col, ok := b.Config.FirstDoing(); ok {
		ch.Column = col.ID
	}
	return ch
}

// DoneChange moves a card to the column for finished work.
func (b *Board) DoneChange() (Change, error) {
	col, ok := b.Config.FirstDone()
	if !ok {
		return Change{}, fmt.Errorf("board %s has no column with done: true", b.Config.ID)
	}
	return Change{Column: col.ID}, nil
}

// AssignChange says whose card it is, without moving it. Names may be given
// one by one or separated by commas. With add, they join the people already
// on the card instead of replacing them.
func (b *Board) AssignChange(id string, names []string, add bool) (Change, error) {
	var people []string
	for _, arg := range names {
		for _, name := range strings.Split(arg, ",") {
			if name = strings.TrimSpace(name); name != "" {
				people = b.with(people, name)
			}
		}
	}
	if len(people) == 0 {
		return Change{}, errors.New("no person named; unassign makes the card nobody's")
	}
	if add {
		people = b.with(b.assigneesOf(id), people...)
	}
	return Change{Set: []string{assignment(people)}}, nil
}

// UnassignChange takes the named people off the card, or everyone if no
// name is given.
func (b *Board) UnassignChange(id string, names []string) Change {
	if len(names) == 0 {
		return Change{Set: []string{assignment(nil)}}
	}
	people := slices.DeleteFunc(slices.Clone(b.assigneesOf(id)), func(p string) bool {
		return slices.ContainsFunc(names, func(n string) bool { return b.SamePerson(n, p) })
	})
	return Change{Set: []string{assignment(people)}}
}
