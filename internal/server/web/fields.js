// The fields of the open card: who it belongs to, size, milestone, tags,
// due date and whatever else the board declares. Each is one row with an
// icon, its name and its value, shown the way the card shows it, and what
// belongs together stands together (see group).
//
// "Edit" (in the actions above) turns the fields into controls; what is
// changed there is a draft until "Save" sends it as one `kb set`, which on
// a board that commits is one commit "edit <card>". Escape or "Cancel"
// drops the draft. Who the card belongs to is changed at once, not in the
// draft (see People in panels.js).

import { html, cls } from "./preact.js";
import { t, day, fromToday } from "./texts.js";
import { state, refresh, isLate } from "./state.js";
import { act, openCard } from "./api.js";
import { Icon } from "./parts.js";

// Enums with up to this many values show them side by side; longer ones
// are a picker.
const SEGMENTS_UP_TO = 6;

// Fields kb fills in itself or that are changed elsewhere.
const NOT_HERE = ["created", "assignee"];

const editKey = (board, card) => board.key + "/" + card.id;

// isEditing says whether the fields of this card are being edited.
export const isEditing = (board, card) => state.edit?.key === editKey(board, card);

export function startEditing(board, card) {
  state.edit = { key: editKey(board, card), draft: {} };
  refresh();
}

export function stopEditing() {
  state.edit = null;
  refresh();
}

// raw writes a value the way `kb set` takes it: lists as "a, b", nothing
// as "", which takes the field off the card.
const raw = (value) => (value == null ? "" : Array.isArray(value) ? value.join(", ") : String(value));

// label is the name a field is shown with: the board's title for it, or
// for the fields kb knows by itself, a name in the language of the page.
const label = (field) => (field.title === field.name && t.fieldNames[field.name]) || field.title;

// ICON_BY_NAME and ICON_BY_TYPE pick a field's icon: by name for the
// usual fields, else by its type.
const ICON_BY_NAME = { size: "size", milestone: "flag", tags: "tag", due: "calendar", depends_on: "link", assignee: "person" };
const ICON_BY_TYPE = { person: "person", list: "tag", date: "calendar", cards: "link", enum: "choice", number: "number", bool: "check", text: "text" };
const iconOf = (field) => ICON_BY_NAME[field.name] ?? ICON_BY_TYPE[field.type] ?? "text";

// Name is the left side of a row in the form: icon and name.
const Name = ({ field }) => html`<dt><${Icon} name=${iconOf(field)} /><span>${label(field)}</span></dt>`;

// group puts fields that belong together next to each other: who, when,
// what kind of work, what it waits for, and the rest.
function group(field) {
  if (field.name === "assignee") return 0;
  if (field.name === "created" || field.name === "due") return 1;
  if (field.type === "enum" || field.name === "tags") return 2;
  if (field.type === "cards") return 3;
  return 4;
}
const inOrder = (fields) => [...fields].sort((a, b) => group(a) - group(b));

// Fields shows the card's fields as a section of its own, apart from the
// actions above. people is the row for who the card belongs to, when it
// can be changed there; without it, the people are a row like the others.
// When people is given, kb may change the card, and the pencil at the
// right of the section starts editing the fields.
export function Fields({ board, card, people = null }) {
  if (isEditing(board, card)) return html`<section class="detail-section"><${FieldsForm} board=${board} card=${card} /></section>`;
  return html`<section class="detail-section">
    <${FieldsList} board=${board} card=${card} people=${people} />
    ${people && html`<button type="button" class="detail-edit" aria-label=${t.edit} title=${t.edit}
      onClick=${() => startEditing(board, card)}><${Icon} name="pencil" /></button>`}
  </section>`;
}

