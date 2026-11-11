// The panel on the right, which shows one of three things: a card, the
// history, or what is wrong. And the notice at the bottom of the page.

import { html, cls, useEffect, useRef } from "./preact.js";
import { t, day, ago, LOCALE } from "./texts.js";
import { render as markdown } from "./md.js";
import { state, refresh, visibleBoards, people, colourOf } from "./state.js";
import { act, openCard } from "./api.js";
import { Icon, Dot, code } from "./parts.js";

// Panel is the frame the three share: a line saying where one is, a button
// to close, the heading, and the content.
function Panel({ kind = "", label, where, title, onClose, children }) {
  return html`<aside class=${cls("panel", kind)} aria-label=${label}>
    <div class="panel-head">
      <div class="panel-where">${where}</div>
      <button class="panel-close" type="button" aria-label=${t.close} title=${t.close} onClick=${onClose}>×</button>
    </div>
    <h1 class="panel-title">${title}</h1>
    ${children}
  </aside>`;
}

export function Notice() {
  const { text, retry } = state.notice;
  const close = () => { state.notice = null; refresh(); };
  return html`<div class="notice" role="alert">
    <span class="notice-text">${text}</span>
    ${retry && html`<button type="button" class="notice-button is-primary" onClick=${retry}>${t.anyway}</button>`}
    <button type="button" class="notice-button" onClick=${close}>${retry ? t.cancel : t.dismiss}</button>
  </div>`;
}

