// A small Markdown renderer for card texts. It covers what planning cards
// use: headings, paragraphs, lists, tables, code, quotes, emphasis and links.
// All text is escaped before any tag is added, and only the tags written
// here can come out, so a card cannot inject markup into the page.

const escape = (s) =>
  s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");

function inline(text) {
  // Take code spans out first so nothing inside them is formatted.
  const spans = [];
  let out = escape(text).replace(/`([^`]+)`/g, (_, code) => {
    spans.push(`<code>${code}</code>`);
    return `@@kbcode${spans.length - 1}@@`;
  });
  out = out
    .replace(/\[([^\]]+)\]\((https?:\/\/[^\s)]+)\)/g, '<a href="$2" target="_blank" rel="noopener noreferrer">$1</a>')
    .replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>")
    .replace(/(^|[\s(])\*([^*\s][^*]*)\*/g, "$1<em>$2</em>")
    .replace(/(^|[\s(])_([^_\s][^_]*)_(?=$|[\s.,;:!?)])/g, "$1<em>$2</em>");
  return out.replace(/@@kbcode(\d+)@@/g, (_, i) => spans[Number(i)]);
}

const cells = (line) =>
  line.trim().replace(/^\|/, "").replace(/\|$/, "").split("|").map((c) => c.trim());

const isRule = (line) => /^\s*\|?\s*:?-{2,}:?\s*(\|\s*:?-{2,}:?\s*)*\|?\s*$/.test(line);
const listItem = (line) => line.match(/^(\s*)([-*+]|\d+[.)])\s+(.*)$/);

// render turns Markdown into an HTML string.
export function render(markdown) {
  const lines = markdown.replace(/\r\n/g, "\n").split("\n");
  const html = [];
  let i = 0;

  while (i < lines.length) {
    const line = lines[i];

    if (line.trim() === "") { i++; continue; }

    const fence = line.match(/^\s*(```|~~~)/);
    if (fence) {
      const code = [];
      i++;
      while (i < lines.length && !lines[i].trim().startsWith(fence[1])) code.push(lines[i++]);
      i++;
      html.push(`<pre><code>${escape(code.join("\n"))}</code></pre>`);
      continue;
    }

    const heading = line.match(/^(#{1,6})\s+(.*?)\s*#*\s*$/);
    if (heading) {
      const level = Math.min(heading[1].length, 4);
      html.push(`<h${level}>${inline(heading[2])}</h${level}>`);
      i++;
      continue;
    }

    if (/^\s*([-*_])(\s*\1){2,}\s*$/.test(line)) { html.push("<hr>"); i++; continue; }

    if (line.includes("|") && i + 1 < lines.length && isRule(lines[i + 1])) {
      const head = cells(line);
      i += 2;
      const rows = [];
      while (i < lines.length && lines[i].includes("|") && lines[i].trim() !== "") rows.push(cells(lines[i++]));
      html.push(
        "<table><thead><tr>" + head.map((c) => `<th>${inline(c)}</th>`).join("") + "</tr></thead><tbody>" +
        rows.map((r) => "<tr>" + head.map((_, k) => `<td>${inline(r[k] ?? "")}</td>`).join("") + "</tr>").join("") +
        "</tbody></table>");
      continue;
    }

    if (/^\s*>/.test(line)) {
      const quote = [];
      while (i < lines.length && /^\s*>/.test(lines[i])) quote.push(lines[i++].replace(/^\s*>\s?/, ""));
      html.push(`<blockquote>${render(quote.join("\n"))}</blockquote>`);
      continue;
    }

    const first = listItem(line);
    if (first) {
      const ordered = /\d/.test(first[2]);
      const indent = first[1].length;
      const items = [];
      while (i < lines.length) {
        const m = listItem(lines[i]);
        if (m && m[1].length <= indent) {
          if (/\d/.test(m[2]) !== ordered) break;
          items.push([m[3]]);
          i++;
        } else if (lines[i].trim() === "") {
          // A blank line ends the list unless the next line continues it.
          const next = lines[i + 1];
          if (next === undefined || !(listItem(next) || /^\s{2,}\S/.test(next))) break;
          i++;
        } else if (/^\s+\S/.test(lines[i]) && items.length) {
          // Indented lines belong to the item: nested lists or more text.
          items[items.length - 1].push(lines[i].replace(new RegExp(`^\\s{0,${indent + 4}}`), ""));
          i++;
        } else {
          break;
        }
      }
      const tag = ordered ? "ol" : "ul";
      html.push(`<${tag}>` + items.map(([head, ...rest]) => {
        const nested = rest.length ? render(rest.join("\n")) : "";
        return `<li>${inline(head)}${nested}</li>`;
      }).join("") + `</${tag}>`);
      continue;
    }

    const para = [];
    while (i < lines.length && lines[i].trim() !== "" &&
      !/^(#{1,6}\s|\s*(```|~~~)|\s*>)/.test(lines[i]) && !listItem(lines[i]) &&
      !(lines[i].includes("|") && i + 1 < lines.length && isRule(lines[i + 1]))) {
      para.push(lines[i++].trim());
    }
    if (para.length === 0) para.push(lines[i++].trim()); // never stand still
    // Lines are joined as in Markdown, with one exception: a line that
    // starts with a bold label ("**Size:** M") keeps its own line. Cards
    // often list their facts that way, one per line.
    const parts = [];
    for (const l of para) {
      if (parts.length && /^\*\*[^*]+:\*\*/.test(l)) parts.push([l]);
      else if (parts.length) parts[parts.length - 1].push(l);
      else parts.push([l]);
    }
    html.push(`<p>${parts.map((p) => inline(p.join(" "))).join("<br>")}</p>`);
  }
  return html.join("\n");
}