// FieldsList shows the fields the card has, each the way the card shows it,
// in the order of the groups: who and when each on a line of their own,
// the short ones below in two columns where there is room. A field
// is named by its icon; the name shows when the pointer rests on it, and
// screen readers read it. When the card was made and when it is due share
// one place.
function FieldsList({ board, card, people }) {
  const facts = [];
  const fact = (key, icon, name, value, wide = false) => facts.push(html`<div key=${key} class=${cls("detail-fact", wide && "is-wide")}>
    <dt title=${name}><${Icon} name=${icon} /><span class="detail-label">${name}</span></dt><dd>${value}</dd></div>`);
  let dates = false;
  for (const field of inOrder(board.fields)) {
    const value = card.fields[field.name];
    if (field.name === "assignee" && people) { fact(field.name, iconOf(field), label(field), people, true); continue; }
    if (group(field) === 1) {
      if (dates || (card.fields.created == null && card.fields.due == null)) continue;
      dates = true;
      fact("dates", "calendar", t.dates, html`<${Dates} board=${board} card=${card} />`, true);
      continue;
    }
    if (value == null) continue;
    fact(field.name, iconOf(field), label(field), html`<${Value} board=${board} card=${card} field=${field} value=${value} />`,
      field.type === "text" || field.type === "person");
  }
  return html`<dl class="detail-facts">${facts}</dl>`;
}

// Dates shows when the card was made and when it is due, as a span of
// time: "11 Nov 2026 → 1 Dec 2026  in 19 days".
function Dates({ board, card }) {
  const { created, due } = card.fields;
  if (due == null) return html`<span class="field-dim">${t.since(day(created))}</span>`;
  return html`${created != null && html`<span class="field-dim">${day(created)}</span><span class="field-dim" aria-hidden="true">→</span>`}
    <span title=${label({ name: "due", title: "due" })}>${day(due)}</span>
    <span class=${cls("field-when", isLate(card, board) && "is-late")}>${fromToday(due)}</span>`;
}

// Value shows one value: enums as small marks like on the card, a due date
// with how far away it is, other cards as buttons that open them.
function Value({ board, card, field, value }) {
  const list = Array.isArray(value) ? value : [value];
  switch (field.type) {
    case "enum":
      return html`<span class="field-chip">${String(value)}</span>`;
    case "date":
      return html`<span>${day(value)}</span><span class="field-when">${fromToday(value)}</span>`;
    case "cards":
      return html`<span class="field-list">${list.map((id) => {
        const other = board.cards.find((c) => c.id === id);
        return other
          ? html`<button key=${id} type="button" class="field-card" title=${other.path} onClick=${() => openCard(board, other)}>${other.title}</button>`
          : html`<span key=${id} class="field-missing">${id}</span>`;
      })}</span>`;
    case "person":
      return html`<span class="field-list">${list.map((name) => html`<span key=${name} class=${cls("field-who", name === state.data.user && "is-me")}>${"@" + name}</span>`)}</span>`;
    case "bool":
      return value ? "✓" : "–";
    case "list":
      return html`<span class="field-list">${list.map((v) => html`<span key=${v}>${v}</span>`)}</span>`;
    default:
      return String(value);
  }
}

// FieldsForm shows every field that can be changed in the draft, including
// the empty ones, with the draft over the card's values. Who the card
// belongs to is left out: that is changed at once, not with "Save".
function FieldsForm({ board, card }) {
  const draft = state.edit.draft;
  const fields = inOrder(board.fields.filter((f) => !NOT_HERE.includes(f.name)));
  const current = (field) => (field.name in draft ? draft[field.name] : card.fields[field.name]);
  const put = (field) => (value) => { draft[field.name] = value; refresh(); };
  const changes = fields
    .filter((f) => f.name in draft && raw(draft[f.name]).trim() !== raw(card.fields[f.name]).trim())
    .map((f) => `${f.name}=${raw(draft[f.name]).trim()}`);

  const save = (e) => {
    e.preventDefault();
    if (changes.length === 0) { stopEditing(); return; }
    // The draft stays until the change is done, also after "Do it anyway".
    act(board, card.id, "set", { set: changes }, false, () => { if (isEditing(board, card)) stopEditing(); });
  };

  return html`<form class="field-form" onSubmit=${save}>
    <dl class="detail-fields is-editable">
      ${fields.map((field, i) => html`<div class=${cls("field-row", i > 0 && group(field) !== group(fields[i - 1]) && "is-group")}>
        <${Name} field=${field} /><dd><${Editor} field=${field} value=${current(field)} put=${put(field)} /></dd></div>`)}
    </dl>
    <div class="field-buttons">
      <button type="submit" class="field-button is-primary">${t.save}</button>
      <button type="button" class="field-button" onClick=${stopEditing}>${t.cancel}</button>
    </div>
  </form>`;
}

