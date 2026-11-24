---
name: kard
description: Read and change a kardboard board, a board of Markdown cards in a project, with the kard command. Use when a project has a board.yaml or mentions kard or kardboard, or when asked to plan work, pick up or finish a task, or look at or change cards, tickets or the board.
---

# Working with a kardboard board

A kardboard board is a folder with a `board.yaml` and one folder per
column. Each card is a Markdown file: a YAML header, then the title as the
first line `# …`, then the text. The folder a card is in is its column; the
file name without `.md` is its id.

## Rules

- **Change cards only with `kard`**, never by writing, moving or deleting
  their files yourself. kard checks the board's rules (required fields,
  limits, who a card belongs to) and, on boards with `sync: commit` or
  `push`, makes exactly one commit per change and syncs it.
- **Read with `--json`.** Every command prints JSON with it, errors too.
- **Do not use `--force` on your own.** It overrides a rule, such as a card
  that belongs to someone else. Ask the person first.
- **Do not guess a person or a column.** If kard says a name is ambiguous,
  ask which of the `candidates` is meant.

## Which board

kard finds the board from the current folder upwards. Elsewhere, pick it
by id with `-b <id>` or by folder with `-C <dir>`. `kard boards` lists the
boards known on this machine; the project's `AGENTS.md` or `CLAUDE.md`
often says which one belongs to it.

## Reading

```bash
kard list --json                 # all cards, with column, title, fields, sum
kard list working_on --json      # one column
kard list --mine --json          # cards of the current person
kard show <card> --json          # one card, with its text (body) and sum
kard schema                      # which fields and values the board allows
kard log <card>                  # the card's history, on boards in Git
kard whoami --json               # who kard thinks you are
```

## Changing

```bash
kard new "Title" -c backlog --set size=M --set tags=feature,ui
echo "What needs doing." | kard new "Title" --text -
kard take <card>                 # assign to the current person, move to the doing column
kard move <card> <column>
kard done <card>
kard assign <card> <person>      # --add keeps the others on it
kard unassign <card>
kard set <card> size=L due=2026-12-01    # field= removes a field
kard rename <card> <new-id>      # also updates references to it
```

To change a card's title or text, read it, write the whole new file and
pass the sum you read, so nothing someone else changed meanwhile is
overwritten:

```bash
kard show <card> --json          # note "sum"
kard edit <card> --file new.md --base <sum>
```

`new.md` holds the complete card: header, `# Title`, text. Never run
`kard edit` without `--file`; it would open an editor and wait.

## Results and errors

- Exit code `0`: done. `1`: a rule refused. `2`: a wrong call, such as an
  unknown card or column.
- With `--json`, an error is an object on stderr with `error` (the reason),
  `kind` and, where it would help, `"force": true`.
- After a change on a side branch of the code repository, kard says that
  main gets it only when the branch is merged. Pass this on to the person.

`kard <command> --help` explains every command with examples.
