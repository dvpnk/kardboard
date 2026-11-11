// kardboard web UI. It shows what `kb ui` reads from disk and follows
// changes. Cards are changed by asking the server for the same actions the
// command line has. No framework and no build step: the page is rebuilt from
// the state whenever the state changes.

import { render as markdown } from "./md.js";

// ── Texts ───────────────────────────────────────────────────────────────────

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
    liveOff: "Not connected: `kb ui` is not running. What you see may be out of date.",
    history: "History", noGit: (name) => `${name} is not in a Git repository, so it has no history.`, noCommits: "No commits yet.",
    hMoved: (a, b) => `${a} → ${b}`, hAdded: (col) => `new in ${col}`, hEdited: "edited", hAlsoEdited: ", edited",
    hRenamed: (id) => `renamed from ${id}`, hRemoved: (col) => `removed from ${col}`, hCards: (n) => `${n} cards`, problems: "What is wrong", unusable: "This board cannot be shown",
    mine: "Only mine", mineHint: "kb does not know who you are. Set `user` in ~/.config/kardboard/config.yaml.",
    open: "Open", doing: "In progress", done: "Done", none: "nothing here", more: (n) => `${n} more`, less: "show fewer",
    boards: (n) => `${n} ${n === 1 ? "board" : "boards"}`, cards: (n) => `${n} ${n === 1 ? "card" : "cards"}`,
    errors: (n) => `${n} ${n === 1 ? "error" : "errors"}`, warnings: (n) => `${n} ${n === 1 ? "warning" : "warnings"}`,
    live: "live", offline: "offline", rescan: "look for new boards", close: "Close",
    noBoardsTitle: "No boards yet",
    noBoards: "Add the board of a project with `kb add <dir>`, or let kardboard find all boards below a directory with `kb scan <dir>`.",
    allHidden: "All projects are hidden. Tick one on the left.",
    noMatch: "No card matches.", column: "column", board: "board", limit: (n, max) => `${n} of ${max}`,
    take: "Take", finish: "Finish", moveTo: "Move to", addPerson: "add person", removePerson: (n) => `take ${n} off the card`,
    anyway: "Do it anyway", cancel: "Cancel", dismiss: "Dismiss",
    readOnly: (why) => `This board cannot be changed here: ${why}.`,
    remote: "Remote", upToDate: "up to date", toPull: (n) => `${n} to pull`, toPush: (n) => `${n} to push`, unreachable: "not reachable",
    syncNow: "Pull what is new and push what is waiting",
    notCommitted: "This card has changes that are not committed. Commit them first, then it can be changed here.",
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
    liveOff: "Nicht verbunden: `kb ui` läuft nicht. Der Stand kann veraltet sein.",
    history: "Verlauf", noGit: (name) => `${name} liegt nicht in einem Git-Repository und hat deshalb keinen Verlauf.`, noCommits: "Noch keine Commits.",
    hMoved: (a, b) => `${a} → ${b}`, hAdded: (col) => `neu in ${col}`, hEdited: "geändert", hAlsoEdited: ", geändert",
    hRenamed: (id) => `umbenannt von ${id}`, hRemoved: (col) => `entfernt aus ${col}`, hCards: (n) => `${n} Karten`, problems: "Was nicht stimmt", unusable: "Dieses Board kann nicht gezeigt werden",
    mine: "Nur meine", mineHint: "kb weiß nicht, wer du bist. Trag `user` in ~/.config/kardboard/config.yaml ein.",
    open: "Offen", doing: "In Arbeit", done: "Erledigt", none: "nichts hier", more: (n) => `${n} weitere`, less: "weniger zeigen",
    boards: (n) => `${n} ${n === 1 ? "Board" : "Boards"}`, cards: (n) => `${n} ${n === 1 ? "Karte" : "Karten"}`,
    errors: (n) => `${n} Fehler`, warnings: (n) => `${n} ${n === 1 ? "Warnung" : "Warnungen"}`,
    live: "live", offline: "getrennt", rescan: "nach neuen Boards suchen", close: "Schließen",
    noBoardsTitle: "Noch keine Boards",
    noBoards: "Melde das Board eines Projekts mit `kb add <ordner>` an, oder lass kardboard mit `kb scan <ordner>` alle Boards unter einem Ordner finden.",
    allHidden: "Alle Projekte sind ausgeblendet. Hake links eines an.",
    noMatch: "Keine Karte passt.", column: "Spalte", board: "Board", limit: (n, max) => `${n} von ${max}`,
    take: "Nehmen", finish: "Abschließen", moveTo: "Verschieben nach", addPerson: "Person hinzufügen", removePerson: (n) => `${n} von der Karte nehmen`,
    anyway: "Trotzdem ausführen", cancel: "Abbrechen", dismiss: "Schließen",
    readOnly: (why) => `Dieses Board lässt sich hier nicht ändern: ${why}.`,
    remote: "Remote", upToDate: "aktuell", toPull: (n) => `${n} zu holen`, toPush: (n) => `${n} zu senden`, unreachable: "nicht erreichbar",
    syncNow: "Neues holen und Wartendes senden",
    notCommitted: "Diese Karte hat Änderungen ohne Commit. Erst committen, dann lässt sie sich hier ändern.",
  },
};
const lang = (navigator.language || "en").toLowerCase().startsWith("de") ? "de" : "en";
const t = TEXTS[lang];
document.documentElement.lang = lang;

// ── State ───────────────────────────────────────────────────────────────────

const stored = (key, fallback) => {
  try { return JSON.parse(localStorage.getItem("kb." + key)) ?? fallback; } catch { return fallback; }
};
const store = (key, value) => {
  try { localStorage.setItem("kb." + key, JSON.stringify(value)); } catch { /* private mode: keep it in memory */ }
};

const state = {
  data: null,           // answer of /api/state
  error: "",            // why the state could not be loaded
  live: false,          // whether the event stream is connected
  hidden: stored("hidden", null), // keys of hidden boards; null = use the settings' default
  mine: stored("mine", false),
  overdue: false,       // show only cards past their due date
  tags: [],             // show only cards with one of these tags; empty = all
  filter: "",
  expanded: new Set(),  // columns shown in full, as "boardKey/columnId"
  open: null,           // { board, id, detail } of the card in the side panel
  notice: null,         // { text, retry } shown at the bottom: an error, or a refusal that can be forced
  menu: null,           // which menu of the top bar is open: "projects", "settings" or none
  history: null,        // { rows, missing } while the history panel is open, see loadHistory
  problems: false,      // whether the panel with what is wrong is open
  cardMenu: null,       // { board, card, top, right } of the card whose menu is open
  focusPerson: false,   // put the cursor into the person field once the card panel is drawn
  theme: stored("theme", "system"), // "system", "light" or "dark"
  syncing: null,        // key of the board that is being synced with its remote right now
};

