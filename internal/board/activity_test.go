package board

import (
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

// agentConfig is a board with two agents and two columns they work in.
const agentConfig = `kardboard: 1
id: test
agents: [vps-1, laptop]
agent:
  max_turns: 80
  prompt: |
    Read CLAUDE.md first.
columns:
  - id: planning
    agent:
      mode: plan
      next: plan_review
      prompt: Write a plan into the card.
  - id: plan_review
  - id: working_on
    doing: true
    agent:
      mode: build
      next: review
  - id: review
  - id: done
    done: true
`

func TestAgentConfig(t *testing.T) {
	cfg, msgs := ParseConfig([]byte(agentConfig))
	if len(msgs) > 0 {
		t.Fatalf("problems: %q", msgs)
	}
	if !slices.Equal(cfg.Agents, []string{"vps-1", "laptop"}) || cfg.Agent.MaxTurns != 80 || cfg.Agent.Prompt != "Read CLAUDE.md first.\n" {
		t.Errorf("agents %q, agent %+v", cfg.Agents, cfg.Agent)
	}
	col, _ := cfg.Column("planning")
	if *col.Agent != (ColumnAgent{Mode: ModePlan, Next: "plan_review", Prompt: "Write a plan into the card."}) {
		t.Errorf("planning agent = %+v", col.Agent)
	}
	if col, _ := cfg.Column("review"); col.Agent != nil {
		t.Errorf("review belongs to people, got %+v", col.Agent)
	}
}

func TestAgentConfigErrors(t *testing.T) {
	base := "kardboard: 1\nid: test\n"
	cases := []struct {
		yaml, want string
	}{
		{"columns:\n  - id: a\n    agent: {next: b}\n  - id: b\n", "columns[0].agent: mode is missing"},
		{"columns:\n  - id: a\n    agent: {mode: fix}\n", `mode "fix" is not one of plan, build`},
		{"columns:\n  - id: a\n    agent: {mode: plan, next: nowhere}\n", `next names unknown column "nowhere"`},
		{"columns:\n  - id: a\n    agent: {mode: plan, next: a}\n", "next must be another column than a itself"},
		{"columns:\n  - id: a\n    agent: {mode: plan, model: opus}\n", "field model not found"},
		{"columns:\n  - id: activity\n", `id "activity" is reserved`},
		{"agent: {max_turns: -1}\ncolumns:\n  - id: a\n", "max_turns must not be negative"},
		{"agent: {turns: 3}\ncolumns:\n  - id: a\n", "field turns not found"},
		{"agents: [vps-1, vps-1]\ncolumns:\n  - id: a\n", `"vps-1" is named twice`},
		{"agents: [Not Valid]\ncolumns:\n  - id: a\n", "neither a short name"},
		{"agents: [vps-1]\ncolumns:\n  - id: a\nfields:\n  assignee:\n    type: person\n    values: [arnold]\n", `"vps-1" cannot be assigned`},
	}
	for _, c := range cases {
		_, msgs := ParseConfig([]byte(base + c.yaml))
		if !slices.ContainsFunc(msgs, func(m string) bool { return strings.Contains(m, c.want) }) {
			t.Errorf("%q: problems %q, want one with %q", c.yaml, msgs, c.want)
		}
	}
	// An agent's address is written in lower case, like any person.
	cfg, msgs := ParseConfig([]byte(base + "agents: [Bot@Example.com]\ncolumns:\n  - id: a\n"))
	if len(msgs) > 0 || cfg.Agents[0] != "bot@example.com" {
		t.Errorf("agents %v, problems %q", cfg, msgs)
	}
}

func at(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestActivityIsWrittenAndReadBack(t *testing.T) {
	b := load(t, map[string]string{ConfigFile: agentConfig, "planning/a.md": "# A\n"})
	entries := []Entry{
		{Time: at("2026-11-27T14:02:31Z"), Author: "vps-1", Kind: KindRunStarted, Values: map[string]string{"session": "5f3c", "column": "planning"}},
		{Time: at("2026-11-27T14:15:09Z"), Author: "vps-1", Kind: KindQuestion, To: []string{"Arnold@Example.com", "sly"}, Text: "Keep the old CSV columns?"},
		// A text that holds what looks like a head stays text.
		{Time: at("2026-11-27T14:31:00+01:00"), Author: "arnold", Kind: KindComment, Text: "\nYes.\n\n## Plan\n## 2026-01-01T00:00Z · sly · comment\n\\## 2026-01-01T00:00Z · x\n\n"},
		{Time: at("2026-11-27T14:40:00Z"), Author: "vps-1", Kind: KindRunPaused, Values: map[string]string{"cost": "1.12 USD", "column": "planning", "zebra": "z"}},
	}
	for _, e := range entries {
		if _, err := b.AddEntry("a", e); err != nil {
			t.Fatal(err)
		}
	}
	want := `## 2026-11-27T14:02:31Z · vps-1 · run started
column: planning
session: 5f3c

## 2026-11-27T14:15:09Z · vps-1 · question to arnold@example.com, sly
Keep the old CSV columns?

## 2026-11-27T13:31:00Z · arnold · comment
Yes.

## Plan
\## 2026-01-01T00:00Z · sly · comment
\\## 2026-01-01T00:00Z · x

## 2026-11-27T14:40:00Z · vps-1 · run paused
column: planning
cost: 1.12 USD
zebra: z
`
	if got := read(t, b, "activity/a.md"); got != want {
		t.Errorf("file =\n%s\nwant\n%s", got, want)
	}
	if got := read(t, b, "planning/a.md"); got != "# A\n" {
		t.Errorf("the card changed: %q", got)
	}

	fresh, err := Load(b.Root)
	if err != nil {
		t.Fatal(err)
	}
	if p := fresh.Problems(); len(p) > 0 {
		t.Errorf("problems: %+v", p)
	}
	got := fresh.Activity("a")
	if !reflect.DeepEqual(got, b.Activity("a")) {
		t.Errorf("read back %+v\nkept %+v", got, b.Activity("a"))
	}
	if len(got) != 4 || got[2].Text != "Yes.\n\n## Plan\n## 2026-01-01T00:00Z · sly · comment\n\\## 2026-01-01T00:00Z · x" {
		t.Errorf("comment text = %q", got[2].Text)
	}
	if !slices.Equal(got[1].To, []string{"arnold@example.com", "sly"}) || got[0].Values["session"] != "5f3c" {
		t.Errorf("entries = %+v", got)
	}
}

func TestEntryRules(t *testing.T) {
	b := load(t, map[string]string{ConfigFile: agentConfig, "planning/a.md": "# A\n"})
	cases := []struct {
		id   string
		e    Entry
		want string
	}{
		{"a", Entry{Author: "arnold", Kind: KindComment, Text: " \n "}, "the comment is empty"},
		{"a", Entry{Author: "arnold", Kind: KindQuestion}, "the question is empty"},
		{"a", Entry{Author: "arnold", Kind: "note", Text: "x"}, "unknown kind"},
		{"a", Entry{Author: "Arnold S", Kind: KindComment, Text: "x"}, "neither a short name"},
		{"a", Entry{Author: "arnold", Kind: KindComment, Text: "x", To: []string{"sly"}}, "only a question is for someone"},
		{"a", Entry{Author: "arnold", Kind: KindQuestion, Text: "x", To: []string{"sly", "sly"}}, "named twice"},
		{"a", Entry{Author: "arnold", Kind: KindComment, Text: "x", Values: map[string]string{"k": "v"}}, "only a run's entry holds values"},
		{"a", Entry{Author: "vps-1", Kind: KindRunFailed, Values: map[string]string{"reason": "a\nb"}}, "must be one line"},
		{"a", Entry{Author: "vps-1", Kind: KindRunFailed, Text: "x"}, "holds values, no text"},
		{"nope", Entry{Author: "arnold", Kind: KindComment, Text: "x"}, `no card "nope"`},
	}
	for _, c := range cases {
		if _, err := b.AddEntry(c.id, c.e); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%+v: err %v, want %q", c.e, err, c.want)
		}
	}
	if has(t, b, "activity/a.md") {
		t.Error("a refused entry left a file")
	}
	// stop and retry may come without a word.
	if _, err := b.AddEntry("a", Entry{Author: "arnold", Kind: KindStop}); err != nil {
		t.Error(err)
	}
}

func TestActivityLint(t *testing.T) {
	b := load(t, map[string]string{
		ConfigFile:           agentConfig,
		"planning/a.md":      "# A\n",
		"activity/gone.md":   "## 2026-11-27T14:02:31Z · arnold · comment\nHi.\n",
		"activity/.hidden":   "x",
		"activity/notes.txt": "not an activity file",
		"activity/a.md": "stray text\n\n" +
			"## 2026-11-27T14:02:31Z · arnold · shout\n" +
			"## 2026-11-27 · arnold · comment\n" +
			"## 2026-11-27T14:02:31Z · vps-1 · run started\nnot a value\ncolumn: planning\n\n" +
			"## 2026-11-27T14:03:00Z · arnold · comment\nFine.\n",
	})
	var got []string
	for _, p := range b.Problems() {
		got = append(got, p.Code+" "+p.Path+" "+p.Message)
	}
	want := []string{
		"E13 activity/a.md line 1: text before the first entry",
		`E13 activity/a.md line 3: unknown kind "shout", expected one of comment, question, retry, stop, run started, run paused, run finished, run failed`,
		`E13 activity/a.md line 4: "2026-11-27" is not a time in UTC like 2026-11-27T14:02:31Z`,
		"E13 activity/a.md line 6: a run's entry holds only `key: value` lines",
		`E12 activity/gone.md activity of card "gone", which is not on the board`,
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("problems\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	// What can be read is still there.
	if e := b.Activity("a"); len(e) != 2 || e[0].Values["column"] != "planning" || e[1].Text != "Fine." {
		t.Errorf("entries = %+v", e)
	}
}

func TestState(t *testing.T) {
	run := func(kind, column string) Entry {
		e := Entry{Author: "vps-1", Kind: kind}
		if column != "" {
			e.Values = map[string]string{"column": column}
		}
		return e
	}
	say := func(who, kind string, to ...string) Entry {
		return Entry{Author: who, Kind: kind, Text: "x", To: to}
	}
	cases := []struct {
		name    string
		card    string // file in a column
		entries []Entry
		want    string
		for_    []string
	}{
		{"agent, agent column, no run", "planning/c.md:vps-1", nil, StateQueued, nil},
		{"person's card", "planning/c.md:arnold", nil, "", nil},
		{"agent in a people column", "plan_review/c.md:vps-1", nil, "", nil},
		{"nobody's card", "planning/c.md:", nil, "", nil},
		{"agent among people", "planning/c.md:arnold,laptop", nil, StateQueued, nil},
		{"running", "planning/c.md:vps-1", []Entry{run(KindRunStarted, "planning")}, StateRunning, nil},
		{"running, a person wrote", "planning/c.md:vps-1", []Entry{run(KindRunStarted, "planning"), say("arnold", KindComment)}, StateRunning, nil},
		{"paused", "planning/c.md:vps-1", []Entry{run(KindRunStarted, "planning"), say("vps-1", KindQuestion, "arnold"), run(KindRunPaused, "planning")}, StateWaiting, []string{"arnold"}},
		{"paused, answered", "planning/c.md:vps-1", []Entry{run(KindRunStarted, "planning"), say("vps-1", KindQuestion), run(KindRunPaused, "planning"), say("sly", KindComment)}, StateQueued, nil},
		{"paused, answered in a people column", "plan_review/c.md:vps-1", []Entry{run(KindRunPaused, "planning"), say("sly", KindComment)}, "", nil},
		{"paused, the agent wrote on", "planning/c.md:vps-1", []Entry{run(KindRunPaused, "planning"), say("laptop", KindComment)}, StateWaiting, nil},
		{"question without a run", "plan_review/c.md:arnold", []Entry{say("vps-1", KindQuestion)}, StateWaiting, nil},
		{"question without a run, answered", "plan_review/c.md:arnold", []Entry{say("vps-1", KindQuestion, "sly"), say("arnold", KindComment)}, "", nil},
		{"a person's question", "plan_review/c.md:arnold", []Entry{say("sly", KindQuestion, "arnold")}, "", nil},
		{"question before the run finished", "plan_review/c.md:vps-1", []Entry{say("vps-1", KindQuestion), run(KindRunFinished, "planning")}, "", nil},
		{"failed", "planning/c.md:vps-1", []Entry{run(KindRunStarted, "planning"), run(KindRunFailed, "planning")}, StateFailed, nil},
		{"failed, retried", "planning/c.md:vps-1", []Entry{run(KindRunFailed, "planning"), say("arnold", KindRetry)}, StateQueued, nil},
		{"failed, moved on", "working_on/c.md:vps-1", []Entry{run(KindRunFailed, "planning")}, StateFailed, nil},
		{"finished here", "planning/c.md:vps-1", []Entry{run(KindRunStarted, "planning"), run(KindRunFinished, "")}, "", nil},
		{"finished elsewhere", "working_on/c.md:vps-1", []Entry{run(KindRunStarted, "planning"), run(KindRunFinished, "")}, StateQueued, nil},
		{"finished, commented", "planning/c.md:vps-1", []Entry{run(KindRunFinished, "planning"), say("arnold", KindComment)}, StateQueued, nil},
	}
	for _, c := range cases {
		path, who, _ := strings.Cut(c.card, ":")
		card := "# C\n"
		if who != "" {
			card = "---\nassignee: [" + who + "]\n---\n\n# C\n"
		}
		b := load(t, map[string]string{ConfigFile: agentConfig, path: card})
		for _, e := range c.entries {
			if _, err := b.AddEntry("c", e); err != nil {
				t.Fatalf("%s: %v", c.name, err)
			}
		}
		c1, _ := b.Card("c")
		if got := b.State(c1); got.State != c.want || !slices.Equal(got.WaitingFor, c.for_) {
			t.Errorf("%s: state %+v, want %q for %q", c.name, got, c.want, c.for_)
		}
	}
}

func TestStateWaitsForDependencies(t *testing.T) {
	b := load(t, map[string]string{
		ConfigFile:      agentConfig,
		"planning/a.md": "---\nassignee: vps-1\ndepends_on: [b, other/x]\n---\n\n# A\n",
		"review/b.md":   "# B\n",
		"planning/c.md": "---\nassignee: vps-1\ndepends_on: [gone]\n---\n\n# C\n",
		"planning/d.md": "---\nassignee: vps-1\ndepends_on: [test/b]\n---\n\n# D\n",
	})
	state := func(id string) string {
		c, _ := b.Card(id)
		return b.State(c).State
	}
	if state("a") != "" || state("c") != "" || state("d") != "" {
		t.Errorf("states a %q, c %q, d %q, want none while b is not done", state("a"), state("c"), state("d"))
	}
	if _, err := b.Change("b", Change{Column: "done"}); err != nil {
		t.Fatal(err)
	}
	// Another board's card does not hold it back; a missing one does.
	if state("a") != StateQueued || state("d") != StateQueued || state("c") != "" {
		t.Errorf("states a %q, c %q, d %q", state("a"), state("c"), state("d"))
	}
}

func TestRenameTakesTheActivityAlong(t *testing.T) {
	b := load(t, map[string]string{ConfigFile: agentConfig, "planning/a.md": "# A\n"})
	if _, err := b.AddEntry("a", Entry{Author: "arnold", Kind: KindComment, Text: "Hi."}); err != nil {
		t.Fatal(err)
	}
	res, err := b.Rename("a", "alpha", RenameOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Activity != "activity/a.md" || res.NewActivity != "activity/alpha.md" {
		t.Errorf("result = %+v", res)
	}
	if has(t, b, "activity/a.md") || !strings.Contains(read(t, b, "activity/alpha.md"), "Hi.") || len(b.Activity("alpha")) != 1 {
		t.Error("the activity did not follow the card")
	}

	// A file in the way stops the rename before anything moves.
	if _, err := b.AddEntry("alpha", Entry{Author: "arnold", Kind: KindComment, Text: "Again."}); err != nil {
		t.Fatal(err)
	}
	b2 := load(t, map[string]string{
		ConfigFile: agentConfig, "planning/a.md": "# A\n",
		"activity/a.md": "## 2026-11-27T14:02:31Z · arnold · comment\nHi.\n",
		"activity/b.md": "## 2026-11-27T14:02:31Z · arnold · comment\nOld.\n",
	})
	if _, err := b2.Rename("a", "b", RenameOptions{}); err == nil || !strings.Contains(err.Error(), "activity/b.md already exists") {
		t.Errorf("err = %v", err)
	}
	if !has(t, b2, "planning/a.md") || !has(t, b2, "activity/a.md") {
		t.Error("something moved")
	}
}
