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
    readOnly: (sync) => `This board uses sync: ${sync}. kb cannot change such boards yet, so the page only shows it.`,
  },
  de: {
    filter: "Karten filtern", projects: "Projekte", only: "nur dieses Projekt zeigen", all: "alle Projekte zeigen",
    allProjects: (n) => `Alle Projekte (${n})`, someProjects: (n, of) => `${n} von ${of} Projekten`, noProjects: "Kein Projekt",
    pickHint: "Haken blendet ein oder aus. Ein Klick auf den Namen zeigt das Projekt allein.",
    settings: "Einstellungen", theme: "Darstellung", system: "System", light: "Hell", dark: "Dunkel",
    you: (name) => `Du bist @${name}`, mineOff: "wieder die Karten aller zeigen",
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
    readOnly: (sync) => `Dieses Board nutzt sync: ${sync}. Solche Boards kann kb noch nicht ändern, deshalb zeigt die Seite es nur an.`,
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
  filter: "",
  expanded: new Set(),  // columns shown in full, as "boardKey/columnId"
  open: null,           // { board, id, detail } of the card in the side panel
  notice: null,         // { text, retry } shown at the bottom: an error, or a refusal that can be forced
  menu: null,           // which menu of the top bar is open: "projects", "settings" or none
  theme: stored("theme", "system"), // "system", "light" or "dark"
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

function matches(card, board) {
  if (state.mine && !people(card).includes(state.data.user)) return false;
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
  draw();
}

async function loadDetail() {
  const { board, id } = state.open;
  try {
    const res = await fetch(`/api/card?board=${encodeURIComponent(board)}&id=${encodeURIComponent(id)}`);
    state.open = res.ok ? { board, id, detail: await res.json() } : null; // the card is gone
  } catch {
    state.open = null;
  }
}

async function openCard(board, card) {
  state.open = { board: board.id, id: card.id, detail: null };
  await loadDetail();
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
    }
  } catch (err) {
    state.notice = { text: String(err.message || err), retry: null };
  }
  await load();
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
    app.replaceChildren(h("header", { class: "top" }, h("h1", { class: "brand" }, "kardboard")),
      h("main", {}, state.error ? h("div", { class: "lane-error" }, state.error) : ""));
    return;
  }

  app.replaceChildren(...[
    drawTop(),
    h("div", { class: "shell" + (state.open?.detail ? " with-detail" : "") },
      drawMain(), state.open?.detail ? drawDetail() : null),
    state.notice && drawNotice(),
  ].filter(Boolean));

  if (focused) {
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
  const boards = visibleBoards();
  const errors = boards.reduce((n, b) => n + b.errors, 0);
  const warnings = boards.reduce((n, b) => n + b.warnings, 0);
  const me = state.data.user;
  return h("header", { class: "top" },
    h("h1", { class: "brand" }, "kardboard"),
    drawProjectMenu(),
    h("input", {
      id: "filter", class: "filter", type: "search", placeholder: t.filter, value: state.filter, "aria-label": t.filter,
      oninput: (e) => { state.filter = e.target.value; draw(); },
    }),
    // "Only mine" is set in the menu. While it is on, it shows here, so
    // nobody wonders where the other cards went.
    state.mine && me && h("button", { type: "button", class: "active-filter", title: t.mineOff, onclick: () => setMine(false) },
      "@" + me, h("span", { "aria-hidden": "true" }, "×")),
    h("span", { class: "spacer" }),
    errors > 0 && h("span", { class: "badge bad" }, t.errors(errors)),
    warnings > 0 && h("span", { class: "badge warn" }, t.warnings(warnings)),
    h("span", { class: "live" + (state.live ? " on" : ""), role: "img", "aria-label": state.live ? t.live : t.offline, title: state.live ? t.live : t.offline }),
    drawSettingsMenu());
}

function setMine(on) {
  state.mine = on;
  store("mine", on);
  draw();
}

