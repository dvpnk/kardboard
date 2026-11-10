# kardboard

Ein Board für Karten, die als Markdown-Dateien im Projekt liegen. Das Programm
läuft einmal auf dem Rechner, erkennt die Boards in den Projekten und zeigt sie
einzeln oder zusammen.

**Stand:** früher Entwurf. Das Format steht als Entwurf, der Befehl `kb` kann
ein Board lesen und prüfen. Verschieben, Git-Anbindung und Oberfläche fehlen
noch.

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

Ohne `-C` sucht `kb` das Board vom aktuellen Ordner aus. `--json` gibt bei
allen drei Befehlen JSON aus.

## Tests

```bash
go test ./...
```
