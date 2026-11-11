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
| `assignee` | `person` | wem die Karte gehört, eine Person oder mehrere |
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

Die Variable `KARDBOARD_USER` geht dem Eintrag `user` vor.

Ein Board ist auf dem Rechner bekannt, wenn es unter `boards` steht oder unter
einem Ordner aus `scan` liegt. Die Suche geht höchstens sechs Ebenen tief und
lässt versteckte Ordner, `node_modules`, `vendor` und das Innere von Boards
aus. Zwei bekannte Boards mit derselben `id` sind ein Fehler: Das erste behält
die `id`, die anderen werden als unbrauchbar gemeldet.

`kb add`, `kb remove` und `kb scan` ändern diese Datei. Kommentare und die
Reihenfolge der Einträge bleiben erhalten, Leerzeilen nicht.

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
| `kb ui [--port n] [--no-open]` | startet den lokalen Server und zeigt alle bekannten Boards im Browser, siehe [Oberfläche](#oberfläche) |

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
| `kb take <karte> [--add]` | geht in die erste Spalte mit `doing` | wird man selbst, mit `--add` kommt man dazu | nimmt die Karte in Arbeit |
| `kb done <karte>` | geht in die erste Spalte mit `done` | bleibt | schließt die Karte ab |
| `kb assign <karte> <person>… [--add]` | bleibt | wird ersetzt, mit `--add` ergänzt | weist die Karte zu. `kb give` ist dasselbe |
| `kb unassign <karte> [person…]` | bleibt | entfällt, mit Namen nur diese | gibt die Karte frei |
| `kb set <karte> feld=wert …` | bleibt | bleibt | ändert Felder im Kopf, `feld=` entfernt eines |
| `kb edit <karte>` | | | öffnet die Karte im Editor |
| `kb rename <karte> <neu>` | | | benennt um und zieht Verweise nach |

Rechner:

| Befehl | Wirkung |
|---|---|
| `kb boards` | listet angemeldete und gefundene Boards |
| `kb add [ordner]` | meldet ein Board an. Der Ordner ist das Board oder liegt in seinem Projekt; ohne Angabe gilt der aktuelle |
| `kb remove <id\|ordner>` | meldet ein Board ab, löscht nichts |
| `kb scan <ordner>` | nimmt einen Ordner in die Suche auf, `--remove` nimmt ihn wieder heraus |
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
- **Eine Karte, die anderen gehört, ändert kein Befehl ohne `--force`**, in
  keiner Spalte. Wer selbst unter `assignee` steht, darf sie ändern, auch wenn
  dort weitere Namen stehen. Eine Karte, die niemandem gehört, darf jeder
  nehmen, verschieben und zuweisen.
- **Dazukommen ist auch eine Änderung.** `kb take --add` auf der Karte anderer
  braucht `--force`. Ohne das holt einen jemand dazu, der schon auf der Karte
  steht: `kb assign <karte> <person> --add`.
- **Ohne `--add` ersetzen `kb take` und `kb assign` alle Namen.** Verlässt die
  letzte Person die Karte, entfällt das Feld.
- **Pflichtfelder gelten beim Ändern.** Eine Karte kommt nicht in eine Spalte,
  deren Pflichtfeld ihr fehlt, und verliert kein Pflichtfeld ihrer Spalte.
  `kb new` legt keine Karte an, der eines fehlt. Mit `--force` geht es
  trotzdem, und `kb lint` meldet die Karte danach. Was einer Karte schon
  vorher fehlte, hindert andere Änderungen an ihr nicht.
- **Ohne Spalte mit `doing`** weist `kb take` nur zu.
- **Wer man ist, muss kardboard nur wissen, wenn es darauf ankommt:** bei
  `kb take`, bei `--mine` und bei Karten, die jemandem gehören.

Für alle Befehle gilt:

- Lesende Befehle kennen `--json`, für Skripte und Agenten. Ein Feld vom Typ
  `person` ist dort immer eine Liste, auch bei einem einzelnen Namen.
- Rückgabecode `0` bei Erfolg, `1` bei einem Regelverstoß, `2` bei falschem
  Aufruf oder wenn es nichts gibt, worauf der Befehl wirken kann (kein Board
  gefunden, Karte oder Spalte unbekannt).
- Die Oberfläche tut nichts, was das CLI nicht kann. Jede Aktion dort ist genau
  einer dieser Befehle.

## Oberfläche

`kb ui` startet einen Webserver auf dem eigenen Rechner und zeigt alle
bekannten Boards. Steht man in einem Board, gehört es dazu, auch wenn es nicht
angemeldet ist.

- **Nur der eigene Rechner.** Der Server lauscht auf `127.0.0.1`, in der
  Vorgabe auf Port 4712, und beantwortet nur Anfragen, die `localhost` als
  Ziel nennen.
- **Ändern wie im CLI.** Karten lassen sich in eine andere Spalte ziehen, und
  die Kartenansicht bietet Nehmen, Abschließen, Verschieben und das Zuweisen
  von Personen. Jede dieser Handlungen ist genau der gleichnamige Befehl, mit
  denselben Regeln. Lehnt eine Regel ab, zeigt die Seite den Grund und bietet
  an, es trotzdem zu tun. Das entspricht `--force`.
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
- **Boards mit `sync: commit` oder `push` zeigt die Seite nur an**, solange
  kb die Git-Schritte noch nicht kann.
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
  ohne Repository hat keinen Verlauf, und die Seite sagt das.
- **Fehler bleiben sichtbar.** Eine fehlerhafte Karte ist markiert und zeigt
  ihre Verstöße, ein unbrauchbares Board zeigt den Grund.
- **Keine Anfrage nach außen.** Alles, was die Seite braucht, steckt im
  Programm.

## Offene Punkte

- **Reihenfolge in einer Spalte.** Der Ordner kennt keine. Entweder sortiert das
  Board nach einem Feld, oder die Reihenfolge braucht einen eigenen Ort.
- **Unterordner in Spalten.** Heute ignoriert. Denkbar als Archiv oder als
  Gruppe zusammengehöriger Karten.
- **Titel in der Überschrift statt im Kopf.** Gewählt, weil die Datei so in
  jedem Markdown-Betrachter richtig aussieht. Andere Werkzeuge führen den Titel
  im Kopf.
- **Dazukommen ohne `--force`.** Wer sich mit `kb take --add` einer Karte
  anschließt, nimmt niemandem etwas weg. Heute verlangt es trotzdem `--force`,
  damit die Regel für fremde Karten keine Ausnahme hat.
- **Grenze je Person.** `limit` zählt die Karten der Spalte, nicht die einer
  Person.
- **Felder in der Oberfläche ändern.** Sie verschiebt und weist zu. Größe,
  Schlagworte und Text ändert man im Editor oder mit `kb set`.
- **Verlauf im CLI.** Den Verlauf gibt es nur in der Oberfläche, einen
  Befehl wie `kb log` noch nicht.
- **Abgleich mit dem Remote.** Ob Commits zu holen oder zu pushen sind, zeigt
  die Seite noch nicht. Das kommt mit `sync: push`.
- **`--all` für lesende Befehle.** `kb list --all` und `kb lint --all` über
  alle bekannten Boards gibt es noch nicht.
- **Sprache dieses Dokuments** vor einer Veröffentlichung.