// menu draws a button that opens a panel below it. Only one menu is open at
// a time; a click elsewhere or Escape closes it.
function menu(name, label, title, content) {
  const open = state.menu === name;
  return h("div", { class: "menu-wrap" },
    h("button", {
      type: "button", class: "menu-button" + (open ? " open" : ""), "aria-haspopup": "true", "aria-expanded": String(open), title,
      onclick: () => { state.menu = open ? null : name; draw(); },
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

function drawSettingsMenu() {
  const me = state.data.user;
  const setTheme = (theme) => { state.theme = theme; store("theme", theme); applyTheme(); draw(); };
  return menu("settings", h("span", { "aria-hidden": "true" }, "⋯"), t.settings, () => [
    h("label", { class: "check" + (me ? "" : " off") },
      h("input", { type: "checkbox", checked: state.mine && !!me, disabled: !me, onchange: (e) => setMine(e.target.checked) }),
      t.mine),
    !me && h("p", { class: "hint" }, code(t.mineHint)),
    h("h2", {}, t.theme),
    h("div", { class: "segments", role: "group", "aria-label": t.theme },
      ["system", "light", "dark"].map((theme) => h("button", {
        type: "button", class: state.theme === theme ? "on" : "", "aria-pressed": String(state.theme === theme),
        onclick: () => setTheme(theme),
      }, t[theme]))),
    me && h("p", { class: "hint you" }, t.you(me)),
  ]);
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
  if (board.error) return h("section", { class: "lane" }, h("div", { class: "lane-error" }, `${board.name}: ${board.error}`));

  return h("section", { class: "lane" },
    h("div", { class: "columns" }, board.columns.map((col) => {
      const all = board.cards.filter((c) => c.column === col.id);
      const shown = all.filter((c) => matches(c, board));
      const over = col.limit > 0 && all.length > col.limit;
      return drawColumn(`${board.key}/${col.id}`, col.title,
        col.limit > 0 ? h("span", { class: over ? "over" : "" }, t.limit(all.length, col.limit)) : String(shown.length),
        shown.map((c) => drawCard(c, board, { done: col.done })),
        () => (dragging?.board === board ? col : null));
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
  const broken = boards.filter((b) => b.error);
  return [
    broken.map((b) => h("div", { class: "lane-error" }, `${b.name}: ${b.error}`)),
    h("div", { class: "columns" }, Object.entries(groups).map(([kind, entries]) =>
      drawColumn("overview/" + kind, t[kind], String(entries.length),
        entries.map(({ card, board }) => drawCard(card, board, { done: kind === "done", project: true })),
        // The shared columns stand for the dragged card's own columns. "Open"
        // may be several of them, so a card cannot be dropped there.
        () => (kind === "open" ? null : dragging?.board.columns.find((c) => (kind === "done" ? c.done : c.doing)) ?? null)))),
  ];
}

// drawColumn draws one column. target returns the board column that a card
// dropped here goes to, for the card being dragged right now.
function drawColumn(key, title, count, cards, target) {
  const full = state.expanded.has(key);
  const shown = full ? cards : cards.slice(0, COLLAPSE_AFTER);
  const rest = cards.length - shown.length;
  const toggle = () => { full ? state.expanded.delete(key) : state.expanded.add(key); draw(); };
  const el = h("div", { class: "column" },
    h("div", { class: "column-head" }, h("span", {}, title), h("span", {}, count)),
    cards.length === 0 && h("p", { class: "none" }, state.filter || state.mine ? t.noMatch : t.none),
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

function drawCard(card, board, { done = false, project = false } = {}) {
  const bad = (card.problems ?? []).some((p) => p.severity === "error");
  const isOpen = state.open?.board === board.id && state.open?.id === card.id;
  const meta = [];
  if (project) meta.push(h("span", { class: "proj" }, dot(colourOf(board)), board.name));
  for (const field of board.fields) {
    const value = card.fields[field.name];
    if (value == null) continue;
    if (field.type === "enum") meta.push(h("span", { class: "chip" }, String(value)));
    else if (field.name === "tags") meta.push(...value.map((tag) => h("span", {}, tag)));
    else if (field.name === "due") {
      const late = !done && value < new Date().toISOString().slice(0, 10);
      meta.push(h("span", { class: late ? "overdue" : "" }, "→ " + value));
    }
  }
  for (const name of people(card)) {
    meta.push(h("span", { class: "who" + (name === state.data.user ? " me" : "") }, "@" + name));
  }
  // Only the first name is pushed to the right; the others follow it.
  meta.filter((el) => el.classList.contains("who")).slice(1).forEach((el) => { el.style.marginLeft = "0"; });

  const el = h("button", {
    class: "card" + (bad ? " bad" : "") + (done ? " done" : "") + (isOpen ? " open" : ""),
    type: "button", title: card.path, onclick: () => openCard(board, card),
  }, h("span", { class: "title" }, code(card.title)), h("span", { class: "meta" }, meta));

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

function drawDetail() {
  const card = state.open.detail;
  const board = state.data.boards.find((b) => b.id === card.board);
  const column = board?.columns.find((c) => c.id === card.column);
  const close = () => { state.open = null; draw(); };

  const rows = [];
  for (const field of board?.fields ?? []) {
    const value = card.fields[field.name];
    if (value == null) continue;
    rows.push(h("dt", {}, field.title), h("dd", {}, Array.isArray(value) ? value.join(", ") : String(value)));
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
    text);
}

// drawActions draws what can be done with the open card. Each control is
// one action of the command line: take, done, move, assign, unassign.
function drawActions(board, card, column) {
  if (!board.writable) return h("p", { class: "readonly" }, t.readOnly(board.sync));
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
});

document.addEventListener("keydown", (e) => {
  if (e.key === "Escape" && state.menu) { state.menu = null; draw(); return; }
  if (e.key === "Escape" && state.open) { state.open = null; draw(); }
  if (e.key === "/" && document.activeElement?.tagName !== "INPUT") {
    e.preventDefault();
    document.getElementById("filter")?.focus();
  }
});

applyTheme();
load().then(follow);
