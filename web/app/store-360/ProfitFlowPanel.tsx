"use client";

import { Card, Empty, Typography } from "antd";
import { Sankey } from "recharts";
import { t, type Language } from "../lib/i18n";
import type { RetailPlFlowResponse } from "../lib/api";
import { fmtMoney } from "../lib/format";

/**
 * SANKEY-001 一期：门店利润流向。单一「营业额」节点 → 四项费用 + 门店贡献。
 * residual 显式展示（左右不平不抹平）；partial 时标注缺失字段。
 * 二期（营收按大类分流）与三期（品类利润）见后端 pl_flow.go 接口注释。
 */
export default function ProfitFlowPanel({ flow, currency, language }: { flow: RetailPlFlowResponse | null; currency?: string; language: Language }) {
  if (!flow || flow.status === "unavailable") {
    return (
      <Card className="store-360-pl-flow" title={t("store360.pl_flow.title", language)} size="small">
        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={flow?.reason ? t("store360.pl_flow.unavailable", language) : t("store360.pl_flow.pick_store", language)} />
      </Card>
    );
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
    <Card className="store-360-pl-flow" title={t("store360.pl_flow.title", language)} size="small" extra={<Typography.Text type="secondary" className="pl-flow-basis">{flow.basis}</Typography.Text>}>
      <Sankey
        data={data}
        nodePadding={24}
        margin={{ top: 8, right: 24, bottom: 8, left: 24 }}
        height={260}
      />
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
    </Card>
  );
}
