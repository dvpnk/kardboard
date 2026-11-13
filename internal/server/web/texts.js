// Texts of the page in English and German, and how dates are written. The
// page speaks the browser's language if it is German, English otherwise.

const TEXTS = {
  en: {
    filter: "Filter cards", projects: "Projects", only: "show only this project", all: "show all projects",
    allProjects: (n) => `All projects (${n})`, someProjects: (n, of) => `${n} of ${of} projects`, noProjects: "No project",
    pickHint: "Tick to show or hide. Click a name to see that project alone.",
    settings: "Settings", theme: "Appearance", system: "System", light: "Light", dark: "Dark",
    you: (name) => `You are @${name}`, mineOff: "show everyone's cards again",
    overdue: (date) => `Overdue since ${date}`,
    cardMenu: "Actions for this card", handOver: "Hand over …", to: (column) => `To ${column}`,
    allGood: "Nothing is wrong.", search: "Search cards", menuTitle: "Menu",
    filters: "Filter", overdueOnly: "Overdue", tags: "Tags", clearFilters: "Clear",
    showing: (n, of) => `${n} of ${of} cards`,
    liveOn: "Connected. The page follows changes to the files.",
    liveOff: "Not connected: `kard ui` is not running. What you see may be out of date.",
    history: "History", noGit: (name) => `${name} is not in a Git repository, so it has no history.`, noCommits: "No commits yet.",
    hMoved: (a, b) => `${a} → ${b}`, hAdded: (col) => `new in ${col}`, hEdited: "edited", hAlsoEdited: ", edited",
    hRenamed: (id) => `renamed from ${id}`, hRemoved: (col) => `removed from ${col}`, hCards: (n) => `${n} cards`, problems: "What is wrong", unusable: "This board cannot be shown",
    mine: "Only mine", mineHint: "kard does not know who you are. Set `user` in ~/.config/kardboard/config.yaml.",
    open: "Open", doing: "In progress", done: "Done", none: "nothing here", more: (n) => `${n} more`, less: "show fewer",
    boards: (n) => `${n} ${n === 1 ? "board" : "boards"}`, cards: (n) => `${n} ${n === 1 ? "card" : "cards"}`,
    errors: (n) => `${n} ${n === 1 ? "error" : "errors"}`, warnings: (n) => `${n} ${n === 1 ? "warning" : "warnings"}`,
    live: "live", offline: "offline", rescan: "look for new boards", close: "Close",
    noBoardsTitle: "No boards yet",
    noBoards: "Add the board of a project with `kard add <dir>`, or let kardboard find all boards below a directory with `kard scan <dir>`.",
    allHidden: "All projects are hidden. Tick one on the left.",
    noMatch: "No card matches.", column: "column", board: "board", limit: (n, max) => `${n} of ${max}`,
    take: "Take", finish: "Finish", moveTo: "Move to", addPerson: "add person", removePerson: (n) => `take ${n} off the card`,
    anyway: "Do it anyway", cancel: "Cancel", dismiss: "Dismiss",
    readOnly: (why) => `This board cannot be changed here: ${why}.`,
    remote: "Remote", upToDate: "up to date", toPull: (n) => `${n} to pull`, toPush: (n) => `${n} to push`, unreachable: "not reachable",
    syncNow: "Pull what is new and push what is waiting",
    notCommitted: "This card has changes that are not committed. Commit them first, then it can be changed here.",
    listHint: "a, b, c", clearValue: "Click again to remove", edit: "Edit card", editShort: "Edit", save: "Save", title: "Title", text: "Text", write: "Write", preview: "Preview", noText: "No text yet", file: "file",
    dates: "Dates", marks: "Size, milestone, tags", since: (d) => `since ${d}`,
    // The fields kard knows without board.yaml, when the board gives them no title.
    fieldNames: { assignee: "People", tags: "Tags", due: "Due", depends_on: "Depends on", created: "Created" },
  },
  de: {
    filter: "Karten filtern", projects: "Projekte", only: "nur dieses Projekt zeigen", all: "alle Projekte zeigen",
    allProjects: (n) => `Alle Projekte (${n})`, someProjects: (n, of) => `${n} von ${of} Projekten`, noProjects: "Kein Projekt",
    pickHint: "Haken blendet ein oder aus. Ein Klick auf den Namen zeigt das Projekt allein.",
    settings: "Einstellungen", theme: "Darstellung", system: "System", light: "Hell", dark: "Dunkel",
    you: (name) => `Du bist @${name}`, mineOff: "wieder die Karten aller zeigen",
    overdue: (date) => `Überfällig seit ${date}`,
    cardMenu: "Aktionen für diese Karte", handOver: "Übergeben …", to: (column) => `Nach ${column}`,
    allGood: "Alles in Ordnung.", search: "Karten suchen", menuTitle: "Menü",
    filters: "Filter", overdueOnly: "Überfällig", tags: "Schlagworte", clearFilters: "Zurücksetzen",
    showing: (n, of) => `${n} von ${of} Karten`,
    liveOn: "Verbunden. Die Seite folgt Änderungen an den Dateien.",
    liveOff: "Nicht verbunden: `kard ui` läuft nicht. Der Stand kann veraltet sein.",
    history: "Verlauf", noGit: (name) => `${name} liegt nicht in einem Git-Repository und hat deshalb keinen Verlauf.`, noCommits: "Noch keine Commits.",
    hMoved: (a, b) => `${a} → ${b}`, hAdded: (col) => `neu in ${col}`, hEdited: "geändert", hAlsoEdited: ", geändert",
    hRenamed: (id) => `umbenannt von ${id}`, hRemoved: (col) => `entfernt aus ${col}`, hCards: (n) => `${n} Karten`, problems: "Was nicht stimmt", unusable: "Dieses Board kann nicht gezeigt werden",
    mine: "Nur meine", mineHint: "kard weiß nicht, wer du bist. Trag `user` in ~/.config/kardboard/config.yaml ein.",
    open: "Offen", doing: "In Arbeit", done: "Erledigt", none: "nichts hier", more: (n) => `${n} weitere`, less: "weniger zeigen",
    boards: (n) => `${n} ${n === 1 ? "Board" : "Boards"}`, cards: (n) => `${n} ${n === 1 ? "Karte" : "Karten"}`,
    errors: (n) => `${n} Fehler`, warnings: (n) => `${n} ${n === 1 ? "Warnung" : "Warnungen"}`,
    live: "live", offline: "getrennt", rescan: "nach neuen Boards suchen", close: "Schließen",
    noBoardsTitle: "Noch keine Boards",
    noBoards: "Melde das Board eines Projekts mit `kard add <ordner>` an, oder lass kardboard mit `kard scan <ordner>` alle Boards unter einem Ordner finden.",
    allHidden: "Alle Projekte sind ausgeblendet. Hake links eines an.",
    noMatch: "Keine Karte passt.", column: "Spalte", board: "Board", limit: (n, max) => `${n} von ${max}`,
    take: "Nehmen", finish: "Abschließen", moveTo: "Verschieben nach", addPerson: "Person hinzufügen", removePerson: (n) => `${n} von der Karte nehmen`,
    anyway: "Trotzdem ausführen", cancel: "Abbrechen", dismiss: "Schließen",
    readOnly: (why) => `Dieses Board lässt sich hier nicht ändern: ${why}.`,
    remote: "Remote", upToDate: "aktuell", toPull: (n) => `${n} zu holen`, toPush: (n) => `${n} zu senden`, unreachable: "nicht erreichbar",
    syncNow: "Neues holen und Wartendes senden",
    notCommitted: "Diese Karte hat Änderungen ohne Commit. Erst committen, dann lässt sie sich hier ändern.",
    listHint: "a, b, c", clearValue: "Noch einmal klicken zum Entfernen", edit: "Karte bearbeiten", editShort: "Bearbeiten", save: "Speichern", title: "Titel", text: "Text", write: "Schreiben", preview: "Vorschau", noText: "Noch kein Text", file: "Datei",
    dates: "Zeitraum", marks: "Größe, Meilenstein, Tags", since: (d) => `seit ${d}`,
    // The fields kard knows without board.yaml, when the board gives them no title.
    fieldNames: { assignee: "Personen", tags: "Tags", due: "Fällig", depends_on: "Hängt ab von", created: "Angelegt" },
  },
};
export const lang = (navigator.language || "en").toLowerCase().startsWith("de") ? "de" : "en";
export const t = TEXTS[lang];
document.documentElement.lang = lang;

