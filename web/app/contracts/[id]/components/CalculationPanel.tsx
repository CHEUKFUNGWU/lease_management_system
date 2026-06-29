"use client";

import { CalculatorOutlined } from "@ant-design/icons";
import { Alert, Card, Col, Row, Statistic, Table } from "antd";
import type { TableProps } from "antd";
import {
  getLeaseScopeLabel,
} from "../../../lib/constants/contracts";
import { t, type Language } from "../../../lib/i18n";
import type { CalculationResult, MonthlyEntry } from "../../../lib/types/contracts";

export function CalculationPanel({
  calcResult,
  calcColumns,
  sortedMonthly,
  language,
}: {
  calcResult: CalculationResult | null;
  calcColumns: TableProps<MonthlyEntry>["columns"];
  sortedMonthly: MonthlyEntry[];
  language: Language;
}) {
  if (!calcResult) {
    return (
      <Card>
        <div style={{ textAlign: "center", padding: 40 }}>
          <CalculatorOutlined style={{ fontSize: 48, color: "#BFBFBF" }} />
          <p style={{ marginTop: 16, color: "#8C8C8C" }}>
            {t("contract.click_calculate", language)}
          </p>
        </div>
      </Card>
    );
  }

  return (
    <>
      <Alert
        message={`计量路径：${calcResult.measurement_basis === "capitalized" ? "资本化计量" : calcResult.measurement_basis === "straight_line_expense" ? "豁免租赁直线法费用化" : "不进入 IFRS 16 计量"}`}
        description={`范围判定：${getLeaseScopeLabel(calcResult.lease_scope || "in_scope") || calcResult.lease_scope}`}
        type={calcResult.measurement_basis === "capitalized" ? "info" : "warning"}
        showIcon
        style={{ marginBottom: 16 }}
      />
      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col span={8}>
          <Card>
            <Statistic
              title={t("contract.initial_liability", language)}
              value={calcResult.initial_liability}
              precision={2}
              prefix="¥"
            />
          </Card>
        </Col>
        <Col span={8}>
          <Card>
            <Statistic
              title={t("contract.initial_rou", language)}
              value={calcResult.initial_rou_asset}
              precision={2}
              prefix="¥"
            />
          </Card>
        </Col>
        <Col span={8}>
          <Card>
            <Statistic
              title={t("contract.total_days", language)}
              value={calcResult.total_days}
              suffix={t("contract_detail.days_unit", language)}
            />
          </Card>
        </Col>
      </Row>
      <Card title={t("contract.monthly_amortization", language)}>
        <Table
          columns={calcColumns}
          dataSource={sortedMonthly}
          rowKey={(row: MonthlyEntry) => `${row.Year}-${row.Month}`}
          pagination={{ pageSize: 12 }}
          size="small"
          scroll={{ x: 1000 }}
        />
      </Card>
    </>
  );
}
