import { describe, it, expect } from "vitest";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { BentoGrid, BentoTile } from "./BentoGrid";

describe("BentoGrid Deep Module", () => {
  it("renders 12-column grid and tile spans properly", () => {
    const html = renderToStaticMarkup(
      React.createElement(
        BentoGrid,
        null,
        React.createElement(
          BentoTile,
          { span: 6, rows: 2, variant: "hero", title: "核心利润总览" },
          "Hero KPI"
        ),
        React.createElement(
          BentoTile,
          { span: 3, rows: 1, variant: "metric", title: "门店覆盖率" },
          "98.5%"
        ),
        React.createElement(
          BentoTile,
          { span: 3, rows: 1, variant: "accent", title: "预警提醒" },
          "2 条待处理"
        )
      )
    );

    expect(html).toContain("bento-grid");
    expect(html).toContain("bento-tile--hero");
    expect(html).toContain("bento-tile--metric");
    expect(html).toContain("bento-tile--accent");
    expect(html).toContain("核心利润总览");
    expect(html).toContain("Hero KPI");
  });
});
