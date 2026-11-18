// Package people answers two questions about the people on a board: who
// the user is there, and whom a name given on the command line means.
//
// Neither needs any setup. Who one is comes from Git when nothing says
// otherwise, and the people a board knows are those who committed to it
// or stand on its cards.
package people

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/dvpnk/kardboard/internal/board"
	"github.com/dvpnk/kardboard/internal/userconfig"
)

// EnvUser overrides every other source of the user's name.
const EnvUser = userconfig.EnvUser

// ProjectDir is the directory in a project that holds settings for the one
// person who has the clone. It never gets committed: it ignores itself.
const ProjectDir = ".kard"

// gitTimeout bounds every Git command run here; all of them only read.
const gitTimeout = 5 * time.Second

// Source says where a name came from.
type Source string

const (
	FromEnv     Source = "env"     // KARDBOARD_USER
	FromProject Source = "project" // .kard/config.yaml
	FromUser    Source = "user"    // ~/.config/kardboard/config.yaml
	FromGit     Source = "git"     // git config user.email
	FromAccount Source = "account" // the operating system's account name
)

// Identity is who the user is on one board.
type Identity struct {
	Name   string `json:"name"`
	Source Source `json:"source"`
	// Where names the file or directory the name was read from, if any.
	Where string `json:"where,omitempty"`
}

// Describe says where the name came from, in words.
func (id Identity) Describe() string {
	switch id.Source {
	case FromEnv:
		return "from " + EnvUser
	case FromProject, FromUser:
		return "from `user` in " + userconfig.Abbrev(id.Where)
	case FromGit:
		return "from git config user.email in " + userconfig.Abbrev(id.Where)
	case FromAccount:
		return "from the operating system's account name"
	}
	return ""
}

// projectConfig is the content of .kard/config.yaml.
type projectConfig struct {
	User string `yaml:"user"`
}

// Me returns who the user is on the board in dir. It takes the first of:
// KARDBOARD_USER, the nearest .kard/config.yaml, `user` in the user's
// settings, git config user.email read in dir, the account name.
func Me(dir string) (Identity, error) {
	if name := os.Getenv(EnvUser); name != "" {
		return checked(Identity{Name: name, Source: FromEnv})
	}
	if path := projectFile(dir); path != "" {
		cfg, err := readProject(path)
		if err != nil {
			return Identity{}, err
		}
		if cfg.User != "" {
			return checked(Identity{Name: cfg.User, Source: FromProject, Where: path})
		}
	}
	cfg, err := userconfig.Load()
	if err != nil {
		return Identity{}, err
	}
	if cfg.User != "" {
		path, _ := userconfig.Path()
		return checked(Identity{Name: cfg.User, Source: FromUser, Where: path})
	}
	if addr := gitAddress(dir); addr != "" {
		return Identity{Name: addr, Source: FromGit, Where: dir}, nil
	}
	if name, ok := accountName(); ok {
		return Identity{Name: name, Source: FromAccount}, nil
	}
	path, _ := userconfig.Path()
	return Identity{}, fmt.Errorf("kard does not know who you are: set git config user.email, add `user: <name>` to %s, or set %s",
		userconfig.Abbrev(path), EnvUser)
}

func checked(id Identity) (Identity, error) {
	name, ok := board.NormalizePerson(id.Name)
	if !ok {
		where := EnvUser
		if id.Where != "" {
			where = userconfig.Abbrev(id.Where)
		}
		return Identity{}, fmt.Errorf("%s: user %q is neither a short name (a-z, 0-9, -) nor an e-mail address", where, id.Name)
	}
	id.Name = name
	return id, nil
}

// searchDirs returns the directories .kard is looked for in: from dir up to
// the top of its repository, or dir alone outside a repository.
func searchDirs(dir string) []string {
	dir, _ = filepath.Abs(dir)
	top := board.RepoRoot(dir)
	if top == "" {
		return []string{dir}
	}
	var dirs []string
	for d := dir; ; d = filepath.Dir(d) {
		dirs = append(dirs, d)
		if d == top || filepath.Dir(d) == d {
			return dirs
		}
	}
}

