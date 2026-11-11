package board

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

const maxIDLength = 80

var (
	dateRe    = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	cardRefRe = regexp.MustCompile(`^([a-z0-9][a-z0-9-]*/)?[a-z0-9][a-z0-9-]*$`)
	titleRe   = regexp.MustCompile(`^# +(\S.*)$`)
)

// Card is one Markdown file in a column.
type Card struct {
	// ID is the file name without ".md". It is unique within the board.
	ID string `json:"id"`
	// Path is relative to the board root and uses forward slashes.
	Path   string `json:"path"`
	Column string `json:"column"`
	Title  string `json:"title"`
	// Fields holds the header values that passed their type check.
	Fields   map[string]any `json:"fields"`
	Problems []Problem      `json:"problems,omitempty"`
	// Body is the file content after the header, title line included.
	Body string `json:"-"`

	// refs are the targets of all fields of type cards.
	refs []string
	// rejected names the fields that are present but failed their check.
	rejected []string
}

// Assignee returns whose card it is, or "" if it is nobody's.
func (c *Card) Assignee() string {
	name, _ := c.Fields[FieldAssignee].(string)
	return name
}

// Valid reports whether the card has no errors. Warnings do not count.
func (c *Card) Valid() bool {
	return !slices.ContainsFunc(c.Problems, func(p Problem) bool { return p.Severity == SeverityError })
}

func (c *Card) fail(code, format string, args ...any) {
	c.Problems = append(c.Problems, Problem{
		Code:     code,
		Severity: severityOf(code),
		Path:     c.Path,
		Message:  fmt.Sprintf(format, args...),
	})
}

// parse fills title, fields and body from the file content and records
// violations of the rules E02 to E05 and E07.
func (c *Card) parse(cfg *Config, text string) {
	c.Fields = map[string]any{}
	text = strings.TrimPrefix(text, "\uFEFF")
	text = strings.ReplaceAll(text, "\r\n", "\n")

	head, body, state := splitHead(text)
	c.Body = body
	switch state {
	case headUnclosed:
		c.fail("E02", "header is not closed by a line `---`")
	case headPresent:
		c.parseHead(cfg, head)
	}

	c.Title = c.ID
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimRight(line, " \t")
		if line == "" {
			continue
		}
		if m := titleRe.FindStringSubmatch(line); m != nil {
			c.Title = strings.TrimSpace(m[1])
		} else {
			c.fail("E07", "first line must be the title `# ...`, found %q", clip(line, 40))
		}
		return
	}
	c.fail("E07", "title line `# ...` is missing")
}

type headState int

const (
	headAbsent headState = iota
	headPresent
	headUnclosed
)

// splitHead separates the YAML header from the rest. A header exists only if
// the very first line is `---`.
func splitHead(text string) (head, body string, state headState) {
	first, rest, found := strings.Cut(text, "\n")
	if strings.TrimRight(first, " \t") != "---" {
		return "", text, headAbsent
	}
	if !found {
		return "", "", headUnclosed
	}
	var b strings.Builder
	for rest != "" {
		line, tail, _ := strings.Cut(rest, "\n")
		rest = tail
		if strings.TrimRight(line, " \t") == "---" {
			return b.String(), rest, headPresent
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return "", "", headUnclosed
}

func (c *Card) parseHead(cfg *Config, head string) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(head), &doc); err != nil {
		c.fail("E02", "header is not valid YAML: %s", strings.TrimPrefix(err.Error(), "yaml: "))
		return
	}
	if doc.Kind == 0 || len(doc.Content) == 0 {
		return // empty header
	}
	root := doc.Content[0]
	if root.Kind == yaml.ScalarNode && root.ShortTag() == "!!null" {
		return
	}
	if root.Kind != yaml.MappingNode {
		c.fail("E02", "header must be a mapping of fields")
		return
	}

	seen := map[string]bool{}
	for i := 0; i+1 < len(root.Content); i += 2 {
		key, value := root.Content[i], root.Content[i+1]
		name := key.Value
		if key.Kind != yaml.ScalarNode {
			c.fail("E02", "header key in line %d is not a plain name", key.Line)
			continue
		}
		if seen[name] {
			c.fail("E02", "header names %q twice", name)
			continue
		}
		seen[name] = true

		if slices.Contains(reservedKeys, name) {
			c.fail("E04", "%q must not be in the header", name)
			continue
		}
		field, ok := cfg.Field(name)
		if !ok {
			c.fail("E03", "unknown field %q", name)
			continue
		}
		v, msg := checkValue(field, value)
		if msg != "" {
			c.fail("E05", "%s: %s", name, msg)
			c.rejected = append(c.rejected, name)
			continue
		}
		c.Fields[name] = v
		if field.Type == TypeCards {
			c.refs = append(c.refs, v.([]string)...)
		}
	}
}

// checkValue checks a header value against its field declaration. It returns
// the value in the form used for output, or a message saying what is wrong.
func checkValue(f Field, n *yaml.Node) (any, string) {
	if n.Kind == yaml.AliasNode && n.Alias != nil {
		n = n.Alias
	}
	isScalar := n.Kind == yaml.ScalarNode && n.ShortTag() != "!!null"

	switch f.Type {
	case TypeText:
		if !isScalar || strings.Contains(n.Value, "\n") {
			return nil, "must be a single line of text"
		}
		return n.Value, ""

	case TypeEnum:
		if !isScalar || !slices.Contains(f.Values, n.Value) {
			return nil, fmt.Sprintf("must be one of %s", strings.Join(f.Values, ", "))
		}
		return n.Value, ""

	case TypeList:
		items, ok := scalarList(n)
		if !ok {
			return nil, "must be a list of plain values"
		}
		if len(f.Values) > 0 {
			for _, item := range items {
				if !slices.Contains(f.Values, item) {
					return nil, fmt.Sprintf("%q is not one of %s", item, strings.Join(f.Values, ", "))
				}
			}
		}
		return items, ""

	case TypeDate:
		if isScalar && dateRe.MatchString(n.Value) {
			if _, err := time.Parse("2006-01-02", n.Value); err == nil {
				return n.Value, ""
			}
		}
		return nil, "must be a date in the form YYYY-MM-DD"

	case TypeNumber:
		if isScalar && (n.ShortTag() == "!!int" || n.ShortTag() == "!!float") {
			var v any
			if err := n.Decode(&v); err == nil {
				return v, ""
			}
		}
		return nil, "must be a number"

	case TypeBool:
		if isScalar && n.ShortTag() == "!!bool" {
			var v bool
			if err := n.Decode(&v); err == nil {
				return v, ""
			}
		}
		return nil, "must be true or false"

	case TypePerson:
		if !isScalar || !slugRe.MatchString(n.Value) {
			return nil, "must be a person's short name (a-z, 0-9, -)"
		}
		if len(f.Values) > 0 && !slices.Contains(f.Values, n.Value) {
			return nil, fmt.Sprintf("must be one of %s", strings.Join(f.Values, ", "))
		}
		return n.Value, ""

	case TypeCards:
		items, ok := scalarList(n)
		if !ok {
			return nil, "must be a list of card references"
		}
		for _, item := range items {
			if !cardRefRe.MatchString(item) {
				return nil, fmt.Sprintf("%q is not a card reference", item)
			}
		}
		return items, ""
	}
	return nil, fmt.Sprintf("field has unknown type %q", f.Type)
}

func clip(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}
