# kardboard

A board for cards that live as Markdown files in your project. The program
runs once on your machine, finds the boards in your projects and shows them
one by one or together.

**Status:** early draft. The format is a draft. The `kard` command can create,
read and check boards, create, move and assign cards, and show every board on
the machine in the browser. There, cards can also be dragged, taken and
assigned. With `sync: commit` or `sync: push` in `board.yaml`, `kard` turns
every change into a commit and syncs the board with its remote.

- [Visual overview](docs/overview.html), to open in a browser
- [Format and CLI](docs/spec.md)
- [Example board](examples/board)

## Build and try it

Needs Go 1.27 or newer.

```bash
go build -o bin/kard ./cmd/kard
```

For other machines, `scripts/dist` builds every system at once into `dist/`:
macOS (Apple silicon, Intel and one file for both), Linux and Windows, plus
`SHA256SUMS`. Each file is the whole program including the UI; the target
machine needs no Go and no installation, only a place in the path, such as
`~/.local/bin`. `kard` needs Git only for history, `sync` and the name from
`git config`.

```bash
scripts/dist
```

Only some systems: each word picks the systems whose name contains it, and
`mac` stands for macOS.

```bash
scripts/dist mac windows
```

`scripts/calculate_version.sh` works out the version from the commits since
the last `vX.Y.Z` tag, following Conventional Commits: `feat:` raises the
second number, `fix:`, `refactor:` and `build:` the third, `feat!:` the
first; `docs:` and `chore:` do not count. `scripts/dist` passes it to the
program, and `kard version` shows it. When you hand out a build, fix the
version as a tag:

```bash
git tag "v$(scripts/calculate_version.sh)" && scripts/dist
```

The files are not signed. When one reaches a Mac through a browser or a
chat, Gatekeeper stops it on first launch; allowing it once is enough.
Windows warns once through SmartScreen.

```bash
xattr -d com.apple.quarantine ~/.local/bin/kard
```

The command is called `kard`. If you prefer something shorter, set up an
alias yourself, such as `alias kb=kard`. No short name is built in, because
short names easily clash with other tools.

```bash
./bin/kard lint -C examples/board
```

```bash
./bin/kard list -C examples/board
```

```bash
./bin/kard show export-as-pdf -C examples/board
```

When a board lives in a Git repository, `kard log` shows which commits
changed it, and with a card only that card's history:

```bash
./bin/kard log export-as-pdf -C examples/board
```

Without `-C`, `kard` looks for the board from the current folder. `--json`
prints JSON.

Create and change cards, best in a copy of the example. For this, `kard`
must be in your path.

```bash
cp -R examples/board /tmp/kard-demo && cd /tmp/kard-demo
```

`kard` usually knows who you are already: the address from
`git config user.email`, in each repository the one that applies there. The
example's cards carry short names, so set the name for the demo yourself:

```bash
export KARDBOARD_USER=alex
```

`kard whoami` shows the name and where it comes from. `kard whoami --set
<name>` stores a name of your own for one project in `.kard/config.yaml`;
the folder ignores itself and is never committed. When assigning, the
beginning of a name or address the board knows, from its commits or its
cards, is enough: `kard assign <card> sam`.

```bash
kard new "Dark mode" -c backlog --set size=M --set tags=feature,ui
```

```bash
kard assign dark-mode alex
```

```bash
kard take dark-mode
```

```bash
kard assign dark-mode sam --add
```

```bash
kard done dark-mode
```

You change a card's text in any editor. `kard edit` opens it in `$VISUAL` or
`$EDITOR` and saves it afterwards like a `kard set`: with the same rules,
and on boards with `sync: commit` or `push` as one commit.

```bash
kard edit dark-mode
```

`kard rename` gives a card a new file name and updates the references in
the board:

```bash
kard rename dark-mode dark-theme
```

```bash
kard list --mine
```

## Create your own board

In the project, in a new folder:

```bash
kard init board
```

