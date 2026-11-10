package board

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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

func place(column, person string) string {
	if person != "" {
		return column + "/" + person
	}
	return column
}

// writable reports whether this code may change the board. Boards that ask
// for Git commits are refused until that is built: changing only the files
// would break what the board promises its users.
func (b *Board) writable() error {
	if b.Config == nil {
		return errors.New("board.yaml is missing or invalid")
	}
	if b.Config.Sync != "none" {
		return fmt.Errorf("board %s uses sync: %s, which kb cannot do yet; it changes only boards with sync: none",
			b.Config.ID, b.Config.Sync)
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

// targetPerson works out whose directory a card goes to in the column.
func targetPerson(col Column, person, user string) (string, error) {
	if !col.PerPerson {
		if person != "" {
			return "", fmt.Errorf("column %s has no directories per person", col.ID)
		}
		return "", nil
	}
	if person == "" {
		person = user
	}
	if person == "" {
		return "", ErrNeedUser
	}
	if !slugRe.MatchString(person) {
		return "", fmt.Errorf("%q is not a valid person name (a-z, 0-9, -)", person)
	}
	return person, nil
}

// missingRequired lists the fields the column demands and the values lack.
func (b *Board) missingRequired(column string, has func(field string) bool) []string {
	var missing []string
	for _, f := range b.Config.AllFields() {
		if f.Required.In(column) && !has(f.Name) {
			missing = append(missing, f.Name)
		}
	}
	return missing
}

// FirstPerPerson returns the first column with directories per person.
func (c *Config) FirstPerPerson() (Column, bool) {
	i := slices.IndexFunc(c.Columns, func(col Column) bool { return col.PerPerson })
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

// ── Move ────────────────────────────────────────────────────────────────────

// MoveOptions controls Move.
type MoveOptions struct {
	// Person is whose directory the card goes to in a per-person column.
	// Empty means the user.
	Person string
	// User is the user's own name. It may be empty if the move does not
	// depend on it.
	User string
	// Force overrides the rules that protect other people's cards and
	// required fields.
	Force bool
}

// MoveResult says what Move did.
type MoveResult struct {
	ID   string `json:"id"`
	From string `json:"from"`
	To   string `json:"to"`
	Path string `json:"path"`
	// Moved is false if the card already was where it should go.
	Moved bool `json:"moved"`
}

// Move puts a card into another column or another person's directory. It
// renames the file and never touches its content.
func (b *Board) Move(id, column string, opt MoveOptions) (*MoveResult, error) {
	if err := b.writable(); err != nil {
		return nil, err
	}
	c, err := b.one(id)
	if err != nil {
		return nil, err
	}
	col, err := b.column(column)
	if err != nil {
		return nil, err
	}
	person, err := targetPerson(col, opt.Person, opt.User)
	if err != nil {
		return nil, err
	}

	res := &MoveResult{ID: c.ID, From: place(c.Column, c.Person), To: place(col.ID, person), Path: c.Path}
	if res.From == res.To {
		return res, nil
	}

	if !opt.Force {
		if c.Person != "" {
			if opt.User == "" {
				return nil, ErrNeedUser
			}
			if c.Person != opt.User {
				return nil, &RefusedError{fmt.Sprintf("card %s is in the directory of %s", c.ID, c.Person)}
			}
		}
		missing := b.missingRequired(col.ID, func(f string) bool { _, ok := c.Fields[f]; return ok })
		if len(missing) > 0 {
			return nil, &RefusedError{fmt.Sprintf("card %s lacks %s, required in %s", c.ID, strings.Join(missing, ", "), col.ID)}
		}
	}

	newPath := res.To + "/" + c.ID + ".md"
	src := filepath.Join(b.Root, filepath.FromSlash(c.Path))
	dst := filepath.Join(b.Root, filepath.FromSlash(newPath))
	if _, err := os.Lstat(dst); err == nil {
		return nil, fmt.Errorf("%s already exists", newPath)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return nil, err
	}
	if err := os.Rename(src, dst); err != nil {
		return nil, err
	}
	if c.Person != "" {
		// A person's directory goes away with its last card, as it does in
		// Git. Remove fails on a directory that still holds something.
		_ = os.Remove(filepath.Dir(src))
	}

	c.Path, c.Column, c.Person = newPath, col.ID, person
	res.Path, res.Moved = newPath, true
	return res, nil
}

// ── New ─────────────────────────────────────────────────────────────────────

// NewOptions controls New.
type NewOptions struct {
	// ID is the file name without ".md". Empty means: derive it from the title.
	ID string
	// Column is where the card starts. Empty means the first column.
	Column string
	// Person and User work as in MoveOptions.
	Person string
	User   string
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
	person, err := targetPerson(col, opt.Person, opt.User)
	if err != nil {
		return nil, err
	}

	values, err := parseSet(b.Config, opt.Set)
	if err != nil {
		return nil, err
	}
	if _, set := values["created"]; !set && opt.Today != "" {
		created, _ := b.Config.Field("created")
		node := valueNode(created, opt.Today)
		if _, msg := checkValue(created, node); msg != "" {
			return nil, fmt.Errorf("created: %s", msg)
		}
		values["created"] = node
	}
	if !opt.Force {
		missing := b.missingRequired(col.ID, func(f string) bool { _, ok := values[f]; return ok })
		if len(missing) > 0 {
			return nil, &RefusedError{fmt.Sprintf("column %s requires %s", col.ID, strings.Join(missing, ", "))}
		}
	}

	var content strings.Builder
	if len(values) > 0 {
		head := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		for _, f := range b.Config.AllFields() {
			if v, ok := values[f.Name]; ok {
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
	path := filepath.Join(b.Root, filepath.FromSlash(place(col.ID, person)), file)
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
	return b.readCard(col, person, file)
}

// parseSet turns "field=value" arguments into checked header values.
func parseSet(cfg *Config, assignments []string) (map[string]*yaml.Node, error) {
	out := map[string]*yaml.Node{}
	for _, a := range assignments {
		name, raw, ok := strings.Cut(a, "=")
		name = strings.TrimSpace(name)
		if !ok || name == "" {
			return nil, fmt.Errorf("expected field=value, got %q", a)
		}
		if slices.Contains(reservedKeys, name) {
			return nil, fmt.Errorf("%q cannot be set: it comes from the card's place, file name or heading", name)
		}
		field, ok := cfg.Field(name)
		if !ok {
			return nil, fmt.Errorf("unknown field %q", name)
		}
		node := valueNode(field, strings.TrimSpace(raw))
		if _, msg := checkValue(field, node); msg != "" {
			return nil, fmt.Errorf("%s: %s", name, msg)
		}
		out[name] = node
	}
	return out, nil
}

// valueNode builds the YAML value for text given on the command line. Text
// and enum values are taken literally. Lists accept "a, b" as well as
// "[a, b]". Everything else is read as YAML.
func valueNode(f Field, raw string) *yaml.Node {
	str := func(s string) *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s} }
	switch f.Type {
	case TypeText, TypeEnum:
		return str(raw)
	case TypeList, TypeCards:
		if !strings.HasPrefix(raw, "[") {
			seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Style: yaml.FlowStyle}
			for _, part := range strings.Split(raw, ",") {
				if p := strings.TrimSpace(part); p != "" {
					seq.Content = append(seq.Content, str(p))
				}
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
