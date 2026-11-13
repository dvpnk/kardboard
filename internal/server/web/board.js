// The board itself. There is one view and no switch. What it shows follows
// from the ticked projects: a single one appears with its own columns,
// several in three shared ones.

import { html, cls, useState, useEffect } from "./preact.js";
import { t, day } from "./texts.js";
import { state, refresh, visibleBoards, people, me, isMe, personOf, isLate, matches, activeFilters, colourOf, COLLAPSE_AFTER } from "./state.js";
import { act, openCard } from "./api.js";
import { Icon, Dot, Who, code } from "./parts.js";

export function Main() {
  if (state.error) return html`<main class="shell-main"><div class="main-error">${state.error}</div></main>`;
  if (state.data.boards.length === 0) {
    return html`<main class="shell-main"><div class="main-empty"><h2>${t.noBoardsTitle}</h2><p>${code(t.noBoards)}</p></div></main>`;
  }
  const boards = visibleBoards();
  if (boards.length === 0) return html`<main class="shell-main"><div class="main-empty"><p>${t.allHidden}</p></div></main>`;
  return html`<main class="shell-main">${boards.length === 1 ? html`<${Lane} board=${boards[0]} />` : html`<${Overview} boards=${boards} />`}</main>`;
}

// Lane draws one board with its own columns. It has no heading: the top bar
// already names the project, and the menu says what is wrong with it.
function Lane({ board }) {
  if (board.error) return html`<div class="main-empty"><h2>${t.unusable}</h2><p>${board.error}</p></div>`;
  return html`<section class="lane"><div class="cols">${board.columns.map((col) => {
    const all = board.cards.filter((c) => c.column === col.id);
    const shown = all.filter((c) => matches(c, board));
    const over = col.limit > 0 && all.length > col.limit;
    const count = col.limit > 0
      ? html`<span class=${over ? "col-over" : ""}>${t.limit(all.length, col.limit)}</span>`
      : String(shown.length);
    const key = `${board.key}/${col.id}`;
    return html`<${Column} key=${key} id=${key} title=${col.title} count=${count} doing=${col.doing}
      cards=${shown.map((c) => html`<${Card} key=${c.id} card=${c} board=${board} done=${col.done} />`)}
      target=${() => (state.drag?.board.key === board.key ? col : null)} />`;
  })}</div></section>`;
}

// Overview puts the cards of several boards into three shared columns:
// open, in progress and done.
function Overview({ boards }) {
  const groups = { open: [], doing: [], done: [] };
  for (const board of boards) {
    if (board.error) continue;
    const kind = new Map(board.columns.map((c) => [c.id, c.done ? "done" : c.doing ? "doing" : "open"]));
    for (const card of board.cards) {
      if (matches(card, board)) groups[kind.get(card.column)].push({ card, board });
    }
  }
  // The shared columns stand for the dragged card's own columns. "Open" may
  // be several of them, so a card cannot be dropped there.
  const target = (kind) => () =>
    kind === "open" ? null : state.drag?.board.columns.find((c) => (kind === "done" ? c.done : c.doing)) ?? null;
  return html`<div class="cols">${Object.entries(groups).map(([kind, entries]) => html`
    <${Column} key=${kind} id=${"overview/" + kind} title=${t[kind]} count=${String(entries.length)} doing=${kind === "doing"}
      cards=${entries.map(({ card, board }) => html`<${Card} key=${board.key + "/" + card.id} card=${card} board=${board} done=${kind === "done"} project />`)}
      target=${target(kind)} />`)}</div>`;
}

// dropAction says what dropping the dragged card on a board column does, or
// null if it cannot be dropped there. Dropping an unassigned card on the
// column for work in progress takes it; everything else is a plain move.
// Columns are compared by id: the boards may have been reloaded meanwhile.
function dropAction(target) {
  const drag = state.drag;
  if (!drag || !target || drag.card.column === target.id) return null;
  const firstDoing = drag.board.columns.find((c) => c.doing);
  if (target.id === firstDoing?.id && people(drag.card).length === 0 && me(drag.board)) return { action: "take" };
  return { action: "move", extra: { column: target.id } };
}

