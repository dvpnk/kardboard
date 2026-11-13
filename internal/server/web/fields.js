// The fields of the open card: who it belongs to, size, milestone, tags,
// due date and whatever else the board declares. Each is one row with an
// icon, its name and its value, shown the way the card shows it, and what
// belongs together stands together (see group).
//
// "Edit" in the actions turns the card into a form: the title in the
// heading, the fields as controls, the text in an editor with a preview.
// What is changed there is a draft until "Save" sends it as one `kard set`
// with the new text, which on a board that commits is one commit
// "edit <card>". Escape or "Cancel" drops the draft. Who the card belongs to is changed at once,
// not in the draft (see People in panels.js).
//
// The draft remembers the card's sum from when editing started. If the
// file changes on disk before "Save", the server refuses rather than
// overwrite what is new; the draft stays.

import { html, cls, useState } from "./preact.js";
import { t, day, fromToday } from "./texts.js";
import { state, refresh, isLate, isMe, personOf } from "./state.js";
import { act, openCard } from "./api.js";
import { Icon, Who } from "./parts.js";
import { render as markdown } from "./md.js";

// Enums with up to this many values show them side by side; longer ones
// are a picker.
const SEGMENTS_UP_TO = 6;

// Fields kard fills in itself or that are changed elsewhere.
const NOT_HERE = ["created", "assignee"];

const editKey = (board, card) => board.key + "/" + card.id;

// isEditing says whether the fields of this card are being edited.
export const isEditing = (board, card) => state.edit?.key === editKey(board, card);

// startEditing opens the form. draft holds changed fields by name. Title
// and text are taken as they are now, together with the sum of the file
// they come from: what the form shows is what the server checks against.
export function startEditing(board, card) {
  const text = textOf(card);
  state.edit = { key: editKey(board, card), draft: {}, title: card.title, text, base: card.sum, from: { title: card.title, text } };
  refresh();
}

