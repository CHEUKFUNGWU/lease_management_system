"use client";

// R1-2：销售人效与工时面板。四项指标（销售人效、单均工时、人工成本率、
// 期末在岗人数）全部取自门店 360 诊断响应的 summary——R1-1 在后端露出的
// 那份，本组件不做任何计算；缺失渲染「—」，hover 给原因。
//
// 同群行：样本不足显式降级为「同群样本不足」（retail-kpi-v1 纪律），
// 不空白、不填 0。
//
// 底部 grain_note 是硬要求：告诉使用者为什么时段排班分析不在这儿
// （事实层是 store-day，没有小时数据）、以及需要什么才能有。
// 它不是免责声明，不随数据状态消失。

import React from "react";
import { Card, Space, Tag, Tooltip, Typography } from "antd";
import { InfoCircleOutlined, TeamOutlined } from "@ant-design/icons";
import { useLanguage } from "../context/LanguageContext";
import { t } from "../lib/i18n";
import type { RetailPeerBenchmark, RetailStore360SummaryMetric } from "../lib/api";
import { formatKPIValue, formatUnitValue, translateReason } from "../operating-pulse/logic";
import { StateBlock } from "../components/StateBlock";

const { Text } = Typography;

interface Props {
  summary?: Record<string, RetailStore360SummaryMetric>;
  benchmarks?: RetailPeerBenchmark[];
  currency?: string;
  dataClassification?: string;
}

/** 指标码 → 标签键。值文案用 R1-2 定稿术语；格式化走既有 formatter。 */
const METRIC_LABEL_KEYS: Record<string, string> = {
  sales_per_labor_hour: "store360.labor.metric.sph",
  labor_hours_per_transaction: "store360.labor.metric.hpt",
  labor_cost_rate: "store360.labor.metric.rate",
  headcount: "store360.labor.metric.hc",
};

const METRIC_BASIS_KEYS: Record<string, string> = {
  sales_per_labor_hour: "store360.labor.sph_basis",
  labor_hours_per_transaction: "store360.labor.hpt_basis",
  labor_cost_rate: "store360.labor.rate_basis",
  headcount: "store360.labor.hc_basis",
};

export function LaborEfficiencyPanel({ summary, benchmarks, currency, dataClassification }: Props) {
  const { language } = useLanguage();
  const codes = Object.keys(METRIC_LABEL_KEYS);
  const benchmarkByCode = new Map((benchmarks ?? []).map((b) => [b.code, b]));

  return (
    <Card
      size="small"
      title={
        <Space>
          <TeamOutlined />
          <span>{t("store360.labor.panel_title", language)}</span>
          {dataClassification === "simulated" && (
            <Tag>{t("trust.classification_simulated", language)}</Tag>
          )}
        </Space>
      }
    >
      {!summary ? (
        <StateBlock state={{ kind: "empty", reason: t("store360.no_observations", language) }} language={language} />
      ) : (
        <div className="labor-grid">
          {codes.map((code) => {
            const metric = summary[code];
            const missing = !metric || metric.current.value == null;
            return (
              <div key={code} className={`labor-metric-${code} pulse-kpi-card labor-kpi-card`}>
                <Space size={4}>
                  <span className="labor-metric-label">{t(METRIC_LABEL_KEYS[code], language)}</span>
                  <Tooltip title={t(METRIC_BASIS_KEYS[code], language)}>
                    <InfoCircleOutlined className="labor-basis-hint" />
                  </Tooltip>
                </Space>
                <div className="labor-metric-value-wrap">
                  {/* Hover reason: backend degradation reason via runtime Tooltip */}
                  <Tooltip title={missing ? translateReason(metric?.current.reason || undefined, language) : ""}>
                    <Text className={`labor-metric-num font-tabular${missing ? " is-missing" : ""}`}>
                      {formatKPIValue(metric?.current ?? null, currency, language)}
                    </Text>
                  </Tooltip>
                </div>
                {/* Peer status: explicit degradation when sample is insufficient - never blank, never zero */}
                {benchmarkByCode.has(code) && (() => {
                  const peer = benchmarkByCode.get(code)!;
                  if (peer.status === "complete" && peer.median != null) {
                    return (
                      <Text type="secondary" className={`labor-peer-${code} labor-peer-line`}>
                        {t("store360.labor.peer_median", language, { value: formatUnitValue(peer.median, peer.unit, currency, language), count: String(peer.peer_count) })}
                      </Text>
                    );
                  }
                  return (
                    <Tooltip title={translateReason(peer.reason || undefined, language)}>
                      <Text type="warning" className={`labor-peer-${code} labor-peer-line labor-peer-insufficient`}>
                        {t("store360.labor.peer_insufficient", language)}
                      </Text>
                    </Tooltip>
                  );
                })()}
              </div>
            );
          })}
        </div>
      )}
      {/* Grain note is persistent: rendered in every data state */}
      <div className="labor-grain-note">{t("store360.labor.grain_note", language)}</div>
    </Card>
  );
}
