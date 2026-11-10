# kardboard: Format und CLI

**Stand:** Entwurf v0, zur Diskussion. Schlüssel, Feldnamen und Befehle sind
englisch und bleiben es. Der Text ist vorerst deutsch.

kardboard zeigt Karten als Board, die als Markdown-Dateien in einem Ordner
liegen. Dieses Dokument legt fest, wie der Ordner und die Dateien aussehen und
welche Befehle es gibt. Wie das Programm gebaut ist, steht hier nicht.

## Grundsätze

1. **Ein Board ist ein Ordner mit einer `board.yaml`.** Wo der Ordner liegt, ist
   egal: im Projekt, daneben oder als eigenes Repo.
2. **Der Ort ist der Zustand.** In welcher Spalte eine Karte liegt und wer an
   ihr arbeitet, ergibt sich aus dem Pfad, nie aus dem Inhalt der Datei.
   Verschieben ändert deshalb keine Zeile Text.
3. **Der Kopf trägt nur, was das Board filtert, sortiert oder prüft.** Alles
   andere ist Text und bleibt frei.
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
  working_on/                  Spalte mit Personenordnern
    alex/
      login-fehler-bei-umlauten.md
  done/
  reference/                   keine Spalte: wird ignoriert
```

- Spalten sind die Ordner, die `board.yaml` unter `columns` nennt. Alle anderen
  Ordner und Dateien im Board ignoriert kardboard. Dort ist Platz für Anhänge.
- Eine Karte ist eine `.md`-Datei direkt in einem Spaltenordner. In einer
  Spalte mit Personenordnern liegt sie genau eine Ebene tiefer:
  `<spalte>/<person>/<karte>.md`.
- Tiefere Unterordner in einer Spalte werden ignoriert (siehe offene Punkte).
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
    per_person: true
    limit: 3
  - id: done
    title: Erledigt
    done: true

fields:
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
| `per_person` | nein | `true`: Karten liegen in `<spalte>/<person>/`. Der Ordner ist die Zuweisung |
| `done` | nein | `true`: Karten hier gelten als erledigt |
| `limit` | nein | Höchstzahl Karten, bei `per_person` je Person. Überschreiten ist eine Warnung |

Unbekannte Schlüssel in der `board.yaml` sind ein Fehler.

## Karte

```markdown
---
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
  eindeutig, über alle Spalten und Personen. Umbenennen heißt neue Karte;
  `kb rename` zieht Verweise nach.
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
| `tags` | `list` | Schlagworte |
| `due` | `date` | Termin |
| `depends_on` | `cards` | Karten, die vorher fertig sein müssen |
| `created` | `date` | Anlagedatum, setzt `kb new` |

Ein eingebautes Feld darf unter `fields` enger gefasst werden, etwa `tags` mit
festen `values`. Der Typ bleibt derselbe.

Verboten im Kopf sind `status`, `column`, `assignee`, `id` und `title`. Diese
Angaben stehen im Pfad, im Dateinamen oder in der Überschrift, und zwei Quellen
für dieselbe Angabe laufen auseinander.

### Feldtypen

| Typ | Wert | Beispiel |
|---|---|---|
| `text` | eine Zeile Text | `owner: Einkauf` |
| `enum` | genau einer der `values` | `size: M` |
| `list` | Liste von Texten, mit `values` nur diese | `tags: [bug, ui]` |
| `date` | `JJJJ-MM-TT` | `due: 2026-12-01` |
| `number` | Zahl | `estimate: 3` |
| `bool` | `true` oder `false` | `blocked: true` |
| `cards` | Liste von Kartenverweisen | `depends_on: [druckvorlagen]` |

Schlüssel einer Felderklärung:

| Schlüssel | Bedeutung |
|---|---|
| `type` | einer der Typen oben, Pflicht |
| `title` | Anzeigename, sonst der Feldname |
| `values` | erlaubte Werte, Pflicht bei `enum`, möglich bei `list` |
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
| E04 | Verbotenes Feld im Kopf (`status`, `column`, `assignee`, `id`, `title`) |
| E05 | Wert passt nicht zum Typ oder steht nicht in `values` |
| E06 | Pflichtfeld fehlt für die Spalte, in der die Karte liegt |
| E07 | Titelzeile `# …` fehlt oder steht nicht an erster Stelle |
| E08 | Dateiname ist kein gültiges Kürzel |
| E09 | Derselbe Dateiname kommt mehrfach im Board vor |
| E10 | Karte liegt direkt in einer Spalte mit Personenordnern |
| E11 | Verweis auf eine Karte, die es im Board nicht gibt |