// The theme is applied to the document; theme.js already did so before the
// first paint, this keeps it in step when the user changes it.
function applyTheme() {
  if (state.theme === "system") delete document.documentElement.dataset.theme;
  else document.documentElement.dataset.theme = state.theme;
}

// The card being dragged, as { board, card }. While it is set the page is
// not rebuilt: replacing the dragged element would end the drag.
let dragging = null;
let redrawAfterDrag = false;

const COLLAPSE_AFTER = 8;
const PALETTE = ["#2f9e44", "#7950f2", "#e8590c", "#1c7ed6", "#c2255c", "#0c8599", "#e67700", "#5c7cfa"];

// A board keeps its colour as long as the set of boards does not change.
const colourOf = (board) => PALETTE[Math.max(0, state.data.boards.indexOf(board)) % PALETTE.length];

const isHidden = (board) => {
  if (state.hidden === null) return state.data.hidden.includes(board.id);
  return state.hidden.includes(board.key);
};
const visibleBoards = () => state.data.boards.filter((b) => !isHidden(b));
const people = (card) => card.fields.assignee ?? [];

const today = () => new Date().toISOString().slice(0, 10);

// Dates are written the way the browser's language writes them: 20.10.2026
// in German, Oct 20, 2026 in American English. The texts of the page only
// come in two languages, but dates follow the browser exactly.
const LOCALE = navigator.language || "en";
const FULL_DATE = new Intl.DateTimeFormat(LOCALE, { dateStyle: "medium" });
const SHORT_DATE = new Intl.DateTimeFormat(LOCALE, { day: "numeric", month: "short" });

// day formats a date written as YYYY-MM-DD. With compact, a date in the
// current year is shown without the year, to fit on a card.
function day(iso, compact = false) {
  const [y, m, d] = String(iso).split("-").map(Number);
  const date = new Date(y, m - 1, d); // local time: no shift across midnight
  if (!y || !m || !d || Number.isNaN(date.getTime())) return String(iso);
  return compact && y === new Date().getFullYear() ? SHORT_DATE.format(date) : FULL_DATE.format(date);
}

// isLate says whether a card is past its due date and not done.
function isLate(card, board) {
  const due = card.fields.due;
  return !!due && due < today() && !board.columns.find((c) => c.id === card.column)?.done;
}

// activeFilters counts the filters that are on, the search aside.
function activeFilters() {
  return (state.mine && state.data.user ? 1 : 0) + (state.overdue ? 1 : 0) + state.tags.length;
}

// matches says whether a card passes the filters and the search.
function matches(card, board) {
  if (state.mine && state.data.user && !people(card).includes(state.data.user)) return false;
  if (state.overdue && !isLate(card, board)) return false;
  if (state.tags.length && !state.tags.some((tag) => (card.fields.tags ?? []).includes(tag))) return false;
  const q = state.filter.trim().toLowerCase();
  if (!q) return true;
  const hay = [card.id, card.title, board.name, ...people(card), ...(card.fields.tags ?? [])].join(" ").toLowerCase();
  return q.split(/\s+/).every((word) => hay.includes(word));
}

// ── Loading ─────────────────────────────────────────────────────────────────

async function load(rescan = false) {
  try {
    const res = await fetch("/api/state" + (rescan ? "?rescan" : ""));
    if (!res.ok) throw new Error(await res.text());
    state.data = await res.json();
    state.error = "";
  } catch (err) {
    state.error = String(err.message || err);
  }
  if (state.open) await loadDetail();
  if (state.history) await loadHistory();
  draw();
}

const getJSON = async (url) => {
  const res = await fetch(url);
  return res.ok ? res.json() : null;
};

async function loadDetail() {
  const { board, id } = state.open;
  const where = `board=${encodeURIComponent(board)}&id=${encodeURIComponent(id)}`;
  try {
    const [detail, log] = await Promise.all([
      getJSON("/api/card?" + where),
      getJSON(`/api/history?board=${encodeURIComponent(board)}&card=${encodeURIComponent(id)}&limit=30`),
    ]);
    state.open = detail ? { board, id, detail, log } : null; // the card is gone
  } catch {
    state.open = null;
  }
}

async function openCard(board, card) {
  state.history = null;
  state.problems = false;
  state.open = { board: board.id, id: card.id, detail: null };
  await loadDetail();
  draw();
}

// loadHistory reads the Git history of every visible board and merges it
// into one list, newest first. Boards outside a Git repository are named
// instead.
async function loadHistory() {
  const boards = visibleBoards().filter((b) => b.id && !b.error);
  const logs = await Promise.all(boards.map((b) =>
    getJSON(`/api/history?board=${encodeURIComponent(b.id)}&limit=60`).catch(() => null)));
  const rows = [];
  const missing = [];
  logs.forEach((log, i) => {
    if (!log?.available) { missing.push(boards[i]); return; }
    for (const entry of log.entries) rows.push({ board: boards[i], entry });
  });
  // Compare moments, not texts: commits come from machines in different
  // time zones, so their dates do not sort as strings.
  rows.sort((a, b) => Date.parse(b.entry.date) - Date.parse(a.entry.date));
  state.history = { rows: rows.slice(0, 120), missing, branch: boards.length === 1 ? logs[0]?.branch : "" };
}

async function openHistory() {
  state.open = null;
  state.problems = false;
  state.history = { rows: [], missing: [], branch: "" };
  await loadHistory();
  draw();
}

// ── Changing ────────────────────────────────────────────────────────────────

// act asks the server for one action on one card. The actions are those of
// the command line. If a rule of the board refuses, the user may repeat the
// action with force, like --force.
async function act(board, cardId, action, extra = {}, force = false) {
  state.notice = null;
  try {
    const res = await fetch("/api/action", {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-Kardboard": "1" },
      body: JSON.stringify({ board: board.id, card: cardId, action, ...extra, force }),
    });
    if (!res.ok) {
      const problem = await res.json().catch(() => ({ error: res.statusText }));
      state.notice = {
        text: problem.error,
        retry: problem.refused && !force ? () => act(board, cardId, action, extra, true) : null,
      };
    } else {
      // The change happened, but may not have reached the remote.
      const done = await res.json().catch(() => ({}));
      if (done.warning) state.notice = { text: done.warning, retry: null };
    }
  } catch (err) {
    state.notice = { text: String(err.message || err), retry: null };
  }
  await load();
}