function endDrag() {
  state.drag = null;
  refresh();
}

// Column draws one column. target returns the board column that a card
// dropped here goes to, for the card being dragged right now.
function Column({ id, title, count, cards, target, doing = false }) {
  const [over, setOver] = useState(false);
  useEffect(() => { if (!state.drag) setOver(false); }, [state.drag]);
  const full = state.expanded.has(id);
  const shown = full ? cards : cards.slice(0, COLLAPSE_AFTER);
  const rest = cards.length - shown.length;
  const toggle = () => { full ? state.expanded.delete(id) : state.expanded.add(id); refresh(); };

  const dragOver = (e) => {
    if (!dropAction(target())) return;
    e.preventDefault(); // this is what allows the drop
    e.dataTransfer.dropEffect = "move";
    setOver(true);
  };
  const dragLeave = (e) => { if (!e.currentTarget.contains(e.relatedTarget)) setOver(false); };
  const drop = (e) => {
    const what = dropAction(target());
    if (!what) return;
    e.preventDefault();
    const { board, card } = state.drag;
    endDrag();
    act(board, card.id, what.action, what.extra);
  };

  return html`<div class=${cls("col", doing && "is-doing", over && state.drag && "is-drop")}
    onDragOver=${dragOver} onDragLeave=${dragLeave} onDrop=${drop}>
    <div class="col-head"><span>${title}</span><span>${count}</span></div>
    ${cards.length === 0 && html`<p class="col-none">${state.filter || activeFilters() > 0 ? t.noMatch : t.none}</p>`}
    ${shown}
    ${rest > 0 && html`<button class="col-more" type="button" onClick=${toggle}>${"+ " + t.more(rest)}</button>`}
    ${full && cards.length > COLLAPSE_AFTER && html`<button class="col-more" type="button" onClick=${toggle}>${t.less}</button>`}
  </div>`;
}

// How many names a card shows before it sums up the rest as "+2".
const MAX_NAMES = 2;

