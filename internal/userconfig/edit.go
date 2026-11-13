package userconfig

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Keys of the lists in config.yaml that kard edits.
const (
	KeyBoards = "boards"
	KeyScan   = "scan"
)

// Expand turns a path from config.yaml into an absolute path: a leading ~
// stands for the home directory.
func Expand(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, strings.TrimPrefix(path, "~"))
		}
	}
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}

// Abbrev is the opposite of Expand: it writes the home directory as ~, so
// config.yaml stays readable and can move between machines.
func Abbrev(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if path == home {
		return "~"
	}
	if rest, ok := strings.CutPrefix(path, home+string(filepath.Separator)); ok {
		return "~/" + filepath.ToSlash(rest)
	}
	return path
}

// AddToList adds a path to the list under key, unless it is there already.
// It reports whether the file changed.
func AddToList(key, path string) (bool, error) {
	changed := false
	err := edit(func(root *yaml.Node) error {
		list, err := listNode(root, key, true)
		if err != nil {
			return err
		}
		for _, item := range list.Content {
			if Expand(item.Value) == path {
				return nil
			}
		}
		list.Content = append(list.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: Abbrev(path)})
		changed = true
		return nil
	})
	return changed, err
}

// RemoveFromList removes a path from the list under key. It reports whether
// the file changed.
func RemoveFromList(key, path string) (bool, error) {
	changed := false
	err := edit(func(root *yaml.Node) error {
		list, err := listNode(root, key, false)
		if err != nil || list == nil {
			return err
		}
		kept := list.Content[:0]
		for _, item := range list.Content {
			if Expand(item.Value) == path {
				changed = true
				continue
			}
			kept = append(kept, item)
		}
		list.Content = kept
		return nil
	})
	return changed, err
}

// edit loads config.yaml as a YAML tree, lets fn change it and writes it
// back. Working on the tree keeps the user's comments and the order of keys;
// blank lines between settings are lost, the YAML library does not keep them.
func edit(fn func(root *yaml.Node) error) error {
	path, err := Path()
	if err != nil {
		return err
	}
	var doc yaml.Node
	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return err
	default:
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	if doc.Kind == 0 || len(doc.Content) == 0 {
		doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}}
	}
	root := doc.Content[0]
	if root.Kind == yaml.ScalarNode && root.ShortTag() == "!!null" {
		root = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		doc.Content[0] = root
	}
	if root.Kind != yaml.MappingNode {
		return fmt.Errorf("%s: must be a mapping of settings", path)
	}

	if err := fn(root); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	if bytes.Equal(buf.Bytes(), data) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after the rename
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// listNode returns the sequence under key in the settings mapping. With
// create it adds an empty one if the key is missing.
func listNode(root *yaml.Node, key string, create bool) (*yaml.Node, error) {
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value != key {
			continue
		}
		value := root.Content[i+1]
		if value.Kind == yaml.ScalarNode && value.ShortTag() == "!!null" {
			*value = yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		}
		if value.Kind != yaml.SequenceNode {
			return nil, fmt.Errorf("%s must be a list", key)
		}
		return value, nil
	}
	if !create {
		return nil, nil
	}
	list := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, list)
	return list, nil
}
