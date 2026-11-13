package board

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// editHead returns the content of a card file with header fields set or
// removed. values maps a field name to its new value; nil removes the field.
// names gives the order in which the fields are applied.
//
// Only the lines of the named fields change. Other fields, comments, the
// order of the header and the text of the card stay as they are.
func editHead(text string, names []string, values map[string]*yaml.Node) (string, error) {
	const bom = "\uFEFF"
	hasBOM := strings.HasPrefix(text, bom)
	work := strings.TrimPrefix(text, bom)
	crlf := strings.Contains(work, "\r\n")
	work = strings.ReplaceAll(work, "\r\n", "\n")

	head, body, state := splitHead(work)
	if state == headUnclosed {
		return "", errors.New("header is not closed by a line `---`")
	}
	var lines []string
	if head != "" {
		lines = strings.Split(strings.TrimSuffix(head, "\n"), "\n")
	}
	for _, name := range names {
		var err error
		if lines, err = editHeadLines(lines, name, values[name]); err != nil {
			return "", err
		}
	}

	empty := !slices.ContainsFunc(lines, func(l string) bool { return strings.TrimSpace(l) != "" })
	var out string
	switch {
	case empty && state == headPresent:
		// The last field is gone: a card without fields has no header.
		out = strings.TrimLeft(body, "\n")
	case empty:
		out = work
	case state == headPresent:
		out = "---\n" + strings.Join(lines, "\n") + "\n---\n" + body
	default:
		out = "---\n" + strings.Join(lines, "\n") + "\n---\n\n" + strings.TrimLeft(work, "\n")
	}

	if crlf {
		out = strings.ReplaceAll(out, "\n", "\r\n")
	}
	if hasBOM {
		out = bom + out
	}
	return out, nil
}

// editHeadLines sets or removes one field in the lines of a header.
func editHeadLines(lines []string, name string, value *yaml.Node) ([]string, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(strings.Join(lines, "\n")), &doc); err != nil {
		return nil, errors.New("header is not valid YAML, fix it by hand first")
	}
	var root *yaml.Node
	if len(doc.Content) > 0 {
		root = doc.Content[0]
		switch {
		case root.Kind == yaml.ScalarNode && root.ShortTag() == "!!null":
			root = nil
		case root.Kind != yaml.MappingNode:
			return nil, errors.New("header must be a mapping of fields, fix it by hand first")
		case root.Style&yaml.FlowStyle != 0:
			return nil, errors.New("header is written as {...} on one line, which kard cannot edit")
		}
	}

	// The field covers the lines from its key up to the next key. Blank
	// lines and comments right above the next key belong to that key.
	start, end := -1, -1
	if root != nil {
		for i := 0; i+1 < len(root.Content); i += 2 {
			if root.Content[i].Value != name {
				continue
			}
			start, end = root.Content[i].Line-1, len(lines)
			if i+2 < len(root.Content) {
				end = root.Content[i+2].Line - 1
			}
			for end > start+1 && isBlankOrComment(lines[end-1]) {
				end--
			}
			break
		}
	}

	var replacement []string
	if value != nil {
		entry := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: name}, value,
		}}
		data, err := yaml.Marshal(entry)
		if err != nil {
			return nil, err
		}
		replacement = strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	}
	switch {
	case start >= 0:
		return slices.Concat(lines[:start], replacement, lines[end:]), nil
	case value != nil:
		return append(lines, replacement...), nil
	}
	return lines, nil
}

func isBlankOrComment(line string) bool {
	t := strings.TrimSpace(line)
	return t == "" || strings.HasPrefix(t, "#")
}

// writeFileAtomic replaces a file in one step, so a reader never sees half
// of it. The temporary file starts with a dot and is ignored by the board.
func writeFileAtomic(path, content string) error {
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".kb-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after the rename
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), mode); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
