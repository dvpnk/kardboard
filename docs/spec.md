# kardboard: Format und CLI

**Stand:** Entwurf v0, zur Diskussion. Schlüssel, Feldnamen und Befehle sind
englisch und bleiben es. Der Text ist vorerst deutsch.

kardboard zeigt Karten als Board, die als Markdown-Dateien in einem Ordner
liegen. Dieses Dokument legt fest, wie der Ordner und die Dateien aussehen und
welche Befehle es gibt. Wie das Programm gebaut ist, steht hier nicht.

## Grundsätze

1. **Ein Board ist ein Ordner mit einer `board.yaml`.** Wo der Ordner liegt, ist
   egal: im Projekt, daneben oder als eigenes Repo.
2. **Der Ordner ist die Spalte.** In welcher Spalte eine Karte liegt, ergibt
   sich aus dem Pfad, nie aus dem Inhalt der Datei. Verschieben ändert deshalb
   keine Zeile Text.
3. **Alles andere über die Karte steht in ihrem Kopf**, auch wem sie gehört.
   Der Kopf trägt nur, was das Board filtert, sortiert oder prüft. Was darüber
   hinausgeht, ist Text und bleibt frei.
4. **Jede Karte ist ohne kardboard lesbar und änderbar.** Editor, `ls`, `grep`
   und `git` reichen immer aus.
5. **Einstieg ohne Umbau.** Ein Ordner mit Spaltenordnern und Markdown-Dateien
   wird durch eine `board.yaml` zum gültigen Board. Felder kommen dazu, wenn man
   sie braucht.

## Verzeichnis

```
board/                         Name frei wählbar
  board.yaml                   macht den Ordner zum Board
  planning/                    Spalte
    offline-modus.md           Karte
  backlog/
    export-als-pdf.md
  working_on/
    login-fehler-bei-umlauten.md
  done/
  reference/                   keine Spalte: wird ignoriert
```

- Spalten sind die Ordner, die `board.yaml` unter `columns` nennt. Alle anderen
  Ordner und Dateien im Board ignoriert kardboard. Dort ist Platz für Anhänge.
- Eine Karte ist eine `.md`-Datei direkt in einem Spaltenordner.
- Unterordner in einer Spalte werden ignoriert (siehe offene Punkte).
- Versteckte Dateien (`.gitkeep`, `.DS_Store`) werden ignoriert.
- Leere Spaltenordner dürfen fehlen. Git kennt keine leeren Ordner.

## `board.yaml`

```yaml
kardboard: 1
id: demo
name: Demo-Projekt
sync: none

columns:
  - id: planning
    title: Planung
  - id: backlog
    title: Backlog
  - id: working_on
    title: In Arbeit
    doing: true
    limit: 3
  - id: done
    title: Erledigt
    done: true

fields:
  assignee:
    type: person
    required: [working_on]
  size:
    title: Größe
    type: enum
    values: [XS, S, M, L, XL]
    required: [backlog, working_on]
  milestone:
    title: Meilenstein
    type: enum
    values: [pilot, release, later]
  tags:
    type: list
    values: [bug, feature, cleanup, security, ui]
```

