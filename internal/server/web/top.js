// The top bar holds the few things that are always needed: which projects,
// the filter, and a menu for the rest.

import { html, cls, useEffect, useRef } from "./preact.js";
import { t } from "./texts.js";
import {
  state, refresh, store, applyTheme, visibleBoards, isHidden, colourOf, people, isLate,
  matches, activeFilters, countProblems, remoteState, setMine, openFilters, knowsMe, isMine,
} from "./state.js";
import { load, openHistory, syncBoard } from "./api.js";
import { Icon, Dot, Brand, code } from "./parts.js";

// From left to right: what is shown, from coarse to fine (which projects,
// which of their cards), then the menu. Nothing is said while all is well;
// only a lost connection shows up, quietly, and the menu explains it.
export function TopBar() {
  return html`<header class="top">
    <${Brand} />
    <${ProjectMenu} />
    <${FilterMenu} />
    <span class="top-spacer"></span>
    ${!state.live && html`<span class="top-offline" title=${t.liveOff.replaceAll("`", "")}>${t.offline}</span>`}
    <${SettingsMenu} />
  </header>`;
}

// Menu draws a button that opens a panel below it. Only one menu is open at
// a time; a click elsewhere or Escape closes it. square makes a button that
// shows only an icon.
function Menu({ name, label, title, content, square = false }) {
  const open = state.menu === name;
  return html`<div class="menu-wrap">
    <button type="button" class=${cls("menu-button", square && "menu-square", open && "is-open")}
      aria-haspopup="true" aria-expanded=${String(open)} title=${title}
      onClick=${() => { state.cardMenu = null; state.menu = open ? null : name; refresh(); }}>${label}</button>
    ${open && html`<div class=${"menu-panel " + name + "-panel"}>${content()}</div>`}
  </div>`;
}

// The usual shortcut for "find something here": Cmd+K on a Mac, Ctrl+K
// elsewhere. "/" works as well.
const IS_MAC = /Mac|iPhone|iPad/.test(navigator.platform || navigator.userAgent);
const SEARCH_KEY = IS_MAC ? "⌘K" : "Ctrl+K";
const SEARCH_KEYS = IS_MAC ? ["⌘", "K"] : ["Ctrl", "K"];

// FilterMenu holds everything that narrows down which cards are shown: the
// search on top, then the filters. Each filter says how many cards it would
// leave, so one can tell before switching it on. The button is dark while
// something narrows the view, and says what: the search text and how many
// filters.
function FilterMenu() {
  const known = knowsMe();
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
    refresh();
  };
  const clear = () => { state.filter = ""; state.overdue = false; state.tags = []; setMine(false); };
  const open = state.menu === "filters";

  return html`<div class="menu-wrap">
    <button type="button" class=${cls("menu-button", "menu-square", (on > 0 || text) && "is-active", open && "is-open")}
      aria-haspopup="true" aria-expanded=${String(open)} title=${`${t.filters} (${SEARCH_KEY})`} aria-label=${t.filters}
      onClick=${() => { if (open) { state.menu = null; refresh(); } else openFilters(); }}>
      <${Icon} name="filter" />
      ${text && html`<span class="filter-term">${text}</span>`}
      ${on > 0 && html`<span class="filter-n">${String(on)}</span>`}
    </button>
    ${open && html`<div class="menu-panel filter-panel">
      <${SearchBox} />
      <div class="filter-rows">
        <${FilterRow} icon="person" label=${t.mine} n=${known ? count(({ card, board }) => isMine(card, board)) : 0}
          checked=${state.mine && known} disabled=${!known} onChange=${(e) => setMine(e.target.checked)} />
        <${FilterRow} icon="clock" label=${t.overdueOnly} n=${count(({ card, board }) => isLate(card, board))}
          checked=${state.overdue} onChange=${(e) => { state.overdue = e.target.checked; refresh(); }} />
      </div>
      ${!known && html`<p class="menu-hint">${code(t.mineHint)}</p>`}
      ${tags.length > 0 && html`
        <h2 class="menu-heading">${t.tags}</h2>
        <div class="filter-tags">${tags.map((tag) => html`
          <button key=${tag} type="button" class=${cls("filter-tag", state.tags.includes(tag) && "is-on")}
            aria-pressed=${String(state.tags.includes(tag))} onClick=${() => toggleTag(tag)}>
            ${tag}<span class="filter-count">${String(tagCounts.get(tag))}</span>
          </button>`)}
        </div>`}
      <div class="filter-foot">
        <span>${t.showing(count(({ card, board }) => matches(card, board)), all.length)}</span>
        ${(on > 0 || text) && html`<button type="button" onClick=${clear}>${t.clearFilters}</button>`}
      </div>
    </div>`}
  </div>`;
}