// syncBoard pulls and pushes one board with sync: push, like `kb sync`.
async function syncBoard(board) {
  if (state.syncing) return;
  state.notice = null;
  state.syncing = board.key;
  draw();
  try {
    const res = await fetch("/api/sync", {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-Kardboard": "1" },
      body: JSON.stringify({ board: board.id }),
    });
    const answer = await res.json().catch(() => ({ error: res.statusText }));
    // Success needs no words: the entry in the menu then says "up to date".
    if (!res.ok) state.notice = { text: answer.error, retry: null };
  } catch (err) {
    state.notice = { text: String(err.message || err), retry: null };
  }
  state.syncing = null;
  await load();
}

// remoteState says in a few words how a board stands against its remote.
function remoteState(board) {
  const git = board.git;
  const parts = [git.behind > 0 && t.toPull(git.behind), git.ahead > 0 && t.toPush(git.ahead)].filter(Boolean);
  if (git.unreachable) parts.push(t.unreachable);
  return parts.length ? parts.join(" · ") : t.upToDate;
}

// dropAction says what dropping the dragged card on a column does, or null
// if it cannot be dropped there. Dropping an unassigned card on the column
// for work in progress takes it; everything else is a plain move.
function dropAction(target) {
  if (!dragging || !target || dragging.card.column === target.id) return null;
  const { board, card } = dragging;
  const firstDoing = board.columns.find((c) => c.doing);
  if (target === firstDoing && people(card).length === 0 && state.data.user) return { action: "take" };
  return { action: "move", extra: { column: target.id } };
}

function follow() {
  const events = new EventSource("/api/events");
  events.onopen = () => { state.live = true; draw(); };
  events.onerror = () => { state.live = false; draw(); };
  events.addEventListener("change", () => load());
}

// ── DOM helper ──────────────────────────────────────────────────────────────

// h builds an element. Text always goes in as text nodes, never as markup.
function h(tag, props = {}, ...children) {
  const el = document.createElement(tag);
  for (const [key, value] of Object.entries(props)) {
    if (value === false || value == null) continue;
    if (key === "class") el.className = value;
    else if (key.startsWith("on")) el.addEventListener(key.slice(2), value);
    else if (key in el && typeof value !== "string") el[key] = value;
    else el.setAttribute(key, value);
  }
  el.append(...children.flat(Infinity).filter((c) => c !== false && c != null));
  return el;
}

const dot = (colour) => {
  const el = h("span", { class: "dot", "aria-hidden": "true" });
  el.style.background = colour;
  return el;
};

// icon draws a small line icon. The shapes are drawn here, so the page needs
// no icon font and no image files.
const ICONS = {
  // a clock: the card is past its due date
  clock: ['<circle cx="12" cy="12" r="9"/>', '<path d="M12 7v5l3.5 2"/>'],
  // filter menu: one's own cards
  person: ['<circle cx="12" cy="8" r="3.6"/>', '<path d="M5 20c.9-3.7 3.6-5.6 7-5.6s6.1 1.9 7 5.6"/>'],
  // top bar: filters, search, the history of the boards
  filter: ['<path d="M4 5.5h16l-6.2 7.2V18l-3.6 1.8v-7.1z"/>'],
  search: ['<circle cx="11" cy="11" r="6.5"/>', '<path d="m16 16 4.5 4.5"/>'],
  history: ['<path d="M3.5 12a8.5 8.5 0 1 0 2.6-6.1L3.5 8.4"/>', '<path d="M3.5 3.8v4.6h4.6"/>', '<path d="M12 7.5V12l3 2"/>'],
  // history rows: a card moved, appeared, went away, or was changed
  moved: ['<path d="M5 12h14M13 6l6 6-6 6"/>'],
  added: ['<path d="M12 5v14M5 12h14"/>'],
  removed: ['<path d="M5 12h14"/>'],
  commit: ['<circle cx="12" cy="12" r="3.5"/>', '<path d="M12 3v5.5M12 15.5V21"/>'],
  // the menu: bring a board level with its remote
  remote: ['<path d="M4 11a8 8 0 0 1 14-4.6l2 2.1"/>', '<path d="M20 4v4.5h-4.5"/>', '<path d="M20 13a8 8 0 0 1-14 4.6l-2-2.1"/>', '<path d="M4 20v-4.5h4.5"/>'],
  // a bug: the card breaks a rule of the format
  bug: [
    '<ellipse cx="12" cy="14" rx="5" ry="6"/>',
    '<path d="M9 8.6a3 3 0 0 1 6 0"/>',
    '<path d="M9.5 5.5 8 3.5M14.5 5.5 16 3.5"/>',
    '<path d="M7 12H3.5M7.2 16.5 4 18M17 12h3.5M16.8 16.5 20 18M12 9v11"/>',
  ],
};
function icon(name, label = "") {
  const holder = label
    ? h("span", { class: "icon icon-" + name, title: label, role: "img", "aria-label": label })
    : h("span", { class: "icon icon-" + name, "aria-hidden": "true" });
  holder.innerHTML = // fixed markup from the table above, never card content
    '<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" ' +
    'stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">' + ICONS[name].join("") + "</svg>";
  return holder;
}

// brand draws the name with the mark in front of it. The mark is the same
// shape as favicon.svg and takes the colour of the text, so it follows the theme.
function brand() {
  const mark = h("span", { class: "mark", "aria-hidden": "true" });
  mark.innerHTML = // fixed markup, never card content
    '<svg viewBox="0 0 32 32" width="16" height="16" fill="currentColor" aria-hidden="true">' +
    '<path d="M2 2h6v28H2z"/>' +
    '<path fill-rule="evenodd" d="M24 2h6v28h-6C22 25 6 23 6 16S22 7 24 2ZM24 8C20 12 12 12 12 16s8 4 12 8Z"/></svg>';
  return h("h1", { class: "brand" }, mark, "kardboard");
}

// code turns `like this` in a UI text into <code> elements.
const code = (text) => text.split("`").map((part, i) => (i % 2 ? h("code", {}, part) : part));

// ── Drawing ─────────────────────────────────────────────────────────────────
//
// There is one view and no switch. What it shows follows from the ticked
// projects: a single one appears as its board, several as an overview.

