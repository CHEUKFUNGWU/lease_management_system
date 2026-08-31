import { readFileSync } from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";

describe("page-fill 会话卡片", () => {
  const page = readFileSync(path.join(import.meta.dirname, "page.tsx"), "utf8");

  it("显示 payload、suggestions 的 provenance 并提供 deep_link 动作", () => {
    expect(page).toContain('artifact.artifact_type === "page_fill"');
    expect(page).toContain("data.payload");
    expect(page).toContain("data.suggestions");
    expect(page).toContain("entry?.provenance?.basis");
    expect(page).toContain("data.document_class");
    expect(page).toContain("data.classification_confidence");
    expect(page).toContain("onOpen(deepLink)");
  });

  it("只接受站内 deep_link", () => {
    expect(page).toContain('data.deep_link.startsWith("/")');
    expect(page).toContain('!data.deep_link.startsWith("//")');
  });
});