// textOf is the card's text below its title line.
export const textOf = (card) => card.body.replace(/^\s*# .*\n?/, "").replace(/^\n+/, "");

// tidy is a text as the server would write it, to tell whether it changed.
const tidy = (text) => text.replace(/\r\n/g, "\n").replace(/[ \t]+$/gm, "").trim();

// FormButtons sit in the panel's footer while the card is edited: they act
// on the whole form, so they sit below all of it, Save at the right. They
// belong to the form by its id, like the title in the heading.
export const FormButtons = () => html`<div class="field-buttons">
  <button type="button" class="field-button" onClick=${stopEditing}>${t.cancel}</button>
  <button type="submit" form="field-form" class="field-button is-primary">${t.save}</button>
</div>`;

// TitleInput takes the place of the heading while the card is edited. It
// belongs to the form below, so Enter in it saves.
export function TitleInput({ card }) {
  return html`<input class="field-title" form="field-form" type="text" required aria-label=${t.title}
    value=${state.edit.title} autocomplete="off"
    onInput=${(e) => { state.edit.title = e.target.value; refresh(); }} />`;
}

export function stopEditing() {
  state.edit = null;
  refresh();
}

// raw writes a value the way `kard set` takes it: lists as "a, b", nothing
// as "", which takes the field off the card.
const raw = (value) => (value == null ? "" : Array.isArray(value) ? value.join(", ") : String(value));

// label is the name a field is shown with: the board's title for it, or
// for the fields kard knows by itself, a name in the language of the page.
const label = (field) => (field.title === field.name && t.fieldNames[field.name]) || field.title;

// ICON_BY_NAME and ICON_BY_TYPE pick a field's icon: by name for the
// usual fields, else by its type.
const ICON_BY_NAME = { size: "size", milestone: "flag", tags: "tag", due: "calendar", depends_on: "link", assignee: "person" };
const ICON_BY_TYPE = { person: "person", list: "tag", date: "calendar", cards: "link", enum: "choice", number: "number", bool: "check", text: "text" };
const iconOf = (field) => ICON_BY_NAME[field.name] ?? ICON_BY_TYPE[field.type] ?? "text";

// Name is the left side of a row in the form: icon and name.
const Name = ({ field }) => html`<dt title=${label(field)}><${Icon} name=${iconOf(field)} /><span>${label(field)}</span></dt>`;

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
export function Fields({ board, card, people = null }) {
  if (isEditing(board, card)) return html`<section class="detail-section"><${FieldsForm} board=${board} card=${card} /></section>`;
  return html`<section class="detail-section">
    <${FieldsList} board=${board} card=${card} people=${people} />
  </section>`;
}

// FieldsList shows the fields the card has, each the way the card shows
// it, one line after the other in the order of the groups: who, when, the
// marks, what the card waits for, and the rest. Each mark has a line of its
// own icon, so it says what it is: the flag of a milestone, the ruler of a
// size, the label of tags. A fact is named by its icon; the name shows when
// the pointer rests on it, and screen readers read it. The people have a
// line of their own; the other facts run on side by side and wrap, so they
// take little room above the text. When the card was made and when it is
// due are one fact.
function FieldsList({ board, card, people }) {
  const facts = [];
  const fact = (key, icon, name, value) => facts.push(html`<div key=${key} class=${cls("detail-fact", key === "assignee" && "is-people")}>
    <dt title=${name}><${Icon} name=${icon} /><span class="detail-label">${name}</span></dt><dd>${value}</dd></div>`);
  let dates = false;
  for (const field of inOrder(board.fields)) {
    const value = card.fields[field.name];
    if (field.name === "assignee" && people) { fact(field.name, iconOf(field), label(field), people); continue; }
    if (group(field) === 1) {
      if (dates || (card.fields.created == null && card.fields.due == null)) continue;
      dates = true;
      fact("dates", "calendar", t.dates, html`<${Dates} board=${board} card=${card} />`);
      continue;
    }
    if (value == null) continue;
    fact(field.name, iconOf(field), label(field), html`<${Value} board=${board} card=${card} field=${field} value=${value} />`);
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
      return html`<span class="field-list">${list.map((id) => html`<${Who} key=${id} person=${personOf(board, id)} mine=${isMe(board, id)} named />`)}</span>`;
    case "bool":
      return value ? "✓" : "–";
    case "list":
      return html`<span class="field-list is-tags">${list.map((v) => html`<span key=${v} class="field-hash">${"#" + v}</span>`)}</span>`;
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
  const title = state.edit.title.trim();
  const text = state.edit.text;
  // Only a text changed here is sent; fields alone never overwrite a text
  // someone else changed meanwhile.
  const { from } = state.edit;
  const textChanged = title !== from.title || tidy(text) !== tidy(from.text);

  const save = (e) => {
    e.preventDefault();
    if (changes.length === 0 && !textChanged) { stopEditing(); return; }
    const extra = { set: changes };
    if (textChanged) Object.assign(extra, { text: { title, body: text }, base: state.edit.base });
    // The draft stays until the change is done, also after "Do it anyway".
    act(board, card.id, "set", extra, false, () => { if (isEditing(board, card)) stopEditing(); });
  };

  return html`<form id="field-form" class="field-form" onSubmit=${save}>
    <dl class="detail-fields is-editable">
      ${fields.map((field, i) => html`<div class=${cls("field-row", i > 0 && group(field) !== group(fields[i - 1]) && "is-group")}>
        <${Name} field=${field} /><dd><${Editor} field=${field} value=${current(field)} put=${put(field)} /></dd></div>`)}
    </dl>
    <${TextEditor} value=${text} put=${(value) => { state.edit.text = value; refresh(); }} />
  </form>`;
}

// TextEditor is the card's text, as Markdown to write or as the page will
// show it. Cmd or Ctrl and Enter saves. Escape leaves the editor first and
// only a second one drops the draft, so a long text is not lost by one key.
function TextEditor({ value, put }) {
  const [preview, setPreview] = useState(false);
  const keyDown = (e) => {
    if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) { e.preventDefault(); e.target.form.requestSubmit(); }
    if (e.key === "Escape") { e.stopPropagation(); e.target.blur(); }
  };
  const tab = (on, label, click) => html`<button type="button" role="tab" aria-selected=${String(on)}
    class=${on ? "is-on" : ""} onClick=${click}>${label}</button>`;
  return html`<div class="field-text">
    <div class="field-segments" role="tablist" aria-label=${t.text}>
      ${tab(!preview, t.write, () => setPreview(false))}${tab(preview, t.preview, () => setPreview(true))}
    </div>
    ${preview
      ? (value.trim()
        ? html`<div class="md field-preview" dangerouslySetInnerHTML=${{ __html: markdown(value) }}></div>`
        : html`<p class="field-preview is-empty">${t.noText}</p>`)
      : html`<textarea class="field-textarea" aria-label=${t.text} value=${value} spellcheck="true"
          onInput=${(e) => put(e.target.value)} onKeyDown=${keyDown}></textarea>`}
  </div>`;
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
