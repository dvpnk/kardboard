<h1>
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="output/branding/kardboard-wordmark-dark.svg">
    <img src="output/branding/kardboard-wordmark.svg" alt="kardboard" width="316">
  </picture>
</h1>

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

## Install

`kard` is a single file with everything inside, the UI too. It needs no Go
and no installer. Git is optional: `kard` uses it for history, `sync` and
your name from `git config`.

Each [release](https://github.com/dvpnk/kardboard/releases) has one file
per system:

| System | File |
| --- | --- |
| Mac, Apple silicon or Intel | `kard-macos` |
| Linux on a PC or server | `kard-linux-amd64` |
| Linux on ARM, such as a Raspberry Pi with a 64-bit system | `kard-linux-arm64` |
| Windows | `kard-windows-amd64.exe` |

The commands below always fetch the newest release. `kard version` says
which one you have; to update, download it again the same way.

### With a coding agent

Give Claude Code, Codex or another coding agent this request, in the
project that should get a board:

```text
Install kard and set up a board for this project, following
https://raw.githubusercontent.com/dvpnk/kardboard/main/INSTALL-AGENT.md
```

[INSTALL-AGENT.md](INSTALL-AGENT.md) takes the agent step by step through
download, checksum, path and board, and has it ask before it changes
anything outside the project. Read it first if you like: it is short and
runs no script.

### macOS and Linux

Download the file as `kard` into `~/.local/bin` and make it executable. On
Linux, write `linux-amd64` or `linux-arm64` instead of `macos`:

```bash
mkdir -p ~/.local/bin
curl -fL -o ~/.local/bin/kard https://github.com/dvpnk/kardboard/releases/latest/download/kard-macos
chmod +x ~/.local/bin/kard
```

Check that your shell finds it:

```bash
kard version
```

If it says `command not found`, `~/.local/bin` is not in your path yet. Add
it, then open a new terminal. On a Mac with zsh:

```bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc
```

With bash, the line goes into `~/.bashrc` instead.

The files are not signed. If you download one with the browser instead of
`curl`, macOS stops it on first launch with a warning. This removes the
mark the browser set:

```bash
xattr -d com.apple.quarantine ~/.local/bin/kard
```

### Windows

In PowerShell, this puts `kard.exe` into a folder of its own and adds the
folder to your path:

```powershell
$dir = "$env:LOCALAPPDATA\Programs\kard"
New-Item -ItemType Directory -Force $dir | Out-Null
Invoke-WebRequest "https://github.com/dvpnk/kardboard/releases/latest/download/kard-windows-amd64.exe" -OutFile "$dir\kard.exe"
[Environment]::SetEnvironmentVariable("Path", [Environment]::GetEnvironmentVariable("Path", "User") + ";$dir", "User")
```

Open a new terminal and check it with `kard version`. If you downloaded the
file with the browser, SmartScreen warns once: "More info", then "Run
anyway".

### Check the download

`SHA256SUMS` in the release lists a checksum for every file. Compare it
with the one of your file, on macOS and Linux with `shasum -a 256
~/.local/bin/kard`, on Windows with `Get-FileHash "$dir\kard.exe"`.

### With Go

With Go 1.27 or newer, this builds and installs it in one step:

```bash
go install github.com/dvpnk/kardboard/cmd/kard@latest
```

### First steps

In a project, create a board in a new folder:

```bash
kard init board
```

Or let `kard` find the boards in a folder of projects:

```bash
kard scan ~/Repos
```

Then open all boards in the browser:

```bash
kard ui
```

`kard --help` lists every command, and every command explains itself with
`--help`. If you prefer something shorter than `kard`, set up an alias
yourself, such as `alias kb=kard`. No short name is built in, because short
names easily clash with other tools.

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
program, and `kard version` shows it. A release is a tag: once it is
pushed, GitHub builds every system with `scripts/dist` and publishes the
files.

```bash
git tag "v$(scripts/calculate_version.sh)" && git push origin --tags
```

Try it on the example board:

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
export KARDBOARD_USER=arnold
```

`kard whoami` shows the name and where it comes from. `kard whoami --set
<name>` stores a name of your own for one project in `.kard/config.yaml`;
the folder ignores itself and is never committed. When assigning, the
beginning of a name or address the board knows, from its commits or its
cards, is enough: `kard assign <card> sly`.

```bash
kard new "Dark mode" -c backlog --set size=M --set tags=feature,ui
```

```bash
kard assign dark-mode arnold
```

```bash
kard take dark-mode
```

```bash
kard assign dark-mode sly --add
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

The best place for a board is usually next to the project, as a Git
repository of its own, such as `../myproject-board`: then it does not
change when you switch branches in the code, and a team can share it with
`sync: push`. A board inside the code repository follows the branches, see
[Cards and branches](#cards-and-branches).

In a new folder:

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

Next to the project, as a repository of its own:

```bash
mkdir ../myproject-board && cd ../myproject-board && git init
```

```bash
kard init . --sync commit
```

With `--sync commit` or `--sync push`, `board.yaml` becomes the board's first
commit, and every change after it is a commit too; see
[A board for a team](#a-board-for-a-team).

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
assign export-as-pdf: arnold
```

What was left behind, for instance without a network, is fetched and sent
by:

```bash
kard sync
```

With `sync: commit`, `kard` only commits and does not push. Git must not
ask for a password then; the key belongs in the agent or the keychain.

## Cards and branches

A board in the code repository, with `sync: none`, belongs to the branch
that is checked out, like the code. Take a card on your feature branch, mark
it done there, and it lands on main with the pull request. Until then main
still has it where it was, and so does everyone else.

kard says when that is the case: after a change on a branch other than the
main one, the line ends with `(on branch feature/pdf: main gets this when it
is merged)`, and the UI shows the branch next to the board's name.

If the team should see every move at once, whatever branch anyone is on,
give the board a home of its own with `sync: push`. The simplest is a
repository of its own, as in [A board for a team](#a-board-for-a-team).
If board and code should stay in one repository, the board can live on a
branch of its own, next to the project as a Git worktree; the
[spec](docs/spec.md#a-board-of-its-own) shows how to set it up.

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
