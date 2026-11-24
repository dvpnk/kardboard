# Install kard and set up a board: steps for a coding agent

This file is written for a coding agent such as Claude Code or Codex that a
person has asked to install kardboard. People can follow it just as well;
the [README](README.md#install) has the same in shorter form.

kard is a single program with nothing else to install. Do the steps in
order. Run each command on its own and check its result before going on.
Ask the person before you change anything outside the current project,
such as a shell file, a new folder next to the project or a Git remote.
Never pipe a download into a shell.

## 1. Is kard there already?

```bash
kard version
```

If this prints a version and the person did not ask for an update, go on
with step 5.

## 2. Pick the file

Find the system with `uname -s` and `uname -m`; on Windows, use the
PowerShell steps in 3b.

| `uname -s` | `uname -m` | File |
| --- | --- | --- |
| Darwin | arm64 or x86_64 | `kard-macos` |
| Linux | x86_64 | `kard-linux-amd64` |
| Linux | aarch64 or arm64 | `kard-linux-arm64` |
| Windows | AMD64 | `kard-windows-amd64.exe` |

There is no file for other systems. Then tell the person, and if Go 1.27
or newer is there, offer
`go install github.com/dvpnk/kardboard/cmd/kard@latest` instead.

## 3a. macOS and Linux: download, check, install

Set `f` to the file from step 2. The links always point to the newest
release.

```bash
f=kard-macos
tmp=$(mktemp -d)
curl -fsSL -o "$tmp/$f" "https://github.com/dvpnk/kardboard/releases/latest/download/$f"
curl -fsSL -o "$tmp/SHA256SUMS" "https://github.com/dvpnk/kardboard/releases/latest/download/SHA256SUMS"
```

Check the file against its checksum. On macOS:

```bash
(cd "$tmp" && grep " $f\$" SHA256SUMS | shasum -a 256 -c -)
```

On Linux:

```bash
(cd "$tmp" && grep " $f\$" SHA256SUMS | sha256sum -c -)
```

Stop if this does not say `OK`, and tell the person. Otherwise:

```bash
mkdir -p ~/.local/bin
mv "$tmp/$f" ~/.local/bin/kard
chmod +x ~/.local/bin/kard
```

Find out whether the shell finds it:

```bash
command -v kard
```

If it finds nothing, `~/.local/bin` is not in the path. Ask the person
whether you may add this line to their shell file, `~/.zshrc` for zsh and
`~/.bashrc` for bash:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

Until a new terminal is opened, call it as `~/.local/bin/kard`.

## 3b. Windows: download, check, install

In PowerShell:

```powershell
$dir = "$env:LOCALAPPDATA\Programs\kard"
New-Item -ItemType Directory -Force $dir | Out-Null
$base = "https://github.com/dvpnk/kardboard/releases/latest/download"
Invoke-WebRequest "$base/kard-windows-amd64.exe" -OutFile "$dir\kard.exe"
Invoke-WebRequest "$base/SHA256SUMS" -OutFile "$dir\SHA256SUMS"
$want = ((Get-Content "$dir\SHA256SUMS" | Select-String "kard-windows-amd64.exe").Line -split " ")[0]
Remove-Item "$dir\SHA256SUMS"
$have = (Get-FileHash "$dir\kard.exe" -Algorithm SHA256).Hash.ToLower()
if ($want -ne $have) { Remove-Item "$dir\kard.exe"; throw "checksum does not match" }
```

Ask the person before adding the folder to their path:

```powershell
[Environment]::SetEnvironmentVariable("Path", [Environment]::GetEnvironmentVariable("Path", "User") + ";$dir", "User")
```

Until a new terminal is opened, call it as `& "$dir\kard.exe"`.

## 4. Check the installation

```bash
kard version
```

This prints the version, for example `kard v0.2.1 (darwin/arm64, …)`.

## 5. Where should the board live?

Ask the person. Unless they want otherwise, suggest a folder next to the
project, as a Git repository of its own:

- **Next to the project (recommended):** for example `../myproject-board`
  beside `../myproject`. The board does not change when someone switches
  branches in the code, and with a Git remote and `sync: push` the whole
  team sees every move right away.
- **Inside the project:** for example `board/` in the code repository, with
  `sync: none`. The cards then belong to the branch that is checked out,
  like the code: a card done on a feature branch is done on main only once
  the branch is merged. That suits one person who plans alongside the code.
- **Markdown notes already exist:** `kard init <dir>` turns their folders
  into columns and changes no file. `kard lint` then shows what does not
  fit the format yet.

## 6. Create the board

Next to the project, with `myproject` as the project's name:

```bash
mkdir ../myproject-board
cd ../myproject-board
git init
kard init . --id myproject --name "My project" --sync commit
```

With `sync: commit`, `board.yaml` is the first commit, and every change kard
makes after it is one too. If the board gets a remote that others clone,
push once with `git push -u` and set `sync: push` in `board.yaml`; then kard
also fetches and pushes. Ask the person before you create a remote or push.

Inside the project:

```bash
kard init board --id myproject --name "My project"
```

`kard init` makes the board known on this machine. From anywhere, `-b
myproject` then picks it, for example `kard list -b myproject`.

## 7. Check the board and the name

```bash
kard lint -b myproject
```

```bash
kard whoami -b myproject
```

kard takes the name from `git config user.email` and otherwise from the
system account. If it is not the name the person wants on their cards, ask
them and set it with `kard whoami --set <name>`; it stays on this machine
and is never committed.

## 8. Tell the project's agents about the board

So that the next agent in the project knows where the board is and how to
change it, add a short section to the project's `AGENTS.md`, or to
`CLAUDE.md` if the project uses that. Ask before creating a new file.

```markdown
## Board

Planning lives in a kard board, id `myproject`, at `../myproject-board`.
Change cards only with `kard`, never by editing their files, so the board's
rules apply and every change is recorded. From anywhere: `kard list -b
myproject`, `kard show <card> -b myproject --json`, `kard take <card> -b
myproject`, `kard done <card> -b myproject`. `kard <command> --help`
explains each command.
```

If the agent supports skills, the [kard skill](skills/kard/SKILL.md)
teaches it the rest. For Claude Code:

```bash
mkdir -p ~/.claude/skills/kard
curl -fsSL -o ~/.claude/skills/kard/SKILL.md https://raw.githubusercontent.com/dvpnk/kardboard/main/skills/kard/SKILL.md
```

## 9. Report

Tell the person:

- which version is installed, and where;
- where the board is, its id, and how it syncs;
- what changed outside the project, such as the shell file;
- that `kard ui` shows all boards in the browser.