// SearchBox is the search field on top of the filter menu. It takes the
// cursor when the menu opens. The keys that bring one here make way for
// the text.
function SearchBox() {
  const input = useRef(null);
  useEffect(() => { input.current?.focus(); }, []);
  return html`<label class="filter-search">
    <${Icon} name="search" />
    <input id="filter" ref=${input} class="filter-input" type="search" placeholder=${t.search} value=${state.filter}
      aria-label=${t.search} onInput=${(e) => { state.filter = e.target.value; refresh(); }} />
    ${!state.filter && html`<span class="filter-keys" aria-hidden="true">${SEARCH_KEYS.map((k) => html`<kbd>${k}</kbd>`)}</span>`}
  </label>`;
}

// FilterRow is one filter: a mark, its name, how many cards it leaves, and
// a switch.
function FilterRow({ icon, label, n, checked, onChange, disabled = false }) {
  return html`<label class=${cls("filter-row", disabled && "is-off")}>
    <${Icon} name=${icon} />
    <span class="filter-name">${label}</span>
    <span class="filter-count">${String(n)}</span>
    <input type="checkbox" class="filter-switch" role="switch" checked=${checked} disabled=${disabled} onChange=${onChange} />
  </label>`;
}

function ProjectMenu() {
  const all = state.data.boards;
  const visible = visibleBoards();
  const setHidden = (keys) => { state.hidden = keys; store("hidden", keys); refresh(); };
  const toggle = (board) => {
    const hidden = new Set(all.filter(isHidden).map((b) => b.key));
    hidden.has(board.key) ? hidden.delete(board.key) : hidden.add(board.key);
    setHidden([...hidden]);
  };
  const alone = (board) => {
    const isAlone = visible.length === 1 && visible[0] === board;
    setHidden(isAlone ? [] : all.filter((b) => b !== board).map((b) => b.key));
  };

  let name;
  if (visible.length === 0) name = t.noProjects;
  else if (visible.length === 1) name = visible[0].name;
  else if (visible.length === all.length) name = t.allProjects(all.length);
  else name = t.someProjects(visible.length, all.length);
  const label = html`
    ${visible.length === 1 && html`<${Dot} colour=${colourOf(visible[0])} />`}
    <span class="menu-label">${name}</span>
    <span class="menu-caret" aria-hidden="true">▾</span>`;

  return html`<${Menu} name="projects" label=${label} title=${t.projects} content=${() => html`
    ${all.map((b) => {
      const isAlone = visible.length === 1 && visible[0] === b;
      return html`<div key=${b.key} class=${cls("proj-row", isHidden(b) && "is-off")}>
        <input type="checkbox" class="proj-check" checked=${!isHidden(b)} aria-label=${b.name} onChange=${() => toggle(b)} />
        <button type="button" class="proj-pick" title=${(isAlone ? t.all : t.only) + "\n" + b.root} onClick=${() => alone(b)}>
          <${Dot} colour=${colourOf(b)} />
          <span class="proj-name">${b.name}</span>
          <span class="proj-count">${b.error ? "!" : String(b.cards.length)}</span>
        </button>
      </div>`;
    })}
    <p class="menu-hint">${t.pickHint}</p>
    <button class="proj-rescan" type="button" onClick=${() => load(true)}>${t.rescan}</button>`} />`;
}

