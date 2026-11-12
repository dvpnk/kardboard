// The panel on the right, which shows one of three things: a card, the
// history, or what is wrong. And the notice at the bottom of the page.

import { html, cls, useEffect, useRef, useState } from "./preact.js";
import { t, ago, LOCALE } from "./texts.js";
import { render as markdown } from "./md.js";
import { state, refresh, visibleBoards, people, colourOf } from "./state.js";
import { act, openCard } from "./api.js";
import { Icon, Dot, code } from "./parts.js";
import { Fields, isEditing, textOf, TitleInput } from "./fields.js";

// Panel is the frame the three share: the heading with the button to
// close beside it, a quiet line saying where one is, and the content.
function Panel({ kind = "", label, where, title, onClose, children }) {
  return html`<aside class=${cls("panel", kind)} aria-label=${label}>
    <div class="panel-head">
      <h1 class="panel-title">${title}</h1>
      <button class="panel-close" type="button" aria-label=${t.close} title=${t.close} onClick=${onClose}>×</button>
    </div>
    ${where && html`<div class="panel-where">${where}</div>`}
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
// fields (see fields.js), what is wrong with it, its text and its own history.
export function Detail() {
  const card = state.open.detail;
  const board = state.data.boards.find((b) => b.id === card.board);
  const column = board?.columns.find((c) => c.id === card.column);
  const close = () => { state.open = null; state.edit = null; refresh(); };

  // The title is shown as the panel's heading, so leave it out of the text.
  // md.js escapes everything it is given, so its markup is safe to insert.
  // While the card is edited, the heading is a field and the text is in
  // the form.
  const editing = !!board && isEditing(board, card);
  const body = textOf(card);
  const where = html`${board && html`<span class="panel-project"><${Dot} colour=${colourOf(board)} />${board.name}</span><span aria-hidden="true">·</span>`}<span>${column?.title ?? card.column}</span>`;
  // Who the card belongs to is the first row of the fields, and can be
  // changed there at once, while kb may change the card.
  const changeable = !!board?.writable && !board.git?.dirty?.includes(card.id);
  const who = changeable && html`<${People} key=${board.key + "/" + card.id} board=${board} card=${card} />`;
  const log = state.open.log;

  // The panel lies over the board. If it covers the card that was opened,
  // the columns get room behind the last one (see .is-room) and the board
  // area scrolls them aside. Without a covered card there is no such
  // room, and so no scrollbar that leads nowhere.
  useEffect(() => {
    const el = document.querySelector(".card.is-open");
    const panel = document.querySelector(".panel");
    if (!el || !panel) return;
    const covered = el.getBoundingClientRect().right + 16 - panel.getBoundingClientRect().left;
    if (covered <= 0) return;
    if (!state.room) { state.room = true; refresh(); }
    requestAnimationFrame(() => el.closest(".shell-main")?.scrollBy({ left: covered, behavior: "smooth" }));
  }, [card.board, card.id]);

  return html`<${Panel} label=${card.title} where=${where} title=${editing ? html`<${TitleInput} card=${card} />` : code(card.title)} onClose=${close}>
    ${board && html`<${Actions} key=${board.key + "/" + card.id} board=${board} card=${card} column=${column} />`}
    ${board && html`<${Fields} board=${board} card=${card} people=${who || null} />`}
    ${(card.problems ?? []).length > 0 && html`<ul class="detail-problems">${card.problems.map((p) => html`<li>${`${p.code} ${p.message}`}</li>`)}</ul>`}
    ${!editing && html`<div class="md" dangerouslySetInnerHTML=${{ __html: markdown(body) }}></div>`}
    ${board && log?.available && log.entries.length > 0 && html`
      <h2 class="panel-section">${t.history}</h2>
      <ul class="hist-list">${log.entries.map((entry) => html`<${HistoryRow} board=${board} entry=${entry} inCard />`)}</ul>`}
    <p class="detail-path" title=${t.file}>${card.path}</p>
  <//>`;
}

// Actions shows what can be done with the open card on its way through
// the columns. Each control is one action of the command line: take,
// done, move. Changing the fields is part of the fields below.
function Actions({ board, card, column }) {
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
  // The picker always shows "…": it is a list of actions, not a value.
  const move = (e) => { if (e.target.value) act(board, card.id, "move", { column: e.target.value }); };

  return html`<div class="act">
    ${unsaved && html`<p class="panel-note">${t.notCommitted}</p>`}
    <div class="act-row">
      ${canTake && html`<button type="button" class="act-button is-primary" onClick=${() => act(board, card.id, "take")}>${t.take}</button>`}
      ${done && !column?.done && html`<button type="button" class="act-button" onClick=${() => act(board, card.id, "done")}>${t.finish}</button>`}
      <span class="act-pick"><select class="act-select" aria-label=${t.moveTo} value="" onChange=${move}>
        <option value="">${t.moveTo + " …"}</option>
        ${board.columns.filter((c) => c.id !== card.column).map((c) => html`<option key=${c.id} value=${c.id}>${c.title}</option>`)}
      </select></span>
    </div>
  </div>`;
}

// People shows who the card belongs to, each with a button that takes the
// person off, and "+" for one more: assign and unassign of the command
// line, done at once. It is drawn anew for each card, so text typed for
// one card does not stay for the next.
function People({ board, card }) {
  const person = useRef(null);
  // The field for a new person shows only after a click on "+", or when
  // "Hand over …" in the card menu opened the card.
  const [adding, setAdding] = useState(false);
  useEffect(() => {
    if (state.focusPerson) { state.focusPerson = false; setAdding(true); }
  });
  useEffect(() => { if (adding) person.current?.focus(); }, [adding]);
  const me = state.data.user;

  const add = async (e) => {
    e.preventDefault();
    const input = person.current;
    const names = input.value.split(",").map((n) => n.trim().toLowerCase()).filter(Boolean);
    if (!names.length) return;
    await act(board, card.id, "assign", { people: names, add: true });
    input.value = "";
    setAdding(false);
  };
  // Leaving the empty field, or Escape in it, puts the "+" back.
  const leave = (e) => { if (!e.target.value.trim()) setAdding(false); };
  const keyDown = (e) => { if (e.key === "Escape") { e.stopPropagation(); e.target.value = ""; setAdding(false); } };

  return html`<form class="act-people" onSubmit=${add}>
    ${people(card).map((name) => html`<span key=${name} class=${cls("act-person", name === me && "is-me")}>${"@" + name}<button
      type="button" class="act-remove" aria-label=${t.removePerson(name)} title=${t.removePerson(name)}
      onClick=${() => act(board, card.id, "unassign", { people: [name] })}>×</button></span>`)}
    ${adding
      ? html`<input ref=${person} class="act-input" name="person" type="text" placeholder=${t.addPerson} aria-label=${t.addPerson}
          autocomplete="off" onBlur=${leave} onKeyDown=${keyDown} />`
      : html`<button type="button" class="act-button act-add" aria-label=${t.addPerson} title=${t.addPerson} onClick=${() => setAdding(true)}>+</button>`}
  </form>`;
}