| Schlüssel | Pflicht | Bedeutung |
|---|---|---|
| `kardboard` | ja | Version des Formats, derzeit `1` |
| `id` | ja | Kürzel des Boards, `[a-z0-9][a-z0-9-]*`. Vorsatz für Verweise über Boards hinweg |
| `name` | nein | Anzeigename, sonst die `id` |
| `sync` | nein | `none` (Vorgabe), `commit` oder `push`, siehe [Git](#git) |
| `columns` | ja | Spalten in Anzeigereihenfolge, mindestens eine |
| `fields` | nein | Felder, die Karten im Kopf tragen dürfen |

Schlüssel einer Spalte:

| Schlüssel | Pflicht | Bedeutung |
|---|---|---|
| `id` | ja | Ordnername, `[a-z0-9][a-z0-9_-]*` |
| `title` | nein | Anzeigename, sonst die `id` |
| `doing` | nein | `true`: Hier liegt, woran gearbeitet wird. Ziel von `kard take` |
| `done` | nein | `true`: Karten hier gelten als erledigt. Ziel von `kard done` |
| `limit` | nein | Höchstzahl Karten in der Spalte. Überschreiten ist eine Warnung |

Unbekannte Schlüssel in der `board.yaml` sind ein Fehler.

## Karte

```markdown
---
assignee: sam
size: M
milestone: release
tags: [feature]
due: 2026-12-01
depends_on: [druckvorlagen]
---

# Export als PDF

Freier Text in Markdown.
```

- **Dateiname:** `<kürzel>.md`, Kürzel aus `[a-z0-9][a-z0-9-]*`, höchstens 80
  Zeichen.
- **Der Dateiname ist der Schlüssel der Karte.** Er ist im ganzen Board
  eindeutig, über alle Spalten. Umbenennen heißt neue Karte; `kard rename` zieht
  Verweise nach.
- **Kopf:** YAML zwischen zwei Zeilen `---` ganz am Anfang der Datei. Der Kopf
  darf fehlen, solange die Spalte kein Pflichtfeld verlangt.
- **Titel:** die erste nicht leere Zeile nach dem Kopf, in der Form `# Titel`.
  Der Titel steht nur dort, nicht im Kopf.
- **Text:** alles danach, frei.
- **Kodierung:** UTF-8, Zeilenende LF.

### Felder im Kopf

Erlaubt sind die eingebauten Felder und die in `board.yaml` unter `fields`
erklärten. Jeder andere Schlüssel ist ein Fehler. So fällt ein Tippfehler auf,
statt still ein neues Feld zu werden.

Eingebaut, ohne Erklärung nutzbar:

| Feld | Typ | Bedeutung |
|---|---|---|
| `assignee` | `person` | wem die Karte gehört, eine Person oder mehrere |
| `tags` | `list` | Schlagworte |
| `due` | `date` | Termin |
| `depends_on` | `cards` | Karten, die vorher fertig sein müssen |
| `created` | `date` | Anlagedatum, setzt `kard new` |

Ein eingebautes Feld darf unter `fields` enger gefasst werden: `tags` mit
festen `values`, `assignee` als Pflicht in einer Spalte. Der Typ bleibt
derselbe.

Verboten im Kopf sind `status`, `column`, `id` und `title`. Diese Angaben
stehen im Pfad, im Dateinamen oder in der Überschrift, und zwei Quellen für
dieselbe Angabe laufen auseinander.

### Wem eine Karte gehört

`assignee` nennt eine Person mit ihrem Kürzel, oder mehrere als Liste:

```yaml
assignee: sam
```

```yaml
assignee: [alex, sam]
```

Das Feld gilt in jeder Spalte: Eine Karte kann im Backlog schon für jemanden
vorgemerkt sein, und eine erledigte Karte behält, wer sie gemacht hat. Fehlt
das Feld, gehört die Karte niemandem. Stehen mehrere Namen dort, gehört sie
jedem von ihnen gleichermaßen.

Eine leere Liste und ein doppelter Name sind Fehler. kardboard schreibt einen
einzelnen Namen ohne Klammern und mehrere als Liste in einer Zeile.

Ob eine Spalte eine Person verlangt, legt das Board selbst fest, mit der
üblichen Regel für Pflichtfelder:

```yaml
fields:
  assignee:
    type: person
    required: [working_on]
```

Mit `values` lässt sich die Liste der Personen festlegen. Ein Tippfehler im
Namen ist dann ein Fehler.

### Feldtypen

| Typ | Wert | Beispiel |
|---|---|---|
| `text` | eine Zeile Text | `owner: Einkauf` |
| `enum` | genau einer der `values` | `size: M` |
| `list` | Liste von Texten, mit `values` nur diese | `tags: [bug, ui]` |
| `date` | `JJJJ-MM-TT` | `due: 2026-12-01` |
| `number` | Zahl | `estimate: 3` |
| `bool` | `true` oder `false` | `blocked: true` |
| `person` | Kürzel einer Person, `[a-z0-9][a-z0-9-]*`, oder eine Liste davon; mit `values` nur diese | `reviewer: sam` |
| `cards` | Liste von Kartenverweisen | `depends_on: [druckvorlagen]` |

Schlüssel einer Felderklärung:

| Schlüssel | Bedeutung |
|---|---|
| `type` | einer der Typen oben, Pflicht |
| `title` | Anzeigename, sonst der Feldname |
| `values` | erlaubte Werte, Pflicht bei `enum`, möglich bei `list` und `person` |
| `required` | `true`: Pflicht in jeder Spalte. Liste von Spalten: Pflicht nur dort |

Feldnamen bestehen aus `[a-z][a-z0-9_]*`.

### Kartenverweise

`druckvorlagen` meint eine Karte im selben Board. `atlas/druckvorlagen` meint
eine Karte im Board mit der `id` `atlas`. Verweise im selben Board müssen
auflösbar sein. Verweise in ein anderes Board prüft kardboard nur, wenn dieses
Board auf dem Rechner angemeldet ist, und meldet sonst eine Warnung.

## Prüfregeln

`kard lint` prüft ein Board und endet mit Code 1, wenn es einen Fehler gibt. Damit
taugt es für einen Pre-Commit-Hook und für CI.

Fehler:

| Nr. | Regel |
|---|---|
| E01 | `board.yaml` fehlt, ist kein gültiges YAML oder verletzt dieses Dokument |
| E02 | Der Kopf einer Karte ist kein gültiges YAML oder keine Zuordnung |
| E03 | Unbekanntes Feld im Kopf |
| E04 | Verbotenes Feld im Kopf (`status`, `column`, `id`, `title`) |
| E05 | Wert passt nicht zum Typ oder steht nicht in `values` |
| E06 | Pflichtfeld fehlt für die Spalte, in der die Karte liegt |
| E07 | Titelzeile `# …` fehlt oder steht nicht an erster Stelle |
| E08 | Dateiname ist kein gültiges Kürzel |
| E09 | Derselbe Dateiname kommt mehrfach im Board vor |
| E10 | Verweis auf eine Karte, die es im Board nicht gibt |

Warnungen:

| Nr. | Regel |
|---|---|
| W01 | Verweis in ein Board, das nicht angemeldet ist |
| W02 | `limit` einer Spalte überschritten |

Eine fehlerhafte Karte wird überall angezeigt und als fehlerhaft markiert, nie
ausgeblendet.

Aus der `board.yaml` erzeugt `kard schema` ein JSON Schema (Draft 2020-12)
für den Kartenkopf. Damit prüfen auch Editoren und fremde Werkzeuge, was
`kard` prüft: welche Felder es gibt (E03, E04), ihre Typen und Werte (E05),
mit denselben Mustern für Kürzel, Verweise und Datum. Ob eine Karte, auf
die verwiesen wird, existiert (E10), steht nicht im Kopf und bleibt
`kard lint`.

Welche Felder Pflicht sind, hängt von der Spalte ab, und die steht nicht im
Kopf. `kard schema` verlangt deshalb nur die Felder, die überall Pflicht
sind. `kard schema <spalte>` verlangt dazu die dieser Spalte (E06); man gibt
dann jedem Spaltenordner sein eigenes Schema.

## Git

`sync` in der `board.yaml` legt fest, was ein ändernder Befehl mit Git tut.

| Wert | Verhalten | Gedacht für |
|---|---|---|
| `none` | ändert nur Dateien | Board im Code-Repo |
| `commit` | ein Commit je Aktion, nur mit dieser Karte | eigenes Board-Repo, allein |
| `push` | holen, ein Commit, pushen; bei Ablehnung einmal neu versuchen | eigenes Board-Repo, im Team |

Für `commit` und `push` muss das Board in einem Git-Repository liegen, für
`push` braucht der Branch außerdem einen Upstream. Fehlt etwas davon, lehnt
`kard` jede Änderung ab und sagt, was fehlt.

**Ein Commit je Aktion.** Er enthält genau die eine Karte, auch wenn im
Repository anderes geändert oder vorgemerkt ist; das bleibt unberührt. Nur
`kard rename` nimmt dazu die Karten, deren Verweise es nachzieht. Wo
nichts zu ändern ist, entsteht kein Commit. Scheitert der Commit, etwa an
einem Hook, liegt die Karte danach wieder, wie sie war.

**Karten mit Änderungen ohne Commit ändert `kard` nicht.** Sie landeten sonst
im Commit der Aktion und gingen bei einem Konflikt mit ihm verloren. Erst
committen, dann verschieben. Das gilt auch für Karten, die Git noch nicht
kennt.

Commit-Nachrichten haben eine feste Form, damit `git log` das Protokoll des
Boards ist:

```
move export-als-pdf: backlog -> working_on
assign export-als-pdf: alex, sam
unassign export-als-pdf
edit export-als-pdf
new export-als-pdf
rename druckvorlagen -> print-templates
```

`edit` steht für jede Änderung an anderen Feldern als `assignee` und am
Text. Tut eine
Aktion mehreres, etwa `kard take` (verschieben und zuweisen), steht das Erste
in der Betreffzeile und das Weitere darunter.

Bei `push` läuft jede Aktion so ab:

1. `git pull --rebase --autostash`. Die Aktion sieht also, was die anderen
   inzwischen getan haben. Ist der Remote nicht erreichbar, geschieht nichts.
2. Regeln prüfen, Karte ändern, ein Commit.
3. `git push`. Lehnt der Remote ab, weil jemand schneller war, holt `kard` den
   neuen Stand und versucht es genau einmal neu.
4. Hat inzwischen jemand dieselbe Karte bewegt oder dieselbe Zeile geändert,
   nimmt `kard` den eigenen Commit zurück, übernimmt den fremden Stand und
   meldet, wo die Karte jetzt liegt. Änderungen an verschiedenen Stellen
   derselben Karte, etwa Verschieben hier und Zuweisen dort, führt Git
   zusammen.

Gelingt der Commit, aber der Push nicht, bleibt die Änderung als Commit auf
dem Rechner. `kard` meldet das und endet mit Code 1; `kard sync` holt den Push
nach.

Git fragt dabei nie nach einem Passwort. Zugangsdaten müssen im Agenten oder
im Schlüsselbund liegen.

Liegt das Board im Code-Repo, hängt sein Stand am ausgecheckten Branch.
kardboard zeigt immer den Arbeitsstand des angemeldeten Ordners.

## Einstellungen des Benutzers

`~/.config/kardboard/config.yaml`, je Rechner und Person:

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

| Schlüssel | Bedeutung |
|---|---|
| `user` | eigenes Kürzel, `[a-z0-9][a-z0-9-]*`. Der Wert, der bei den eigenen Karten unter `assignee` steht |
| `boards` | einzeln angemeldete Boards |
| `scan` | Ordner, in denen kardboard nach `board.yaml` sucht |
| `hidden` | Boards, die in der Gesamtansicht ausgeblendet sind |

Die Variable `KARDBOARD_USER` geht dem Eintrag `user` vor. Wie der Name ohne
Eintrag aus Git kommen könnte, beschreibt der
[Vorschlag: Personen aus Git](#vorschlag-personen-aus-git).

Ein Board ist auf dem Rechner bekannt, wenn es unter `boards` steht oder unter
einem Ordner aus `scan` liegt. Die Suche geht höchstens sechs Ebenen tief und
lässt versteckte Ordner, `node_modules`, `vendor` und das Innere von Boards
aus. Zwei bekannte Boards mit derselben `id` sind ein Fehler: Das erste behält
die `id`, die anderen werden als unbrauchbar gemeldet.

`kard add`, `kard remove` und `kard scan` ändern diese Datei. Kommentare und die
Reihenfolge der Einträge bleiben erhalten, Leerzeilen nicht.

## CLI

Der Befehl heißt `kard`. Ohne weitere Angabe wirkt er auf das Board, in dem man
steht: kardboard sucht vom aktuellen Ordner aufwärts nach einer `board.yaml`
und danach in den Unterordnern des Git-Repos. Liegen dort mehrere Boards, muss
man eines wählen. `-C <ordner>` nennt den Board-Ordner direkt, `-b <id>` wählt
ein angemeldetes Board. `kard lint` und `kard list` nehmen mit `--all` alle
Boards, die `kard boards` zeigt, dazu das, in dem man steht. Sie zeigen sie
nacheinander, jedes unter seinem Namen; `kard lint --all` zählt am Ende
zusammen und endet mit Code 1, wenn irgendwo ein Fehler steckt oder ein
Board nicht lesbar ist. `kard list <spalte> --all` lässt Boards ohne diese
Spalte aus. `--all` geht nicht zusammen mit `-C` oder `-b`.

Board:

| Befehl | Wirkung |
|---|---|
| `kard init [ordner] [--id kürzel] [--name name] [--sync modus] [--columns a,b,c] [--no-add]` | macht einen Ordner zum Board, siehe unten |
| `kard lint [--all]` | prüft das Board, mit `--all` alle bekannten |
| `kard schema [spalte]` | gibt das JSON Schema für den Kartenkopf aus, mit Spalte samt ihren Pflichtfeldern |
| `kard ui [--port n] [--no-open]` | startet den lokalen Server und zeigt alle bekannten Boards im Browser, siehe [Oberfläche](#oberfläche) |

Karten lesen:

| Befehl | Wirkung |
|---|---|
| `kard list [spalte] [--mine] [--all]` | listet Karten, mit `--mine` nur die eigenen, aus jeder Spalte, mit `--all` aus allen bekannten Boards |
| `kard show <karte>` | zeigt eine Karte |
| `kard log [karte] [-n anzahl]` | zeigt, welche Commits das Board oder eine Karte geändert haben, die neuesten zuerst, in der Vorgabe 20 |

Karten ändern:

| Befehl | Spalte | Person | Wirkung |
|---|---|---|---|
| `kard new "<titel>" [-c spalte] [--set feld=wert] [--id kürzel] [--text datei]` | | | legt eine Karte an, mit `--text` samt Text |
| `kard move <karte> <spalte>` | wechselt | bleibt | verschiebt die Karte |
| `kard take <karte> [--add]` | geht in die erste Spalte mit `doing` | wird man selbst, mit `--add` kommt man dazu | nimmt die Karte in Arbeit |
| `kard done <karte>` | geht in die erste Spalte mit `done` | bleibt | schließt die Karte ab |
| `kard assign <karte> <person>… [--add]` | bleibt | wird ersetzt, mit `--add` ergänzt | weist die Karte zu. `kard give` ist dasselbe |
| `kard unassign <karte> [person…]` | bleibt | entfällt, mit Namen nur diese | gibt die Karte frei |
| `kard set <karte> feld=wert …` | bleibt | bleibt | ändert Felder im Kopf, `feld=` entfernt eines |
| `kard edit <karte> [--file datei] [--base sum]` | bleibt | wie im Editor geändert | öffnet die Karte im Editor, mit `--file` ohne, und speichert sie als eine Änderung, siehe unten |
| `kard rename <karte> <neu>` | bleibt | bleibt | benennt um und zieht Verweise nach, siehe unten |

Rechner:

| Befehl | Wirkung |
|---|---|
| `kard boards` | listet angemeldete und gefundene Boards |
| `kard add [ordner]` | meldet ein Board an. Der Ordner ist das Board oder liegt in seinem Projekt; ohne Angabe gilt der aktuelle |
| `kard remove <id\|ordner>` | meldet ein Board ab, löscht nichts |
| `kard scan <ordner>` | nimmt einen Ordner in die Suche auf, `--remove` nimmt ihn wieder heraus |
| `kard sync` | holt, was auf dem Remote neu ist, und pusht, was hier wartet. Nur für Boards mit `sync: push` |

Zu `kard edit`:

- **Der Editor bekommt eine Kopie.** `kard edit` nimmt `$VISUAL`, sonst
  `$EDITOR`, sonst `vi`. Erst wenn der Editor geschlossen ist, ersetzt die
  Kopie die Karte, in einem Schritt. Bleibt sie unverändert, geschieht
  nichts.
- **Dieselben Regeln wie `kard set`.** Was im Kopf anders ist, gilt als
  gesetzt oder entfernt: Eine fremde Karte braucht `--force`, ein
  Pflichtfeld darf nicht wegfallen. Auf Boards mit `sync: commit` oder
  `push` entsteht ein Commit, `edit <karte>` oder `assign …`, wenn sich
  `assignee` geändert hat. Bei `push` holt `kard` vorher den Stand der
  anderen.
- **Nichts geht verloren.** Eine Karte mit Änderungen ohne Commit öffnet
  `kard edit` gar nicht erst. Hätte die Kopie Fehler, die die Karte nicht
  hatte (E02 bis E05, E07), oder hat sich die Karte inzwischen geändert,
  bleibt die Karte, wie sie war, und `kard` nennt den Ort der Kopie.

Zu `kard rename`:

- **Nur der Dateiname ändert sich.** Die Karte bleibt in ihrer Spalte, ihr
  Inhalt bleibt, wie er war. Der neue Name muss ein gültiges Kürzel sein,
  das es im Board noch nicht gibt.
- **Verweise ziehen nach.** Jedes Feld vom Typ `cards` im selben Board, das
  die Karte nennt, ob als `druckvorlagen` oder als `<board>/druckvorlagen`,
  nennt danach den neuen Namen. Geändert wird nur diese Zeile des Kopfes.
  Das gilt auch für Karten anderer Personen, denn ihr Inhalt ändert sich
  nicht. Verweise aus anderen Boards und Erwähnungen im Text bleiben, wie
  sie sind; `kard lint` zeigt, was danach ins Leere zeigt.
- **Eine fremde Karte braucht `--force`,** wie bei jeder anderen Änderung.
- **Ein Commit.** Auf Boards mit `sync: commit` oder `push` enthält er die
  Karte und jede Karte, deren Verweis nachgezogen wurde. Keine davon darf
  Änderungen ohne Commit haben.

Zu `kard init`:

- **Es schreibt die `board.yaml` und sonst nichts um.** Keine vorhandene
  Datei wird geändert oder verschoben. Eine vorhandene `board.yaml` wird nie
  überschrieben. Ohne Angabe gilt der aktuelle Ordner; fehlt der genannte
  Ordner, entsteht er.
- **Vorhandene Ordner werden die Spalten.** Jeder Unterordner, der
  Markdown-Dateien enthält, direkt oder eine Ebene tiefer, wird eine Spalte,
  wenn sein Name als Spaltenkürzel taugt. Bekannte Namen kommen in die
  übliche Reihenfolge (`planning`, `backlog`, …, `working_on`, …, `done`),
  unbekannte dazwischen. `working_on`, `doing`, `in_progress` oder `wip`
  bekommt `doing: true`; `done`, `finished` oder `closed` bekommt
  `done: true`. Was nicht passt, ändert man danach in der `board.yaml`.
- **Ein Ordner ohne solche Unterordner** bekommt `backlog`, `working_on` und
  `done`.
- **`--columns`** nennt die Spalten selbst, in ihrer Reihenfolge.
- **Die `id`** kommt aus dem Ordnernamen, gebildet wie ein Kartenkürzel.
  Heißt der Ordner `board`, `kanban` oder `cards`, gilt der Name des Ordners
  darüber.
- **`--sync`** setzt den Git-Modus, Vorgabe `none`. `kard init` selbst
  committet nichts.
- **Das Board wird auf dem Rechner angemeldet**, wie mit `kard add`.
  `--no-add` lässt das weg. Trägt schon ein anderes Board dieselbe `id`,
  entsteht das Board trotzdem, bleibt aber unangemeldet, und `kard` sagt das.
- **Am Ende steht, was vorgefunden wurde:** die Spalten, die Zahl der Karten
  und wie viele davon eine Regel verletzen.

Zum Anlegen:

- `kard new` legt die Karte in die erste Spalte, wenn `-c` fehlt, und setzt
  `created` auf das heutige Datum.
- Der Dateiname entsteht aus dem Titel: klein geschrieben, Umlaute
  ausgeschrieben (`ä` wird `ae`), alles andere als `a–z` und `0–9` wird zu
  einem Bindestrich. Gibt es das Kürzel schon, lehnt `kard new` ab und verlangt
  `--id`.
- `--set` darf mehrfach stehen. Listen gehen als `tags=bug,ui` oder
  `tags=[bug, ui]`. Die Person kommt mit `--set assignee=sam`.

Zum Ändern:

- **Verschieben benennt nur die Datei um.** Der Text bleibt unberührt.
- **Felder ändern schreibt nur die Zeilen dieser Felder neu.** Andere Felder,
  Kommentare, die Reihenfolge im Kopf und der Text bleiben, wie sie sind. Ein
  neues Feld kommt ans Ende des Kopfes. Entfällt das letzte Feld, entfällt der
  Kopf.
- **Ein Schritt geschieht ganz oder gar nicht.** `kard take` weist zu und
  verschiebt; lehnt eine Regel eines von beidem ab, bleibt die Karte, wie sie
  war.
- **Eine Karte, die anderen gehört, ändert kein Befehl ohne `--force`**, in
  keiner Spalte. Wer selbst unter `assignee` steht, darf sie ändern, auch wenn
  dort weitere Namen stehen. Eine Karte, die niemandem gehört, darf jeder
  nehmen, verschieben und zuweisen.
- **Dazukommen ist auch eine Änderung.** `kard take --add` auf der Karte anderer
  braucht `--force`. Ohne das holt einen jemand dazu, der schon auf der Karte
  steht: `kard assign <karte> <person> --add`.
- **Ohne `--add` ersetzen `kard take` und `kard assign` alle Namen.** Verlässt die
  letzte Person die Karte, entfällt das Feld.
- **Pflichtfelder gelten beim Ändern.** Eine Karte kommt nicht in eine Spalte,
  deren Pflichtfeld ihr fehlt, und verliert kein Pflichtfeld ihrer Spalte.
  `kard new` legt keine Karte an, der eines fehlt. Mit `--force` geht es
  trotzdem, und `kard lint` meldet die Karte danach. Was einer Karte schon
  vorher fehlte, hindert andere Änderungen an ihr nicht.
- **Ohne Spalte mit `doing`** weist `kard take` nur zu.
- **Wer man ist, muss kardboard nur wissen, wenn es darauf ankommt:** bei
  `kard take`, bei `--mine` und bei Karten, die jemandem gehören.

Für alle Befehle gilt:

- Lesende Befehle kennen `--json`, für Skripte und Agenten. Ein Feld vom Typ
  `person` ist dort immer eine Liste, auch bei einem einzelnen Namen.
  `kard show --json` nennt auch den Fingerabdruck der Datei (`sum`).
- Mit `--json` steht ein Fehler als ein Objekt auf stderr: `error` (der
  Grund), `kind` (`refused`, `conflict`, `not_pushed`, `stale` oder `usage`)
  und `force`, wenn `--force` es trotzdem täte. stdout trägt nur, was der
  Befehl ausgibt.
- Nichts braucht ein Gegenüber. Text, den sonst der Editor liefert, nimmt
  `kard new --text` und `kard edit --file` aus einer Datei oder mit `-` von
  stdin. `kard edit --file … --base <sum>` speichert nur, wenn die Karte seit
  dem Lesen unverändert ist.
- Jeder Befehl hat `--help` (oder `-h`, `kard help <befehl>`) mit Beispielen.
- Rückgabecode `0` bei Erfolg, `1` bei einem Regelverstoß, bei einer
  Änderung, die wegen eines Konflikts zurückgenommen wurde, und bei einem
  Commit, der nicht gepusht werden konnte. `2` bei falschem Aufruf oder wenn
  der Befehl nicht wirken kann (kein Board gefunden, Karte oder Spalte
  unbekannt, Remote nicht erreichbar).
- Die Oberfläche tut nichts, was das CLI nicht kann. Jede Aktion dort ist genau
  einer dieser Befehle.

## Oberfläche

`kard ui` startet einen Webserver auf dem eigenen Rechner und zeigt alle
bekannten Boards. Steht man in einem Board, gehört es dazu, auch wenn es nicht
angemeldet ist.

- **Nur der eigene Rechner.** Der Server lauscht auf `127.0.0.1`, in der
  Vorgabe auf Port 4712, und beantwortet nur Anfragen, die `localhost` als
  Ziel nennen.
- **Ändern wie im CLI.** Karten lassen sich in eine andere Spalte ziehen, und
  die Kartenansicht bietet Nehmen, Abschließen, Verschieben, das Zuweisen
  von Personen und das Ändern von Titel, Feldern und Text. Jede dieser
  Handlungen ist genau der gleichnamige Befehl, mit denselben Regeln. Lehnt
  eine Regel ab, zeigt die Seite den Grund und bietet an, es trotzdem zu
  tun. Das entspricht `--force`.
- **Bearbeiten in einem Schritt.** Titel, Felder und Text ändert man in
  einem Bearbeiten-Modus, den Text als Markdown mit Vorschau. Was dort
  geändert wird, geht erst mit „Speichern“ als eine Änderung an die Karte,
  also als ein Commit `edit <karte>`. Der Kopf bleibt dabei bis auf die
  geänderten Felder, wie er war. Ein geleertes Feld entfällt wie bei
  `kard set <karte> feld=`. Abbrechen verwirft alles.
- **Kein Überschreiben.** Titel und Text werden beim Öffnen festgehalten,
  zusammen mit einem Fingerabdruck der Datei. Hat sich die Datei bis zum
  Speichern geändert, speichert die Seite den Text nicht, sagt das und
  behält den Entwurf. Das lässt sich nicht erzwingen, denn es würde
  überschreiben, was ein anderer geschrieben hat. Wer nur Felder ändert,
  ändert nur diese Zeilen.
- **Ziehen auf die Arbeitsspalte nimmt die Karte.** Gehört sie noch
  niemandem, wird man selbst eingetragen. Sonst ist Ziehen ein reines
  Verschieben. In der gemeinsamen Ansicht mehrerer Boards steht „in Arbeit“
  für die Arbeitsspalte und „erledigt“ für die erledigte Spalte des Boards
  der Karte. Auf „offen“ lässt sich dort nicht ziehen, weil es mehrere
  Spalten sein können.
- **Fremde Webseiten können nichts ändern.** Eine ändernde Anfrage muss von
  der eigenen Seite stammen: gleiche Herkunft, ein eigener Kopf in der
  Anfrage und JSON als Inhalt. Ein Formular oder Skript einer anderen Seite
  im selben Browser kann das nicht erfüllen.
- **Git wie im CLI.** Auf Boards mit `sync: commit` oder `push` macht jede
  Handlung denselben Commit wie der Befehl. Nimmt ein Konflikt die Änderung
  zurück oder gelingt der Push nicht, sagt die Seite das. Eine Karte mit
  Änderungen ohne Commit sagt in ihrer Ansicht, dass sie sich so nicht
  ändern lässt. Ein Board, dem das Repository oder der Upstream fehlt, zeigt
  die Seite nur an und nennt den Grund.
- **Abgleich mit dem Remote.** Für Boards mit `sync: push` fragt der Server
  jede Minute den Remote (`git fetch`). Das Menü nennt je Board den Stand:
  aktuell, wie viele Commits zu holen oder zu senden sind, oder dass der
  Remote nicht erreichbar ist. Ein Klick darauf ist `kard sync`. Solange etwas
  wartet, trägt der Menüknopf einen Punkt.
- **Sie folgt den Dateien.** Der Server prüft die Boards jede Sekunde auf
  Änderungen und meldet sie der offenen Seite. Nach neuen Boards unter `scan`
  sucht er alle 30 Sekunden und auf Wunsch sofort.
- **Eine Ansicht, kein Umschalter.** Was sie zeigt, folgt aus den angehakten
  Boards. Ist eines angehakt, erscheint es mit seinen eigenen Spalten. Sind es
  mehrere, liegen ihre Karten in drei gemeinsamen Spalten: offen, in Arbeit
  (`doing`), erledigt (`done`). Jede Karte zeigt dann, zu welchem Board sie
  gehört.
- **Ein- und Ausblenden.** Die Projektauswahl klappt aus der Kopfzeile auf.
  Das Kästchen blendet ein Board ein oder aus, ein Klick auf den Namen zeigt
  es allein. Welche Boards zu sehen sind, merkt sich der Browser. `hidden` in
  den Einstellungen ist die Vorgabe, solange dort noch nichts gewählt wurde.
- **Filter.** Ein Suchfeld und „Nur meine“ wirken auf alles, was zu sehen ist.
  „Nur meine“ steht im Menü. Solange es an ist, zeigt die Kopfzeile das an.
- **Hell und dunkel.** Die Seite folgt dem System. Im Menü lässt sich Hell
  oder Dunkel fest wählen; auch das merkt sich der Browser.
- **Verlauf aus Git.** Liegt ein Board in einem Git-Repository, zeigt die
  Seite, welche Commits es berührt haben, für das ganze Board und für jede
  Karte einzeln. Was ein Commit getan hat, liest kardboard aus seinem Inhalt
  und nicht aus seiner Nachricht: Eine Datei, die Git als von `backlog/` nach
  `done/` umbenannt führt, ist eine verschobene Karte, gleich wer sie wie
  committet hat. Das gilt für jedes Board, auch mit `sync: none`. Ein Board
  ohne Repository hat keinen Verlauf, und die Seite sagt das. Denselben
  Verlauf zeigt `kard log` im Terminal, eine Zeile je Commit.
- **Fehler bleiben sichtbar.** Eine fehlerhafte Karte ist markiert und zeigt
  ihre Verstöße, ein unbrauchbares Board zeigt den Grund.
- **Keine Anfrage nach außen.** Alles, was die Seite braucht, steckt im
  Programm.

## Vorschlag: Personen aus Git

**Stand:** Vorschlag, noch nicht umgesetzt. Wo er den Kapiteln oben
widerspricht, gelten bis zur Umsetzung die Kapitel oben.

Heute muss jeder einmal `user: <name>` eintragen, und der Name gilt für alle
Boards gleich. Dabei hat Git schon eine Identität, oft je Repository eine
andere, und die Commits von `kard` tragen sie ohnehin. Der Vorschlag nimmt
diese Identität, ohne dass jemand etwas einrichtet, und lässt sie
überschreiben, wo man das will.

### Wer man ist

kardboard bestimmt den eigenen Namen je Board. Es nimmt den ersten Eintrag,
den es findet:

| Nr. | Quelle | Gedacht für |
|---|---|---|
| 1 | `KARDBOARD_USER` | einen Aufruf, Skripte und Tests |
| 2 | `user` in `.kard/config.yaml`, der nächsten vom Board aus | einen anderen Namen in einem Repository oder Board |
| 3 | `user` in `~/.config/kardboard/config.yaml` | einen festen Namen in allen Boards |
| 4 | `git config user.email`, gelesen im Ordner des Boards | den Normalfall: nichts einrichten |
| 5 | der Kontoname im Betriebssystem | Rechner ohne Git oder ohne Git-Adresse |

Was ausdrücklich eingetragen ist, geht dem Abgeleiteten vor. Wer schon heute
`user: alex` eingetragen hat, merkt also keinen Unterschied.

Zu den Quellen:

- **Git je Board.** `git config user.email` wird im Ordner des Boards
  gelesen. So greift, was Git dort gilt, auch eine Adresse je Repository
  über `includeIf`. Liegt das Board in keinem Repository, gilt die globale
  Git-Adresse. kardboard liest die Git-Config nur und schreibt nie hinein.
- **Ein eigener Name je Repository oder Board** steht in `.kard/config.yaml`.
  Er ist für den, der dort nicht unter seiner Adresse erscheinen will.
- **Der Kontoname** ist der letzte Ausweg. Er wird klein geschrieben, unter
  Windows ohne den Teil vor `\`. Ist er danach kein gültiges Kürzel, fehlt
  ein Name, und `kard` sagt, wo man ihn einträgt. Der Kontoname kann auf
  jedem Rechner ein anderer sein. Für ein Board, das man allein nutzt, ist
  das gleich; im Team sollte ein Name aus Git oder den Einstellungen
  kommen.
- **`kard whoami`** zeigt den Namen und woher er kommt, mit Pfad der Datei,
  für das Board, in dem man steht, oder mit `--all` für jedes. `--json`
  gibt beides als Felder aus. `kard whoami --set <name>` schreibt den Namen
  in `.kard/config.yaml` an der Wurzel des Repositorys, ohne Repository in
  den Board-Ordner.

### Persönliche Einstellungen im Projekt: `.kard/`

```
projekt/
  .kard/
    .gitignore      enthält nur: *
    config.yaml     user: angelique-l
  board/
    board.yaml
```

- **Nur für einen selbst.** `.kard/` gehört dem, der den Klon hat. Was alle
  angeht, steht in der `board.yaml`. In jedem neuen Klon legt man `.kard/`
  neu an.
- **Wird nie committet.** Die `.gitignore` in `.kard/` schließt den ganzen
  Ordner aus, sich selbst eingeschlossen. Die `.gitignore` des Projekts
  bleibt unberührt. `kard` legt diese Datei an, wenn es den Ordner anlegt,
  und `kard lint` meldet einen Fehler, wenn Git eine Datei aus `.kard/`
  verfolgt (neue Regel E11).
- **Gesucht wird vom Board-Ordner aufwärts** bis zur Wurzel des
  Repositorys, ohne Repository nur im Board-Ordner. Die erste
  `.kard/config.yaml` gilt. So gilt eine an der Wurzel für alle Boards
  eines Repositorys, und ein Board kann trotzdem seine eigene haben.
- **Dieselbe Form wie `~/.config/kardboard/config.yaml`**, aber nur mit
  `user`. Jeder andere Schlüssel ist ein Fehler, bis es einen Grund für ihn
  gibt.

### Was in der Karte steht

Ein Wert vom Typ `person` ist ein Kürzel wie bisher oder eine E-Mail-Adresse.
kardboard schreibt die Adresse klein und vergleicht ohne Rücksicht auf
Groß- und Kleinschreibung.

```yaml
assignee: angelique.lambert@firma.de
```

```yaml
assignee: [angelique.lambert@firma.de, sam]
```

Die Datei bleibt ohne kardboard lesbar, und `grep` findet die Person. Die
Adresse ist dieselbe wie in den Commits des Boards. Wer sie in einem
öffentlichen Repository nicht zeigen will, nutzt dort eine
noreply-Adresse oder einen eigenen Namen in `.kard/config.yaml`.

Bestehende Karten mit Kürzeln bleiben gültig und werden nicht umgeschrieben.

### Andere zuweisen, ohne Adressen zu tippen

Ein Board kennt seine Leute schon, ohne Liste in der `board.yaml`:

- die Autoren der Commits, die das Board berührt haben, mit Name und Adresse
- die Personen, die schon in den Karten stehen

Git führt diese Personen über die `.mailmap` des Repositorys zusammen.
kardboard nutzt dieselbe Datei: Zwei Adressen, die sie einer Person zuordnet,
gelten auch auf Karten als dieselbe Person, etwa für `--mine` und für die
Frage, ob eine Karte einem selbst gehört.

`kard assign <karte> <person>` und `kard new --set assignee=…` nehmen eine
Person so:

1. Ist der Wert ein Kürzel oder eine Adresse, die das Board kennt, gilt sie.
2. Sonst sucht `kard` unter den bekannten Personen nach dem Anfang einer
   Adresse oder eines Namens, ohne Rücksicht auf Groß- und Kleinschreibung
   und Umlaute. `sam` findet `sam@firma.de` und Sam Berger. Genau ein
   Treffer gilt.
3. Mehrere Treffer sind ein Fehler, der sie nennt. Mit `--json` hat er die
   Art `ambiguous` und eine Liste `candidates`.
4. Kein Treffer: Ein gültiges Kürzel oder eine gültige Adresse wird so
   genommen, wie sie dasteht. Das ist eine neue Person. Hat das Feld
   `values`, gilt das wie bisher als Fehler.

### In der Oberfläche

- **Ein Monogramm statt des Namens.** Zwei Buchstaben, aus dem Namen in Git,
  sonst aus der Adresse vor dem `@` oder aus dem Kürzel: Angélique Lambert wird
  AL, `sam@firma.de` wird SA. Das Monogramm ist quadratisch und in Mono
  gesetzt, wie alles, was das Werkzeug sagt. Das eigene ist mit Tinte
  gefüllt, die anderen haben eine Haarlinie.
- **Name und Adresse beim Darüberfahren.** Zwei Personen mit gleichen
  Buchstaben unterscheidet erst das; mehr soll das Monogramm nicht leisten.
- **Zuweisen aus einer Auswahl** der bekannten Personen des Boards, mit
  demselben Suchen wie im CLI.
- **Wer man ist, gilt je Board.** „Nur meine“ meint in jedem angehakten
  Board den Namen, den kardboard für dieses Board bestimmt. Kommt er dort
  nur aus dem Kontonamen, sagt das Menü es.

### Was dafür noch zu klären ist

- **Umstellen bestehender Karten.** Ob es einen Befehl braucht, der ein
  Kürzel in allen Karten eines Boards durch eine Adresse ersetzt, oder ob
  ein eingetragenes `user` dafür reicht.
- **Kartenverweise auf Personen anderer Boards.** Eine Adresse ist über
  Boards hinweg dieselbe, ein Kürzel nicht unbedingt. „Nur meine“ über
  mehrere Boards hängt davon ab, wie man dort heißt.

## Offene Punkte

- **Reihenfolge in einer Spalte.** Der Ordner kennt keine. Entweder sortiert das
  Board nach einem Feld, oder die Reihenfolge braucht einen eigenen Ort.
- **Unterordner in Spalten.** Heute ignoriert. Denkbar als Archiv oder als
  Gruppe zusammengehöriger Karten.
- **Titel in der Überschrift statt im Kopf.** Gewählt, weil die Datei so in
  jedem Markdown-Betrachter richtig aussieht. Andere Werkzeuge führen den Titel
  im Kopf.
- **Dazukommen ohne `--force`.** Wer sich mit `kard take --add` einer Karte
  anschließt, nimmt niemandem etwas weg. Heute verlangt es trotzdem `--force`,
  damit die Regel für fremde Karten keine Ausnahme hat.
- **Grenze je Person.** `limit` zählt die Karten der Spalte, nicht die einer
  Person.
- **Sprache dieses Dokuments** vor einer Veröffentlichung.