// The menu behind the three dots holds everything that is not needed all
// the time: the history, what is wrong, the remotes, the appearance, and
// whether the page is connected. Its button carries a small mark while
// something is wrong or waiting.
function SettingsMenu() {
  // Who the user is, once for all boards where it is the same.
  const names = [...new Set(visibleBoards().filter((b) => b.user).map((b) => b.user.name))];
  const fromAccount = visibleBoards().some((b) => b.user?.source === "account");
  const you = names.length === 1 ? [t.you(names[0])]
    : visibleBoards().filter((b) => b.user).map((b) => t.youOn(b.user.name, b.name));
  const { errors, warnings } = countProblems();
  const setTheme = (theme) => { state.theme = theme; store("theme", theme); applyTheme(); refresh(); };
  const go = (fn) => () => { state.menu = null; fn(); };
  const showProblems = () => { state.open = null; state.edit = null; state.history = null; state.problems = true; refresh(); };
  // Boards that push to a remote. Each gets an entry that says how it
  // stands and brings it level on a click.
  const remotes = visibleBoards().filter((b) => b.sync === "push" && b.git && !b.git.error);
  const waiting = remotes.some((b) => b.git.behind > 0 || b.git.ahead > 0 || b.git.unreachable);
  const pip = errors > 0 ? "is-bad" : warnings > 0 ? "is-warn" : waiting ? "is-news" : "";
  const label = html`<span aria-hidden="true">⋯</span>${pip && html`<span class=${"menu-pip " + pip}></span>`}`;
  const count = [errors > 0 && t.errors(errors), warnings > 0 && t.warnings(warnings)].filter(Boolean).join(" · ");

  return html`<${Menu} name="settings" label=${label} title=${t.menuTitle} square content=${() => html`
    <button type="button" class="menu-item" onClick=${go(openHistory)}><${Icon} name="history" /><span>${t.history}</span></button>
    <button type="button" class="menu-item" onClick=${go(showProblems)}>
      <${Icon} name="bug" /><span>${t.problems}</span>
      ${count && html`<span class=${cls("menu-count", errors > 0 ? "is-bad" : "is-warn")}>${count}</span>`}
    </button>
    ${remotes.length > 0 && html`
      <h2 class="menu-heading">${t.remote}</h2>
      ${remotes.map((b) => html`<button key=${b.key} type="button" class="menu-item" title=${t.syncNow}
        disabled=${state.syncing !== null} onClick=${() => syncBoard(b)}>
        <${Icon} name="remote" /><span class="menu-item-name">${b.name}</span>
        <span class=${cls("menu-count", b.git.unreachable && "is-warn")}>${state.syncing === b.key ? "…" : remoteState(b)}</span>
      </button>`)}`}
    <h2 class="menu-heading">${t.theme}</h2>
    <div class="settings-segments" role="group" aria-label=${t.theme}>
      ${["system", "light", "dark"].map((theme) => html`<button key=${theme} type="button"
        class=${state.theme === theme ? "is-on" : ""} aria-pressed=${String(state.theme === theme)}
        onClick=${() => setTheme(theme)}>${t[theme]}</button>`)}
    </div>
    <hr class="menu-rule" />
    <p class="menu-hint settings-status"><span class=${cls("settings-live", state.live && "is-on")}></span><span>${code(state.live ? t.liveOn : t.liveOff)}</span></p>
    ${you.map((line) => html`<p class="menu-hint">${line}</p>`)}
    ${fromAccount && html`<p class="menu-hint">${code(t.fromAccount)}</p>`}`} />`;
}
