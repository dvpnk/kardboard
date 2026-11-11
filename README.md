# kardboard

Ein Board für Karten, die als Markdown-Dateien im Projekt liegen. Das Programm
läuft einmal auf dem Rechner, erkennt die Boards in den Projekten und zeigt sie
einzeln oder zusammen.

**Stand:** früher Entwurf. Das Format steht als Entwurf. Der Befehl `kb` kann
ein Board lesen und prüfen, Karten anlegen, verschieben und zuweisen. Ändern kann er nur
Boards mit `sync: none`; Git-Anbindung, die Anmeldung mehrerer Boards und die
Oberfläche fehlen noch.

- [Format und CLI](docs/spec.md)
- [Beispiel-Board](examples/board)

## Bauen und ausprobieren

Braucht Go 1.27 oder neuer.

```bash
go build -o bin/kb ./cmd/kb
```

```bash
./bin/kb lint -C examples/board
```

```bash
./bin/kb list -C examples/board
```

```bash
./bin/kb show export-als-pdf -C examples/board
```

Ohne `-C` sucht `kb` das Board vom aktuellen Ordner aus. `--json` gibt JSON
aus.

Karten anlegen und ändern, am besten in einer Kopie des Beispiels. Dafür muss
`kb` im Pfad liegen.

```bash
cp -R examples/board /tmp/kb-demo && cd /tmp/kb-demo
```

Wer man selbst ist, steht als `user: <name>` in
`~/.config/kardboard/config.yaml` oder in der Variable `KARDBOARD_USER`:

```bash
export KARDBOARD_USER=alex
```

```bash
kb new "Dunkles Design" -c backlog --set size=M --set tags=feature,ui
```

```bash
kb assign dunkles-design alex
```

```bash
kb take dunkles-design
```

```bash
kb done dunkles-design
```

```bash
kb list --mine
```

## Tests

```bash
go test ./...
```
