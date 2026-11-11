// The fields of the open card: size, milestone, tags, due date and whatever
// else the board declares. They are shown as a quiet list. "Edit" (in the
// actions above) turns them into controls; what is changed there is a
// draft until "Save" sends it as one `kb set`, which on a board that
// commits is one commit "edit <card>". Escape or "Cancel" drops the draft.
// Who the card belongs to is changed in the actions, not here.

import { html, cls } from "./preact.js";
import { t, day } from "./texts.js";
import { state, refresh } from "./state.js";
import { act } from "./api.js";

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

// shown turns a field's value into text for the list.
function shown(field, value) {
  if (field.type === "date") return day(value);
  return Array.isArray(value) ? value.join(", ") : String(value);
}

export function Fields({ board, card }) {
  return isEditing(board, card)
    ? html`<${FieldsForm} board=${board} card=${card} />`
    : html`<${FieldsList} board=${board} card=${card} />`;
}

// FieldsList shows the fields the card has, as text.
function FieldsList({ board, card }) {
  const rows = [];
  for (const field of board.fields) {
    const value = card.fields[field.name];
    if (value != null) rows.push(html`<dt>${field.title}</dt><dd>${shown(field, value)}</dd>`);
  }
  return rows.length ? html`<dl class="detail-fields">${rows}</dl>` : null;
}

// FieldsForm shows every field that can be changed here, including the
// empty ones, with the draft over the card's values.
function FieldsForm({ board, card }) {
  const draft = state.edit.draft;
  const fields = board.fields.filter((f) => !NOT_HERE.includes(f.name));
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
      ${fields.map((field) => html`<dt>${field.title}</dt>
        <dd><${Editor} field=${field} value=${current(field)} put=${put(field)} /></dd>`)}
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
        : html`<select class="field-select" aria-label=${field.title} value=${value ?? ""} onChange=${(e) => put(e.target.value)}>
            <option value="">–</option>
            ${field.values.map((v) => html`<option key=${v} value=${v}>${v}</option>`)}
          </select>`;
    case "list":
      return field.values?.length
        ? html`<${Pills} field=${field} value=${list} put=${put} />`
        : html`<${TextField} field=${field} value=${raw(list)} put=${put} placeholder=${t.listHint} />`;
    case "date":
      return html`<input class="field-date" type="date" value=${value ?? ""} aria-label=${field.title}
        onInput=${(e) => put(e.target.value)} />`;
    case "bool":
      return html`<input class="field-switch" type="checkbox" role="switch" checked=${value === true} aria-label=${field.title}
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
  return html`<div class="field-segments" role="group" aria-label=${field.title}>
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
  return html`<div class="field-tags" role="group" aria-label=${field.title}>
    ${all.map((v) => html`<button key=${v} type="button" class=${cls("field-tag", value.includes(v) && "is-on")}
      aria-pressed=${String(value.includes(v))} onClick=${() => toggle(v)}>${v}</button>`)}
  </div>`;
}

// TextField holds its text in the draft. Enter saves the whole form.
function TextField({ field, value, put, type = "text", placeholder = "" }) {
  return html`<input class="field-input" type=${type} value=${value} placeholder=${placeholder}
    aria-label=${field.title} autocomplete="off" onInput=${(e) => put(e.target.value)} />`;
}
