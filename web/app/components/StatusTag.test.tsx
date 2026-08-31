import { describe, expect, it } from "vitest";
import React from "react";
import { readFileSync } from "node:fs";
import path from "node:path";
import { renderToStaticMarkup } from "react-dom/server";
import { StatusTag } from "./StatusTag";

const css = readFileSync(path.join(import.meta.dirname, "../globals.css"), "utf8");
const statusRule = /\.status-tag\s*\{([^}]*)\}/.exec(css)?.[1] || "";

describe("StatusTag", () => {
  it("publishes a semantic kind class and keeps custom layout styles", () => {
    const html = renderToStaticMarkup(
      <StatusTag kind="success" style={{ marginLeft: 8 }}>
        Ready
      </StatusTag>,
    );

    expect(html).toContain("status-tag status-tag-success");
    expect(html).toContain("margin-left:8px");
    expect(html).toContain('class="status-tag-icon"');
  });

  it("uses a dot and text instead of a filled pill", () => {
    expect(htmlFor("success")).toContain("status-tag-dot");
    expect(statusRule).toMatch(/padding:\s*0/);
    expect(statusRule).toMatch(/border:\s*0/);
    expect(statusRule).toMatch(/background:\s*transparent/);
  });
});

function htmlFor(kind: "success" | "processing" | "warning" | "error" | "neutral") {
  return renderToStaticMarkup(<StatusTag kind={kind}>Ready</StatusTag>);
}
