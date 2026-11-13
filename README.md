# kardboard

Ein Board für Karten, die als Markdown-Dateien im Projekt liegen. Das Programm
läuft einmal auf dem Rechner, erkennt die Boards in den Projekten und zeigt sie
einzeln oder zusammen.

**Stand:** früher Entwurf. Das Format steht als Entwurf. Der Befehl `kard` kann
Boards anlegen, lesen und prüfen, Karten anlegen, verschieben und zuweisen und alle
Boards des Rechners im Browser zeigen. Dort lassen sich Karten auch ziehen,
nehmen und zuweisen. Mit `sync: commit` oder `sync: push` in der `board.yaml`
macht `kard` aus jeder Änderung einen Commit und gleicht das Board mit seinem
Remote ab.

- [Übersicht in Bildern](docs/overview.html), zum Öffnen im Browser
- [Format und CLI](docs/spec.md)
- [Beispiel-Board](examples/board)

## Bauen und ausprobieren

Braucht Go 1.27 oder neuer.

```bash
go build -o bin/kard ./cmd/kard
```

Der Befehl heißt `kard`. Wer es kürzer mag, legt sich selbst einen Alias an,
etwa `alias kb=kard`. Eingebaut ist kein Kurzname, weil kurze Namen leicht
mit anderen Werkzeugen zusammenstoßen.

```bash
./bin/kard lint -C examples/board
```

```bash
./bin/kard list -C examples/board
```

```bash
./bin/kard show export-als-pdf -C examples/board
```

Liegt ein Board in einem Git-Repository, zeigt `kard log`, welche Commits es
geändert haben, mit einer Karte nur deren Geschichte:

```bash
./bin/kard log export-als-pdf -C examples/board
```

Ohne `-C` sucht `kard` das Board vom aktuellen Ordner aus. `--json` gibt JSON
aus.

Karten anlegen und ändern, am besten in einer Kopie des Beispiels. Dafür muss
`kard` im Pfad liegen.

```bash
cp -R examples/board /tmp/kard-demo && cd /tmp/kard-demo
```

Wer man selbst ist, weiß `kard` meist schon: die Adresse aus
`git config user.email`, in jedem Repository die dort gültige. Die Karten
des Beispiels tragen Kürzel, also setzt man den Namen für die Demo selbst:

```bash
export KARDBOARD_USER=alex
```

`kard whoami` zeigt den Namen und woher er kommt. Einen eigenen Namen für
ein Projekt legt `kard whoami --set <name>` in `.kard/config.yaml` ab; der
Ordner ignoriert sich selbst und wird nie committet. Beim Zuweisen reicht
der Anfang eines Namens oder einer Adresse, die das Board kennt, aus seinen
Commits oder seinen Karten: `kard assign <karte> sam`.

```bash
kard new "Dunkles Design" -c backlog --set size=M --set tags=feature,ui
```

```bash
kard assign dunkles-design alex
```

```bash
kard take dunkles-design
```

```bash
kard assign dunkles-design sam --add
```

```bash
kard done dunkles-design
```

Den Text einer Karte ändert man in jedem Editor. `kard edit` öffnet sie in
`$VISUAL` oder `$EDITOR` und speichert sie danach wie ein `kard set`: mit
denselben Regeln und auf Boards mit `sync: commit` oder `push` als ein
Commit.

```bash
kard edit dunkles-design
```

`kard rename` gibt einer Karte einen neuen Dateinamen und zieht die Verweise
im Board nach:

```bash
kard rename dunkles-design dark-mode
```

```bash
kard list --mine
```

## Eigenes Board anlegen

Im Projekt, in einem neuen Ordner:

```bash
kard init board
```

Das schreibt eine `board.yaml` mit den Spalten `backlog`, `working_on` und
`done` und meldet das Board auf dem Rechner an. Gibt es schon Ordner mit
Markdown-Dateien, werden sie die Spalten; keine Datei wird dabei geändert:

```bash
kard init ~/Repos/notizen
```

Was an den vorhandenen Karten noch nicht zum Format passt, zeigt danach
`kard lint`.

Damit auch ein Editor oder ein anderes Werkzeug den Kopf der Karten prüft,
gibt `kard schema` ein JSON Schema aus der `board.yaml` aus. Mit einer Spalte,
etwa `kard schema working_on`, verlangt es auch deren Pflichtfelder:

```bash
kard schema > card.schema.json
```

## Alle Boards im Browser

Ein Board anmelden, oder gleich einen Ordner mit Projekten durchsuchen lassen:

```bash
kard add examples/board
```

```bash
kard scan ~/Repos
```

```bash
kard boards
```

`--all` prüft oder listet alle diese Boards auf einmal, etwa die eigenen
Karten über alle Projekte:

```bash
kard lint --all
```

```bash
kard list --mine --all
```

Dann die Oberfläche starten. Sie öffnet sich im Browser, folgt Änderungen an
den Dateien und läuft, bis man sie mit Strg+C beendet:

```bash
kard ui
```

Karten lassen sich in eine andere Spalte ziehen. Ein Klick auf eine Karte
öffnet ihren Text und bietet Nehmen, Abschließen, Verschieben und Zuweisen an.
Der Stift ändert Titel, Felder und Text in einem Schritt, mit Vorschau.
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
kard sync
```

Mit `sync: commit` committet `kard` nur und pusht nicht. Git darf dabei nicht
nach einem Passwort fragen; der Schlüssel gehört in den Agenten oder den
Schlüsselbund.

## Für Skripte und KI-Agenten

`kard` braucht kein Gegenüber. Jeder Befehl erklärt sich mit `--help`, samt
Beispielen, und gibt mit `--json` JSON aus, auch Fehler: als Objekt auf
stderr, mit Grund, Art und ob `--force` hilft. Der Rückgabecode ist `0` bei
Erfolg, `1` bei einer Regel, die ablehnt, `2` bei falschem Aufruf.

Wer eine Karte ändert, tut das über `kard` und nicht in der Datei: Dann gelten
die Regeln des Boards, und auf Boards mit Git entsteht der richtige Commit.
Text, den sonst ein Editor liefert, kommt aus einer Datei oder von stdin:

```bash
echo "Was zu tun ist." | kard new "Login prüfen" -c backlog --set size=S --text -
```

Zum Ändern liest man die Karte mit ihrem Fingerabdruck und gibt ihn beim
Speichern mit. Hat inzwischen jemand anderes die Karte geändert, wird nichts
überschrieben:

```bash
kard show login-pruefen --json   # body, fields und sum
```

```bash
kard edit login-pruefen --file neu.md --base <sum>
```

Welche Felder und Werte erlaubt sind, beschreibt `kard schema` als JSON
Schema. Wer man selbst ist, sagt `kard whoami --json`; `KARDBOARD_USER`
setzt es für einen Aufruf. Passt ein Name auf mehrere Personen, ist der
Fehler `ambiguous` und nennt sie unter `candidates`.

## Lizenz

kardboard steht unter der [MIT License](LICENSE). Für die fremden
Bestandteile unten gelten ihre eigenen Lizenzen.

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
