package board

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

// The activity of a card is everything said about it and every run on it,
// in activity/<card>.md next to the columns. Entries are only added, never
// changed, and only by kard:
//
//	## 2026-11-27T14:02:31Z · vps-1 · run started
//	column: planning
//	session: 5f3c2a9e-41d7
//
//	## 2026-11-27T14:15:09Z · vps-1 · question to arnold
//	Should the export keep the old CSV columns?
//
// The kind decides what follows the head: the entries of a run hold
// `key: value` lines, all others free Markdown. A line of text that would
// read as the head of an entry is written with a backslash in front.

// Kinds of entries.
const (
	KindComment     = "comment"
	KindQuestion    = "question"
	KindRetry       = "retry"
	KindStop        = "stop"
	KindRunStarted  = "run started"
	KindRunPaused   = "run paused"
	KindRunFinished = "run finished"
	KindRunFailed   = "run failed"
)

var entryKinds = []string{KindComment, KindQuestion, KindRetry, KindStop, KindRunStarted, KindRunPaused, KindRunFinished, KindRunFailed}

// IsRun reports whether an entry of this kind belongs to a run.
func IsRun(kind string) bool { return strings.HasPrefix(kind, "run ") }

// Entry is one entry of a card's activity.
type Entry struct {
	// Time is when the entry was written, in UTC.
	Time   time.Time `json:"time"`
	Author string    `json:"author"`
	Kind   string    `json:"kind"`
	// To names whom a question is for. Empty: whoever sees it.
	To []string `json:"to,omitempty"`
	// Text is the Markdown of every kind but a run's.
	Text string `json:"text,omitempty"`
	// Values are the `key: value` lines of a run's entry.
	Values map[string]string `json:"values,omitempty"`
}

// Activity is the activity file of one card.
type Activity struct {
	Card string
	// Path is relative to the board root and uses forward slashes.
	Path    string
	Entries []Entry
}

// ActivityPath returns where the activity of a card is kept, relative to
// the board root.
func ActivityPath(id string) string { return ActivityDir + "/" + id + ".md" }

const (
	timeLayout      = "2006-01-02T15:04:05Z"
	shortTimeLayout = "2006-01-02T15:04Z"
	headSep         = " · "
)

var (
	// looksLikeHead catches every line that starts like the head of an
	// entry. Such a line in a text is escaped, and in a file it must be a
	// head that can be read.
	looksLikeHead = regexp.MustCompile(`^## \d{4}-\d{2}-\d{2}`)
	escapedHead   = regexp.MustCompile(`^\\+## \d{4}-\d{2}-\d{2}`)
	valueKeyRe    = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
)

// valueOrder is the order in which kard writes the values of a run. Keys it
// does not know follow, sorted.
var valueOrder = []string{"column", "session", "duration", "turns", "tokens", "cost", "by", "reason"}

// parseHead reads the head line of an entry. ok is false if the line is no
// head at all; a message says what is wrong with a line that looks like one.
func parseHead(line string) (e Entry, ok bool, msg string) {
	if !looksLikeHead.MatchString(line) {
		return e, false, ""
	}
	parts := strings.SplitN(strings.TrimPrefix(line, "## "), headSep, 3)
	if len(parts) != 3 {
		return e, false, "head must be `## <time> · <person> · <kind>`"
	}
	t, err := time.Parse(timeLayout, parts[0])
	if err != nil {
		if t, err = time.Parse(shortTimeLayout, parts[0]); err != nil {
			return e, false, fmt.Sprintf("%q is not a time in UTC like 2026-11-27T14:02:31Z", parts[0])
		}
	}
	author, valid := NormalizePerson(parts[1])
	if !valid {
		return e, false, fmt.Sprintf("%q is neither a short name nor an e-mail address", parts[1])
	}
	e = Entry{Time: t, Author: author, Kind: parts[2]}
	if rest, found := strings.CutPrefix(e.Kind, KindQuestion+" to "); found {
		e.Kind = KindQuestion
		for _, raw := range strings.Split(rest, ",") {
			name, valid := NormalizePerson(raw)
			if !valid {
				return e, false, fmt.Sprintf("question to %q: neither a short name nor an e-mail address", strings.TrimSpace(raw))
			}
			e.To = append(e.To, name)
		}
	}
	if !slices.Contains(entryKinds, e.Kind) {
		return e, false, fmt.Sprintf("unknown kind %q, expected one of %s", parts[2], strings.Join(entryKinds, ", "))
	}
	return e, true, ""
}

