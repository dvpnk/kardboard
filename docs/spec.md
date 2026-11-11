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
| `doing` | nein | `true`: Hier liegt, woran gearbeitet wird. Ziel von `kb take` |
| `done` | nein | `true`: Karten hier gelten als erledigt. Ziel von `kb done` |
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
  eindeutig, über alle Spalten. Umbenennen heißt neue Karte; `kb rename` zieht
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
| `assignee` | `person` | wem die Karte gehört |
| `tags` | `list` | Schlagworte |
| `due` | `date` | Termin |
| `depends_on` | `cards` | Karten, die vorher fertig sein müssen |
| `created` | `date` | Anlagedatum, setzt `kb new` |

Ein eingebautes Feld darf unter `fields` enger gefasst werden: `tags` mit
festen `values`, `assignee` als Pflicht in einer Spalte. Der Typ bleibt
derselbe.

Verboten im Kopf sind `status`, `column`, `id` und `title`. Diese Angaben
stehen im Pfad, im Dateinamen oder in der Überschrift, und zwei Quellen für
dieselbe Angabe laufen auseinander.

### Wem eine Karte gehört

`assignee` nennt eine Person mit ihrem Kürzel. Das Feld gilt in jeder Spalte:
Eine Karte kann im Backlog schon für jemanden vorgemerkt sein, und eine
erledigte Karte behält, wer sie gemacht hat. Fehlt das Feld, gehört die Karte
niemandem.

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
| `person` | Kürzel einer Person, `[a-z0-9][a-z0-9-]*`, mit `values` nur diese | `reviewer: sam` |
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

`kb lint` prüft ein Board und endet mit Code 1, wenn es einen Fehler gibt. Damit
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

Aus der `board.yaml` erzeugt `kb schema` ein JSON Schema für den Kartenkopf.
Damit prüfen auch Editoren und fremde Werkzeuge.

## Git

`sync` in der `board.yaml` legt fest, was ein ändernder Befehl mit Git tut.

| Wert | Verhalten | Gedacht für |
|---|---|---|
| `none` | ändert nur Dateien | Board im Code-Repo |
| `commit` | ein Commit je Aktion, nur mit dieser Karte | eigenes Board-Repo, allein |
| `push` | holen, ein Commit, pushen; bei Ablehnung einmal neu versuchen | eigenes Board-Repo, im Team |

Commit-Nachrichten haben eine feste Form, damit `git log` das Protokoll des
Boards ist:

```
move export-als-pdf: backlog -> working_on
assign export-als-pdf: alex
edit export-als-pdf
new export-als-pdf
```

Bei `push` gilt: Hat inzwischen jemand dieselbe Karte bewegt oder dieselbe
Zeile geändert, nimmt kardboard die eigene Änderung zurück, übernimmt den
fremden Stand und meldet, wie die Karte jetzt aussieht.

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

Die Variable `KARDBOARD_USER` geht dem Eintrag `user` vor. Zwei angemeldete
Boards mit derselben `id` sind ein Fehler.

## CLI

Der Befehl heißt `kb`. Ohne weitere Angabe wirkt er auf das Board, in dem man
steht: kardboard sucht vom aktuellen Ordner aufwärts nach einer `board.yaml`
und danach in den Unterordnern des Git-Repos. Liegen dort mehrere Boards, muss
man eines wählen. `-C <ordner>` nennt den Board-Ordner direkt, `-b <id>` wählt
ein angemeldetes Board, `--all` alle.

Board:

| Befehl | Wirkung |
|---|---|
| `kb init [ordner]` | legt `board.yaml` und Spaltenordner an |
| `kb lint` | prüft das Board |
| `kb schema` | gibt das JSON Schema für den Kartenkopf aus |
| `kb ui [--port n] [--no-open]` | startet den lokalen Server und öffnet das Board im Browser |

Karten lesen:

| Befehl | Wirkung |
|---|---|
| `kb list [spalte] [--mine]` | listet Karten, mit `--mine` nur die eigenen, aus jeder Spalte |
| `kb show <karte>` | zeigt eine Karte |

Karten ändern:

| Befehl | Spalte | Person | Wirkung |
|---|---|---|---|
| `kb new "<titel>" [-c spalte] [--set feld=wert] [--id kürzel]` | | | legt eine Karte an |
| `kb move <karte> <spalte>` | wechselt | bleibt | verschiebt die Karte |
| `kb take <karte>` | geht in die erste Spalte mit `doing` | wird man selbst | nimmt die Karte in Arbeit |
| `kb done <karte>` | geht in die erste Spalte mit `done` | bleibt | schließt die Karte ab |
| `kb assign <karte> <person>` | bleibt | wird gesetzt | weist die Karte zu. `kb give` ist dasselbe |
| `kb unassign <karte>` | bleibt | entfällt | gibt die Karte frei |
| `kb set <karte> feld=wert …` | bleibt | bleibt | ändert Felder im Kopf, `feld=` entfernt eines |
| `kb edit <karte>` | | | öffnet die Karte im Editor |
| `kb rename <karte> <neu>` | | | benennt um und zieht Verweise nach |

