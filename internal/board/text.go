package board

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// The text of a card is what follows its header: the title line and the
// Markdown below it. Change replaces it in one of two ways. The web UI
// sends a new title and text and leaves the header to the fields; `kb
// edit` brings back the whole file from the editor, header included.
// Either way the file must not have changed since it was read: what
// someone else wrote in the meantime is not overwritten.

// Text is a new title and text for a card. The header stays as it is.
type Text struct {
	Title string `json:"title"`
	// Body is the Markdown below the title line.
	Body string `json:"body"`
}

// StaleError means the card's file changed after the text to be saved was
// read from it. Nothing was saved; the text has to be read again.
type StaleError struct{ ID string }

func (e *StaleError) Error() string {
	return fmt.Sprintf("card %s was changed since it was opened; nothing was saved", e.ID)
}

// Sum is the fingerprint of a card's file, as Card.Sum and Change.Base use
// it.
func Sum(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:12])
}

// canonical brings text into the form kb writes: no byte order mark, line
// ends as "\n", one "\n" at the end.
func canonical(text string) string {
	text = strings.TrimPrefix(text, "\uFEFF")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	return strings.TrimRight(text, "\n") + "\n"
}

// withText puts a new title and text below the header of content.
func withText(content string, t Text) (string, error) {
	title := strings.TrimSpace(t.Title)
	if title == "" || strings.ContainsAny(title, "\r\n") {
		return "", errors.New("the title must be one line of text")
	}
	content = canonical(content)
	_, body, state := splitHead(content)
	if state == headUnclosed {
		return "", errors.New("the card's header is not closed by a line `---`; fix it in an editor first")
	}
	var out strings.Builder
	if head := content[:len(content)-len(body)]; head != "" {
		out.WriteString(head)
		out.WriteString("\n")
	}
	out.WriteString("# " + title + "\n")
	if text := strings.Trim(strings.ReplaceAll(t.Body, "\r\n", "\n"), "\n"); strings.TrimSpace(text) != "" {
		out.WriteString("\n" + strings.TrimRight(text, " \t\n") + "\n")
	}
	return out.String(), nil
}

// problemCodes returns the codes of a card's problems.
func problemCodes(c *Card) []string {
	out := make([]string, len(c.Problems))
	for i, p := range c.Problems {
		out[i] = p.Code
	}
	return out
}

// readable checks a whole file brought back from the editor before it
// replaces the card's: it must not have errors the card did not have
// already. Problems that need the rest of the board are left to `kb lint`.
// It returns the card as the file would make it.
func (b *Board) readable(old *Card, content string) (*Card, error) {
	c := &Card{ID: old.ID, Path: old.Path, Column: old.Column}
	c.parse(b.Config, content)
	had := problemCodes(old)
	var worse []string
	for _, p := range c.Problems {
		if !slices.Contains(had, p.Code) {
			worse = append(worse, p.Code+" "+p.Message)
		}
	}
	if len(worse) > 0 {
		return nil, fmt.Errorf("card %s would have errors, nothing was saved: %s", old.ID, strings.Join(worse, "; "))
	}
	return c, nil
}
