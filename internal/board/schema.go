package board

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// The header of a card as a JSON Schema, so editors and other tools check
// what kb checks: which fields exist, their types and values. Rules that
// need more than the header are left to `kb lint`: whether a referenced
// card exists, and which column a card lies in. As the column is not part
// of the header, a schema is made either for every column, requiring only
// the fields required everywhere, or for one column.

// object is a JSON object that keeps its keys in the order they were added.
type object []member

type member struct {
	key   string
	value any
}

func (o object) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, m := range o {
		if i > 0 {
			buf.WriteByte(',')
		}
		key, err := json.Marshal(m.key)
		if err != nil {
			return nil, err
		}
		value, err := json.Marshal(m.value)
		if err != nil {
			return nil, err
		}
		buf.Write(key)
		buf.WriteByte(':')
		buf.Write(value)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// The patterns are the very ones kb checks with; they are written so that
// JSON Schema reads them the same.
var (
	slugPattern    = slugRe.String()
	cardRefPattern = cardRefRe.String()
	datePattern    = dateRe.String()
)

// plain is what a YAML scalar may turn into when a tool reads it as JSON.
var plain = []string{"string", "number", "boolean"}

// fieldSchema describes the values a field takes.
func fieldSchema(f Field) object {
	s := object{{"title", f.Label()}}
	enum := func(values []string) member { return member{"enum", values} }
	switch f.Type {
	case TypeText:
		s = append(s, member{"type", plain}, member{"pattern", `^[^\n]*$`})
	case TypeEnum:
		s = append(s, enum(f.Values))
	case TypeList:
		items := object{{"type", plain}}
		if len(f.Values) > 0 {
			items = object{enum(f.Values)}
		}
		s = append(s, member{"type", "array"}, member{"items", items})
	case TypeDate:
		s = append(s, member{"type", "string"}, member{"format", "date"}, member{"pattern", datePattern})
	case TypeNumber:
		s = append(s, member{"type", "number"})
	case TypeBool:
		s = append(s, member{"type", "boolean"})
	case TypePerson:
		name := object{{"type", "string"}, {"pattern", slugPattern}}
		if len(f.Values) > 0 {
			name = append(name, enum(f.Values))
		}
		list := object{{"type", "array"}, {"items", name}, {"minItems", 1}, {"uniqueItems", true}}
		s = append(s, member{"oneOf", []object{name, list}})
	case TypeCards:
		s = append(s, member{"type", "array"}, member{"items", object{{"type", "string"}, {"pattern", cardRefPattern}}})
	}
	if where := f.Required; !where.All && len(where.Columns) > 0 {
		s = append(s, member{"description", "required in " + strings.Join(where.Columns, ", ")})
	}
	return s
}

// Schema returns the JSON Schema for the header of the board's cards. With
// a column, the fields required there are required; without one, only
// those required in every column.
func (c *Config) Schema(column string) ([]byte, error) {
	if column != "" {
		if _, ok := c.Column(column); !ok {
			ids := make([]string, len(c.Columns))
			for i, col := range c.Columns {
				ids[i] = col.ID
			}
			return nil, fmt.Errorf("no column %q, the board has %s", column, strings.Join(ids, ", "))
		}
	}
	title := "Card header, board " + c.ID
	if column != "" {
		title += ", column " + column
	}
	props := object{}
	required := []string{}
	for _, f := range c.AllFields() {
		props = append(props, member{f.Name, fieldSchema(f)})
		if f.Required.All || (column != "" && f.Required.In(column)) {
			required = append(required, f.Name)
		}
	}
	schema := object{
		{"$schema", "https://json-schema.org/draft/2020-12/schema"},
		{"title", title},
		{"description", "Made by `kb schema` from board.yaml. References to other cards and the rules of the board are checked by `kb lint`."},
		{"type", "object"},
		{"properties", props},
		{"required", required},
		{"additionalProperties", false},
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(schema); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
