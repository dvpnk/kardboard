// The state of the page and what follows from it. Components read the state
// directly. Whoever changes it calls refresh(): Preact then renders the page
// again and changes only what differs, so focus, scroll positions and a card
// being dragged stay as they are.

import { t } from "./texts.js";

// The keys start with "kb.", the command's name when they were made. They
// stay so, or every browser would lose its theme and hidden boards.
export const stored = (key, fallback) => {
  try { return JSON.parse(localStorage.getItem("kb." + key)) ?? fallback; } catch { return fallback; }
};
export const store = (key, value) => {
  try { localStorage.setItem("kb." + key, JSON.stringify(value)); } catch { /* private mode: keep it in memory */ }
};

export const state = {
  data: null,           // answer of /api/state
  error: "",            // why the state could not be loaded
  live: false,          // whether the event stream is connected
  hidden: stored("hidden", null), // keys of hidden boards; null = use the settings' default
  mine: stored("mine", false),
  overdue: false,       // show only cards past their due date
  tags: [],             // show only cards with one of these tags; empty = all
  values: {},           // per enum field, show only cards with one of these values, e.g. { size: ["S", "M"] }
  filter: "",
  expanded: new Set(),  // columns shown in full, as "boardKey/columnId"
  open: null,           // { board, id, detail, log } of the card in the side panel
  notice: null,         // { text, retry } shown at the bottom: an error, or a refusal that can be forced
  menu: null,           // which menu of the top bar is open: "projects", "filters", "settings" or none
  history: null,        // { rows, missing, branch } while the history panel is open, see loadHistory
  problems: false,      // whether the panel with what is wrong is open
  cardMenu: null,       // { board, card, top, right } of the card whose menu is open
  focusPerson: false,   // put the cursor into the person field once the card panel is shown
  theme: stored("theme", "system"), // "system", "light" or "dark"
  syncing: null,        // key of the board that is being synced with its remote right now
  drag: null,           // { board, card } while a card is being dragged
  edit: null,           // { key, draft } while the fields of the open card are being edited, see fields.js
  room: false,          // whether the columns have room to move out from under the panel, see Detail
};

let rerender = () => {};
// onRefresh is called by the app with the function that renders it again.
export const onRefresh = (fn) => { rerender = fn; };
export const refresh = () => rerender();

// The theme is applied to the document; theme.js already did so before the
// first paint, this keeps it in step when the user changes it.
export function applyTheme() {
  if (state.theme === "system") delete document.documentElement.dataset.theme;
  else document.documentElement.dataset.theme = state.theme;
}

export const COLLAPSE_AFTER = 8;
const PALETTE = ["#2f9e44", "#7950f2", "#e8590c", "#1c7ed6", "#c2255c", "#0c8599", "#e67700", "#5c7cfa"];

// A board keeps its colour as long as the set of boards does not change.
export const colourOf = (board) => PALETTE[Math.max(0, state.data.boards.indexOf(board)) % PALETTE.length];

export const isHidden = (board) => {
  if (state.hidden === null) return state.data.hidden.includes(board.id);
  return state.hidden.includes(board.key);
};
export const visibleBoards = () => state.data.boards.filter((b) => !isHidden(b));
export const people = (card) => card.fields.assignee ?? [];

// Who the user is depends on the board: Git may know them by another
// address in each repository (see kard whoami).
//
// me is the user's name on a board, or "" if kard does not know it.
export const me = (board) => board.user?.name ?? "";
// isMe says whether a name on a card means the user on that board: their
// own, or an address the repository's .mailmap joins with it.
export const isMe = (board, name) => board.me.includes(name);
export const isMine = (card, board) => people(card).some((name) => isMe(board, name));
// knowsMe says whether kard knows who the user is on any visible board.
export const knowsMe = () => visibleBoards().some((b) => b.user);

// personOf returns what a board knows about a person: the name in Git of
// whoever committed under that address.
export const personOf = (board, id) => board.people.find((p) => p.id === id) ?? { id };
export const personName = (person) => person.name || person.id;
export const personTitle = (person) => (person.name ? `${person.name} · ${person.id}` : person.id);

// monogram gives a person two letters: the first of the first and the last
// word of the name, else of the address before the @ or of the short name
// (Angélique Lambert → AL, anna.lang@… → AL, sam → SA).
const LETTER = /[\p{L}\p{N}]/u;
export function monogram(person) {
  const words = (person.name || person.id.split("@")[0]).split(/[\s._+-]+/).filter(Boolean);
  const letters = (w) => [...w].filter((c) => LETTER.test(c));
  const two = words.length > 1
    ? (letters(words[0])[0] ?? "") + (letters(words[words.length - 1])[0] ?? "")
    : letters(words[0] ?? "").slice(0, 2).join("");
  return two.toUpperCase() || "?";
}

const today = () => new Date().toISOString().slice(0, 10);

// isLate says whether a card is past its due date and not done.
export function isLate(card, board) {
  const due = card.fields.due;
  return !!due && due < today() && !board.columns.find((c) => c.id === card.column)?.done;
}

// activeFilters counts the filters that are on, the search aside.
export function activeFilters() {
  return (state.mine && knowsMe() ? 1 : 0) + (state.overdue ? 1 : 0) + state.tags.length
    + Object.values(state.values).filter((v) => v.length).length;
}

// matches says whether a card passes the filters and the search.
export function matches(card, board) {
  if (state.mine && knowsMe() && !isMine(card, board)) return false;
  if (state.overdue && !isLate(card, board)) return false;
  if (state.tags.length && !state.tags.some((tag) => (card.fields.tags ?? []).includes(tag))) return false;
  // Values of one field are alternatives; different fields must all fit.
  for (const [name, chosen] of Object.entries(state.values)) {
    if (chosen.length && !chosen.includes(card.fields[name])) return false;
  }
  const q = state.filter.trim().toLowerCase();
  if (!q) return true;
  const who = people(card).flatMap((id) => [id, personOf(board, id).name ?? ""]);
  const hay = [card.id, card.title, board.name, ...who, ...(card.fields.tags ?? [])].join(" ").toLowerCase();
  return q.split(/\s+/).every((word) => hay.includes(word));
}

// countProblems sums up what is wrong on the visible boards.
export function countProblems() {
  const boards = visibleBoards();
  return {
    errors: boards.reduce((n, b) => n + b.errors + (b.error ? 1 : 0), 0),
    warnings: boards.reduce((n, b) => n + b.warnings, 0),
  };
}

// remoteState says in a few words how a board stands against its remote.
export function remoteState(board) {
  const git = board.git;
  const parts = [git.behind > 0 && t.toPull(git.behind), git.ahead > 0 && t.toPush(git.ahead)].filter(Boolean);
  if (git.unreachable) parts.push(t.unreachable);
  return parts.length ? parts.join(" · ") : t.upToDate;
}

export function setMine(on) {
  state.mine = on;
  store("mine", on);
  refresh();
}

// openFilters opens the filter menu with the cursor in its search field.
// The field puts the cursor into itself when it appears; if the menu is
// open already, it gets the cursor here.
export function openFilters() {
  state.cardMenu = null;
  if (state.menu === "filters") document.getElementById("filter")?.focus();
  state.menu = "filters";
  refresh();
}
