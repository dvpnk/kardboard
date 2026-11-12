package board

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"unicode"

	"go.yaml.in/yaml/v3"
)

// RefusedError means a rule of the board forbids the action. The caller may
// repeat it with Force set.
type RefusedError struct{ Reason string }

func (e *RefusedError) Error() string { return e.Reason }

// ErrNeedUser means the action depends on who the user is, and no name was
// given.
var ErrNeedUser = errors.New("the user's name is needed")

// writable reports whether the board can be changed at all. Change and New
// only touch files. What a board with sync: commit or push promises on top
// of that is the business of package gitsync, which every caller outside
// this package goes through.
func (b *Board) writable() error {
	if b.Config == nil {
		return errors.New("board.yaml is missing or invalid")
	}
	return nil
}

// one returns the card with the given id and insists that it is unambiguous.
func (b *Board) one(id string) (*Card, error) {
	id = strings.TrimSuffix(id, ".md")
	var found []*Card
	for _, c := range b.Cards {
		if c.ID == id {
			found = append(found, c)
		}
	}
	switch len(found) {
	case 0:
		return nil, fmt.Errorf("no card %q on board %s", id, b.Config.ID)
	case 1:
		return found[0], nil
	default:
		paths := make([]string, len(found))
		for i, c := range found {
			paths[i] = c.Path
		}
		return nil, fmt.Errorf("card %q exists %d times (%s); rename one of them first", id, len(found), strings.Join(paths, ", "))
	}
}

func (b *Board) column(id string) (Column, error) {
	col, ok := b.Config.Column(id)
	if !ok {
		ids := make([]string, len(b.Config.Columns))
		for i, c := range b.Config.Columns {
			ids[i] = c.ID
		}
		return Column{}, fmt.Errorf("no column %q, the board has %s", id, strings.Join(ids, ", "))
	}
	return col, nil
}

// FirstDoing returns the first column for work in progress.
func (c *Config) FirstDoing() (Column, bool) {
	i := slices.IndexFunc(c.Columns, func(col Column) bool { return col.Doing })
	if i < 0 {
		return Column{}, false
	}
	return c.Columns[i], true
}

// FirstDone returns the first column whose cards count as done.
func (c *Config) FirstDone() (Column, bool) {
	i := slices.IndexFunc(c.Columns, func(col Column) bool { return col.Done })
	if i < 0 {
		return Column{}, false
	}
	return c.Columns[i], true
}

// ── Change ──────────────────────────────────────────────────────────────────

// Change describes what to do with a card: move it, change header fields,
// or both in one step.
type Change struct {
	// Column is the column to move the card to. Empty means: stay.
	Column string
	// Set holds header values in the form "field=value". An empty value
	// ("field=") removes the field.
	Set []string
	// User is the user's own name. It may be empty as long as the card is
	// nobody's.
	User string
	// Force overrides the rules that protect other people's cards and
	// required fields.
	Force bool
	// Text replaces the title and the text below it. Nil leaves them.
	Text *Text
	// File replaces the whole file, header included: what `kb edit` brings
	// back from the editor. It cannot go with Column, Set or Text. The
	// fields it changes count as set or removed, with the same rules.
	File *string
	// Base is the card's Sum when Text or File was read. If the file has
	// changed since, Change refuses with a *StaleError instead of
	// overwriting what is new. Empty skips the check.
	Base string
}

// ChangeResult says what Change did.
type ChangeResult struct {
	ID   string `json:"id"`
	From string `json:"from"`
	To   string `json:"to"`
	Path string `json:"path"`
	// Moved is false if the card already was in the column.
	Moved bool `json:"moved"`
	// Set holds the fields that got a new value, Removed the ones that went
	// away. Fields that already had the requested value are in neither.
	Set     map[string]any `json:"set,omitempty"`
	Removed []string       `json:"removed,omitempty"`
	// Text says whether the title or the text changed, or the file in a
	// way that is no field.
	Text bool `json:"text,omitempty"`
}

// Changed reports whether anything happened.
func (r *ChangeResult) Changed() bool {
	return r.Moved || len(r.Set) > 0 || len(r.Removed) > 0 || r.Text
}

