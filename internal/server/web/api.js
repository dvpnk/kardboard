// Talking to `kard ui`: loading the boards, a card and the history, following
// changes, and asking for the actions the command line has.

import { state, refresh, visibleBoards } from "./state.js";

export async function load(rescan = false) {
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
  refresh();
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

export async function openCard(board, card) {
  state.history = null;
  state.problems = false;
  state.open = { board: board.id, id: card.id, detail: null };
  state.edit = null;
  await loadDetail();
  refresh();
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

export async function openHistory() {
  state.open = null;
  state.edit = null;
  state.problems = false;
  state.history = { rows: [], missing: [], branch: "" };
  await loadHistory();
  refresh();
}

const post = (url, body) => fetch(url, {
  method: "POST",
  headers: { "Content-Type": "application/json", "X-Kardboard": "1" },
  body: JSON.stringify(body),
});

// act asks the server for one action on one card. The actions are those of
// the command line. If a rule of the board refuses, the user may repeat the
// action with force, like --force. done is called once the action has
// happened, also if that took the second try.
export async function act(board, cardId, action, extra = {}, force = false, done = null) {
  state.notice = null;
  try {
    const res = await post("/api/action", { board: board.id, card: cardId, action, ...extra, force });
    if (!res.ok) {
      const problem = await res.json().catch(() => ({ error: res.statusText }));
      state.notice = {
        text: problem.error,
        retry: problem.refused && !force ? () => act(board, cardId, action, extra, true, done) : null,
      };
    } else {
      // The change happened, but may not have reached the remote.
      const answer = await res.json().catch(() => ({}));
      if (answer.warning) state.notice = { text: answer.warning, retry: null };
      done?.();
    }
  } catch (err) {
    state.notice = { text: String(err.message || err), retry: null };
  }
  await load();
}

// syncBoard pulls and pushes one board with sync: push, like `kard sync`.
export async function syncBoard(board) {
  if (state.syncing) return;
  state.notice = null;
  state.syncing = board.key;
  refresh();
  try {
    const res = await post("/api/sync", { board: board.id });
    const answer = await res.json().catch(() => ({ error: res.statusText }));
    // Success needs no words: the entry in the menu then says "up to date".
    if (!res.ok) state.notice = { text: answer.error, retry: null };
  } catch (err) {
    state.notice = { text: String(err.message || err), retry: null };
  }
  state.syncing = null;
  await load();
}

// follow listens for changes the server sees on disk.
export function follow() {
  const events = new EventSource("/api/events");
  events.onopen = () => { state.live = true; refresh(); };
  events.onerror = () => { state.live = false; refresh(); };
  events.addEventListener("change", () => load());
}