Rechner:

| Befehl | Wirkung |
|---|---|
| `kb boards` | listet angemeldete und gefundene Boards |
| `kb add <pfad>` | meldet ein Board an |
| `kb remove <id>` | meldet ein Board ab, löscht nichts |
| `kb scan <ordner>` | nimmt einen Ordner in die Suche auf |
| `kb sync` | holt und pusht bei Boards mit `sync: push` |

Zum Anlegen:

- `kb new` legt die Karte in die erste Spalte, wenn `-c` fehlt, und setzt
  `created` auf das heutige Datum.
- Der Dateiname entsteht aus dem Titel: klein geschrieben, Umlaute
  ausgeschrieben (`ä` wird `ae`), alles andere als `a–z` und `0–9` wird zu
  einem Bindestrich. Gibt es das Kürzel schon, lehnt `kb new` ab und verlangt
  `--id`.
- `--set` darf mehrfach stehen. Listen gehen als `tags=bug,ui` oder
  `tags=[bug, ui]`. Die Person kommt mit `--set assignee=sam`.

Zum Ändern:

- **Verschieben benennt nur die Datei um.** Der Text bleibt unberührt.
- **Felder ändern schreibt nur die Zeilen dieser Felder neu.** Andere Felder,
  Kommentare, die Reihenfolge im Kopf und der Text bleiben, wie sie sind. Ein
  neues Feld kommt ans Ende des Kopfes. Entfällt das letzte Feld, entfällt der
  Kopf.
- **Ein Schritt geschieht ganz oder gar nicht.** `kb take` weist zu und
  verschiebt; lehnt eine Regel eines von beidem ab, bleibt die Karte, wie sie
  war.
- **Eine Karte, die jemand anderem gehört, ändert kein Befehl ohne `--force`**,
  in keiner Spalte. Eine Karte, die niemandem gehört, darf jeder nehmen,
  verschieben und zuweisen.
- **Pflichtfelder gelten beim Ändern.** Eine Karte kommt nicht in eine Spalte,
  deren Pflichtfeld ihr fehlt, und verliert kein Pflichtfeld ihrer Spalte.
  `kb new` legt keine Karte an, der eines fehlt. Mit `--force` geht es
  trotzdem, und `kb lint` meldet die Karte danach. Was einer Karte schon
  vorher fehlte, hindert andere Änderungen an ihr nicht.
- **Ohne Spalte mit `doing`** weist `kb take` nur zu.
- **Wer man ist, muss kardboard nur wissen, wenn es darauf ankommt:** bei
  `kb take`, bei `--mine` und bei Karten, die jemandem gehören.

Für alle Befehle gilt:

- Lesende Befehle kennen `--json`, für Skripte und Agenten.
- Rückgabecode `0` bei Erfolg, `1` bei einem Regelverstoß, `2` bei falschem
  Aufruf oder wenn es nichts gibt, worauf der Befehl wirken kann (kein Board
  gefunden, Karte oder Spalte unbekannt).
- Die Oberfläche tut nichts, was das CLI nicht kann. Jede Aktion dort ist genau
  einer dieser Befehle.

## Offene Punkte

- **Reihenfolge in einer Spalte.** Der Ordner kennt keine. Entweder sortiert das
  Board nach einem Feld, oder die Reihenfolge braucht einen eigenen Ort.
- **Unterordner in Spalten.** Heute ignoriert. Denkbar als Archiv oder als
  Gruppe zusammengehöriger Karten.
- **Titel in der Überschrift statt im Kopf.** Gewählt, weil die Datei so in
  jedem Markdown-Betrachter richtig aussieht. Andere Werkzeuge führen den Titel
  im Kopf.
- **Mehrere Personen je Karte.** Heute genau eine oder keine. `assignee`
  könnte neben einem Namen auch eine Liste annehmen, ohne dass bestehende
  Karten sich ändern. Offen ist dann, wem die Karte „gehört“ und was `kb take`
  mit den anderen Namen tut.
- **Grenze je Person.** `limit` zählt die Karten der Spalte, nicht die einer
  Person.
- **Sprache dieses Dokuments** vor einer Veröffentlichung.