// Change moves a card, edits its header and replaces its text. Moving
// renames the file; only the lines of the named fields are rewritten.
// Nothing is changed unless every rule allows the whole step.
func (b *Board) Change(id string, ch Change) (*ChangeResult, error) {
	if err := b.writable(); err != nil {
		return nil, err
	}
	if ch.File != nil && (ch.Column != "" || len(ch.Set) > 0 || ch.Text != nil) {
		return nil, errors.New("a whole file replaces everything; it cannot go with a move, fields or a text")
	}
	c, err := b.one(id)
	if err != nil {
		return nil, err
	}
	target := c.Column
	if ch.Column != "" {
		col, err := b.column(ch.Column)
		if err != nil {
			return nil, err
		}
		target = col.ID
	}
	names, nodes, values, err := parseSet(b.Config, ch.Set, true)
	if err != nil {
		return nil, err
	}

	src := filepath.Join(b.Root, filepath.FromSlash(c.Path))
	// The file is read when its content will change, and only once, so
	// what is checked against Base is what gets changed.
	var data string
	if len(names) > 0 || ch.Text != nil || ch.File != nil {
		raw, err := os.ReadFile(src)
		if err != nil {
			return nil, err
		}
		data = string(raw)
		if ch.Base != "" && (ch.Text != nil || ch.File != nil) && Sum(data) != ch.Base {
			return nil, &StaleError{ID: c.ID}
		}
	}

	res := &ChangeResult{ID: c.ID, From: c.Column, To: target, Path: c.Path, Moved: target != c.Column}
	newText := data
	if ch.File != nil {
		after, err := b.readable(c, *ch.File)
		if err != nil {
			return nil, err
		}
		newText = canonical(*ch.File)
		// The fields the editor changed, in the board's order.
		for _, f := range b.Config.AllFields() {
			old, had := c.Fields[f.Name]
			now, has := after.Fields[f.Name]
			switch {
			case had && !has:
				res.Removed = append(res.Removed, f.Name)
			case has && !reflect.DeepEqual(old, now):
				if res.Set == nil {
					res.Set = map[string]any{}
				}
				res.Set[f.Name] = now
			}
		}
		fieldsChanged := len(res.Set) > 0 || len(res.Removed) > 0
		res.Text = after.Body != c.Body || (!fieldsChanged && newText != data)
	} else {
		// Leave out fields that already are as requested.
		var edits []string
		for _, name := range names {
			old, had := c.Fields[name]
			switch {
			case nodes[name] == nil:
				if had || slices.Contains(c.rejected, name) {
					edits = append(edits, name)
					res.Removed = append(res.Removed, name)
				}
			case !had || !reflect.DeepEqual(old, values[name]):
				edits = append(edits, name)
				if res.Set == nil {
					res.Set = map[string]any{}
				}
				res.Set[name] = values[name]
			}
		}
		if len(edits) > 0 {
			if newText, err = editHead(data, edits, nodes); err != nil {
				return nil, fmt.Errorf("%s: %w", c.Path, err)
			}
		}
		if ch.Text != nil {
			withNew, err := withText(newText, *ch.Text)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", c.Path, err)
			}
			res.Text = withNew != canonical(newText)
			if res.Text {
				newText = withNew
			}
		}
	}
	if !res.Changed() {
		return res, nil
	}

	if !ch.Force {
		// A card belongs to everyone it is assigned to. Whoever is not
		// among them leaves it alone.
		if owners := c.Assignees(); len(owners) > 0 {
			if ch.User == "" {
				return nil, ErrNeedUser
			}
			if !slices.Contains(owners, ch.User) {
				return nil, &RefusedError{fmt.Sprintf("card %s is assigned to %s", c.ID, strings.Join(owners, ", "))}
			}
		}
		// A moved card must bring everything its new column requires. A
		// card that stays must not lose a required field; what it lacked
		// before is not this change's business.
		var missing []string
		for _, f := range b.Config.AllFields() {
			if !f.Required.In(target) {
				continue
			}
			_, has := c.Fields[f.Name]
			_, set := res.Set[f.Name]
			removed := slices.Contains(res.Removed, f.Name)
			if (res.Moved && !set && (!has || removed)) || (!res.Moved && removed) {
				missing = append(missing, f.Name)
			}
		}
		if len(missing) > 0 {
			return nil, &RefusedError{fmt.Sprintf("card %s would lack %s, required in %s", c.ID, strings.Join(missing, ", "), target)}
		}
	}

	newPath := target + "/" + c.ID + ".md"
	dst := filepath.Join(b.Root, filepath.FromSlash(newPath))

	// Work out everything that can fail before touching the disk.
	rewrite := newText != data
	if res.Moved {
		if _, err := os.Lstat(dst); err == nil {
			return nil, fmt.Errorf("%s already exists", newPath)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return nil, err
		}
	}

	if rewrite {
		if err := writeFileAtomic(src, newText); err != nil {
			return nil, err
		}
		c.Problems, c.refs, c.rejected = nil, nil, nil
		c.parse(b.Config, newText)
	}
	if res.Moved {
		if err := os.Rename(src, dst); err != nil {
			return nil, err
		}
		c.Path, c.Column, res.Path = newPath, target, newPath
	}
	return res, nil
}

