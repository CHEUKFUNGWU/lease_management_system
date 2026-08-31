"use client";

import React, { useMemo } from "react";
import { Typography } from "antd";
import { ResponsiveContainer, Sankey, Tooltip as ChartTooltip } from "recharts";
import { t, type Language } from "../lib/i18n";
import type { RetailPlFlowResponse } from "../lib/api";
import { fmtMoney } from "../lib/format";
import { StateBlock } from "../components/StateBlock";
import { kpiLabel } from "../operating-pulse/logic";

/**
 * SANKEY-001: 门店利润流向桑基图。
 * 遵循 CONTRACT-001：节点与流向完全由后端契约驱动 (flow.nodes.map / flow.links.map)，前端不硬编码节点清单。
 * 遵循 FIX-028：嵌入 .chart-frame 容器，内部使用 ResponsiveContainer 确保自适应布局。
 */
function FlowNode({ x, y, width, height, index, payload, unit }: any) {
  const isSource = payload?.sourceNodes?.length === 0 || payload?.depth === 0;
  return (
    <g>
      <rect x={x} y={y} width={width} height={height} fill="var(--chart-blue)" rx={2} />
      <text
        x={isSource ? x - 8 : x + width + 8}
        y={y + height / 2}
        textAnchor={isSource ? "end" : "start"}
        dominantBaseline="middle"
        fontSize={11}
        fill="var(--fg-secondary)"
        key={`label-${index}`}
      >
        <tspan fontWeight={500}>{payload?.name}</tspan>
        <tspan fill="var(--fg-tertiary)"> {fmtMoney(payload?.value, unit)}</tspan>
      </text>
    </g>
  );
}

export default function ProfitFlowPanel({
  flow,
  error,
  currency,
  language,
}: {
  flow: RetailPlFlowResponse | null;
  error?: string | null;
  currency?: string;
  language: Language;
}) {
  const data = useMemo(() => {
    if (!flow || flow.status === "unavailable" || !flow.nodes) {
      return { nodes: [], links: [] };
    }
    const indexByKey = new Map(flow.nodes.map((node, index) => [node.key, index]));
    return {
      nodes: flow.nodes.map((node) => ({ name: node.label })),
      links: flow.links.map((link) => ({
        source: indexByKey.get(link.from) ?? 0,
        target: indexByKey.get(link.to) ?? 0,
        value: Math.max(link.value, 0),
      })),
    };
  }, [flow]);

  if (error) {
    return (
      <StateBlock
        state={{
          kind: "failed",
          message: t("store360.pl_flow.load_failed", language),
          reason: error,
        }}
        language={language}
      />
    );
  }

  if (!flow || flow.status === "unavailable") {
    return (
      <StateBlock
        state={{
          kind: "empty",
          reason: flow?.reason
            ? t("store360.pl_flow.unavailable", language)
            : t("store360.pl_flow.pick_store", language),
        }}
        language={language}
      />
    );
  }

  const unit = currency || flow.currency || "";

  return (
    <>
      <div className="chart-frame profit-flow-chart">
        <ResponsiveContainer width="100%" height={280}>
          <Sankey
            data={data}
            nodePadding={24}
            margin={{ top: 16, right: 140, bottom: 16, left: 130 }}
            link={{ stroke: "var(--chart-blue)", strokeOpacity: 0.25 }}
            node={<FlowNode unit={unit} />}
          >
            <ChartTooltip formatter={(value) => fmtMoney(Number(value), unit)} />
          </Sankey>
        </ResponsiveContainer>
      </div>
      <div className="pl-flow-meta">
        <span>{t("store360.pl_flow.status", language)}: {flow.status === "complete" ? t("store360.pl_flow.status_complete", language) : flow.status === "partial" ? t("store360.pl_flow.status_partial", language) : flow.status}</span>
        <span>
          {t("store360.pl_flow.residual", language)}: {fmtMoney(flow.residual, unit)}
        </span>
        {flow.missing && flow.missing.length > 0 && (
          <span>
            {t("store360.pl_flow.missing", language)}: {flow.missing.map((m) => kpiLabel(m, language) || m).join(", ")}
          </span>
        )}
        <span>{t("trust.formula", language)}: {flow.formula_version}</span>
      </div>
    </>
  );
}
