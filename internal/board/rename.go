package board

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// RenameOptions controls Rename.
type RenameOptions struct {
	// User is the user's own name. It may be empty as long as the card is
	// nobody's.
	User string
	// Force renames a card assigned to someone else.
	Force bool
}

// RenameResult says what Rename did.
type RenameResult struct {
	ID    string `json:"id"`
	NewID string `json:"new_id"`
	Path  string `json:"path"`
	// NewPath is where the card is now: the same column, the new name.
	NewPath string `json:"new_path"`
	// Updated holds the paths of the other cards whose references now name
	// the new id.
	Updated []string `json:"updated,omitempty"`
}

// refersTo reports whether ref names the card id on board boardID: plainly,
// or with the board's own id in front.
func refersTo(ref, boardID, id string) bool {
	return ref == id || ref == boardID+"/"+id
}

// Rename gives a card a new file name, which is its id, and makes every
// reference to it on this board name the new id. The card keeps its column
// and its content; a referencing card changes only in the lines of the
// fields that name it. References from other boards are not touched: kb
// does not change other boards.
//
// Renaming someone else's card follows the same rule as Change. Updating
// a reference is no change of the referring card's own matter, so it is
// done whoever that card belongs to.
func (b *Board) Rename(id, newID string, opt RenameOptions) (*RenameResult, error) {
	if err := b.writable(); err != nil {
		return nil, err
	}
	c, err := b.one(id)
	if err != nil {
		return nil, err
	}
	newID = strings.TrimSuffix(newID, ".md")
	if !slugRe.MatchString(newID) || len(newID) > maxIDLength {
		return nil, fmt.Errorf("%q is not a valid card id (a-z, 0-9, -, at most %d characters)", newID, maxIDLength)
	}
	if newID == c.ID {
		return nil, fmt.Errorf("card %s already has that name", c.ID)
	}
	if other, ok := b.Card(newID); ok {
		return nil, fmt.Errorf("card %q already exists at %s", newID, other.Path)
	}
	if !opt.Force {
		if owners := c.Assignees(); len(owners) > 0 {
			if opt.User == "" {
				return nil, ErrNeedUser
			}
			if !slices.Contains(owners, opt.User) {
				return nil, &RefusedError{fmt.Sprintf("card %s is assigned to %s", c.ID, strings.Join(owners, ", "))}
			}
		}
	}

	res := &RenameResult{ID: c.ID, NewID: newID, Path: c.Path, NewPath: c.Column + "/" + newID + ".md"}
	src := filepath.Join(b.Root, filepath.FromSlash(c.Path))
	dst := filepath.Join(b.Root, filepath.FromSlash(res.NewPath))
	if _, err := os.Lstat(dst); err == nil {
		return nil, fmt.Errorf("%s already exists", res.NewPath)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	// Work out every referencing card's new text before touching the disk.
	type rewrite struct {
		card *Card
		text string
	}
	var rewrites []rewrite
	for _, other := range b.Cards {
		var names []string
		nodes := map[string]*yaml.Node{}
		for _, f := range b.Config.AllFields() {
			refs, _ := other.Fields[f.Name].([]string)
			if f.Type != TypeCards || !slices.ContainsFunc(refs, func(r string) bool { return refersTo(r, b.Config.ID, c.ID) }) {
				continue
			}
			renamed := make([]string, len(refs))
			for i, r := range refs {
				switch {
				case r == c.ID:
					renamed[i] = newID
				case r == b.Config.ID+"/"+c.ID:
					renamed[i] = b.Config.ID + "/" + newID
				default:
					renamed[i] = r
				}
			}
			names = append(names, f.Name)
			nodes[f.Name] = valueNode(f, strings.Join(renamed, ", "))
		}
		if len(names) == 0 {
			continue
		}
		path := filepath.Join(b.Root, filepath.FromSlash(other.Path))
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		text, err := editHead(string(data), names, nodes)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", other.Path, err)
		}
		rewrites = append(rewrites, rewrite{other, text})
	}

	if err := os.Rename(src, dst); err != nil {
		return nil, err
	}
	c.ID, c.Path = newID, res.NewPath
	for _, r := range rewrites {
		path := filepath.Join(b.Root, filepath.FromSlash(r.card.Path))
		if err := writeFileAtomic(path, r.text); err != nil {
			return res, fmt.Errorf("card %s is renamed, but the reference in %s could not be updated: %w", newID, r.card.Path, err)
		}
		r.card.Problems, r.card.refs, r.card.rejected = nil, nil, nil
		r.card.parse(b.Config, r.text)
		res.Updated = append(res.Updated, r.card.Path)
	}
	return res, nil
}