// ParseActivity reads an activity file. It returns the entries it could
// read and a message for every line that breaks the format.
func ParseActivity(text string) ([]Entry, []string) {
	text = strings.TrimPrefix(text, "\uFEFF")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	var entries []Entry
	var msgs []string
	var body []string
	var bodyStart int
	cur := -1

	flush := func() {
		if cur < 0 {
			return
		}
		e := &entries[cur]
		lines := trimBlank(body)
		if IsRun(e.Kind) {
			for i, line := range lines {
				if strings.TrimSpace(line) == "" {
					continue
				}
				k, v, found := strings.Cut(line, ":")
				k, v = strings.TrimSpace(k), strings.TrimSpace(v)
				if !found || !valueKeyRe.MatchString(k) {
					msgs = append(msgs, fmt.Sprintf("line %d: a run's entry holds only `key: value` lines", bodyStart+i))
					continue
				}
				if e.Values == nil {
					e.Values = map[string]string{}
				}
				e.Values[k] = v
			}
		} else {
			for i, line := range lines {
				if escapedHead.MatchString(line) {
					lines[i] = line[1:]
				}
			}
			e.Text = strings.Join(lines, "\n")
		}
		body = nil
	}

	for i, line := range strings.Split(text, "\n") {
		n := i + 1
		e, ok, msg := parseHead(line)
		switch {
		case ok:
			flush()
			entries = append(entries, e)
			cur, bodyStart = len(entries)-1, n+1
		case msg != "":
			msgs = append(msgs, fmt.Sprintf("line %d: %s", n, msg))
		case cur < 0:
			if strings.TrimSpace(line) != "" {
				msgs = append(msgs, fmt.Sprintf("line %d: text before the first entry", n))
			}
		default:
			body = append(body, line)
		}
	}
	flush()
	return entries, msgs
}