// Problems is the one place that says what is wrong: boards that cannot be
// shown, cards that break a rule, exceeded limits. A card in the list opens
// that card.
export function Problems() {
  const close = () => { state.problems = false; refresh(); };
  const groups = visibleBoards().map((board) => {
    const rows = [];
    if (board.error) rows.push(html`<p class="issues-row"><strong>${t.unusable}</strong><span>${board.error}</span></p>`);
    for (const p of board.problems) {
      rows.push(html`<p class="issues-row"><strong>${p.path}</strong><span>${`${p.code} ${p.message}`}</span></p>`);
    }
    for (const card of board.cards) {
      const found = card.problems ?? [];
      if (found.length === 0) continue;
      rows.push(html`<button type="button" class="issues-row" onClick=${() => openCard(board, card)}>
        <strong>${code(card.title)}</strong>${found.map((p) => html`<span>${`${p.code} ${p.message}`}</span>`)}
      </button>`);
    }
    return rows.length ? html`<h2 class="issues-group"><${Dot} colour=${colourOf(board)} />${board.name}</h2>${rows}` : null;
  }).filter(Boolean);

  return html`<${Panel} kind="issues" label=${t.problems} title=${t.problems} onClose=${close}>
    ${groups.length ? groups : html`<p class="panel-note">${t.allGood}</p>`}
  <//>`;
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

// HistoryRow draws one commit. If it changed exactly one card, the row is
// about that card and a click opens it; otherwise it shows the commit's own
// subject. inCard is true in a card's own history, where the card need not
// be named again.
function HistoryRow({ board, entry, inCard = false, project = false }) {
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

  const content = html`
    <${Icon} name=${placeChanged ? one.kind : "commit"} />
    <span class="hist-headline">${project && html`<${Dot} colour=${colourOf(board)} />`}${code(headline)}</span>
    <time class="hist-time" datetime=${entry.date} title=${new Date(entry.date).toLocaleString(LOCALE)}>${ago(entry.date)}</time>
    <span class="hist-what">${[what, entry.author, entry.short].filter(Boolean).join(" · ")}</span>`;
  const title = `${entry.subject}\n${entry.author} · ${entry.short}`;
  if (card && !inCard) {
    return html`<li><button type="button" class="hist" title=${title} onClick=${() => openCard(board, card)}>${content}</button></li>`;
  }
  return html`<li><div class="hist" title=${title}>${content}</div></li>`;
}

export function History() {
  const { rows, missing, branch } = state.history;
  const several = visibleBoards().length > 1;
  const close = () => { state.history = null; refresh(); };
  return html`<${Panel} kind="history" label=${t.history} where=${branch || ""} title=${t.history} onClose=${close}>
    ${missing.map((b) => html`<p class="panel-note">${t.noGit(b.name)}</p>`)}
    ${rows.length === 0 && missing.length === 0 && html`<p class="panel-note">${t.noCommits}</p>`}
    <ul class="hist-list">${rows.map((row) => html`<${HistoryRow} board=${row.board} entry=${row.entry} project=${several} />`)}</ul>
  <//>`;
}

// Detail shows the open card: where it lies, what can be done with it, its
// fields, what is wrong with it, its text and its own history.
export function Detail() {
  const card = state.open.detail;
  const board = state.data.boards.find((b) => b.id === card.board);
  const column = board?.columns.find((c) => c.id === card.column);
  const close = () => { state.open = null; refresh(); };

  const rows = [];
  for (const field of board?.fields ?? []) {
    const value = card.fields[field.name];
    if (value == null) continue;
    const shown = field.type === "date" ? day(value) : Array.isArray(value) ? value.join(", ") : String(value);
    rows.push(html`<dt>${field.title}</dt><dd>${shown}</dd>`);
  }

  // The title is shown as the panel's heading, so leave it out of the text.
  // md.js escapes everything it is given, so its markup is safe to insert.
  const body = card.body.replace(/^\s*# .*\n?/, "");
  const where = html`${board && html`<span class="panel-project"><${Dot} colour=${colourOf(board)} />${board.name}</span>`} · ${column?.title ?? card.column}<br />${card.path}`;
  const log = state.open.log;

  return html`<${Panel} label=${card.title} where=${where} title=${code(card.title)} onClose=${close}>
    ${board && html`<${Actions} key=${board.key + "/" + card.id} board=${board} card=${card} column=${column} />`}
    ${rows.length > 0 && html`<dl class="detail-fields">${rows}</dl>`}
    ${(card.problems ?? []).length > 0 && html`<ul class="detail-problems">${card.problems.map((p) => html`<li>${`${p.code} ${p.message}`}</li>`)}</ul>`}
    <div class="md" dangerouslySetInnerHTML=${{ __html: markdown(body) }}></div>
    ${board && log?.available && log.entries.length > 0 && html`
      <h2 class="panel-section">${t.history}</h2>
      <ul class="hist-list">${log.entries.map((entry) => html`<${HistoryRow} board=${board} entry=${entry} inCard />`)}</ul>`}
  <//>`;
}

// Actions shows what can be done with the open card. Each control is one
// action of the command line: take, done, move, assign, unassign. It is
// drawn anew for each card, so text typed for one card does not stay for
// the next.
function Actions({ board, card, column }) {
  const person = useRef(null);
  // "Hand over …" in the card menu opens the card with the cursor here.
  useEffect(() => {
    if (!state.focusPerson) return;
    state.focusPerson = false;
    person.current?.focus();
  });

  if (!board.writable) return html`<p class="panel-note">${t.readOnly(board.git?.error ?? board.sync)}</p>`;
  // A card with changes that are not committed cannot be changed through
  // kb on a board that commits: say so before the action is refused.
  const unsaved = board.git?.dirty?.includes(card.id);
  const who = people(card);
  const me = state.data.user;
  const doing = board.columns.find((c) => c.doing);
  const done = board.columns.find((c) => c.done);
  const mineAlone = who.length === 1 && who[0] === me;
  const canTake = me && !(mineAlone && (!doing || card.column === doing.id));

  const add = async (e) => {
    e.preventDefault();
    const input = person.current;
    const names = input.value.split(",").map((n) => n.trim().toLowerCase()).filter(Boolean);
    if (!names.length) return;
    await act(board, card.id, "assign", { people: names, add: true });
    input.value = "";
  };
  // The picker always shows "…": it is a list of actions, not a value.
  const move = (e) => { if (e.target.value) act(board, card.id, "move", { column: e.target.value }); };

  return html`<div class="act">
    ${unsaved && html`<p class="panel-note">${t.notCommitted}</p>`}
    <div class="act-row">
      ${canTake && html`<button type="button" class="act-button is-primary" onClick=${() => act(board, card.id, "take")}>${t.take}</button>`}
      ${done && !column?.done && html`<button type="button" class="act-button" onClick=${() => act(board, card.id, "done")}>${t.finish}</button>`}
      <label class="act-move">${t.moveTo}
        <select class="act-select" aria-label=${t.moveTo} value="" onChange=${move}>
          <option value="">…</option>
          ${board.columns.filter((c) => c.id !== card.column).map((c) => html`<option key=${c.id} value=${c.id}>${c.title}</option>`)}
        </select>
      </label>
    </div>
    <form class="act-row" onSubmit=${add}>
      ${who.map((name) => html`<span key=${name} class=${cls("act-person", name === me && "is-me")}>${"@" + name}<button
        type="button" class="act-remove" aria-label=${t.removePerson(name)} title=${t.removePerson(name)}
        onClick=${() => act(board, card.id, "unassign", { people: [name] })}>×</button></span>`)}
      <input ref=${person} class="act-input" name="person" type="text" placeholder=${t.addPerson} aria-label=${t.addPerson} autocomplete="off" />
    </form>
  </div>`;
}
