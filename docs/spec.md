# kardboard: format and CLI

**Status:** Draft v0, open for discussion. Keys, field names and commands are
English and stay that way.

kardboard shows cards as a board. The cards are Markdown files in a directory.
This document defines what the directory and the files look like and which
commands exist. How the program is built is not described here.

## Principles

1. **A board is a directory with a `board.yaml`.** Where the directory lives
   does not matter: in the project, next to it or as a repo of its own.
2. **The directory is the column.** Which column a card is in follows from
   its path, never from the content of the file. So moving a card changes no
   line of text.
3. **Everything else about the card is in its header**, including whose card it
   is. The header only holds what the board filters, sorts or checks. Anything
   beyond that is text and stays free.
4. **Every card can be read and changed without kardboard.** An editor, `ls`,
   `grep` and `git` are always enough.
5. **Start without rebuilding.** A directory with column directories and
   Markdown files becomes a valid board by adding a `board.yaml`. Fields come
   in when you need them.

## Directory

```
board/                         any name
  board.yaml                   makes the directory a board
  planning/                    column
    offline-mode.md           card
  backlog/
    export-as-pdf.md
  working_on/
    login-fails-with-umlauts.md
  done/
  reference/                   not a column: ignored
```

- Columns are the directories that `board.yaml` names under `columns`.
  kardboard ignores all other directories and files in the board. That is
  where attachments can go.
- A card is a `.md` file directly in a column directory.
- Subdirectories in a column are ignored (see open questions).
- Hidden files (`.gitkeep`, `.DS_Store`) are ignored.
- Empty column directories may be missing. Git does not know empty
  directories.

## `board.yaml`

```yaml
kardboard: 1
id: demo
name: Demo project
sync: none

columns:
  - id: planning
    title: Planning
  - id: backlog
    title: Backlog
  - id: working_on
    title: In progress
    doing: true
    limit: 3
  - id: done
    title: Done
    done: true

fields:
  assignee:
    type: person
    required: [working_on]
  size:
    title: Size
    type: enum
    values: [XS, S, M, L, XL]
    required: [backlog, working_on]
  milestone:
    title: Milestone
    type: enum
    values: [pilot, release, later]
  tags:
    type: list
    values: [bug, feature, cleanup, security, ui]
```