// Card draws one card. Every fact has its own fixed place, so cards with
// more or fewer facts look alike and nothing jumps from one to the next:
//
//   title ................................ marks (overdue, broken)
//   size, tags ............................. due date
//   project ................................. people
//
// The second and third line appear only if the card has something for them.
// Neither wraps; what does not fit is cut off and stays in the card's panel.
function Card({ card, board, done = false, project = false }) {
  const problems = (card.problems ?? []).filter((p) => p.severity === "error");
  const isOpen = state.open?.board === board.id && state.open?.id === card.id;
  const due = card.fields.due;
  const late = isLate(card, board);
  const dragged = state.drag?.board.key === board.key && state.drag?.card.id === card.id;

  const facts = [];
  for (const field of board.fields) {
    const value = card.fields[field.name];
    if (value == null) continue;
    if (field.type === "enum") facts.push(html`<span class="card-chip" title=${field.title}>${String(value)}</span>`);
    else if (field.name === "tags") facts.push(...value.map((tag) => html`<span class="card-tag">${"#" + tag}</span>`));
  }
  const who = people(card);
  const names = who.slice(0, MAX_NAMES).map((id) =>
    html`<${Who} key=${id} person=${personOf(board, id)} mine=${isMe(board, id)} />`);
  if (who.length > MAX_NAMES) names.push(html`<span class="card-who">${"+" + (who.length - MAX_NAMES)}</span>`);

  const menuOpen = state.cardMenu?.board === board && state.cardMenu?.card.id === card.id;
  const toggleMenu = (e) => {
    e.stopPropagation();
    const box = e.currentTarget.getBoundingClientRect();
    state.menu = null;
    state.cardMenu = menuOpen ? null : { board, card, top: box.bottom + 4, right: window.innerWidth - box.right };
    refresh();
  };
  const keyDown = (e) => {
    if (e.target === e.currentTarget && (e.key === "Enter" || e.key === " ")) { e.preventDefault(); openCard(board, card); }
  };
  const dragStart = (e) => {
    // The browser takes its picture of the card when this handler is done;
    // for that moment the card gets its ground and frame back.
    const el = e.currentTarget;
    el.classList.add("is-lifted");
    requestAnimationFrame(() => el.classList.remove("is-lifted"));
    state.drag = { board, card };
    e.dataTransfer.effectAllowed = "move";
    e.dataTransfer.setData("text/plain", card.id);
    refresh();
  };

  return html`<div class=${cls("card", done && "is-done", isOpen && "is-open", dragged && "is-dragging")}
    role="button" tabindex="0" title=${card.path} draggable=${board.writable ? true : undefined}
    onClick=${() => openCard(board, card)} onKeyDown=${keyDown}
    onDragStart=${board.writable ? dragStart : undefined} onDragEnd=${board.writable ? endDrag : undefined}>
    <span class="card-head">
      <span class="card-title">${code(card.title)}</span>
      <span class=${cls("card-marks", menuOpen && "is-aside")}>
        ${late && html`<${Icon} name="clock" label=${t.overdue(day(due))} />`}
        ${problems.length > 0 && html`<${Icon} name="bug" label=${problems.map((p) => `${p.code} ${p.message}`).join("\n")} />`}
      </span>
      ${board.writable && html`<button type="button" class=${cls("card-dots", menuOpen && "is-open")}
        aria-label=${t.cardMenu} title=${t.cardMenu} aria-haspopup="true" aria-expanded=${String(menuOpen)}
        onClick=${toggleMenu}>⋯</button>`}
    </span>
    ${(facts.length > 0 || due) && html`<span class="card-line">
      <span class="card-facts">${facts}</span>
      ${due && html`<span class="card-due" title=${day(due)}>${"→ " + day(due, true)}</span>`}
    </span>`}
    ${(project || who.length > 0) && html`<span class="card-line">
      ${project
        ? html`<span class="card-project"><${Dot} colour=${colourOf(board)} /><span class="card-project-name">${board.name}</span></span>`
        : html`<span></span>`}
      ${who.length > 0 && html`<span class="card-people">${names}</span>`}
    </span>`}
  </div>`;
}

// CardMenu is the menu of one card: what can be done with it without
// opening it. Each entry is an action of the command line. The entries for
// moving are the other columns of the card's own board, so this also works
// in the shared view, where "open" stands for several columns.
export function CardMenu() {
  const { board, card, top, right } = state.cardMenu;
  const who = people(card);
  const doing = board.columns.find((c) => c.doing);
  const done = board.columns.find((c) => c.done);
  const mineAlone = who.length === 1 && isMe(board, who[0]);
  const run = (action, extra) => { state.cardMenu = null; act(board, card.id, action, extra); };
  const handOver = () => { state.cardMenu = null; state.focusPerson = true; openCard(board, card); };
  const item = (label, onClick) => html`<button type="button" role="menuitem" onClick=${onClick}>${label}</button>`;

  return html`<div class="menu-panel cmenu" role="menu" aria-label=${t.cardMenu} style=${{ top: top + "px", right: right + "px" }}>
    ${me(board) && !(mineAlone && (!doing || card.column === doing.id)) && item(t.take, () => run("take"))}
    ${item(t.handOver, handOver)}
    ${done && card.column !== done.id && item(t.finish, () => run("done"))}
    <hr class="cmenu-rule" />
    ${board.columns.filter((c) => c.id !== card.column && c !== done).map((c) => item(t.to(c.title), () => run("move", { column: c.id })))}
  </div>`;
}
