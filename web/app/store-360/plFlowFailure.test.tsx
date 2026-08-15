/**
 * FIX-024: a failed /pl-flow request must not be presented as an empty one.
 *
 * The regression this pins is concrete: the store-360 page swallowed the
 * rejection (`.catch(() => setPlFlow(null))`), so when the backend did not
 * serve the route at all the panel rendered its "pick a store" empty state.
 * A dead endpoint and a store with no flow looked identical.
 */
import { describe, expect, it } from "vitest";
import React from "react";
import { readFileSync } from "node:fs";
import path from "node:path";
import { renderToStaticMarkup } from "react-dom/server";
import ProfitFlowPanel from "./ProfitFlowPanel";
import { LanguageProvider } from "../context/LanguageContext";
import { t, type Language } from "../lib/i18n";

const zh = "zh-CN" as Language;
const page = readFileSync(path.join(import.meta.dirname, "page.tsx"), "utf8");

function render(children: React.ReactNode) {
  return renderToStaticMarkup(React.createElement(LanguageProvider, null, children));
}

describe("FIX-024: pl-flow failures surface", () => {
  it("renders the failure with its reason, not the empty state", () => {
    const markup = render(
      React.createElement(ProfitFlowPanel, {
        flow: null,
        error: "请求未成功（/api/v1/retail/stores/x/pl-flow），请重试。",
        language: zh,
      })
    );
    expect(markup).toContain(t("store360.pl_flow.load_failed", zh));
    expect(markup).toContain("/pl-flow");
    // The empty-state copy must not be what a broken endpoint shows.
    expect(markup).not.toContain(t("store360.pl_flow.pick_store", zh));
  });

  it("still shows the empty state when there is no error", () => {
    const markup = render(React.createElement(ProfitFlowPanel, { flow: null, error: null, language: zh }));
    expect(markup).toContain(t("store360.pl_flow.pick_store", zh));
    expect(markup).not.toContain(t("store360.pl_flow.load_failed", zh));
  });

  it("the page passes the rejection on instead of discarding it", () => {
    expect(page).toContain("setPlFlowError(apiErrorMessage(");
    expect(page).toContain("error={plFlowError}");
    // The old swallow-everything catch is gone.
    expect(page).not.toMatch(/\.catch\(\(\)\s*=>\s*\{[^}]*setPlFlow\(null\)[^}]*\}\)/);
  });
});
