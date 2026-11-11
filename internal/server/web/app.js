// kardboard web UI. It shows what `kb ui` reads from disk and follows
// changes. Cards are changed by asking the server for the same actions the
// command line has.
//
// The page is built with Preact and htm, which lie in vendor/ as plain
// files: no npm and no build step. The parts live in their own modules:
//
//   state.js   the state of the page and what follows from it
//   api.js     loading, following changes, actions
//   top.js     the top bar and its menus
//   board.js   columns, cards, dragging, the card menu
//   panels.js  the panel on the right and the notice
//
// CSS classes start with the name of their part (top-, menu-, filter-,
// proj-, settings-, col-, card-, cmenu-, panel-, detail-, act-, hist-,
// issues-, notice-), so two parts cannot share a name by accident. States
// are is-open, is-on and so on, and are only styled together with such a
// class.

import { html, render, useReducer } from "./preact.js";
import { state, refresh, onRefresh, applyTheme, openFilters } from "./state.js";
import { load, follow } from "./api.js";
import { Brand } from "./parts.js";
import { TopBar } from "./top.js";
import { Main, CardMenu } from "./board.js";
import { Detail, History, Problems, Notice } from "./panels.js";

function App() {
  const [, tick] = useReducer((n) => n + 1, 0);
  onRefresh(tick);

  if (!state.data) {
    return html`<header class="top"><${Brand} /></header>
      <main class="shell-main">${state.error && html`<div class="main-error">${state.error}</div>`}</main>`;
  }
  const panel = state.history ? html`<${History} />`
    : state.problems ? html`<${Problems} />`
    : state.open?.detail ? html`<${Detail} />` : null;
  return html`
    <${TopBar} />
    <div class=${"shell" + (panel ? " is-split" : "")}>
      <${Main} />
      ${panel}
    </div>
    ${state.notice && html`<${Notice} />`}
    ${state.cardMenu && html`<${CardMenu} />`}`;
}

// within says whether an event started inside an element with this class.
// It asks the path the event took, not the element's ancestors now: a click
// may have removed the element it landed on before the event gets here.
const within = (e, ...names) => e.composedPath().some((el) => names.some((n) => el.classList?.contains(n)));

// A click outside an open menu closes it.
document.addEventListener("click", (e) => {
  if (state.menu && !within(e, "menu-wrap")) { state.menu = null; refresh(); }
  if (state.cardMenu && !within(e, "cmenu", "card-dots")) { state.cardMenu = null; refresh(); }
});

// The card menu is placed where its button was. When the page scrolls, that
// place is no longer right, so the menu closes.
document.addEventListener("scroll", () => {
  if (state.cardMenu) { state.cardMenu = null; refresh(); }
}, true);

document.addEventListener("keydown", (e) => {
  if (e.key === "Escape" && state.cardMenu) { state.cardMenu = null; refresh(); return; }
  if (e.key === "Escape" && state.menu) { state.menu = null; refresh(); return; }
  if (e.key === "Escape" && state.edit) { state.edit = null; refresh(); return; }
  if (e.key === "Escape" && state.history) { state.history = null; refresh(); return; }
  if (e.key === "Escape" && state.problems) { state.problems = false; refresh(); return; }
  if (e.key === "Escape" && state.open) { state.open = null; refresh(); }
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
render(html`<${App} />`, document.getElementById("app"));
load().then(follow);
