package board

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

const (
	// ConfigFile is the file that turns a directory into a board.
	ConfigFile = "board.yaml"
	// FormatVersion is the only format version this code reads.
	FormatVersion = 1
)

var (
	slugRe      = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	columnIDRe  = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
	fieldNameRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
)

// FieldType is the type of a header field.
type FieldType string

const (
	TypeText   FieldType = "text"
	TypeEnum   FieldType = "enum"
	TypeList   FieldType = "list"
	TypeDate   FieldType = "date"
	TypeNumber FieldType = "number"
	TypeBool   FieldType = "bool"
	TypePerson FieldType = "person"
	TypeCards  FieldType = "cards"
)

var fieldTypes = []FieldType{TypeText, TypeEnum, TypeList, TypeDate, TypeNumber, TypeBool, TypePerson, TypeCards}

// reservedKeys must not appear in a card header: the path, the file name and
// the heading already carry this information.
var reservedKeys = []string{"status", "column", "id", "title"}

// FieldAssignee is the built-in field that says whose card it is.
const FieldAssignee = "assignee"

// builtinFields may be used without a declaration in board.yaml.
var builtinFields = []Field{
	{Name: FieldAssignee, Type: TypePerson},
	{Name: "tags", Type: TypeList},
	{Name: "due", Type: TypeDate},
	{Name: "depends_on", Type: TypeCards},
	{Name: "created", Type: TypeDate},
}

// Config is the content of board.yaml.
type Config struct {
	Version int      `yaml:"kardboard"`
	ID      string   `yaml:"id"`
	Name    string   `yaml:"name"`
	Sync    string   `yaml:"sync"`
	Columns []Column `yaml:"columns"`
	Fields  Fields   `yaml:"fields"`
}

// Column is one column of the board. Its ID is the directory name.
type Column struct {
	ID    string `yaml:"id"`
	Title string `yaml:"title"`
	// Doing marks the column for work in progress, the target of `kard take`.
	Doing bool `yaml:"doing"`
	// Done marks the column for finished work, the target of `kard done`.
	Done  bool `yaml:"done"`
	Limit int  `yaml:"limit"`
}

// Label is the display name of the column.
func (c Column) Label() string {
	if c.Title != "" {
		return c.Title
	}
	return c.ID
}

// Field declares one header field.
type Field struct {
	Name     string
	Type     FieldType
	Title    string
	Values   []string
	Required Required
}

// Label is the display name of the field.
func (f Field) Label() string {
	if f.Title != "" {
		return f.Title
	}
	return f.Name
}

// Required says where a field is mandatory: everywhere or in some columns.
type Required struct {
	All     bool
	Columns []string
}

// In reports whether the field is mandatory in the given column.
func (r Required) In(column string) bool {
	return r.All || slices.Contains(r.Columns, column)
}

// Fields keeps the declared fields in the order of board.yaml.
type Fields []Field

// UnmarshalYAML reads the `fields` mapping by hand: the order of the keys
// matters for display, and unknown keys must be reported.
func (fs *Fields) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return &yaml.TypeError{Errors: []string{fmt.Sprintf("line %d: fields must be a mapping of field names", node.Line)}}
	}
	var errs []string
	fail := func(n *yaml.Node, format string, args ...any) {
		errs = append(errs, fmt.Sprintf("line %d: ", n.Line)+fmt.Sprintf(format, args...))
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		key, def := node.Content[i], node.Content[i+1]
		f := Field{Name: key.Value}
		if def.Kind != yaml.MappingNode {
			fail(def, "field %s must be a mapping with at least `type`", f.Name)
			*fs = append(*fs, f)
			continue
		}
		for j := 0; j+1 < len(def.Content); j += 2 {
			k, v := def.Content[j], def.Content[j+1]
			switch k.Value {
			case "type":
				f.Type = FieldType(v.Value)
			case "title":
				f.Title = v.Value
			case "values":
				vals, ok := scalarList(v)
				if !ok {
					fail(v, "field %s: values must be a list of plain values", f.Name)
				}
				f.Values = vals
			case "required":
				switch {
				case v.Kind == yaml.ScalarNode && v.ShortTag() == "!!bool":
					f.Required.All = v.Value == "true"
				case v.Kind == yaml.SequenceNode:
					cols, ok := scalarList(v)
					if !ok {
						fail(v, "field %s: required must be true, false or a list of columns", f.Name)
					}
					f.Required.Columns = cols
				default:
					fail(v, "field %s: required must be true, false or a list of columns", f.Name)
				}
			default:
				fail(k, "field %s: unknown key %q", f.Name, k.Value)
			}
		}
		*fs = append(*fs, f)
	}
	if len(errs) > 0 {
		return &yaml.TypeError{Errors: errs}
	}
	return nil
}

