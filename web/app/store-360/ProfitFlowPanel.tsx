"use client";

import { Typography } from "antd";
import { ResponsiveContainer, Sankey, Tooltip as ChartTooltip } from "recharts";
import { t, type Language } from "../lib/i18n";
import type { RetailPlFlowResponse } from "../lib/api";
import { fmtMoney } from "../lib/format";
import { StateBlock } from "../components/StateBlock";

/**
 * SANKEY-001 一期：门店利润流向。单一「营业额」节点 → 四项费用 + 门店贡献。
 * residual 显式展示（左右不平不抹平）；partial 时标注缺失字段。
 * 二期（营收按大类分流）与三期（品类利润）见后端 pl_flow.go 接口注释。
 *
 * FIX-028: this is the *body* of a view, not a card. It used to own an outer
 * <Card> and render a bare <Sankey height={260}> — with no ResponsiveContainer
 * and therefore no width, recharts laid out nothing and the card showed a tall
 * blank box under a "complete" status line. It now renders inside the change
 * card's existing .chart-frame, which is what gives every other chart on this
 * page its measured box.
 */
/** recharts draws Sankey nodes as bare rectangles — a flow diagram with no
 *  labels cannot be read at all, so the node carries its own name and amount.
 *  Sources sit left of their rectangle, sinks right of theirs, which keeps the
 *  text off the ribbons. */
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
        {payload?.name}
        <tspan fill="var(--fg-tertiary)"> {fmtMoney(payload?.value, unit)}</tspan>
      </text>
    </g>
  );
}

export default function ProfitFlowPanel({ flow, error, currency, language }: { flow: RetailPlFlowResponse | null; error?: string | null; currency?: string; language: Language }) {
  // FIX-024: a failed request is not an empty one. It gets its own presentation
  // with the reason attached, so "the endpoint is not deployed" can never read
  // as "this store has no profit flow".
  // STATE-004: both degraded states render through StateBlock (failed keeps
  // the reason and no retry affordance — the panel has no retry channel).
  if (error) {
    return <StateBlock state={{ kind: "failed", message: t("store360.pl_flow.load_failed", language), reason: error }} language={language} />;
  }
  if (!flow || flow.status === "unavailable") {
    return <StateBlock state={{ kind: "empty", reason: flow?.reason ? t("store360.pl_flow.unavailable", language) : t("store360.pl_flow.pick_store", language) }} language={language} />;
  }
  const unit = currency || flow.currency || "";
  const indexByKey = new Map(flow.nodes.map((node, index) => [node.key, index]));
  const data = {
    nodes: flow.nodes.map((node) => ({ name: node.label })),
    links: flow.links.map((link) => ({
      source: indexByKey.get(link.from) ?? 0,
      target: indexByKey.get(link.to) ?? 0,
      value: Math.max(link.value, 0),
    })),
  };
  return (
    <>
      <div className="chart-frame">
        <ResponsiveContainer width="100%" height="100%">
          <Sankey
            data={data}
            nodePadding={24}
            // Both margins are label gutters, not padding: the source label
            // sits left of its node and the sink labels right of theirs, and
            // each carries a name plus a formatted amount.
            margin={{ top: 12, right: 140, bottom: 12, left: 124 }}
            link={{ stroke: "var(--chart-blue)", strokeOpacity: 0.25 }}
            node={<FlowNode unit={unit} />}
          >
            <ChartTooltip formatter={(value) => fmtMoney(Number(value), unit)} />
          </Sankey>
        </ResponsiveContainer>
      </div>
      <div className="pl-flow-meta">
        <span>{t("store360.pl_flow.status", language)}: {flow.status}</span>
        <span>
          {t("store360.pl_flow.residual", language)}: {fmtMoney(flow.residual, unit)}
        </span>
        {flow.missing && flow.missing.length > 0 && (
          <span>
            {t("store360.pl_flow.missing", language)}: {flow.missing.join(", ")}
          </span>
        )}
        <span>{t("store360.pl_flow.formula", language)}: {flow.formula_version}</span>
      </div>
    </>
  );
}