| Key | Required | Meaning |
|---|---|---|
| `kardboard` | yes | Version of the format, currently `1` |
| `id` | yes | Short name of the board, `[a-z0-9][a-z0-9-]*`. Prefix for references across boards |
| `name` | no | Display name, otherwise the `id` |
| `sync` | no | `none` (default), `commit` or `push`, see [Git](#git) |
| `columns` | yes | Columns in display order, at least one |
| `fields` | no | Fields that cards may carry in their header |

Keys of a column:

| Key | Required | Meaning |
|---|---|---|
| `id` | yes | Directory name, `[a-z0-9][a-z0-9_-]*` |
| `title` | no | Display name, otherwise the `id` |
| `doing` | no | `true`: This is where the work in progress is. Target of `kard take` |
| `done` | no | `true`: Cards here count as done. Target of `kard done` |
| `limit` | no | Maximum number of cards in the column. Going over it is a warning |

Unknown keys in `board.yaml` are an error.

## Card

```markdown
---
assignee: sam
size: M
milestone: release
tags: [feature]
due: 2026-12-01
depends_on: [print-templates]
---

# Export as PDF

Free text in Markdown.
```

- **File name:** `<short-name>.md`, short name from `[a-z0-9][a-z0-9-]*`, at
  most 80 characters.
- **The file name is the key of the card.** It is unique in the whole board,
  across all columns. Renaming means a new card; `kard rename` updates the
  references.
- **Head:** YAML between two `---` lines at the very start of the file. The
  header may be missing as long as the column requires no field.
- **Title:** the first non-empty line after the header, in the form `# Title`.
  The title is only there, not in the header.
- **Text:** everything after that, free.
- **Encoding:** UTF-8, LF line endings.

### Fields in the header

Allowed are the built-in fields and the fields declared in `board.yaml` under
`fields`. Any other key is an error. This way a typo stands out instead of
silently becoming a new field.

Built in, usable without a declaration:

| Field | Type | Meaning |
|---|---|---|
| `assignee` | `person` | whose card it is, one person or several |
| `tags` | `list` | tags |
| `due` | `date` | due date |
| `depends_on` | `cards` | cards that must be finished first |
| `created` | `date` | creation date, set by `kard new` |

A built-in field may be narrowed under `fields`: `tags` with fixed `values`,
`assignee` as required in a column. The type stays the same.

`status`, `column`, `id` and `title` are forbidden in the header. This
information is in the path, the file name or the heading, and two sources for
the same information drift apart.

### Who a card belongs to

`assignee` names one person by their short name or email address, or several
as a list (see [People](#people)):

```yaml
assignee: sam
```

```yaml
assignee: [alex, sam]
```

The field applies in every column: A card in the backlog can already be set
aside for someone, and a done card keeps who did it. Without the field, the
card belongs to nobody. With several names, it belongs to each of them
equally.

An empty list and a duplicate name are errors. kardboard writes a single name
without brackets and several names as a list on one line.

Whether a column requires a person is up to the board itself, with the usual
rule for required fields:

```yaml
fields:
  assignee:
    type: person
    required: [working_on]
```

`values` fixes the list of people. A typo in a name is then an error.

### Field types

| Type | Value | Example |
|---|---|---|
| `text` | one line of text | `owner: Purchasing` |
| `enum` | exactly one of the `values` | `size: M` |
| `list` | list of texts, with `values` only those | `tags: [bug, ui]` |
| `date` | `YYYY-MM-DD` | `due: 2026-12-01` |
| `number` | number | `estimate: 3` |
| `bool` | `true` or `false` | `blocked: true` |
| `person` | short name of a person, `[a-z0-9][a-z0-9-]*`, or their email address, or a list of these; with `values` only those | `reviewer: sam` |
| `cards` | list of card references | `depends_on: [print-templates]` |

Keys of a field declaration:

| Key | Meaning |
|---|---|
| `type` | one of the types above, required |
| `title` | display name, otherwise the field name |
| `values` | allowed values, required for `enum`, possible for `list` and `person` |
| `required` | `true`: required in every column. List of columns: required only there |

Field names consist of `[a-z][a-z0-9_]*`.

### Card references

`print-templates` means a card in the same board. `atlas/print-templates` means a
card in the board with the `id` `atlas`. References in the same board must
resolve. kardboard checks references into another board only if that board is
registered on the machine, and reports a warning otherwise.

## Lint rules

`kard lint` checks a board and exits with code 1 if there is an error. This
makes it fit for a pre-commit hook and for CI.

Errors:

| No. | Rule |
|---|---|
| E01 | `board.yaml` is missing, is not valid YAML or violates this document |
| E02 | The header of a card is not valid YAML or not a mapping |
| E03 | Unknown field in the header |
| E04 | Forbidden field in the header (`status`, `column`, `id`, `title`) |
| E05 | Value does not match the type or is not in `values` |
| E06 | A field required by the card's column is missing |
| E07 | Title line `# …` is missing or does not come first |
| E08 | File name is not a valid short name |
| E09 | The same file name occurs more than once in the board |
| E10 | Reference to a card that does not exist in the board |
| E11 | Git tracks a file from `.kard/`, see [People](#people) |

Warnings:

| No. | Rule |
|---|---|
| W01 | Reference into a board that is not registered |
| W02 | `limit` of a column exceeded |

A faulty card is shown everywhere and marked as faulty, never hidden.

From `board.yaml`, `kard schema` generates a JSON Schema (draft 2020-12) for
the card header. With it, editors and other tools check what `kard` checks:
which fields exist (E03, E04), their types and values (E05), with the same
patterns for short names, references and dates. Whether a referenced card
exists (E10) is not in the header and stays with `kard lint`.

Which fields are required depends on the column, and the column is not in the
header. So `kard schema` only requires the fields that are required everywhere.
`kard schema <column>` also requires those of that column (E06); you then give
each column directory its own schema.

## Git

`sync` in `board.yaml` defines what a changing command does with Git.

| Value | Behavior | Meant for |
|---|---|---|
| `none` | only changes files | board in the code repo |
| `commit` | one commit per action, with only this card | own board repo, alone |
| `push` | pull, one commit, push; on rejection retry once | own board repo, in a team |

For `commit` and `push` the board must be in a Git repository, and for `push`
the branch also needs an upstream. If any of this is missing, `kard` refuses
every change and says what is missing.

**One commit per action.** It contains exactly the one card, even if other
things in the repository are changed or staged; those stay untouched. Only
`kard rename` adds the cards whose references it updates. Where there is
nothing to change, no commit is made. If the commit fails, for example
because of a hook, the card is left as it was.

**`kard` does not change cards with uncommitted changes.** Otherwise they
would end up in the action's commit and be lost in a conflict with it. Commit
first, then move. This also applies to cards that Git does not know yet.

Commit messages have a fixed form, so that `git log` is the board's record:

```
move export-as-pdf: backlog -> working_on
assign export-as-pdf: alex, sam
unassign export-as-pdf
edit export-as-pdf
new export-as-pdf
rename print-templates -> pdf-templates
```

`edit` stands for any change to fields other than `assignee` and to the text. If
an action does several things, such as `kard take` (move and assign), the
first is in the subject line and the rest below it.

With `push`, every action runs like this:

1. `git pull --rebase --autostash`. So the action sees what the others have
   done in the meantime. If the remote is not reachable, nothing happens.
2. Check the rules, change the card, one commit.
3. `git push`. If the remote rejects it because someone was faster, `kard`
   pulls the new state and tries again exactly once.
4. If someone has meanwhile moved the same card or changed the same line,
   `kard` takes back its own commit, takes over the other state and reports
   where the card is now. Git merges changes at different places of the same
   card, such as a move here and an assignment there.

If the commit succeeds but the push does not, the change stays as a commit on
the machine. `kard` reports this and exits with code 1; `kard sync` does the
push later.

Git never asks for a password in this. Credentials must be in the agent or in
the keychain.

### Board in the code repository

With `sync: none` the board is part of the working tree, like the code. Its
state is the state of the checked-out branch, and kardboard always shows the
working tree of the registered directory. A card moved on a feature branch
is moved only there; after `git switch main` it is back where main has it,
and main sees the move when the branch is merged. This is meant: a card goes
to `done` in the same pull request as the code that finishes it.

So that this does not surprise anyone, kard says when a board is seen on a
branch other than the repository's main one:

- After a changing command, the line ends with
  `(on branch feature/pdf: main gets this when it is merged)`; with `--json`
  the result has `branch` with `name` and `main`. With no branch checked out,
  only a commit, `detached` is true.
- The UI shows the branch next to the board's name.

The main branch is the one `origin/HEAD` points at, else Git's
`init.defaultBranch`, `main` or `master`, as far as such a branch exists. If
kard cannot tell, it says nothing. Boards with `sync: commit` or `push` are
not marked: they live on the branch they are meant to.

If two branches move the same card to different columns, Git reports a
conflict when merging, as for any file moved to two places. Whoever merges
keeps one of the two files.

### Board on a branch of its own

When the team should see the board's state at once, whichever branch anyone
works on, the board gets a branch of its own without the code, checked out
next to the project as a Git worktree, with `sync: push`:

```bash
git worktree add --orphan -b kardboard ../myproject-board
cd ../myproject-board && kard init .
```

Set `sync: push` in `board.yaml`, then:

```bash
git add -A && git commit -m "new board" && git push -u origin kardboard
```

`kard init` has registered the directory, so `kard ui` shows it. Others get
it with `git worktree add ../myproject-board kardboard`. Switching branches
in the project leaves the board alone. In exchange, a card can no longer
change in the same pull request as the code. A repository of its own for the
board works the same way.

## User settings

`~/.config/kardboard/config.yaml`, per machine and person:

```yaml
user: alex
boards:
  - ~/Repos/demo/board
  - ~/Repos/atlas-board
scan:
  - ~/Repos/Private
hidden:
  - atlas
```

| Key | Meaning |
|---|---|
| `user` | your own short name or address for all boards. Without an entry the name comes from Git, see [People](#people) |
| `boards` | boards registered one by one |
| `scan` | directories in which kardboard looks for `board.yaml` |
| `hidden` | boards hidden in the overall view |

`KARDBOARD_USER` and a `.kard/config.yaml` in the project take precedence over
the `user` entry, see [People](#people).

A board is known on the machine if it is listed under `boards` or lies below a
directory from `scan`. The search goes at most six levels deep and skips
hidden directories, `node_modules`, `vendor` and the inside of boards. Two
known boards with the same `id` are an error: The first keeps the `id`, the
others are reported as unusable.

`kard add`, `kard remove` and `kard scan` change this file. Comments and the
order of the entries are kept, blank lines are not.

## CLI

The command is called `kard`. Without further arguments it works on the board
you are in: kardboard looks for a `board.yaml` from the current directory
upwards and then in the subdirectories of the Git repo. If there are several
boards, you have to pick one. `-C <dir>` names the board directory directly,
`-b <id>` picks a registered board. With `--all`, `kard lint` and `kard list`
take all boards that `kard boards` shows, plus the one you are in. They show
them one after the other, each under its name; `kard lint --all` adds up at
the end and exits with code 1 if there is an error anywhere or a board cannot
be read. `kard list <column> --all` skips boards without that column. `--all`
does not go together with `-C` or `-b`.

Board:

| Command | Effect |
|---|---|
| `kard init [dir] [--id id] [--name name] [--sync mode] [--columns a,b,c] [--no-add]` | turns a directory into a board, see below |
| `kard lint [--all]` | checks the board, with `--all` all known boards |
| `kard schema [column]` | prints the JSON Schema for the card header, with a column including its required fields |
| `kard ui [--port n] [--no-open]` | starts the local server and shows all known boards in the browser, see [UI](#ui) |

Reading cards:

| Command | Effect |
|---|---|
| `kard list [column] [--mine] [--all]` | lists cards, with `--mine` only your own, from every column, with `--all` from all known boards |
| `kard show <card>` | shows one card |
| `kard log [card] [-n number]` | shows which commits changed the board or one card, newest first, 20 by default |

Changing cards:

| Command | Column | Person | Effect |
|---|---|---|---|
| `kard new "<title>" [-c column] [--set field=value] [--id id] [--text file]` | | | creates a card, with `--text` including its text |
| `kard move <card> <column>` | changes | stays | moves the card |
| `kard take <card> [--add]` | goes to the first column with `doing` | becomes you, with `--add` you join | starts work on the card |
| `kard done <card>` | goes to the first column with `done` | stays | finishes the card |
| `kard assign <card> <person>… [--add]` | stays | is replaced, with `--add` added to | assigns the card. `kard give` is the same |
| `kard unassign <card> [person…]` | stays | is removed, with names only those | makes the card nobody's |
| `kard set <card> field=value …` | stays | stays | changes fields in the header, `field=` removes one |
| `kard edit <card> [--file file] [--base sum]` | stays | as changed in the editor | opens the card in the editor, with `--file` without one, and saves it as one change, see below |
| `kard rename <card> <new>` | stays | stays | renames and updates references, see below |

Machine:

| Command | Effect |
|---|---|
| `kard boards` | lists registered and found boards |
| `kard add [dir]` | registers a board. The directory is the board or lies in its project; without it, the current one applies |
| `kard remove <id\|dir>` | unregisters a board, deletes nothing |
| `kard scan <dir>` | adds a directory to the search, `--remove` takes it out again |
| `kard sync` | pulls what is new on the remote and pushes what is waiting here. Only for boards with `sync: push` |
| `kard whoami [--all] [--set name]` | says who you are on the board and where the name comes from, with `--set` sets it for the project, see [People](#people) |
| `kard version` | says which version this is: the Git tag `kard` was built from, otherwise commit and date, plus the system. `kard --version` is the same |

On `kard edit`:

- **The editor gets a copy.** `kard edit` uses `$VISUAL`, otherwise
  `$EDITOR`, otherwise `vi`. Only when the editor is closed does the copy
  replace the card, in one step. If the copy is unchanged, nothing happens.
- **The same rules as `kard set`.** Whatever is different in the header counts
  as set or removed: A card assigned to someone else needs `--force`, a
  required field must not go away. On boards with `sync: commit` or `push`
  this makes a commit, `edit <card>` or `assign …` if `assignee` changed. With
  `push`, `kard` first pulls the others' state.
- **Nothing gets lost.** `kard edit` does not even open a card with
  uncommitted changes. If the copy has errors the card did not have (E02 to
  E05, E07), or if the card has changed in the meantime, the card stays as it
  was, and `kard` names the location of the copy.

On `kard rename`:

- **Only the file name changes.** The card stays in its column, its content
  stays as it was. The new name must be a valid short name that does not yet
  exist in the board.
- **References follow.** Every field of type `cards` in the same board that
  names the card, whether as `print-templates` or as `<board>/print-templates`,
  names the new name afterwards. Only that line of the header is changed. This
  also applies to cards of other people, because their content does not
  change. References from other boards and mentions in the text stay as they
  are; `kard lint` shows what points nowhere afterwards.
- **A card assigned to someone else needs `--force`,** as with any other
  change.
- **One commit.** On boards with `sync: commit` or `push` it contains the card
  and every card whose reference was updated. None of them may have
  uncommitted changes.

On `kard init`:

- **It writes the `board.yaml` and rewrites nothing else.** No existing file
  is changed or moved. An existing `board.yaml` is never overwritten. Without
  an argument the current directory applies; if the named directory does not
  exist, it is created.
- **Existing directories become the columns.** Every subdirectory that holds
  Markdown files, directly or one level deeper, becomes a column if its name
  works as a column short name. Known names go in the usual order
  (`planning`, `backlog`, …, `working_on`, …, `done`), unknown ones in
  between. `working_on`, `doing`, `in_progress` or `wip` gets `doing: true`;
  `done`, `finished` or `closed` gets `done: true`. Whatever does not fit you
  change afterwards in `board.yaml`.
- **A directory without such subdirectories** gets `backlog`, `working_on`
  and `done`.
- **`--columns`** names the columns yourself, in their order.
- **The `id`** comes from the directory name, formed like a card short name.
  If the directory is called `board`, `kanban` or `cards`, the name of the
  directory above it applies.
- **`--sync`** sets the Git mode, default `none`. `kard init` itself commits
  nothing.
- **The board is registered on the machine**, as with `kard add`.
  `--no-add` leaves that out. If another board already has the same `id`, the
  board is still created but stays unregistered, and `kard` says so.
- **At the end it says what it found:** the columns, the number of cards and
  how many of them break a rule.

On creating:

- `kard new` puts the card in the first column if `-c` is missing, and sets
  `created` to today's date.
- The file name is made from the title: lower case, umlauts spelled out (`ä`
  becomes `ae`), anything other than `a–z` and `0–9` becomes a hyphen. If the
  short name already exists, `kard new` refuses and asks for `--id`.
- `--set` may be repeated. Lists go as `tags=bug,ui` or `tags=[bug, ui]`. The
  person comes with `--set assignee=sam`.

On changing:

- **Moving only renames the file.** The text stays untouched.
- **Changing fields only rewrites the lines of those fields.** Other fields,
  comments, the order in the header and the text stay as they are. A new field
  goes to the end of the header. If the last field goes away, the header goes
  away.
- **A step happens completely or not at all.** `kard take` assigns and moves;
  if a rule refuses either of the two, the card stays as it was.
- **No command changes a card that belongs to others without `--force`**, in
  any column. Whoever is listed under `assignee` may change it, even if other
  names are listed there too. Anyone may take, move and assign a card that
  belongs to nobody.
- **Joining is a change too.** `kard take --add` on someone else's card needs
  `--force`. Without it, someone already on the card brings you in:
  `kard assign <card> <person> --add`.
- **Without `--add`, `kard take` and `kard assign` replace all names.** When
  the last person leaves the card, the field goes away.
- **Required fields apply when changing.** A card does not go into a column
  whose required field it lacks, and does not lose a required field of its
  column. `kard new` does not create a card that lacks one. With `--force` it
  works anyway, and `kard lint` reports the card afterwards. What a card
  already lacked before does not block other changes to it.
- **Without a column with `doing`**, `kard take` only assigns.
- **kardboard only needs to know who you are when it matters:** for
  `kard take`, for `--mine` and for cards that belong to someone.

For all commands:

- Reading commands know `--json`, for scripts and agents. A field of type
  `person` is always a list there, even with a single name.
  `kard show --json` also gives the fingerprint of the file (`sum`).
- With `--json` an error is one object on stderr: `error` (the reason),
  `kind` (`refused`, `conflict`, `not_pushed`, `stale`, `ambiguous` or
  `usage`) and `force` if `--force` would do it anyway. With `ambiguous`,
  `candidates` names the people a name matches. stdout only carries what the
  command prints.
- Nothing needs someone at the keyboard. Text that the editor would otherwise
  supply is taken by `kard new --text` and `kard edit --file` from a file or,
  with `-`, from stdin. `kard edit --file … --base <sum>` only saves if the
  card is unchanged since it was read.
- Every command has `--help` (or `-h`, `kard help <command>`) with examples.
- Exit code `0` on success, `1` on a rule violation, on a change that was
  taken back because of a conflict, and on a commit that could not be pushed.
  `2` on a wrong invocation or when the command cannot take effect (no board
  found, card or column unknown, remote not reachable).
- The UI does nothing the CLI cannot do. Every action there is exactly one of
  these commands.

## UI

`kard ui` starts a web server on your own machine and shows all known boards.
If you are in a board, it is included, even if it is not registered.

- **Only your own machine.** The server listens on `127.0.0.1`, by default on
  port 4712, and only answers requests that name `localhost` as their target.
- **Changing as in the CLI.** Cards can be dragged to another column, and the
  card view offers Take, Finish, Move, assigning people and changing title,
  fields and text. Each of these actions is exactly the command of the same
  name, with the same rules. If a rule refuses, the page shows the reason and
  offers to do it anyway. That corresponds to `--force`.
- **Editing in one step.** You change title, fields and text in an edit mode,
  the text as Markdown with a preview. What you change there only goes to the
  card with "Save", as one change, so as one commit `edit <card>`. The header
  stays as it was, apart from the changed fields. A cleared field goes away,
  as with `kard set <card> field=`. Cancel discards everything.
- **No overwriting.** Title and text are recorded on opening, together with a
  fingerprint of the file. If the file has changed by the time you save, the
  page does not save the text, says so and keeps the draft. This cannot be
  forced, because it would overwrite what someone else wrote. Changing only
  fields changes only those lines.
- **Dragging onto the working column takes the card.** If it does not belong
  to anyone yet, you are added. Otherwise dragging is a plain move. In the
  shared view of several boards, "In progress" stands for the working column
  and "Done" for the done column of the card's board. You cannot drag onto
  "Open" there, because it can be several columns.
- **Other websites cannot change anything.** A changing request must come
  from the page itself: same origin, a header of its own in the request and
  JSON as the content. A form or script of another page in the same browser
  cannot meet this.
- **Git as in the CLI.** On boards with `sync: commit` or `push` every action
  makes the same commit as the command. If a conflict takes the change back or
  the push fails, the page says so. A card with uncommitted changes says in
  its view that it cannot be changed this way. A board that lacks the
  repository or the upstream is only shown, and the page names the reason.
- **Keeping up with the remote.** For boards with `sync: push` the server
  asks the remote every minute (`git fetch`). The menu shows the state per
  board: up to date, how many commits there are to pull or to push, or that
  the remote is not reachable. A click on it is `kard sync`. As long as
  something is waiting, the menu button carries a dot.
- **It follows the files.** The server checks the boards for changes every
  second and reports them to the open page. It looks for new boards under
  `scan` every 30 seconds and on request right away.
- **One view, no switch.** What it shows follows from the ticked boards. If
  one is ticked, it appears with its own columns. If several are, their cards
  are in three shared columns: Open, In progress (`doing`), Done (`done`).
  Each card then shows which board it belongs to.
- **Showing and hiding.** The project picker opens from the top bar. The
  checkbox shows or hides a board, a click on the name shows it alone. The
  browser remembers which boards are shown. `hidden` in the settings is the
  default as long as nothing has been picked there yet.
- **Filter.** A search field and "Only mine" apply to everything that is
  shown. "Only mine" is in the menu. While it is on, the top bar shows it.
- **Light and dark.** The page follows the system. In the menu you can fix
  Light or Dark; the browser remembers that too.
- **History from Git.** If a board is in a Git repository, the page shows
  which commits touched it, for the whole board and for each card on its own.
  kardboard reads what a commit did from its content and not from its
  message: A file that Git lists as renamed from `backlog/` to `done/` is a
  moved card, no matter who committed it and how. This applies to every
  board, also with `sync: none`. A board without a repository has no history,
  and the page says so. `kard log` shows the same history in the terminal,
  one line per commit.
- **Errors stay visible.** A faulty card is marked and shows its violations,
  an unusable board shows the reason.
- **No requests to the outside.** Everything the page needs is in the
  program.

## People

Git already has an identity, often a different one per repository, and the
commits of `kard` carry it anyway. kardboard takes this identity without
anyone setting anything up, and lets you override it where you want to.

### Who you are

kardboard determines your own name per board. It takes the first entry it
finds:

| No. | Source | Meant for |
|---|---|---|
| 1 | `KARDBOARD_USER` | a single call, scripts and tests |
| 2 | `user` in `.kard/config.yaml`, the nearest one from the board | a different name in one repository or board |
| 3 | `user` in `~/.config/kardboard/config.yaml` | a fixed name in all boards |
| 4 | `git config user.email`, read in the board's directory | the normal case: set up nothing |
| 5 | the account name in the operating system | machines without Git or without a Git address |

What is set explicitly takes precedence over what is derived. So whoever
already has `user: alex` set notices no difference.

On the sources:

- **Git per board.** `git config user.email` is read in the board's
  directory. So whatever applies for Git there takes effect, including an
  address per repository via `includeIf`. If the board is not in a
  repository, the global Git address applies. kardboard only reads the Git
  config and never writes to it.
- **A name of your own per repository or board** goes in `.kard/config.yaml`.
  It is for whoever does not want to appear there under their address.
- **The account name** is the last resort. It is lower-cased, on Windows
  without the part before `\`. If it is then not a valid short name, there is
  no name, and `kard` says where to set one. The account name can be
  different on every machine. For a board you use alone that does not matter;
  in a team the name should come from Git or the settings.
- **`kard whoami`** shows the name and where it comes from, with the path of
  the file, for the board you are in, or with `--all` for every board.
  `--json` prints both as fields. `kard whoami --set <name>` writes the name
  to `.kard/config.yaml` at the root of the repository, without a repository
  to the board directory.

### Personal settings in the project: `.kard/`

```
project/
  .kard/
    .gitignore      contains only: *
    config.yaml     user: angelique-l
  board/
    board.yaml
```

- **Only for yourself.** `.kard/` belongs to whoever has the clone. What
  concerns everyone goes in `board.yaml`. In every new clone you create
  `.kard/` again.
- **Never committed.** The `.gitignore` in `.kard/` excludes the whole
  directory, itself included. The project's `.gitignore` stays untouched.
  `kard` creates this file when it creates the directory, and `kard lint`
  reports an error if Git tracks a file from `.kard/` (new rule E11).
- **The search goes from the board directory upwards** to the root of the
  repository, without a repository only in the board directory. The first
  `.kard/config.yaml` applies. So one at the root applies to all boards of a
  repository, and a board can still have its own.
- **The same form as `~/.config/kardboard/config.yaml`**, but only with
  `user`. Any other key is an error, until there is a reason for it.

### What the card holds

A value of type `person` is a short name as before or an email address.
kardboard writes the address in lower case and compares without regard to
case.

```yaml
assignee: angelique.lambert@example.com
```

```yaml
assignee: [angelique.lambert@example.com, sam]
```

The file stays readable without kardboard, and `grep` finds the person. The
address is the same as in the board's commits. Whoever does not want to show
it in a public repository uses a noreply address there or a name of their own
in `.kard/config.yaml`.

Existing cards with short names stay valid and are not rewritten.

### Assigning others without typing addresses

A board already knows its people, without a list in `board.yaml`:

- the authors of the commits that touched the board, with name and address
- the people already named in the cards

Git merges these people via the repository's `.mailmap`. kardboard uses the
same file: Two addresses that it maps to one person also count as the same
person on cards, for example for `--mine` and for the question whether a card
is your own.

`kard assign <card> <person>` and `kard new --set assignee=…` take a person
like this:

1. If the value is a short name or an address the board knows, it applies.
2. Otherwise `kard` searches the known people for the start of an address or
   a name, ignoring case and umlauts. `sam` finds `sam@example.com` and Sam
   Berger. Exactly one match applies.
3. Several matches are an error that names them. With `--json` it has the
   kind `ambiguous` and a list `candidates`.
4. No match: A valid short name or a valid address is taken as it stands.
   That is a new person. If the field has `values`, this is an error as
   before.

### In the UI

- **A monogram instead of the name.** Two letters, from the name in Git,
  otherwise from the address before the `@` or from the short name: Angélique
  Lambert becomes AL, `sam@example.com` becomes SA. The monogram is square and
  set in mono, like everything the tool says. Your own is filled with ink,
  the others have a hairline.
- **Name and address on hover.** Only that tells apart two people with the
  same letters; the monogram is not meant to do more.
- **Assigning from a list** of the board's known people, with the same search
  as in the CLI.
- **Who you are applies per board.** "Only mine" means, in each ticked board,
  the name that kardboard determines for that board. If the name there comes
  only from the account name, the menu says so.

## Open questions

- **Switching short names in existing cards.** Whoever had `user: alex` set
  and removes it is then named after their Git address, and their old cards
  are no longer theirs. Whether this needs a command that replaces a short
  name with an address in all cards of a board, or whether a set `user` is
  enough.
- **People across boards.** An address is the same across boards, a short
  name not necessarily. "Only mine" across several boards depends on what you
  are called in each of them.

- **Order within a column.** The directory has none. Either the board sorts by
  a field, or the order needs a place of its own.
- **Subdirectories in columns.** Ignored today. Conceivable as an archive or
  as a group of cards that belong together.
- **Title in the heading instead of the header.** Chosen because that way the
  file looks right in every Markdown viewer. Other tools keep the title in the
  header.
- **Joining without `--force`.** Whoever joins a card with `kard take --add`
  takes nothing away from anyone. Today it still requires `--force`, so that
  the rule for other people's cards has no exception.
- **Limit per person.** `limit` counts the cards of the column, not those of a
  person.
