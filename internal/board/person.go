package board

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// A person on a card is a short name (arnold) or an e-mail address
// (arnold@firma.de). Addresses are written and compared in lower case.

// addressRe accepts what Git accepts as an author's address, minus what
// would break a YAML list or a comma separated value.
var addressRe = regexp.MustCompile(`^[^\s@,\[\]{}"'<>#]+@[^\s@,\[\]{}"'<>#]+$`)

// NormalizePerson returns how kardboard writes a person: a short name as it
// is, an address in lower case. ok is false for anything else.
func NormalizePerson(v string) (name string, ok bool) {
	v = strings.TrimSpace(v)
	switch {
	case slugRe.MatchString(v):
		return v, true
	case addressRe.MatchString(v):
		return strings.ToLower(v), true
	}
	return "", false
}

// IsAddress reports whether a person is given by an e-mail address.
func IsAddress(person string) bool { return strings.Contains(person, "@") }

// RepoRoot returns the top directory of the Git repository that dir lies
// in, or "" if there is none. It looks for .git upwards and runs no Git, so
// it works where Git is not installed.
func RepoRoot(dir string) string {
	for d := dir; ; {
		if _, err := os.Lstat(filepath.Join(d, ".git")); err == nil {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			return ""
		}
		d = parent
	}
}

// mailmap maps an address found in commits to the one the person goes by,
// as the repository's .mailmap says.
type mailmap map[string]string

var mailmapAddrRe = regexp.MustCompile(`<([^<>]*)>`)

// readMailmap reads the .mailmap at the top of the repository. Lines that
// name only one address change a name, not an address, and are left out.
func readMailmap(repo string) mailmap {
	m := mailmap{}
	if repo == "" {
		return m
	}
	f, err := os.Open(filepath.Join(repo, ".mailmap"))
	if err != nil {
		return m
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line, _, _ := strings.Cut(sc.Text(), "#")
		addrs := mailmapAddrRe.FindAllStringSubmatch(line, -1)
		if len(addrs) < 2 {
			continue
		}
		proper := strings.ToLower(strings.TrimSpace(addrs[0][1]))
		commit := strings.ToLower(strings.TrimSpace(addrs[1][1]))
		if proper != "" && commit != "" {
			m[commit] = proper
		}
	}
	return m
}

// Canonical returns the address a person goes by, following the .mailmap
// of the board's repository. A short name stays as it is.
func (b *Board) Canonical(person string) string {
	if !IsAddress(person) {
		return person
	}
	if b.mailmap == nil {
		b.mailmap = readMailmap(RepoRoot(b.Root))
	}
	p := strings.ToLower(person)
	if proper, ok := b.mailmap[p]; ok {
		return proper
	}
	return p
}

// SamePerson reports whether two names mean the same person: equal, or two
// addresses the .mailmap joins.
func (b *Board) SamePerson(x, y string) bool {
	return x == y || b.Canonical(x) == b.Canonical(y)
}

// HasPerson reports whether person is among people.
func (b *Board) HasPerson(people []string, person string) bool {
	return slices.ContainsFunc(people, func(p string) bool { return b.SamePerson(p, person) })
}

// Aliases returns person and every address the .mailmap joins with it, so
// a reader that cannot ask the board can still tell whose a card is.
func (b *Board) Aliases(person string) []string {
	out := []string{person}
	if !IsAddress(person) {
		return out
	}
	canon := b.Canonical(person)
	if !slices.Contains(out, canon) {
		out = append(out, canon)
	}
	for commit, proper := range b.mailmap {
		if proper == canon && !slices.Contains(out, commit) {
			out = append(out, commit)
		}
	}
	slices.Sort(out[1:])
	return out
}
