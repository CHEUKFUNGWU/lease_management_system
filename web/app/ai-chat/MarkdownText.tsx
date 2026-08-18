"use client";

import React from "react";

/**
 * A deliberately small Markdown subset for assistant replies: headings, bold,
 * inline code, bullet and ordered lists, and pipe tables.
 *
 * It builds React elements directly and never touches dangerouslySetInnerHTML.
 * Assistant text is only semi-trusted — the deterministic path is ours, but an
 * LLM turn is not — so a markdown-to-HTML pipeline would need a sanitiser to be
 * safe. Emitting elements sidesteps that class of bug entirely, at the cost of
 * supporting less syntax than a full parser.
 *
 * Anything it does not recognise falls through as literal text, so an
 * unsupported construct degrades to what the reader saw before rather than
 * disappearing.
 */

type Block =
  | { kind: "heading"; level: number; text: string }
  | { kind: "paragraph"; text: string }
  | { kind: "list"; ordered: boolean; items: string[] }
  | { kind: "table"; header: string[]; rows: string[][] };

const HEADING = /^(#{1,4})\s+(.*)$/;
const BRACKET_HEADING = /^\s*【([^】]+)】\s*(.*)$/;
const BULLET = /^\s*[-*]\s+(.*)$/;
const ORDERED = /^\s*\d+[.)]\s+(.*)$/;
const TABLE_ROW = /^\s*\|(.+)\|\s*$/;
const TABLE_DIVIDER = /^\s*\|?[\s:|-]+\|?\s*$/;

function splitTableRow(line: string): string[] {
  const match = TABLE_ROW.exec(line);
  if (!match) return [];
  return match[1].split("|").map((cell) => cell.trim());
}

export function parseBlocks(source: string): Block[] {
  const lines = source.split("\n");
  const blocks: Block[] = [];
  let paragraph: string[] = [];

  const flushParagraph = () => {
    const text = paragraph.map((l) => l.trim()).filter(Boolean).join("\n");
    if (text) blocks.push({ kind: "paragraph", text });
    paragraph = [];
  };

  for (let i = 0; i < lines.length; i += 1) {
    const rawLine = lines[i];
    const line = rawLine.trim();

    if (!line) {
      flushParagraph();
      continue;
    }

    const heading = HEADING.exec(line);
    if (heading) {
      flushParagraph();
      blocks.push({ kind: "heading", level: heading[1].length, text: heading[2].trim() });
      continue;
    }

    const bracket = BRACKET_HEADING.exec(line);
    if (bracket) {
      flushParagraph();
      blocks.push({ kind: "heading", level: 3, text: bracket[2] ? `【${bracket[1]}】 ${bracket[2]}` : `【${bracket[1]}】` });
      continue;
    }

    // A table needs a header row followed by a divider; without the divider the
    // pipes are just characters in a sentence.
    if (TABLE_ROW.test(line) && i + 1 < lines.length && TABLE_DIVIDER.test(lines[i + 1].trim()) && TABLE_ROW.test(lines[i + 1].trim())) {
      flushParagraph();
      const header = splitTableRow(line);
      const rows: string[][] = [];
      i += 2;
      while (i < lines.length && TABLE_ROW.test(lines[i].trim())) {
        rows.push(splitTableRow(lines[i].trim()));
        i += 1;
      }
      i -= 1;
      blocks.push({ kind: "table", header, rows });
      continue;
    }

    if (BULLET.test(line) || ORDERED.test(line)) {
      flushParagraph();
      const ordered = !BULLET.test(line);
      const items: string[] = [];
      while (i < lines.length) {
        const itemLine = lines[i].trim();
        const bullet = BULLET.exec(itemLine);
        const numbered = ORDERED.exec(itemLine);
        if (ordered && numbered) items.push(numbered[1]);
        else if (!ordered && bullet) items.push(bullet[1]);
        else break;
        i += 1;
      }
      i -= 1;
      blocks.push({ kind: "list", ordered, items });
      continue;
    }

    paragraph.push(line);
  }
  flushParagraph();
  return blocks;
}

/** Renders `**bold**` and `` `code` `` inside a line of text. */
export function renderInline(text: string, keyPrefix: string): React.ReactNode[] {
  const nodes: React.ReactNode[] = [];
  const pattern = /(\*\*[^*]+\*\*|`[^`]+`)/g;
  let lastIndex = 0;
  let match: RegExpExecArray | null;
  let index = 0;

  while ((match = pattern.exec(text)) !== null) {
    if (match.index > lastIndex) nodes.push(text.slice(lastIndex, match.index));
    const token = match[0];
    if (token.startsWith("**")) {
      nodes.push(<strong key={`${keyPrefix}-b${index}`}>{token.slice(2, -2)}</strong>);
    } else {
      nodes.push(<code key={`${keyPrefix}-c${index}`} className="ai-md-code">{token.slice(1, -1)}</code>);
    }
    lastIndex = match.index + token.length;
    index += 1;
  }
  if (lastIndex < text.length) nodes.push(text.slice(lastIndex));
  return nodes;
}

export default function MarkdownText({ content }: { content: string }) {
  const blocks = React.useMemo(() => parseBlocks(content), [content]);

  return (
    <div className="ai-md">
      {blocks.map((block, idx) => {
        if (block.kind === "heading") {
          return (
            <div key={idx} className={`ai-md-heading ai-md-heading-${Math.min(block.level, 4)}`}>
              {renderInline(block.text, `h${idx}`)}
            </div>
          );
        }
        if (block.kind === "list") {
          const items = block.items.map((item, itemIdx) => (
            <li key={itemIdx}>{renderInline(item, `l${idx}-${itemIdx}`)}</li>
          ));
          return block.ordered
            ? <ol key={idx} className="ai-md-list">{items}</ol>
            : <ul key={idx} className="ai-md-list">{items}</ul>;
        }
        if (block.kind === "table") {
          return (
            <div key={idx} className="ai-md-table-wrap">
              <table className="ai-md-table">
                <thead>
                  <tr>{block.header.map((cell, cellIdx) => <th key={cellIdx}>{renderInline(cell, `th${idx}-${cellIdx}`)}</th>)}</tr>
                </thead>
                <tbody>
                  {block.rows.map((row, rowIdx) => (
                    <tr key={rowIdx}>{row.map((cell, cellIdx) => <td key={cellIdx}>{renderInline(cell, `td${idx}-${rowIdx}-${cellIdx}`)}</td>)}</tr>
                  ))}
                </tbody>
              </table>
            </div>
          );
        }
        return <p key={idx} className="ai-md-paragraph">{renderInline(block.text, `p${idx}`)}</p>;
      })}
    </div>
  );
}
