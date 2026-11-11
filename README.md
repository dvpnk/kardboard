# kardboard

Ein Board für Karten, die als Markdown-Dateien im Projekt liegen. Das Programm
läuft einmal auf dem Rechner, erkennt die Boards in den Projekten und zeigt sie
einzeln oder zusammen.

**Stand:** früher Entwurf. Das Format steht als Entwurf. Der Befehl `kb` kann
Boards anlegen, lesen und prüfen, Karten anlegen, verschieben und zuweisen und alle
Boards des Rechners im Browser zeigen. Dort lassen sich Karten auch ziehen,
nehmen und zuweisen. Mit `sync: commit` oder `sync: push` in der `board.yaml`
macht `kb` aus jeder Änderung einen Commit und gleicht das Board mit seinem
Remote ab.

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

## Eigenes Board anlegen

Im Projekt, in einem neuen Ordner:

```bash
kb init board
```

Das schreibt eine `board.yaml` mit den Spalten `backlog`, `working_on` und
`done` und meldet das Board auf dem Rechner an. Gibt es schon Ordner mit
Markdown-Dateien, werden sie die Spalten; keine Datei wird dabei geändert:

```bash
kb init ~/Repos/notizen
```

Was an den vorhandenen Karten noch nicht zum Format passt, zeigt danach
`kb lint`.

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

Karten lassen sich in eine andere Spalte ziehen. Ein Klick auf eine Karte
öffnet ihren Text und bietet Nehmen, Abschließen, Verschieben und Zuweisen an.
Jede dieser Handlungen ist der gleichnamige Befehl, mit denselben Regeln.

Liegt ein Board in einem Git-Repository, zeigt „Verlauf“ oben rechts die
Commits, die es berührt haben, und jede Karte ihre eigene Geschichte. Dafür
muss `git` installiert sein.

Der Server ist nur vom eigenen Rechner aus erreichbar und nimmt Änderungen nur
von der eigenen Seite an.

## Board im Team

Liegt das Board in einem eigenen Git-Repository, das mehrere Leute klonen,
steht in seiner `board.yaml`:

```yaml
sync: push
```

Dann holt jeder ändernde Befehl zuerst den Stand der anderen, macht genau
einen Commit mit der einen Karte und pusht ihn. Bewegen zwei dieselbe Karte
gleichzeitig, gewinnt, wer zuerst pusht; der andere bekommt gesagt, wo die
Karte jetzt liegt. `git log` ist das Protokoll des Boards:

```
move export-als-pdf: backlog -> working_on
assign export-als-pdf: alex
```

Was liegen geblieben ist, etwa ohne Netz, holt und sendet:

```bash
kb sync
```

Mit `sync: commit` committet `kb` nur und pusht nicht. Git darf dabei nicht
nach einem Passwort fragen; der Schlüssel gehört in den Agenten oder den
Schlüsselbund.

## Fremde Bestandteile

Die Oberfläche nutzt die Schrift [JetBrains Mono](https://github.com/JetBrains/JetBrainsMono),
Copyright 2020 The JetBrains Mono Project Authors, unter der SIL Open Font
License 1.1. Die Schriftdateien und der Lizenztext liegen unter
[internal/server/web/fonts](internal/server/web/fonts) und sind ins Programm
eingebettet. Die Lizenz gilt nur für die Schrift.

Die Oberfläche ist mit [Preact](https://preactjs.com) 11.0.1 (MIT License,
Copyright 2015-present Jason Miller) und [htm](https://github.com/developit/htm)
3.1.1 (Apache License 2.0) gebaut. Beide liegen als fertige Dateien unter
[internal/server/web/vendor](internal/server/web/vendor), mit ihren
Lizenztexten, Herkunft und Prüfsummen. Es gibt kein npm und keinen
Build-Schritt; ein Test prüft, dass sich die Dateien nicht unbemerkt ändern.

## Tests

```bash
go test ./...
```