// Editor picks the control that fits the field's type.
function Editor({ field, value, put }) {
  const list = Array.isArray(value) ? value : value == null || value === "" ? [] : [value];
  switch (field.type) {
    case "enum":
      return field.values.length <= SEGMENTS_UP_TO
        ? html`<${Segments} field=${field} value=${value} put=${put} />`
        : html`<span class="field-pick"><select class="field-select" aria-label=${label(field)} value=${value ?? ""} onChange=${(e) => put(e.target.value)}>
            <option value="">–</option>
            ${field.values.map((v) => html`<option key=${v} value=${v}>${v}</option>`)}
          </select></span>`;
    case "list":
      return field.values?.length
        ? html`<${Pills} field=${field} value=${list} put=${put} />`
        : html`<${TextField} field=${field} value=${raw(list)} put=${put} placeholder=${t.listHint} />`;
    case "date":
      return html`<input class="field-date" type="date" value=${value ?? ""} aria-label=${label(field)}
        onInput=${(e) => put(e.target.value)} />`;
    case "bool":
      return html`<input class="field-switch" type="checkbox" role="switch" checked=${value === true} aria-label=${label(field)}
        onChange=${(e) => put(e.target.checked)} />`;
    case "number":
      return html`<${TextField} field=${field} value=${raw(value)} put=${put} type="number" />`;
    case "person": case "cards":
      return html`<${TextField} field=${field} value=${raw(list)} put=${put} placeholder=${t.listHint} />`;
    default:
      return html`<${TextField} field=${field} value=${raw(value)} put=${put} />`;
  }
}

// Segments shows the values of a short enum side by side. A click on the
// chosen one takes the value off.
function Segments({ field, value, put }) {
  return html`<div class="field-segments" role="group" aria-label=${label(field)}>
    ${field.values.map((v) => html`<button key=${v} type="button" class=${v === value ? "is-on" : ""}
      aria-pressed=${String(v === value)} title=${v === value ? t.clearValue : ""}
      onClick=${() => put(v === value ? "" : v)}>${v}</button>`)}
  </div>`;
}

// Pills shows every allowed tag; the chosen ones are dark, like the tags
// in the filter menu. Values the board does not allow (any more) stay
// visible, so they can be taken off.
function Pills({ field, value, put }) {
  const toggle = (v) => put(value.includes(v)
    ? value.filter((x) => x !== v)
    : [...field.values, ...value].filter((x, i, all) => (x === v || value.includes(x)) && all.indexOf(x) === i));
  const all = [...field.values, ...value.filter((v) => !field.values.includes(v))];
  return html`<div class="field-tags" role="group" aria-label=${label(field)}>
    ${all.map((v) => html`<button key=${v} type="button" class=${cls("field-tag", value.includes(v) && "is-on")}
      aria-pressed=${String(value.includes(v))} onClick=${() => toggle(v)}>${v}</button>`)}
  </div>`;
}

// TextField holds its text in the draft. Enter saves the whole form.
function TextField({ field, value, put, type = "text", placeholder = "" }) {
  return html`<input class="field-input" type=${type} value=${value} placeholder=${placeholder}
    aria-label=${label(field)} autocomplete="off" onInput=${(e) => put(e.target.value)} />`;
}