// projectFile returns the nearest .kard/config.yaml for dir, or "".
func projectFile(dir string) string {
	for _, d := range searchDirs(dir) {
		path := filepath.Join(d, ProjectDir, "config.yaml")
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func readProject(path string) (*projectConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := &projectConfig{}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(cfg); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%s: %w", userconfig.Abbrev(path), err)
	}
	return cfg, nil
}

// SetProject writes name as the user in .kard/config.yaml at the top of the
// repository dir lies in, or in dir outside a repository. It creates .kard
// with a .gitignore that keeps the whole directory out of Git, and returns
// the path of the file it wrote.
func SetProject(dir, name string) (string, error) {
	norm, ok := board.NormalizePerson(name)
	if !ok {
		return "", fmt.Errorf("%q is neither a short name (a-z, 0-9, -) nor an e-mail address", name)
	}
	dir, _ = filepath.Abs(dir)
	base := board.RepoRoot(dir)
	if base == "" {
		base = dir
	}
	kard := filepath.Join(base, ProjectDir)
	if err := os.MkdirAll(kard, 0o755); err != nil {
		return "", err
	}
	ignore := filepath.Join(kard, ".gitignore")
	if _, err := os.Stat(ignore); errors.Is(err, fs.ErrNotExist) {
		text := "# Settings for the person who has this clone. Never committed.\n*\n"
		if err := os.WriteFile(ignore, []byte(text), 0o644); err != nil {
			return "", err
		}
	}
	path := filepath.Join(kard, "config.yaml")
	if err := os.WriteFile(path, []byte("user: "+norm+"\n"), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// gitAddress returns git config user.email as Git sees it in dir, in lower
// case, or "" if Git is missing, has no address or one kard cannot use.
func gitAddress(dir string) string {
	out, err := git(context.Background(), dir, "config", "--get", "user.email")
	if err != nil {
		return ""
	}
	addr := strings.TrimSpace(out)
	if name, ok := board.NormalizePerson(addr); ok && board.IsAddress(name) {
		return name
	}
	return ""
}

// LookupAccount finds the operating system's account. Tests replace it to
// act as a machine where it says nothing.
var LookupAccount = user.Current

// accountName returns the operating system's account name as a short name:
// lower case, on Windows without the domain.
func accountName() (string, bool) {
	u, err := LookupAccount()
	if err != nil {
		return "", false
	}
	name := u.Username
	if i := strings.LastIndex(name, `\`); i >= 0 {
		name = name[i+1:]
	}
	name = strings.ToLower(name)
	if name, ok := board.NormalizePerson(name); ok && !board.IsAddress(name) {
		return name, true
	}
	return "", false
}

// git runs one read-only Git command in dir.
func git(ctx context.Context, dir string, args ...string) (string, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, gitTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir, "-c", "core.quotePath=false"}, args...)...)
	cmd.Env = append(cmd.Environ(), "GIT_OPTIONAL_LOCKS=0", "LC_ALL=C")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return stdout.String(), nil
}

// TrackedProject returns a problem for every file in a .kard directory of
// the board's project that Git tracks: those settings are one person's and
// must not reach the others.
func TrackedProject(ctx context.Context, b *board.Board) []board.Problem {
	var problems []board.Problem
	for _, d := range searchDirs(b.Root) {
		kard := filepath.Join(d, ProjectDir)
		if info, err := os.Stat(kard); err != nil || !info.IsDir() {
			continue
		}
		out, err := git(ctx, d, "ls-files", "--", ProjectDir)
		if err != nil {
			continue
		}
		for _, file := range strings.Fields(out) {
			rel, err := filepath.Rel(b.Root, filepath.Join(d, file))
			if err != nil {
				rel = file
			}
			problems = append(problems, board.Problem{
				Code: "E11", Severity: board.SeverityError, Path: filepath.ToSlash(rel),
				Message: "is tracked by Git, but .kard holds one person's settings; git rm --cached it",
			})
		}
	}
	return problems
}

// ── Who a name means ─────────────────────────────────────────────────────────

// Person is someone a board knows.
type Person struct {
	// ID is how the person stands on cards: an address or a short name.
	ID string `json:"id"`
	// Name is the person's name in Git, if they ever committed.
	Name string `json:"name,omitempty"`
}

// Known returns the people of a board: the authors of the commits that
// touched it, as Git names them after the .mailmap, and everyone on its
// cards or in the values of its person fields. Two addresses the .mailmap
// joins are one person, under the address it prefers.
func Known(ctx context.Context, b *board.Board) []Person {
	return With(b, Authors(ctx, b.Root))
}

// Authors returns the authors of the commits that touched the board in
// dir, newest first, with the names and addresses Git gives them after the
// .mailmap. Outside a repository, or without Git, there are none.
func Authors(ctx context.Context, dir string) []Person {
	var out []Person
	log, err := git(ctx, dir, "log", "--max-count=5000", "--format=%aE%x1f%aN", "--", ".")
	if err != nil {
		return nil
	}
	for _, line := range strings.Split(log, "\n") {
		if addr, name, ok := strings.Cut(line, "\x1f"); ok {
			out = append(out, Person{ID: addr, Name: strings.TrimSpace(name)})
		}
	}
	return out
}

// With returns authors together with the people on the board's cards and
// in the values of its person fields, each once, sorted.
func With(b *board.Board, authors []Person) []Person {
	var out []Person
	add := func(id, name string) {
		norm, ok := board.NormalizePerson(id)
		if !ok {
			return
		}
		norm = b.Canonical(norm)
		i := slices.IndexFunc(out, func(p Person) bool { return p.ID == norm })
		switch {
		case i < 0:
			out = append(out, Person{ID: norm, Name: name})
		case out[i].Name == "":
			out[i].Name = name
		}
	}
	for _, p := range authors {
		add(p.ID, p.Name)
	}
	if b.Config != nil {
		for _, f := range b.Config.AllFields() {
			if f.Type != board.TypePerson {
				continue
			}
			for _, v := range f.Values {
				add(v, "")
			}
			for _, c := range b.Cards {
				names, _ := c.Fields[f.Name].([]string)
				for _, n := range names {
					add(n, "")
				}
			}
		}
	}
	slices.SortFunc(out, func(x, y Person) int { return strings.Compare(x.ID, y.ID) })
	if out == nil {
		out = []Person{}
	}
	return out
}

// AmbiguousError says that a name fits more than one person.
type AmbiguousError struct {
	Name       string
	Candidates []Person
}

func (e *AmbiguousError) Error() string {
	ids := make([]string, len(e.Candidates))
	for i, p := range e.Candidates {
		ids[i] = p.ID
		if p.Name != "" {
			ids[i] = fmt.Sprintf("%s (%s)", p.ID, p.Name)
		}
	}
	return fmt.Sprintf("%q fits %d people: %s", e.Name, len(e.Candidates), strings.Join(ids, ", "))
}

// Resolve returns whom name means among the known people:
//
//  1. a known short name or address, exactly;
//  2. else the one person whose address, short name or name in Git begins
//     with name, ignoring case and accents;
//  3. several such people are an AmbiguousError;
//  4. nobody: a valid short name or address as it stands, a new person.
func Resolve(name string, known []Person) (string, error) {
	name = strings.TrimSpace(name)
	if norm, ok := board.NormalizePerson(name); ok {
		for _, p := range known {
			if p.ID == norm {
				return norm, nil
			}
		}
	}
	key := fold(name)
	var hits []Person
	if key != "" {
		for _, p := range known {
			if matches(p, key) {
				hits = append(hits, p)
			}
		}
	}
	switch {
	case len(hits) == 1:
		return hits[0].ID, nil
	case len(hits) > 1:
		return "", &AmbiguousError{Name: name, Candidates: hits}
	}
	if norm, ok := board.NormalizePerson(name); ok {
		return norm, nil
	}
	return "", fmt.Errorf("%q is neither a short name (a-z, 0-9, -) nor an e-mail address, nor the beginning of a person's name on this board", name)
}

func matches(p Person, key string) bool {
	if strings.HasPrefix(fold(p.ID), key) {
		return true
	}
	name := fold(p.Name)
	if name == "" {
		return false
	}
	if strings.HasPrefix(name, key) {
		return true
	}
	for _, word := range strings.FieldsFunc(name, func(r rune) bool { return r == ' ' || r == '-' || r == '.' }) {
		if strings.HasPrefix(word, key) {
			return true
		}
	}
	return false
}

// accents maps letters to what they are compared as, so "angelique" finds
// Angélique and "muller" finds Müller.
var accents = strings.NewReplacer(
	"ä", "a", "ö", "o", "ü", "u", "ß", "ss",
	"á", "a", "à", "a", "â", "a", "å", "a", "ã", "a",
	"é", "e", "è", "e", "ê", "e", "ë", "e",
	"í", "i", "ì", "i", "î", "i", "ï", "i",
	"ó", "o", "ò", "o", "ô", "o", "õ", "o", "ø", "o",
	"ú", "u", "ù", "u", "û", "u",
	"ç", "c", "ñ", "n", "ý", "y", "ÿ", "y",
)

func fold(s string) string {
	return accents.Replace(strings.ToLower(strings.TrimSpace(s)))
}

// ResolveAll resolves every name in names; a name may hold several,
// separated by commas.
func ResolveAll(names []string, known []Person) ([]string, error) {
	var out []string
	for _, arg := range names {
		for _, n := range strings.Split(arg, ",") {
			if n = strings.TrimSpace(n); n == "" {
				continue
			}
			id, err := Resolve(n, known)
			if err != nil {
				return nil, err
			}
			out = append(out, id)
		}
	}
	return out, nil
}

// ResolveSet resolves the people in "field=value" items for fields of type
// person, so --set assignee=sly works like kard assign. Other items stay
// as they are, and so does an empty value, which removes the field.
func ResolveSet(cfg *board.Config, set []string, known []Person) ([]string, error) {
	out := slices.Clone(set)
	for i, item := range out {
		name, value, ok := strings.Cut(item, "=")
		if !ok || strings.TrimSpace(value) == "" {
			continue
		}
		f, ok := cfg.Field(strings.TrimSpace(name))
		if !ok || f.Type != board.TypePerson {
			continue
		}
		ids, err := ResolveAll([]string{strings.Trim(strings.TrimSpace(value), "[]")}, known)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.Name, err)
		}
		out[i] = name + "=" + strings.Join(ids, ",")
	}
	return out, nil
}