// ── New ─────────────────────────────────────────────────────────────────────

// NewOptions controls New.
type NewOptions struct {
	// ID is the file name without ".md". Empty means: derive it from the title.
	ID string
	// Column is where the card starts. Empty means the first column.
	Column string
	// Set holds header values in the form "field=value".
	Set []string
	// Today is the value for `created`, in the form YYYY-MM-DD.
	Today string
	// Force creates the card even if the column requires a field it lacks.
	Force bool
}

// New creates a card and returns it as read back from disk.
func (b *Board) New(title string, opt NewOptions) (*Card, error) {
	if err := b.writable(); err != nil {
		return nil, err
	}
	title = strings.TrimSpace(title)
	if title == "" || strings.ContainsAny(title, "\r\n") {
		return nil, errors.New("the title must be one line of text")
	}

	id := opt.ID
	if id == "" {
		if id = Slug(title); id == "" {
			return nil, fmt.Errorf("cannot derive a file name from the title %q, give one with --id", title)
		}
	}
	if !slugRe.MatchString(id) || len(id) > maxIDLength {
		return nil, fmt.Errorf("%q is not a valid card id (a-z, 0-9, -, at most %d characters)", id, maxIDLength)
	}
	if existing, ok := b.Card(id); ok {
		return nil, fmt.Errorf("card %q already exists at %s, choose another name with --id", id, existing.Path)
	}

	columnID := opt.Column
	if columnID == "" {
		columnID = b.Config.Columns[0].ID
	}
	col, err := b.column(columnID)
	if err != nil {
		return nil, err
	}

	_, nodes, _, err := parseSet(b.Config, opt.Set, false)
	if err != nil {
		return nil, err
	}
	if _, set := nodes["created"]; !set && opt.Today != "" {
		created, _ := b.Config.Field("created")
		node := valueNode(created, opt.Today)
		if _, msg := checkValue(created, node); msg != "" {
			return nil, fmt.Errorf("created: %s", msg)
		}
		nodes["created"] = node
	}
	if !opt.Force {
		var missing []string
		for _, f := range b.Config.AllFields() {
			if _, ok := nodes[f.Name]; !ok && f.Required.In(col.ID) {
				missing = append(missing, f.Name)
			}
		}
		if len(missing) > 0 {
			return nil, &RefusedError{fmt.Sprintf("column %s requires %s", col.ID, strings.Join(missing, ", "))}
		}
	}

	var content strings.Builder
	if len(nodes) > 0 {
		head := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		for _, f := range b.Config.AllFields() {
			if v, ok := nodes[f.Name]; ok {
				head.Content = append(head.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: f.Name}, v)
			}
		}
		data, err := yaml.Marshal(head)
		if err != nil {
			return nil, err
		}
		content.WriteString("---\n")
		content.Write(data)
		content.WriteString("---\n\n")
	}
	content.WriteString("# " + title + "\n")

	file := id + ".md"
	path := filepath.Join(b.Root, col.ID, file)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return nil, err
	}
	if _, err := f.WriteString(content.String()); err != nil {
		f.Close()
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	return b.readCard(col, file)
}