func trimBlank(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// check says what is wrong with an entry before it is written.
func (e Entry) check() error {
	if !slices.Contains(entryKinds, e.Kind) {
		return fmt.Errorf("unknown kind of entry %q", e.Kind)
	}
	if _, ok := NormalizePerson(e.Author); !ok {
		return fmt.Errorf("%q is neither a short name nor an e-mail address", e.Author)
	}
	if len(e.To) > 0 && e.Kind != KindQuestion {
		return errors.New("only a question is for someone")
	}
	for i, p := range e.To {
		name, ok := NormalizePerson(p)
		switch {
		case !ok:
			return fmt.Errorf("%q is neither a short name nor an e-mail address", p)
		case slices.Contains(e.To[:i], name):
			return fmt.Errorf("%q is named twice", name)
		}
	}
	if IsRun(e.Kind) {
		if strings.TrimSpace(e.Text) != "" {
			return errors.New("a run's entry holds values, no text")
		}
		for k, v := range e.Values {
			if !valueKeyRe.MatchString(k) {
				return fmt.Errorf("%q is not a key (a-z, 0-9, _)", k)
			}
			if strings.ContainsAny(v, "\r\n") {
				return fmt.Errorf("value of %s must be one line", k)
			}
		}
		return nil
	}
	if len(e.Values) > 0 {
		return fmt.Errorf("only a run's entry holds values, not a %s", e.Kind)
	}
	if (e.Kind == KindComment || e.Kind == KindQuestion) && strings.TrimSpace(e.Text) == "" {
		return fmt.Errorf("the %s is empty", e.Kind)
	}
	return nil
}

// Format returns the entry as kard writes it, ending in a newline.
func (e Entry) Format() string {
	var b strings.Builder
	kind := e.Kind
	if len(e.To) > 0 {
		kind += " to " + strings.Join(e.To, ", ")
	}
	fmt.Fprintf(&b, "## %s%s%s%s%s\n", e.Time.UTC().Format(timeLayout), headSep, e.Author, headSep, kind)
	if IsRun(e.Kind) {
		for _, k := range valueKeys(e.Values) {
			fmt.Fprintf(&b, "%s: %s\n", k, strings.TrimSpace(e.Values[k]))
		}
		return b.String()
	}
	text := strings.ReplaceAll(e.Text, "\r\n", "\n")
	for _, line := range trimBlank(strings.Split(text, "\n")) {
		if looksLikeHead.MatchString(line) || escapedHead.MatchString(line) {
			line = `\` + line
		}
		b.WriteString(strings.TrimRight(line, " \t") + "\n")
	}
	return b.String()
}

func valueKeys(values map[string]string) []string {
	var keys, rest []string
	for _, k := range valueOrder {
		if _, ok := values[k]; ok {
			keys = append(keys, k)
		}
	}
	for k := range values {
		if !slices.Contains(valueOrder, k) {
			rest = append(rest, k)
		}
	}
	slices.Sort(rest)
	return append(keys, rest...)
}

// scanActivity reads the activity directory. A file whose card does not
// exist is reported (E12), and so is a file that breaks the format (E13).
func (b *Board) scanActivity() error {
	b.activity = map[string]*Activity{}
	entries, err := os.ReadDir(filepath.Join(b.Root, ActivityDir))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") || !isCardFile(e) {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".md")
		rel := ActivityPath(id)
		data, err := os.ReadFile(filepath.Join(b.Root, ActivityDir, e.Name()))
		if err != nil {
			return err
		}
		list, msgs := ParseActivity(string(data))
		b.activity[id] = &Activity{Card: id, Path: rel, Entries: list}
		for _, m := range msgs {
			b.problems = append(b.problems, Problem{Code: "E13", Severity: SeverityError, Path: rel, Message: m})
		}
		if _, ok := b.Card(id); !ok {
			b.problems = append(b.problems, Problem{Code: "E12", Severity: SeverityError, Path: rel,
				Message: fmt.Sprintf("activity of card %q, which is not on the board", id)})
		}
	}
	return nil
}

// Activity returns the entries of a card's activity, oldest first.
func (b *Board) Activity(id string) []Entry {
	if a, ok := b.activity[strings.TrimSuffix(id, ".md")]; ok {
		return a.Entries
	}
	return nil
}

// HasActivity reports whether a card has an activity file.
func (b *Board) HasActivity(id string) bool {
	_, ok := b.activity[strings.TrimSuffix(id, ".md")]
	return ok
}

// AddEntry adds an entry to the activity of a card and returns the path of
// the activity file, relative to the board root. The card's own file stays
// untouched. The file is written in one step: either the entry is in it,
// or the file is as it was.
func (b *Board) AddEntry(id string, e Entry) (string, error) {
	if err := b.writable(); err != nil {
		return "", err
	}
	c, err := b.one(id)
	if err != nil {
		return "", err
	}
	e.Author, _ = NormalizePerson(e.Author)
	e.To = slices.Clone(e.To)
	for i, p := range e.To {
		e.To[i], _ = NormalizePerson(p)
	}
	if e.Time.IsZero() {
		e.Time = time.Now()
	}
	e.Time = e.Time.UTC().Truncate(time.Second)
	if err := e.check(); err != nil {
		return "", err
	}

	rel := ActivityPath(c.ID)
	path := filepath.Join(b.Root, filepath.FromSlash(rel))
	old, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	content := strings.TrimRight(strings.ReplaceAll(string(old), "\r\n", "\n"), "\n")
	if content != "" {
		content += "\n\n"
	}
	content += e.Format()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := writeFileAtomic(path, content); err != nil {
		return "", err
	}
	if b.activity == nil {
		b.activity = map[string]*Activity{}
	}
	a, ok := b.activity[c.ID]
	if !ok {
		a = &Activity{Card: c.ID, Path: rel}
		b.activity[c.ID] = a
	}
	// What was written is what a reader gets back.
	if list, _ := ParseActivity(e.Format()); len(list) == 1 {
		e = list[0]
	}
	a.Entries = append(a.Entries, e)
	return rel, nil
}
