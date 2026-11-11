# kardboard

Ein Board für Karten, die als Markdown-Dateien im Projekt liegen. Das Programm
läuft einmal auf dem Rechner, erkennt die Boards in den Projekten und zeigt sie
einzeln oder zusammen.

**Stand:** früher Entwurf. Das Format steht als Entwurf. Der Befehl `kb` kann
Boards lesen und prüfen, Karten anlegen, verschieben und zuweisen und alle
Boards des Rechners im Browser zeigen. Die Oberfläche zeigt nur an. Ändern kann
`kb` nur Boards mit `sync: none`; die Git-Anbindung fehlt noch.

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
kb assign dunkles-design sam --add
```

```bash
kb done dunkles-design
```

```bash
kb list --mine
```

## Alle Boards im Browser

Ein Board anmelden, oder gleich einen Ordner mit Projekten durchsuchen lassen:

```bash
kb add examples/board
```

```bash
kb scan ~/Repos
```

```bash
kb boards
```

Dann die Oberfläche starten. Sie öffnet sich im Browser, folgt Änderungen an
den Dateien und läuft, bis man sie mit Strg+C beendet:

```bash
kb ui
```

Der Server ist nur vom eigenen Rechner aus erreichbar, und die Oberfläche
ändert nichts an den Boards.

## Fremde Bestandteile

Die Oberfläche nutzt die Schrift [JetBrains Mono](https://github.com/JetBrains/JetBrainsMono),
Copyright 2020 The JetBrains Mono Project Authors, unter der SIL Open Font
License 1.1. Die Schriftdateien und der Lizenztext liegen unter
[internal/server/web/fonts](internal/server/web/fonts) und sind ins Programm
eingebettet. Die Lizenz gilt nur für die Schrift.

## Tests

```bash
go test ./...
```