function draw() {
  if (dragging) { redrawAfterDrag = true; return; }
  const app = document.getElementById("app");
  const focused = document.activeElement?.id;
  const scroll = { main: app.querySelector("main")?.scrollTop ?? 0, detail: app.querySelector(".detail")?.scrollTop ?? 0 };

  if (!state.data) {
    app.replaceChildren(h("header", { class: "top" }, brand()),
      h("main", {}, state.error ? h("div", { class: "lane-error" }, state.error) : ""));
    return;
  }

  app.replaceChildren(...[
    drawTop(),
    h("div", { class: "shell" + (state.history || state.problems || state.open?.detail ? " with-detail" : "") },
      drawMain(), state.history ? drawHistory() : state.problems ? drawProblems() : state.open?.detail ? drawDetail() : null),
    state.notice && drawNotice(),
    state.cardMenu && drawCardMenu(),
  ].filter(Boolean));

  if (state.focusPerson && state.open?.detail) {
    state.focusPerson = false;
    app.querySelector(".actions input[name=person]")?.focus();
    return;
  }

  if (focusNext) {
    document.getElementById(focusNext)?.focus();
    focusNext = null;
  } else if (focused) {
    const el = document.getElementById(focused);
    el?.focus();
    if (el?.id === "filter") el.setSelectionRange(el.value.length, el.value.length);
  }
  const main = app.querySelector("main");
  if (main) main.scrollTop = scroll.main;
  const detail = app.querySelector(".detail");
  if (detail) detail.scrollTop = scroll.detail;
}

function drawNotice() {
  const { text, retry } = state.notice;
  const close = () => { state.notice = null; draw(); };
  return h("div", { class: "notice", role: "alert" },
    h("span", { class: "text" }, text),
    retry && h("button", { type: "button", class: "primary", onclick: retry }, t.anyway),
    h("button", { type: "button", onclick: close }, retry ? t.cancel : t.dismiss));
}

// The top bar holds the few things that are always needed: which projects,
// the filter, what is wrong, and a menu for the rest.
function drawTop() {
  return h("header", { class: "top" },
    brand(),
    // What is shown, from coarse to fine: which projects, and which of
    // their cards.
    drawProjectMenu(),
    drawFilterMenu(),
    h("span", { class: "spacer" }),
    // Nothing is said while all is well. Only a lost connection shows up
    // here, quietly; the menu explains it.
    !state.live && h("span", { class: "offline", title: t.liveOff.replaceAll("`", "") }, t.offline),
    drawSettingsMenu());
}

// The usual shortcut for "find something here": Cmd+K on a Mac, Ctrl+K
// elsewhere. "/" works as well.
const IS_MAC = /Mac|iPhone|iPad/.test(navigator.platform || navigator.userAgent);
const SEARCH_KEY = IS_MAC ? "⌘K" : "Ctrl+K";
const SEARCH_KEYS = IS_MAC ? ["⌘", "K"] : ["Ctrl", "K"];

// The id of a field to put the cursor into once the page is rebuilt.
let focusNext = null;

// openFilters opens the filter menu with the cursor in its search field.
function openFilters() {
  state.cardMenu = null;
  state.menu = "filters";
  focusNext = "filter";
  draw();
}

// drawFilterMenu holds everything that narrows down which cards are shown:
// the search on top, then the filters. Each filter says how many cards it
// would leave, so one can tell before switching it on. The button is dark
// while something narrows the view, and says what: the search text and how
// many filters.
function drawFilterMenu() {
  const me = state.data.user;
  const on = activeFilters();
  const text = state.filter.trim();
  const all = visibleBoards().flatMap((board) => board.cards.map((card) => ({ card, board })));
  const count = (test) => all.filter(test).length;
  const tagCounts = new Map();
  for (const { card } of all) for (const tag of card.fields.tags ?? []) tagCounts.set(tag, (tagCounts.get(tag) ?? 0) + 1);
  const tags = [...tagCounts.keys()].sort();
  // Tags that no visible card has any more cannot be switched off by hand.
  state.tags = state.tags.filter((tag) => tagCounts.has(tag));
  const toggleTag = (tag) => {
    state.tags = state.tags.includes(tag) ? state.tags.filter((x) => x !== tag) : [...state.tags, tag];
    draw();
  };
  const clear = () => { state.filter = ""; state.overdue = false; state.tags = []; setMine(false); };

  // One filter: a mark, its name, how many cards it leaves, and a switch.
  const row = (mark, label, n, checked, onchange, disabled = false) => h("label", { class: "filter-row" + (disabled ? " off" : "") },
    icon(mark), h("span", { class: "name" }, label), h("span", { class: "count" }, String(n)),
    h("input", { type: "checkbox", class: "switch", role: "switch", checked, disabled, onchange }));

  const label = [
    icon("filter"),
    text && h("span", { class: "term" }, text),
    on > 0 && h("span", { class: "n" }, String(on)),
  ];
  const open = state.menu === "filters";
  return h("div", { class: "menu-wrap" },
    h("button", {
      type: "button", class: "menu-button icon" + (on > 0 || text ? " active" : "") + (open ? " open" : ""),
      "aria-haspopup": "true", "aria-expanded": String(open), title: `${t.filters} (${SEARCH_KEY})`, "aria-label": t.filters,
      onclick: () => { if (open) { state.menu = null; draw(); } else openFilters(); },
    }, label),
    open && h("div", { class: "menu filters" },
      h("label", { class: "searchbox" },
        icon("search"),
        h("input", {
          id: "filter", type: "search", placeholder: t.search, value: state.filter, "aria-label": t.search,
          oninput: (e) => { state.filter = e.target.value; draw(); },
        }),
        // The keys that bring one here. They make way for the text.
        !state.filter && h("span", { class: "keys", "aria-hidden": "true" }, SEARCH_KEYS.map((k) => h("kbd", {}, k)))),
      h("div", { class: "filter-rows" },
        row("person", t.mine, me ? count(({ card }) => people(card).includes(me)) : 0,
          state.mine && !!me, (e) => setMine(e.target.checked), !me),
        row("clock", t.overdueOnly, count(({ card, board }) => isLate(card, board)),
          state.overdue, (e) => { state.overdue = e.target.checked; draw(); })),
      !me && h("p", { class: "hint" }, code(t.mineHint)),
      tags.length > 0 && [
        h("h2", {}, t.tags),
        h("div", { class: "tag-list" }, tags.map((tag) => h("button", {
          type: "button", class: "tag-pill" + (state.tags.includes(tag) ? " on" : ""),
          "aria-pressed": String(state.tags.includes(tag)), onclick: () => toggleTag(tag),
        }, tag, h("span", { class: "count" }, String(tagCounts.get(tag)))))),
      ],
      h("div", { class: "filter-foot" },
        h("span", {}, t.showing(count(({ card, board }) => matches(card, board)), all.length)),
        (on > 0 || text) && h("button", { type: "button", onclick: clear }, t.clearFilters))));
}

