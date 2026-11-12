package board

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"
)

// validate checks value against the parts of JSON Schema that Schema
// writes: type, enum, pattern, items, minItems, uniqueItems, oneOf,
// properties, required and additionalProperties. It returns what is wrong.
func validate(schema map[string]any, value any) []string {
	var errs []string
	fail := func(format string, args ...any) { errs = append(errs, fmt.Sprintf(format, args...)) }

	if types, ok := schema["type"]; ok {
		var want []any
		if list, isList := types.([]any); isList {
			want = list
		} else {
			want = []any{types}
		}
		if !slices.ContainsFunc(want, func(t any) bool { return jsonType(value, t.(string)) }) {
			fail("%v is not of type %v", value, types)
			return errs
		}
	}
	if enum, ok := schema["enum"].([]any); ok && !slices.ContainsFunc(enum, func(e any) bool { return reflect.DeepEqual(e, value) }) {
		fail("%v is not one of %v", value, enum)
	}
	if pattern, ok := schema["pattern"].(string); ok {
		if s, isString := value.(string); isString && !regexp.MustCompile(pattern).MatchString(s) {
			fail("%q does not match %s", s, pattern)
		}
	}
	if list, ok := value.([]any); ok {
		if min, has := schema["minItems"].(float64); has && len(list) < int(min) {
			fail("fewer than %v items", min)
		}
		if unique, _ := schema["uniqueItems"].(bool); unique {
			for i := range list {
				if slices.ContainsFunc(list[:i], func(x any) bool { return reflect.DeepEqual(x, list[i]) }) {
					fail("%v is there twice", list[i])
				}
			}
		}
		if items, has := schema["items"].(map[string]any); has {
			for _, item := range list {
				errs = append(errs, validate(items, item)...)
			}
		}
	}
	if oneOf, ok := schema["oneOf"].([]any); ok {
		n := 0
		for _, s := range oneOf {
			if len(validate(s.(map[string]any), value)) == 0 {
				n++
			}
		}
		if n != 1 {
			fail("%v matches %d of oneOf", value, n)
		}
	}
	if obj, ok := value.(map[string]any); ok {
		props, _ := schema["properties"].(map[string]any)
		for key, v := range obj {
			p, known := props[key]
			if !known {
				if extra, has := schema["additionalProperties"].(bool); has && !extra {
					fail("field %q is not allowed", key)
				}
				continue
			}
			errs = append(errs, validate(p.(map[string]any), v)...)
		}
		for _, r := range schema["required"].([]any) {
			if _, has := obj[r.(string)]; !has {
				fail("field %q is missing", r)
			}
		}
	}
	return errs
}

func jsonType(v any, t string) bool {
	switch v.(type) {
	case string:
		return t == "string"
	case float64:
		return t == "number"
	case bool:
		return t == "boolean"
	case []any:
		return t == "array"
	case map[string]any:
		return t == "object"
	}
	return false
}

// header reads a card's header the way a YAML 1.2 tool hands it to a JSON
// Schema check: dates stay text.
func header(t *testing.T, head string) any {
	t.Helper()
	var v any
	if err := yaml.Unmarshal([]byte(head), &v); err != nil {
		t.Fatal(err)
	}
	var walk func(any) any
	walk = func(v any) any {
		switch x := v.(type) {
		case time.Time:
			return x.Format("2006-01-02")
		case int:
			return float64(x)
		case map[string]any:
			for k, e := range x {
				x[k] = walk(e)
			}
		case []any:
			for i, e := range x {
				x[i] = walk(e)
			}
		}
		return v
	}
	if v == nil {
		return map[string]any{}
	}
	return walk(v)
}

func schemaOf(t *testing.T, cfg *Config, column string) map[string]any {
	t.Helper()
	data, err := cfg.Schema(column)
	if err != nil {
		t.Fatal(err)
	}
	var s map[string]any
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, data)
	}
	return s
}

func TestSchemaAcceptsTheExampleCards(t *testing.T) {
	b, err := Load(filepath.Join("..", "..", "examples", "board"))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range b.Cards {
		head, _, _ := splitHead(read(t, b, c.Path))
		for _, column := range []string{"", c.Column} {
			if errs := validate(schemaOf(t, b.Config, column), header(t, head)); len(errs) > 0 {
				t.Errorf("%s, schema for %q: %v", c.Path, column, errs)
			}
		}
	}
}

func TestSchemaRejectsWhatLintRejects(t *testing.T) {
	cfg := load(t, map[string]string{}).Config
	all, backlog := schemaOf(t, cfg, ""), schemaOf(t, cfg, "backlog")
	good := "size: S\ntags: [bug]\ndue: 2026-12-01\nestimate: 3\nblocked: true\nreviewer: [alex, sam]\nrelated: [a, other/b]\nowner: Einkauf\n"
	if errs := validate(backlog, header(t, good)); len(errs) > 0 {
		t.Fatalf("a good header is rejected: %v", errs)
	}
	for head, want := range map[string]string{
		"size: XXL\n":                 "not one of",
		"tags: [nope]\n":              "not one of",
		"due: 1.12.2026\n":            "does not match",
		"estimate: many\n":            "not of type",
		"blocked: maybe\n":            "not of type",
		"reviewer: Alex\n":            "matches 0 of oneOf",
		"reviewer: [alex, alex]\n":    "matches 0 of oneOf",
		"related: [Not A Card]\n":     "does not match",
		"colour: red\n":               "not allowed",
		"title: Something\n":          "not allowed",
		"size: S\nowner: [a, b]\n":    "not of type",
		"tags: [bug]\n# no size\n":    "", // fine for every column ...
		"assignee: alex\nsize: S\n":   "",
		"assignee: [alex]\nsize: S\n": "",
	} {
		errs := validate(all, header(t, head))
		switch {
		case want == "" && len(errs) > 0:
			t.Errorf("%q: %v, want no error", head, errs)
		case want != "" && !slices.ContainsFunc(errs, func(e string) bool { return regexp.MustCompile(want).MatchString(e) }):
			t.Errorf("%q: %v, want %q", head, errs, want)
		}
	}
	// ... but backlog requires size.
	if errs := validate(backlog, header(t, "tags: [bug]\n")); !slices.Contains(errs, `field "size" is missing`) {
		t.Errorf("backlog without size: %v", errs)
	}
	if _, err := cfg.Schema("nope"); err == nil {
		t.Error("schema for a column that does not exist")
	}
}