// Dates are written the way the browser's language writes them: 20.10.2026
// in German, Oct 20, 2026 in American English. The texts of the page only
// come in two languages, but dates follow the browser exactly.
export const LOCALE = navigator.language || "en";
const FULL_DATE = new Intl.DateTimeFormat(LOCALE, { dateStyle: "medium" });
const SHORT_DATE = new Intl.DateTimeFormat(LOCALE, { day: "numeric", month: "short" });

// day formats a date written as YYYY-MM-DD. With compact, a date in the
// current year is shown without the year, to fit on a card.
export function day(iso, compact = false) {
  const [y, m, d] = String(iso).split("-").map(Number);
  const date = new Date(y, m - 1, d); // local time: no shift across midnight
  if (!y || !m || !d || Number.isNaN(date.getTime())) return String(iso);
  return compact && y === new Date().getFullYear() ? SHORT_DATE.format(date) : FULL_DATE.format(date);
}

const relative = new Intl.RelativeTimeFormat(lang, { numeric: "always", style: "short" });
const relativeDay = new Intl.RelativeTimeFormat(lang, { numeric: "auto" });

// fromToday says how far a day ("2026-12-01") is from today: "tomorrow",
// "in 19 days", "3 days ago".
export function fromToday(iso) {
  const [y, m, d] = String(iso).split("-").map(Number);
  if (!y || !m || !d) return "";
  const now = new Date();
  const days = Math.round((new Date(y, m - 1, d) - new Date(now.getFullYear(), now.getMonth(), now.getDate())) / 86400000);
  return relativeDay.format(days, "day");
}
export function ago(date) {
  const seconds = (new Date(date).getTime() - Date.now()) / 1000;
  for (const [unit, size] of [["year", 31536000], ["month", 2592000], ["day", 86400], ["hour", 3600], ["minute", 60]]) {
    if (Math.abs(seconds) >= size) return relative.format(Math.round(seconds / size), unit);
  }
  return relative.format(Math.round(seconds / 60), "minute");
}