func scalarList(n *yaml.Node) ([]string, bool) {
	if n.Kind != yaml.SequenceNode {
		return nil, false
	}
	out := make([]string, 0, len(n.Content))
	for _, item := range n.Content {
		if item.Kind != yaml.ScalarNode || item.ShortTag() == "!!null" {
			return out, false
		}
		out = append(out, item.Value)
	}
	return out, true
}

// ParseConfig reads and checks board.yaml. It returns every violation it
// finds; the config is only usable when the list is empty.
func ParseConfig(data []byte) (*Config, []string) {
	cfg := &Config{}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(cfg); err != nil {
		var typeErr *yaml.TypeError
		switch {
		case errors.Is(err, io.EOF):
			return nil, []string{"file is empty"}
		case errors.As(err, &typeErr):
			return nil, typeErr.Errors
		default:
			return nil, []string{strings.TrimPrefix(err.Error(), "yaml: ")}
		}
	}
	if msgs := cfg.validate(); len(msgs) > 0 {
		return nil, msgs
	}
	return cfg, nil
}

func (c *Config) validate() []string {
	var msgs []string
	fail := func(format string, args ...any) { msgs = append(msgs, fmt.Sprintf(format, args...)) }

	switch {
	case c.Version == 0:
		fail("kardboard: format version is missing (expected %d)", FormatVersion)
	case c.Version != FormatVersion:
		fail("kardboard: format version %d is not supported (expected %d)", c.Version, FormatVersion)
	}
	if !slugRe.MatchString(c.ID) {
		fail("id: %q is not a valid board id (a-z, 0-9, -)", c.ID)
	}
	switch c.Sync {
	case "":
		c.Sync = "none"
	case "none", "commit", "push":
	default:
		fail("sync: %q is not one of none, commit, push", c.Sync)
	}

	if len(c.Columns) == 0 {
		fail("columns: at least one column is required")
	}
	seen := map[string]bool{}
	for i, col := range c.Columns {
		switch {
		case !columnIDRe.MatchString(col.ID):
			fail("columns[%d]: id %q is not a valid directory name (a-z, 0-9, -, _)", i, col.ID)
		case seen[col.ID]:
			fail("columns[%d]: id %q is used twice", i, col.ID)
		}
		seen[col.ID] = true
		if col.Limit < 0 {
			fail("columns[%d]: limit must not be negative", i)
		}
	}

	names := map[string]bool{}
	for _, f := range c.Fields {
		switch {
		case !fieldNameRe.MatchString(f.Name):
			fail("fields: %q is not a valid field name (a-z, 0-9, _)", f.Name)
			continue
		case slices.Contains(reservedKeys, f.Name):
			fail("fields: %q is reserved and cannot be a field", f.Name)
			continue
		case names[f.Name]:
			fail("fields: %q is declared twice", f.Name)
			continue
		}
		names[f.Name] = true

		switch {
		case f.Type == "":
			fail("fields.%s: type is missing", f.Name)
		case !slices.Contains(fieldTypes, f.Type):
			fail("fields.%s: type %q is not one of %s", f.Name, f.Type, joinTypes())
		}
		if b, ok := builtin(f.Name); ok && f.Type != "" && f.Type != b.Type {
			fail("fields.%s: built-in field must keep type %s", f.Name, b.Type)
		}
		switch f.Type {
		case TypeEnum:
			if len(f.Values) == 0 {
				fail("fields.%s: enum needs values", f.Name)
			}
		case TypeList, TypePerson:
		default:
			if len(f.Values) > 0 {
				fail("fields.%s: values are only allowed for enum, list and person", f.Name)
			}
		}
		for _, col := range f.Required.Columns {
			if !seen[col] {
				fail("fields.%s: required names unknown column %q", f.Name, col)
			}
		}
	}
	return msgs
}

func joinTypes() string {
	parts := make([]string, len(fieldTypes))
	for i, t := range fieldTypes {
		parts[i] = string(t)
	}
	return strings.Join(parts, ", ")
}

func builtin(name string) (Field, bool) {
	for _, f := range builtinFields {
		if f.Name == name {
			return f, true
		}
	}
	return Field{}, false
}

// Field returns the declaration of a header field, built-in ones included.
func (c *Config) Field(name string) (Field, bool) {
	for _, f := range c.Fields {
		if f.Name == name {
			return f, true
		}
	}
	return builtin(name)
}

// AllFields returns the declared fields in file order, followed by the
// built-in fields that board.yaml does not redeclare.
func (c *Config) AllFields() []Field {
	out := slices.Clone([]Field(c.Fields))
	for _, b := range builtinFields {
		if !slices.ContainsFunc(out, func(f Field) bool { return f.Name == b.Name }) {
			out = append(out, b)
		}
	}
	return out
}

// Column returns the column with the given id.
func (c *Config) Column(id string) (Column, bool) {
	for _, col := range c.Columns {
		if col.ID == id {
			return col, true
		}
	}
	return Column{}, false
}