This writes a `board.yaml` with the columns `backlog`, `working_on` and
`done` and registers the board on the machine. If there are folders with
Markdown files already, they become the columns; no file is changed:

```bash
kard init ~/Repos/notes
```

`kard lint` then shows what about the existing cards does not fit the format
yet.

So that an editor or another tool can check the cards' header too,
`kard schema` prints a JSON Schema from `board.yaml`. With a column, such as
`kard schema working_on`, it also requires that column's required fields:

```bash
kard schema > card.schema.json
```

## All boards in the browser

Register a board, or have a folder of projects searched right away:

```bash
kard add examples/board
```

```bash
kard scan ~/Repos
```

```bash
kard boards
```

`--all` checks or lists all these boards at once, for instance your own
cards across every project:

```bash
kard lint --all
```

```bash
kard list --mine --all
```

Then start the UI. It opens in the browser, follows changes to the files and
runs until you stop it with Ctrl+C:

```bash
kard ui
```

Cards can be dragged into another column. A click on a card opens its text
and offers take, done, move and assign. The pencil changes title, fields and
text in one step, with a preview. Each of these actions is the command of
the same name, with the same rules.

When a board lives in a Git repository, "History" at the top right shows the
commits that touched it, and every card its own history. This needs `git`
to be installed.

The server can only be reached from your own machine and accepts changes
only from its own page.

## A board for a team

When the board lives in a Git repository of its own that several people
clone, its `board.yaml` says:

```yaml
sync: push
```

Then every changing command first fetches the others' work, makes exactly
one commit with that one card and pushes it. If two people move the same
card at the same time, whoever pushes first wins; the other is told where
the card is now. `git log` is the board's record:

```
move export-as-pdf: backlog -> working_on
assign export-as-pdf: alex
```

What was left behind, for instance without a network, is fetched and sent
by:

```bash
kard sync
```

With `sync: commit`, `kard` only commits and does not push. Git must not
ask for a password then; the key belongs in the agent or the keychain.

## For scripts and AI agents

`kard` needs no one at the keyboard. Every command explains itself with
`--help`, including examples, and prints JSON with `--json`, errors too: as
an object on stderr, with the reason, the kind and whether `--force` helps.
The exit code is `0` on success, `1` when a rule refuses, `2` for a wrong
call.

Whoever changes a card does so through `kard`, not in the file: then the
board's rules apply, and on boards with Git the right commit is made. Text
an editor would otherwise provide comes from a file or from stdin:

```bash
echo "What needs doing." | kard new "Check login" -c backlog --set size=S --text -
```

To change a card, read it with its fingerprint and pass the fingerprint
along when saving. If someone else has changed the card in the meantime,
nothing is overwritten:

```bash
kard show check-login --json   # body, fields and sum
```

```bash
kard edit check-login --file new.md --base <sum>
```

`kard schema` describes which fields and values are allowed, as a JSON
Schema. `kard whoami --json` says who you are; `KARDBOARD_USER` sets it for
one call. When a name fits several people, the error is `ambiguous` and
lists them under `candidates`.

## License

kardboard is under the [MIT License](LICENSE). The third-party components
below are under their own licenses.

## Third-party components

The UI uses the font [JetBrains Mono](https://github.com/JetBrains/JetBrainsMono),
Copyright 2020 The JetBrains Mono Project Authors, under the SIL Open Font
License 1.1. The font files and the license text are in
[internal/server/web/fonts](internal/server/web/fonts) and are embedded in
the program. The license applies to the font only.

The UI is built with [Preact](https://preactjs.com) 11.0.1 (MIT License,
Copyright 2015-present Jason Miller) and [htm](https://github.com/developit/htm)
3.1.1 (Apache License 2.0). Both are in
[internal/server/web/vendor](internal/server/web/vendor) as ready-made
files, with their license texts, origin and checksums. There is no npm and
no build step; a test checks that the files do not change unnoticed.

## Tests

```bash
go test ./...
```