function setMine(on) {
  state.mine = on;
  store("mine", on);
  draw();
}

// menu draws a button that opens a panel below it. Only one menu is open at
// a time; a click elsewhere or Escape closes it.
function menu(name, label, title, content, kind = "") {
  const open = state.menu === name;
  return h("div", { class: "menu-wrap" },
    h("button", {
      type: "button", class: "menu-button " + kind + (open ? " open" : ""), "aria-haspopup": "true", "aria-expanded": String(open), title,
      onclick: () => { state.cardMenu = null; state.menu = open ? null : name; draw(); },
    }, label),
    open && h("div", { class: "menu " + name }, content()));
}

function drawProjectMenu() {
  const all = state.data.boards;
  const visible = visibleBoards();
  const setHidden = (keys) => { state.hidden = keys; store("hidden", keys); draw(); };
  const toggle = (board) => {
    const hidden = new Set(all.filter(isHidden).map((b) => b.key));
    hidden.has(board.key) ? hidden.delete(board.key) : hidden.add(board.key);
    setHidden([...hidden]);
  };
  const alone = (board) => {
    const isAlone = visible.length === 1 && visible[0] === board;
    setHidden(isAlone ? [] : all.filter((b) => b !== board).map((b) => b.key));
  };

  let label;
  if (visible.length === 1) label = [dot(colourOf(visible[0])), h("span", { class: "name" }, visible[0].name)];
  else if (visible.length === 0) label = [h("span", { class: "name" }, t.noProjects)];
  else if (visible.length === all.length) label = [h("span", { class: "name" }, t.allProjects(all.length))];
  else label = [h("span", { class: "name" }, t.someProjects(visible.length, all.length))];
  label.push(h("span", { class: "caret", "aria-hidden": "true" }, "▾"));

  return menu("projects", label, t.projects, () => [
    all.map((b) => {
      const isAlone = visible.length === 1 && visible[0] === b;
      return h("div", { class: "project" + (isHidden(b) ? " off" : "") },
        h("input", { type: "checkbox", checked: !isHidden(b), "aria-label": b.name, onchange: () => toggle(b) }),
        h("button", { type: "button", class: "pick", title: (isAlone ? t.all : t.only) + "\n" + b.root, onclick: () => alone(b) },
          dot(colourOf(b)),
          h("span", { class: "name" }, b.name),
          h("span", { class: "count" }, b.error ? "!" : String(b.cards.length))));
    }),
    h("p", { class: "hint" }, t.pickHint),
    h("button", { class: "linkish", type: "button", onclick: () => load(true) }, t.rescan),
  ]);
}

// countProblems sums up what is wrong on the visible boards.
function countProblems() {
  const boards = visibleBoards();
  return {
    errors: boards.reduce((n, b) => n + b.errors + (b.error ? 1 : 0), 0),
    warnings: boards.reduce((n, b) => n + b.warnings, 0),
  };
}

// drawProblems is the one place that says what is wrong: boards that cannot
// be shown, cards that break a rule, exceeded limits. A card in the list
// opens that card.
function drawProblems() {
  const close = () => { state.problems = false; draw(); };
  const groups = visibleBoards().map((board) => {
    const rows = [];
    if (board.error) rows.push(h("p", { class: "problem" }, h("strong", {}, t.unusable), h("span", {}, board.error)));
    for (const p of board.problems) {
      rows.push(h("p", { class: "problem" }, h("strong", {}, p.path), h("span", {}, `${p.code} ${p.message}`)));
    }
    for (const card of board.cards) {
      const found = card.problems ?? [];
      if (found.length === 0) continue;
      rows.push(h("button", { type: "button", class: "problem", onclick: () => openCard(board, card) },
        h("strong", {}, code(card.title)), found.map((p) => h("span", {}, `${p.code} ${p.message}`))));
    }
    return rows.length ? [h("h2", { class: "group" }, dot(colourOf(board)), board.name), rows] : null;
  }).filter(Boolean);

  return h("aside", { class: "detail issues", "aria-label": t.problems },
    h("div", { class: "detail-head" },
      h("div", { class: "where" }),
      h("button", { class: "close", type: "button", "aria-label": t.close, title: t.close, onclick: close }, "×")),
    h("h1", {}, t.problems),
    groups.length ? groups : h("p", { class: "readonly" }, t.allGood));
}

// The menu behind the three dots holds everything that is not needed all
// the time: the history, what is wrong, the filter for one's own cards, the
// appearance, and whether the page is connected. Its button carries a small
// mark while something is wrong.
function drawSettingsMenu() {
  const me = state.data.user;
  const { errors, warnings } = countProblems();
  const setTheme = (theme) => { state.theme = theme; store("theme", theme); applyTheme(); draw(); };
  const go = (fn) => () => { state.menu = null; fn(); };
  // Boards that push to a remote. Each gets an entry that says how it
  // stands and brings it level on a click.
  const remotes = visibleBoards().filter((b) => b.sync === "push" && b.git && !b.git.error);
  const waiting = remotes.some((b) => b.git.behind > 0 || b.git.ahead > 0 || b.git.unreachable);
  const label = [
    h("span", { "aria-hidden": "true" }, "⋯"),
    errors + warnings > 0 ? h("span", { class: "pip " + (errors > 0 ? "bad" : "warn") })
      : waiting && h("span", { class: "pip news" }),
  ];
  const count = [errors > 0 && t.errors(errors), warnings > 0 && t.warnings(warnings)].filter(Boolean).join(" · ");

  return menu("settings", label, t.menuTitle, () => [
    h("button", { type: "button", class: "menu-item", onclick: go(openHistory) }, icon("history"), h("span", {}, t.history)),
    h("button", {
      type: "button", class: "menu-item",
      onclick: go(() => { state.open = null; state.history = null; state.problems = true; draw(); }),
    }, icon("bug"), h("span", {}, t.problems), count && h("span", { class: "count" + (errors > 0 ? " bad" : " warn") }, count)),
    remotes.length > 0 && [
      h("h2", {}, t.remote),
      remotes.map((b) => h("button", {
        type: "button", class: "menu-item", title: t.syncNow, disabled: state.syncing !== null,
        onclick: () => syncBoard(b),
      }, icon("remote"), h("span", { class: "name" }, b.name),
      h("span", { class: "count" + (b.git.unreachable ? " warn" : "") }, state.syncing === b.key ? "…" : remoteState(b)))),
    ],
    h("h2", {}, t.theme),
    h("div", { class: "segments", role: "group", "aria-label": t.theme },
      ["system", "light", "dark"].map((theme) => h("button", {
        type: "button", class: state.theme === theme ? "on" : "", "aria-pressed": String(state.theme === theme),
        onclick: () => setTheme(theme),
      }, t[theme]))),
    h("hr"),
    h("p", { class: "hint status" }, h("span", { class: "live" + (state.live ? " on" : "") }), h("span", {}, code(state.live ? t.liveOn : t.liveOff))),
    me && h("p", { class: "hint" }, t.you(me)),
  ], "icon");
}