// parseSet turns "field=value" arguments into checked header values. It
// returns the field names in the order given, the YAML node and the checked
// value of each. With allowRemove, "field=" stands for removing the field
// and yields a nil node.
func parseSet(cfg *Config, assignments []string, allowRemove bool) ([]string, map[string]*yaml.Node, map[string]any, error) {
	var names []string
	nodes := map[string]*yaml.Node{}
	values := map[string]any{}
	for _, a := range assignments {
		name, raw, ok := strings.Cut(a, "=")
		name, raw = strings.TrimSpace(name), strings.TrimSpace(raw)
		if !ok || name == "" {
			return nil, nil, nil, fmt.Errorf("expected field=value, got %q", a)
		}
		if slices.Contains(reservedKeys, name) {
			return nil, nil, nil, fmt.Errorf("%q cannot be set: it comes from the card's place, file name or heading", name)
		}
		field, ok := cfg.Field(name)
		if !ok {
			return nil, nil, nil, fmt.Errorf("unknown field %q", name)
		}
		if _, twice := nodes[name]; twice {
			return nil, nil, nil, fmt.Errorf("field %q is given twice", name)
		}
		names = append(names, name)
		if raw == "" && allowRemove {
			nodes[name] = nil
			continue
		}
		node := valueNode(field, raw)
		value, msg := checkValue(field, node)
		if msg != "" {
			return nil, nil, nil, fmt.Errorf("%s: %s", name, msg)
		}
		nodes[name], values[name] = node, value
	}
	return names, nodes, values, nil
}

// valueNode builds the YAML value for text given on the command line. Text
// and enum values are taken literally. Lists accept "a, b" as well as
// "[a, b]"; a single person is written as a plain name, several as a list.
// Everything else is read as YAML.
func valueNode(f Field, raw string) *yaml.Node {
	str := func(s string) *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s} }
	switch f.Type {
	case TypeText, TypeEnum:
		return str(raw)
	case TypeList, TypeCards, TypePerson:
		if !strings.HasPrefix(raw, "[") {
			seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Style: yaml.FlowStyle}
			for _, part := range strings.Split(raw, ",") {
				if p := strings.TrimSpace(part); p != "" {
					seq.Content = append(seq.Content, str(p))
				}
			}
			if f.Type == TypePerson && len(seq.Content) == 1 {
				return seq.Content[0]
			}
			return seq
		}
	}
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil || len(doc.Content) == 0 {
		return str(raw) // not YAML: checkValue rejects it for these types
	}
	node := doc.Content[0]
	if node.Kind == yaml.SequenceNode {
		node.Style = yaml.FlowStyle
	}
	return node
}

// ── Slug ────────────────────────────────────────────────────────────────────

// transliterations for letters that have a customary ASCII spelling.
var transliterations = map[rune]string{
	'ä': "ae", 'ö': "oe", 'ü': "ue", 'ß': "ss", 'æ': "ae", 'œ': "oe", 'ø': "o", 'å': "a",
	'à': "a", 'á': "a", 'â': "a", 'ã': "a", 'ą': "a",
	'ç': "c", 'ć': "c", 'č': "c",
	'è': "e", 'é': "e", 'ê': "e", 'ë': "e", 'ę': "e", 'ě': "e",
	'ì': "i", 'í': "i", 'î': "i", 'ï': "i", 'ı': "i",
	'ñ': "n", 'ń': "n", 'ł': "l", 'ğ': "g", 'ř': "r",
	'ò': "o", 'ó': "o", 'ô': "o", 'õ': "o", 'ő': "o",
	'ś': "s", 'š': "s", 'ş': "s",
	'ù': "u", 'ú': "u", 'û': "u", 'ű': "u",
	'ý': "y", 'ÿ': "y", 'ź': "z", 'ż': "z", 'ž': "z",
}

// Slug derives a card id from a title: lower case, a-z and 0-9 kept, known
// letters spelled out, everything else turned into single hyphens.
func Slug(title string) string {
	var b strings.Builder
	hyphen := false
	for _, r := range title {
		r = unicode.ToLower(r)
		var s string
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			s = string(r)
		default:
			s = transliterations[r]
		}
		if s == "" {
			hyphen = b.Len() > 0
			continue
		}
		if hyphen {
			b.WriteByte('-')
			hyphen = false
		}
		b.WriteString(s)
	}
	slug := b.String()
	if len(slug) > maxIDLength {
		cut := slug[:maxIDLength]
		// Prefer to end on a whole word, unless that loses half the name.
		if slug[maxIDLength] != '-' {
			if i := strings.LastIndexByte(cut, '-'); i >= maxIDLength/2 {
				cut = cut[:i]
			}
		}
		slug = strings.TrimRight(cut, "-")
	}
	return slug
}
