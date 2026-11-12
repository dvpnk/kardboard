// Small pieces used all over the page: icons, the coloured dot of a
// project, the name in the top bar, and code in texts.

import { html } from "./preact.js";

// The line icons. The shapes are drawn here, so the page needs no icon font
// and no image files. Each is a function, so every use gets its own nodes.
const ICONS = {
  // a clock: the card is past its due date
  clock: () => html`<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3.5 2"/>`,
  // filter menu: one's own cards
  person: () => html`<circle cx="12" cy="8" r="3.6"/><path d="M5 20c.9-3.7 3.6-5.6 7-5.6s6.1 1.9 7 5.6"/>`,
  // top bar: filters, search, the history of the boards
  filter: () => html`<path d="M4 5.5h16l-6.2 7.2V18l-3.6 1.8v-7.1z"/>`,
  search: () => html`<circle cx="11" cy="11" r="6.5"/><path d="m16 16 4.5 4.5"/>`,
  history: () => html`<path d="M3.5 12a8.5 8.5 0 1 0 2.6-6.1L3.5 8.4"/><path d="M3.5 3.8v4.6h4.6"/><path d="M12 7.5V12l3 2"/>`,
  // history rows: a card moved, appeared, went away, or was changed
  moved: () => html`<path d="M5 12h14M13 6l6 6-6 6"/>`,
  added: () => html`<path d="M12 5v14M5 12h14"/>`,
  removed: () => html`<path d="M5 12h14"/>`,
  commit: () => html`<circle cx="12" cy="12" r="3.5"/><path d="M12 3v5.5M12 15.5V21"/>`,
  // the menu: bring a board level with its remote
  remote: () => html`<path d="M4 11a8 8 0 0 1 14-4.6l2 2.1"/><path d="M20 4v4.5h-4.5"/><path d="M20 13a8 8 0 0 1-14 4.6l-2-2.1"/><path d="M4 20v-4.5h4.5"/>`,
  // the fields of a card, in the panel: by name for the usual ones, else
  // by type (see fields.js)
  calendar: () => html`<rect x="4" y="5.5" width="16" height="14.5" rx="1.5"/><path d="M4 10h16M8.5 3.5v4M15.5 3.5v4"/>`,
  tag: () => html`<path d="M3.5 12.2V4.5a1 1 0 0 1 1-1h7.7l8.3 8.3-8.7 8.7z"/><circle cx="8.3" cy="8.3" r="1.2"/>`,
  link: () => html`<path d="M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1"/><path d="M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1-1"/>`,
  flag: () => html`<path d="M5 21V4"/><path d="M5 4h12l-2.5 4 2.5 4H5"/>`,
  size: () => html`<rect x="3" y="8" width="18" height="8" rx="1"/><path d="M7 8v3M11 8v4M15 8v3"/>`,
  number: () => html`<path d="M9.5 4 7.5 20M16.5 4l-2 16M4.5 9h15.5M4 15h15.5"/>`,
  check: () => html`<rect x="4" y="4" width="16" height="16" rx="2"/><path d="m8.5 12 2.5 2.5 5-5"/>`,
  text: () => html`<path d="M5 6h14M5 12h14M5 18h9"/>`,
  choice: () => html`<circle cx="12" cy="12" r="8"/><circle cx="12" cy="12" r="2.5"/>`,
  // the panel: change the fields of the card
  pencil: () => html`<path d="M4 20h4L19 9l-4-4L4 16z"/><path d="m13.5 6.5 4 4"/>`,
  // a bug: the card breaks a rule of the format
  bug: () => html`<ellipse cx="12" cy="14" rx="5" ry="6"/><path d="M9 8.6a3 3 0 0 1 6 0"/><path d="M9.5 5.5 8 3.5M14.5 5.5 16 3.5"/><path d="M7 12H3.5M7.2 16.5 4 18M17 12h3.5M16.8 16.5 20 18M12 9v11"/>`,
};

// Icon draws a small line icon. With a label it is an image with that
// name; without one, screen readers skip it.
export function Icon({ name, label = "" }) {
  const svg = html`<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${ICONS[name]()}</svg>`;
  return label
    ? html`<span class=${"ic ic-" + name} title=${label} role="img" aria-label=${label}>${svg}</span>`
    : html`<span class=${"ic ic-" + name} aria-hidden="true">${svg}</span>`;
}

// Dot is the colour of a project.
export const Dot = ({ colour }) => html`<span class="dot" aria-hidden="true" style=${{ background: colour }}></span>`;

// Brand draws the name with the mark in front of it. The mark is the same
// shape as favicon.svg and takes the colour of the text, so it follows the theme.
export const Brand = () => html`<h1 class="top-brand"><span class="top-mark" aria-hidden="true"><svg viewBox="0 0 32 32" width="16" height="16" fill="currentColor" aria-hidden="true"><path d="M2 2h6v28H2z"/><path fill-rule="evenodd" d="M24 2h6v28h-6C22 25 6 23 6 16S22 7 24 2ZM24 8C20 12 12 12 12 16s8 4 12 8Z"/></svg></span>kardboard</h1>`;

// code turns `like this` in a UI text into <code> elements.
export const code = (text) => text.split("`").map((part, i) => (i % 2 ? html`<code>${part}</code>` : part));