function drawMain() {
  if (state.error) return h("main", {}, h("div", { class: "lane-error" }, state.error));
  if (state.data.boards.length === 0) {
    return h("main", {}, h("div", { class: "empty" }, h("h2", {}, t.noBoardsTitle), h("p", {}, code(t.noBoards))));
  }
  const boards = visibleBoards();
  if (boards.length === 0) return h("main", {}, h("div", { class: "empty" }, h("p", {}, t.allHidden)));
  // One project shows its own columns. Several projects have different
  // columns, so their cards meet in three shared ones.
  return h("main", {}, boards.length === 1 ? drawLane(boards[0]) : drawOverview(boards));
}

// drawLane draws one board with its own columns. It has no heading: the top
// bar already names the project and counts what is wrong with it.
function drawLane(board) {
  if (board.error) return h("div", { class: "empty" }, h("h2", {}, t.unusable), h("p", {}, board.error));

  return h("section", { class: "lane" },
    h("div", { class: "columns" }, board.columns.map((col) => {
      const all = board.cards.filter((c) => c.column === col.id);
      const shown = all.filter((c) => matches(c, board));
      const over = col.limit > 0 && all.length > col.limit;
      return drawColumn(`${board.key}/${col.id}`, col.title,
        col.limit > 0 ? h("span", { class: over ? "over" : "" }, t.limit(all.length, col.limit)) : String(shown.length),
        shown.map((c) => drawCard(c, board, { done: col.done })),
        () => (dragging?.board === board ? col : null), col.doing);
    })));
}

function drawOverview(boards) {
  const groups = { open: [], doing: [], done: [] };
  for (const board of boards) {
    if (board.error) continue;
    const kind = new Map(board.columns.map((c) => [c.id, c.done ? "done" : c.doing ? "doing" : "open"]));
    for (const card of board.cards) {
      if (matches(card, board)) groups[kind.get(card.column)].push({ card, board });
    }
  }
  return [
    h("div", { class: "columns" }, Object.entries(groups).map(([kind, entries]) =>
      drawColumn("overview/" + kind, t[kind], String(entries.length),
        entries.map(({ card, board }) => drawCard(card, board, { done: kind === "done", project: true })),
        // The shared columns stand for the dragged card's own columns. "Open"
        // may be several of them, so a card cannot be dropped there.
        () => (kind === "open" ? null : dragging?.board.columns.find((c) => (kind === "done" ? c.done : c.doing)) ?? null),
        kind === "doing"))),
  ];
}

// drawColumn draws one column. target returns the board column that a card
// dropped here goes to, for the card being dragged right now.
function drawColumn(key, title, count, cards, target, doing = false) {
  const full = state.expanded.has(key);
  const shown = full ? cards : cards.slice(0, COLLAPSE_AFTER);
  const rest = cards.length - shown.length;
  const toggle = () => { full ? state.expanded.delete(key) : state.expanded.add(key); draw(); };
  const el = h("div", { class: "column" + (doing ? " doing" : "") },
    h("div", { class: "column-head" }, h("span", {}, title), h("span", {}, count)),
    cards.length === 0 && h("p", { class: "none" }, state.filter || activeFilters() > 0 ? t.noMatch : t.none),
    shown,
    rest > 0 && h("button", { class: "more", type: "button", onclick: toggle }, "+ " + t.more(rest)),
    full && cards.length > COLLAPSE_AFTER && h("button", { class: "more", type: "button", onclick: toggle }, t.less));

  el.addEventListener("dragover", (e) => {
    if (!dropAction(target())) return;
    e.preventDefault(); // this is what allows the drop
    e.dataTransfer.dropEffect = "move";
    el.classList.add("drop");
  });
  el.addEventListener("dragleave", (e) => { if (!el.contains(e.relatedTarget)) el.classList.remove("drop"); });
  el.addEventListener("drop", (e) => {
    const what = dropAction(target());
    if (!what) return;
    e.preventDefault();
    const { board, card } = dragging;
    endDrag();
    act(board, card.id, what.action, what.extra);
  });
  return el;
}

function endDrag() {
  dragging = null;
  document.querySelectorAll(".drop, .dragging").forEach((el) => el.classList.remove("drop", "dragging"));
  if (redrawAfterDrag) { redrawAfterDrag = false; draw(); }
}

// How many names a card shows before it sums up the rest as "+2".
const MAX_NAMES = 2;