Warnungen:

| Nr. | Regel |
|---|---|
| W01 | Verweis in ein Board, das nicht angemeldet ist |
| W02 | `limit` einer Spalte überschritten |

Eine fehlerhafte Karte wird überall angezeigt und als fehlerhaft markiert, nie
ausgeblendet. `kb move` lehnt ab, wenn der Karte ein Pflichtfeld der Zielspalte
fehlt; mit `--force` geht es trotzdem.

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
move export-als-pdf: backlog -> working_on/alex
edit export-als-pdf
new export-als-pdf
```

Bei `push` gilt: Hat inzwischen jemand dieselbe Karte bewegt, nimmt kardboard
die eigene Bewegung zurück, übernimmt den fremden Stand und meldet, wo die Karte
jetzt liegt.

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
| `user` | eigenes Kürzel, `[a-z0-9][a-z0-9-]*`. Name des Personenordners |
| `boards` | einzeln angemeldete Boards |
| `scan` | Ordner, in denen kardboard nach `board.yaml` sucht |
| `hidden` | Boards, die in der Gesamtansicht ausgeblendet sind |

Zwei angemeldete Boards mit derselben `id` sind ein Fehler.

## CLI

Der Befehl heißt `kb`. Ohne weitere Angabe wirkt er auf das Board, in dem man
steht: kardboard sucht vom aktuellen Ordner aufwärts nach einer `board.yaml`
und danach in den Unterordnern des Git-Repos. `-b <id>` wählt ein angemeldetes
Board, `--all` alle.

Board:

| Befehl | Wirkung |
|---|---|
| `kb init [ordner]` | legt `board.yaml` und Spaltenordner an |
| `kb lint` | prüft das Board |
| `kb schema` | gibt das JSON Schema für den Kartenkopf aus |
| `kb ui [--port n] [--no-open]` | startet den lokalen Server und öffnet das Board im Browser |

Karten:

| Befehl | Wirkung |
|---|---|
| `kb new "<titel>" [-c spalte] [--set feld=wert]` | legt eine Karte an, Dateiname aus dem Titel |
| `kb list [spalte] [--mine]` | listet Karten |
| `kb show <karte>` | zeigt eine Karte |
| `kb edit <karte>` | öffnet die Karte im Editor |
| `kb set <karte> feld=wert` | ändert ein Feld im Kopf |
| `kb move <karte> <spalte> [--as person]` | verschiebt eine Karte |
| `kb take <karte>` | nimmt die Karte in Arbeit: erste Spalte mit `per_person`, eigener Ordner |
| `kb give <karte> <person>` | übergibt die Karte |
| `kb done <karte>` | verschiebt in die erste Spalte mit `done` |
| `kb rename <karte> <neu>` | benennt um und zieht Verweise nach |

Rechner:

| Befehl | Wirkung |
|---|---|
| `kb boards` | listet angemeldete und gefundene Boards |
| `kb add <pfad>` | meldet ein Board an |
| `kb remove <id>` | meldet ein Board ab, löscht nichts |
| `kb scan <ordner>` | nimmt einen Ordner in die Suche auf |
| `kb sync` | holt und pusht bei Boards mit `sync: push` |

Für alle Befehle gilt:

- Lesende Befehle kennen `--json`, für Skripte und Agenten.
- Eine Karte im Ordner einer anderen Person bewegt oder ändert kein Befehl ohne
  `--force`.
- Rückgabecode `0` bei Erfolg, `1` bei einem Regelverstoß, `2` bei falschem
  Aufruf.
- Die Oberfläche tut nichts, was das CLI nicht kann. Jede Aktion dort ist genau
  einer dieser Befehle.

## Offene Punkte

- **Reihenfolge in einer Spalte.** Der Ordner kennt keine. Entweder sortiert das
  Board nach einem Feld, oder die Reihenfolge braucht einen eigenen Ort.
- **Unterordner in Spalten ohne Personen.** Heute ignoriert. Denkbar als Archiv
  oder als Gruppe zusammengehöriger Karten.
- **Titel in der Überschrift statt im Kopf.** Gewählt, weil die Datei so in
  jedem Markdown-Betrachter richtig aussieht. Andere Werkzeuge führen den Titel
  im Kopf.
- **Zuweisung nur über den Ordner.** Eine Karte hat damit nur in einer
  `per_person`-Spalte eine Person, und immer genau eine.
- **Sprache dieses Dokuments** vor einer Veröffentlichung.