// drawCard draws one card. Every fact has its own fixed place, so cards
// with more or fewer facts look alike and nothing jumps from one to the next:
//
//   title ................................ marks (overdue, broken)
//   size, tags ............................. due date
//   project ................................. people
//
// The second and third line appear only if the card has something for them.
// Neither wraps; what does not fit is cut off and stays in the card's panel.
function drawCard(card, board, { done = false, project = false } = {}) {
  const problems = (card.problems ?? []).filter((p) => p.severity === "error");
  const isOpen = state.open?.board === board.id && state.open?.id === card.id;
  const due = card.fields.due;
  const late = isLate(card, board);

  const facts = [];
  for (const field of board.fields) {
    const value = card.fields[field.name];
    if (value == null) continue;
    if (field.type === "enum") facts.push(h("span", { class: "chip", title: field.title }, String(value)));
    else if (field.name === "tags") facts.push(...value.map((tag) => h("span", { class: "tag" }, tag)));
  }
  const who = people(card);
  const names = who.slice(0, MAX_NAMES).map((name) =>
    h("span", { class: "who" + (name === state.data.user ? " me" : "") }, "@" + name));
  if (who.length > MAX_NAMES) names.push(h("span", { class: "who" }, "+" + (who.length - MAX_NAMES)));

  const menuOpen = state.cardMenu?.board === board && state.cardMenu?.card.id === card.id;
  const el = h("div", {
    class: "card" + (done ? " done" : "") + (isOpen ? " open" : ""),
    role: "button", tabindex: "0", title: card.path,
    onclick: () => openCard(board, card),
    onkeydown: (e) => { if (e.target === el && (e.key === "Enter" || e.key === " ")) { e.preventDefault(); openCard(board, card); } },
  },
    h("span", { class: "head" },
      h("span", { class: "title" }, code(card.title)),
      // The marks sit at the right edge. They make way for the three dots,
      // which only show under the pointer or with keyboard focus.
      h("span", { class: "marks" + (menuOpen ? " aside" : "") },
        late && icon("clock", t.overdue(day(due))),
        problems.length > 0 && icon("bug", problems.map((p) => `${p.code} ${p.message}`).join("\n"))),
      board.writable && h("button", {
        type: "button", class: "card-menu-button" + (menuOpen ? " open" : ""),
        "aria-label": t.cardMenu, title: t.cardMenu, "aria-haspopup": "true", "aria-expanded": String(menuOpen),
        onclick: (e) => {
          e.stopPropagation();
          const box = e.currentTarget.getBoundingClientRect();
          state.menu = null;
          state.cardMenu = menuOpen ? null : { board, card, top: box.bottom + 4, right: window.innerWidth - box.right };
          draw();
        },
      }, "⋯")),
    (facts.length > 0 || due) && h("span", { class: "line" },
      h("span", { class: "facts" }, facts),
      due && h("span", { class: "due", title: day(due) }, "→ " + day(due, true))),
    (project || who.length > 0) && h("span", { class: "line" },
      project ? h("span", { class: "proj" }, dot(colourOf(board)), h("span", { class: "proj-name" }, board.name)) : h("span"),
      who.length > 0 && h("span", { class: "people", title: who.map((n) => "@" + n).join(" ") }, names)));

  if (board.writable) {
    el.draggable = true;
    el.addEventListener("dragstart", (e) => {
      dragging = { board, card };
      e.dataTransfer.effectAllowed = "move";
      e.dataTransfer.setData("text/plain", card.id);
      el.classList.add("dragging");
    });
    el.addEventListener("dragend", endDrag);
  }
  return el;
}

// drawCardMenu draws the menu of one card: what can be done with it without
// opening it. Each entry is an action of the command line. The entries for
// moving are the other columns of the card's own board, so this also works
// in the shared view, where "open" stands for several columns.
function drawCardMenu() {
  const { board, card, top, right } = state.cardMenu;
  const who = people(card);
  const me = state.data.user;
  const doing = board.columns.find((c) => c.doing);
  const done = board.columns.find((c) => c.done);
  const mineAlone = who.length === 1 && who[0] === me;
  const run = (action, extra) => { state.cardMenu = null; act(board, card.id, action, extra); };
  const item = (label, onclick) => h("button", { type: "button", role: "menuitem", onclick }, label);

  const el = h("div", { class: "menu card-menu", role: "menu", "aria-label": t.cardMenu },
    me && !(mineAlone && (!doing || card.column === doing.id)) && item(t.take, () => run("take")),
    item(t.handOver, () => { state.cardMenu = null; state.focusPerson = true; openCard(board, card); }),
    done && card.column !== done.id && item(t.finish, () => run("done")),
    h("hr"),
    board.columns.filter((c) => c.id !== card.column && c !== done)
      .map((c) => item(t.to(c.title), () => run("move", { column: c.id }))));
  el.style.top = top + "px";
  el.style.right = right + "px";
  return el;
}

// ── History ─────────────────────────────────────────────────────────────────

const relative = new Intl.RelativeTimeFormat(lang, { numeric: "always", style: "short" });
function ago(date) {
  const seconds = (new Date(date).getTime() - Date.now()) / 1000;
  for (const [unit, size] of [["year", 31536000], ["month", 2592000], ["day", 86400], ["hour", 3600], ["minute", 60]]) {
    if (Math.abs(seconds) >= size) return relative.format(Math.round(seconds / size), unit);
  }
  return relative.format(Math.round(seconds / 60), "minute");
}

// describe says in words what a commit did to one card.
function describe(change, board) {
  const column = (id) => board.columns.find((c) => c.id === id)?.title ?? id;
  const also = change.edited ? t.hAlsoEdited : "";
  switch (change.kind) {
    case "moved": return t.hMoved(column(change.from), column(change.to)) + also;
    case "added": return t.hAdded(column(change.to));
    case "removed": return t.hRemoved(column(change.from));
    case "renamed": return t.hRenamed(change.was) + also;
    default: return t.hEdited;
  }
}

// drawHistoryRow draws one commit. If it changed exactly one card, the row
// is about that card and a click opens it; otherwise it shows the commit's
// own subject. inCard is true in a card's own history, where the card need
// not be named again.
function drawHistoryRow({ board, entry }, { inCard = false, project = false } = {}) {
  const cards = entry.changes.filter((c) => c.kind !== "other");
  const one = cards.length === 1 ? cards[0] : null;
  const card = one && board.cards.find((c) => c.id === one.card);

  // A card that moved, appeared or went away is the news itself: the row
  // names the card and says where it went. A commit that changed a card's
  // text says more in its own subject, so that leads and the card follows.
  const placeChanged = one && ["moved", "added", "removed"].includes(one.kind);
  let headline = entry.subject;
  let what;
  if (inCard && one) { headline = describe(one, board); what = entry.subject; }
  else if (placeChanged) { headline = card?.title ?? one.card; what = describe(one, board); }
  else if (one) what = card?.title ?? one.card;
  else if (cards.length > 1) what = t.hCards(cards.length);
  else what = entry.changes.map((c) => c.path).slice(0, 3).join(", ");

  const kind = placeChanged ? one.kind : "commit";
  const content = [
    icon(kind, ""),
    h("span", { class: "headline" }, project && dot(colourOf(board)), code(headline)),
    h("time", { datetime: entry.date, title: new Date(entry.date).toLocaleString(LOCALE) }, ago(entry.date)),
    h("span", { class: "what" }, [what, entry.author, entry.short].filter(Boolean).join(" · ")),
  ];
  const title = `${entry.subject}\n${entry.author} · ${entry.short}`;
  if (card && !inCard) {
    return h("li", {}, h("button", { type: "button", class: "hist", title, onclick: () => openCard(board, card) }, content));
  }
  return h("li", {}, h("div", { class: "hist", title }, content));
}

function drawHistory() {
  const { rows, missing, branch } = state.history;
  const several = visibleBoards().length > 1;
  const close = () => { state.history = null; draw(); };
  return h("aside", { class: "detail history", "aria-label": t.history },
    h("div", { class: "detail-head" },
      h("div", { class: "where" }, branch || ""),
      h("button", { class: "close", type: "button", "aria-label": t.close, title: t.close, onclick: close }, "×")),
    h("h1", {}, t.history),
    missing.map((b) => h("p", { class: "readonly" }, t.noGit(b.name))),
    rows.length === 0 && missing.length === 0 && h("p", { class: "readonly" }, t.noCommits),
    h("ul", { class: "hist-list" }, rows.map((row) => drawHistoryRow(row, { project: several }))));
}

function drawDetail() {
  const card = state.open.detail;
  const board = state.data.boards.find((b) => b.id === card.board);
  const column = board?.columns.find((c) => c.id === card.column);
  const close = () => { state.open = null; draw(); };

  const rows = [];
  for (const field of board?.fields ?? []) {
    const value = card.fields[field.name];
    if (value == null) continue;
    const shown = field.type === "date" ? day(value) : Array.isArray(value) ? value.join(", ") : String(value);
    rows.push(h("dt", {}, field.title), h("dd", {}, shown));
  }

  // The title is shown as the panel's heading, so leave it out of the text.
  const body = card.body.replace(/^\s*# .*\n?/, "");
  const text = h("div", { class: "md" });
  text.innerHTML = markdown(body); // md.js escapes everything it is given

  return h("aside", { class: "detail", "aria-label": card.title },
    h("div", { class: "detail-head" },
      h("div", { class: "where" },
        board && h("span", { class: "proj" }, dot(colourOf(board)), board.name), " · ", column?.title ?? card.column,
        h("br"), card.path),
      h("button", { class: "close", type: "button", "aria-label": t.close, title: t.close, onclick: close }, "×")),
    h("h1", {}, code(card.title)),
    board && drawActions(board, card, column),
    rows.length > 0 && h("dl", { class: "fields" }, rows),
    (card.problems ?? []).length > 0 && h("ul", { class: "problems" },
      card.problems.map((p) => h("li", {}, `${p.code} ${p.message}`))),
    text,
    board && state.open.log?.available && state.open.log.entries.length > 0 && [
      h("h2", { class: "section" }, t.history),
      h("ul", { class: "hist-list" }, state.open.log.entries.map((entry) => drawHistoryRow({ board, entry }, { inCard: true }))),
    ]);
}

// drawActions draws what can be done with the open card. Each control is
// one action of the command line: take, done, move, assign, unassign.
function drawActions(board, card, column) {
  if (!board.writable) return h("p", { class: "readonly" }, t.readOnly(board.git?.error ?? board.sync));
  // A card with changes that are not committed cannot be changed through
  // kb on a board that commits: say so before the action is refused.
  const unsaved = board.git?.dirty?.includes(card.id);
  const who = people(card);
  const me = state.data.user;
  const doing = board.columns.find((c) => c.doing);
  const done = board.columns.find((c) => c.done);
  const mineAlone = who.length === 1 && who[0] === me;
  const canTake = me && !(mineAlone && (!doing || card.column === doing.id));

  const add = (e) => {
    e.preventDefault();
    const input = e.target.elements.person;
    const names = input.value.split(",").map((n) => n.trim().toLowerCase()).filter(Boolean);
    if (names.length) act(board, card.id, "assign", { people: names, add: true });
  };

  return h("div", { class: "actions" },
    unsaved && h("p", { class: "readonly" }, t.notCommitted),
    h("div", { class: "row" },
      canTake && h("button", { type: "button", class: "primary", onclick: () => act(board, card.id, "take") }, t.take),
      done && !column?.done && h("button", { type: "button", onclick: () => act(board, card.id, "done") }, t.finish),
      h("label", { class: "move" }, t.moveTo,
        h("select", {
          "aria-label": t.moveTo,
          onchange: (e) => { if (e.target.value) act(board, card.id, "move", { column: e.target.value }); },
        },
          h("option", { value: "" }, "…"),
          board.columns.filter((c) => c.id !== card.column).map((c) => h("option", { value: c.id }, c.title))))),
    h("form", { class: "row people", onsubmit: add },
      who.map((name) => h("span", { class: "person" + (name === me ? " me" : "") }, "@" + name,
        h("button", {
          type: "button", "aria-label": t.removePerson(name), title: t.removePerson(name),
          onclick: () => act(board, card.id, "unassign", { people: [name] }),
        }, "×"))),
      h("input", { name: "person", type: "text", placeholder: t.addPerson, "aria-label": t.addPerson, autocomplete: "off" })));
}

// A click outside an open menu closes it.
document.addEventListener("click", (e) => {
  if (state.menu && !e.target.closest(".menu-wrap")) { state.menu = null; draw(); }
  if (state.cardMenu && !e.target.closest(".card-menu, .card-menu-button")) { state.cardMenu = null; draw(); }
});

// The card menu is placed where its button was. When the page scrolls, that
// place is no longer right, so the menu closes.
document.addEventListener("scroll", () => {
  if (state.cardMenu) { state.cardMenu = null; draw(); }
}, true);

document.addEventListener("keydown", (e) => {
  if (e.key === "Escape" && state.cardMenu) { state.cardMenu = null; draw(); return; }
  if (e.key === "Escape" && state.menu) { state.menu = null; draw(); return; }
  if (e.key === "Escape" && state.history) { state.history = null; draw(); return; }
  if (e.key === "Escape" && state.problems) { state.problems = false; draw(); return; }
  if (e.key === "Escape" && state.open) { state.open = null; draw(); }
  if ((e.metaKey || e.ctrlKey) && !e.altKey && !e.shiftKey && e.key.toLowerCase() === "k") {
    e.preventDefault();
    openFilters();
    return;
  }
  if (e.key === "/" && document.activeElement?.tagName !== "INPUT") {
    e.preventDefault();
    openFilters();
  }
});

applyTheme();
load().then(follow);
