"use client";

import { Button, Card, Col, DatePicker, Input, Row, Select, Space } from "antd";
import { ClearOutlined, DownloadOutlined, RobotOutlined, SearchOutlined, TagOutlined } from "@ant-design/icons";
import dayjs from "dayjs";
import { t, type Language } from "../../lib/i18n";

const { RangePicker } = DatePicker;

interface AmortizationControlsProps {
  reportMode: "working" | "official";
  amortView: "contract" | "store" | "tag" | "summary";
  amortGranularity: "day" | "month" | "quarter" | "half_year" | "year";
  amortDateRange: [dayjs.Dayjs, dayjs.Dayjs] | null;
  amortContractId: string;
  amortStore: string;
  selectedTags: string[];
  tagLoading: boolean;
  availableTags: string[];
  discountRateOverride: string;
  reportCurrency: string;
  exchangeRate: string;
  showFilters: boolean;
  amortLoading: boolean;
  amortDataLength: number;
  language: Language;
  onViewChange: (value: "contract" | "store" | "tag" | "summary") => void;
  onGranularityChange: (value: "day" | "month" | "quarter" | "half_year" | "year") => void;
  onDateRangeChange: (value: [dayjs.Dayjs, dayjs.Dayjs] | null) => void;
  onSearch: () => void;
  onReset: () => void;
  onExportCsv: () => void;
  onExportExcel: () => void;
  onAiAnalysis: () => void;
  onToggleFilters: () => void;
  onContractIdChange: (value: string) => void;
  onStoreChange: (value: string) => void;
  onSelectedTagsChange: (value: string[]) => void;
  onDiscountRateOverrideChange: (value: string) => void;
  onReportCurrencyChange: (value: string) => void;
  onExchangeRateChange: (value: string) => void;
}

export function AmortizationControls({
  reportMode,
  amortView,
  amortGranularity,
  amortDateRange,
  amortContractId,
  amortStore,
  selectedTags,
  tagLoading,
  availableTags,
  discountRateOverride,
  reportCurrency,
  exchangeRate,
  showFilters,
  amortLoading,
  amortDataLength,
  language,
  onViewChange,
  onGranularityChange,
  onDateRangeChange,
  onSearch,
  onReset,
  onExportCsv,
  onExportExcel,
  onAiAnalysis,
  onToggleFilters,
  onContractIdChange,
  onStoreChange,
  onSelectedTagsChange,
  onDiscountRateOverrideChange,
  onReportCurrencyChange,
  onExchangeRateChange,
}: AmortizationControlsProps) {
  return (
    <Card style={{ marginBottom: 16 }}>
      <Row gutter={[12, 10]} align="middle" style={{ marginBottom: showFilters ? 8 : 0 }}>
        <Col>
          <Space size={4}>
            <span style={{ fontSize: 13, color: "#595959" }}>
              {t("reports.view_dimension", language)}
            </span>
            <Select
              value={amortView}
              onChange={onViewChange}
              style={{ width: 110 }}
              size="small"
              options={[
                { value: "contract", label: t("reports.contract_view", language) },
                { value: "store", label: t("reports.store_view", language) },
                { value: "tag", label: t("reports.tag_view", language) },
                { value: "summary", label: t("reports.summary_view", language) },
              ]}
            />
          </Space>
        </Col>
        <Col>
          <Space size={4}>
            <span style={{ fontSize: 13, color: "#595959" }}>
              {t("reports.granularity", language)}
            </span>
            <Select
              value={amortGranularity}
              onChange={onGranularityChange}
              style={{ width: 90 }}
              size="small"
              options={[
                { value: "day", label: t("reports.day", language) },
                { value: "month", label: t("reports.month", language) },
                { value: "quarter", label: t("reports.quarter", language) },
                { value: "half_year", label: t("reports.half_year", language) },
                { value: "year", label: t("reports.year", language) },
              ]}
            />
          </Space>
        </Col>
        <Col>
          <Space size={4}>
            <span style={{ fontSize: 13, color: "#595959" }}>
              {t("reports.date_range", language)}
            </span>
            <RangePicker
              value={amortDateRange}
              onChange={(dates) => onDateRangeChange(dates as [dayjs.Dayjs, dayjs.Dayjs] | null)}
              allowClear={false}
              size="small"
              style={{ width: 220 }}
            />
          </Space>
        </Col>
        <Col>
          <Space size={6}>
            <Button
              type="primary"
              size="small"
              icon={<SearchOutlined />}
              onClick={onSearch}
              loading={amortLoading}
            >
              {t("reports.search", language)}
            </Button>
            <Button
              size="small"
              icon={<ClearOutlined />}
              onClick={onReset}
              disabled={amortLoading}
            >
              {t("reports.reset", language)}
            </Button>
          </Space>
        </Col>

        <Col flex="auto" />
        <Col>
          <Space size={6}>
            <Button
              size="small"
              icon={<DownloadOutlined />}
              onClick={onExportCsv}
              disabled={!amortDataLength}
            >
              CSV
            </Button>
            <Button
              size="small"
              icon={<DownloadOutlined />}
              onClick={onExportExcel}
              disabled={!amortDataLength}
            >
              Excel
            </Button>
            <Button
              type="primary"
              size="small"
              icon={<RobotOutlined />}
              onClick={onAiAnalysis}
            >
              {t("reports.ai_analysis", language)}
            </Button>
          </Space>
        </Col>
      </Row>

      <Button
        type="link"
        onClick={onToggleFilters}
        style={{ padding: 0, fontSize: 12 }}
      >
        {showFilters
          ? t("reports.collapse_filters", language)
          : t("reports.expand_filters", language)}
      </Button>

      {showFilters && (
        <>
          <Row gutter={[12, 10]} style={{ marginTop: 8 }}>
            {amortView !== "contract" && (
              <Col xs={24} sm={8}>
                <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
                  <span style={{ fontSize: 12, color: "#8C8C8C" }}>
                    {t("reports.contract_id", language)}
                  </span>
                  <Input
                    size="small"
                    value={amortContractId}
                    onChange={(e) => onContractIdChange(e.target.value)}
                    placeholder={t("reports.filter_contract_id", language)}
                    allowClear
                  />
                </div>
              </Col>
            )}
            {amortView !== "store" && (
              <Col xs={24} sm={8}>
                <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
                  <span style={{ fontSize: 12, color: "#8C8C8C" }}>
                    {t("reports.store", language)}
                  </span>
                  <Input
                    size="small"
                    value={amortStore}
                    onChange={(e) => onStoreChange(e.target.value)}
                    placeholder={t("reports.filter_store", language)}
                    allowClear
                  />
                </div>
              </Col>
            )}
            <Col xs={24} sm={8}>
              <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
                <span style={{ fontSize: 12, color: "#8C8C8C" }}>
                  {t("reports.tags", language)}
                </span>
                <Select
                  mode="tags"
                  size="small"
                  value={selectedTags}
                  onChange={onSelectedTagsChange}
                  style={{ width: "100%" }}
                  placeholder={t("reports.filter_tags", language)}
                  loading={tagLoading}
                  options={availableTags.map((tag) => ({ value: tag, label: tag }))}
                />
              </div>
            </Col>
          </Row>

          <div
            style={{
              marginTop: 12,
              padding: "10px 14px",
              borderRadius: 8,
              background: "#F7F7F7",
              border: "1px solid #E5E5E5",
              fontSize: 12,
              color: "#8C8C8C",
            }}
          >
            <span style={{ fontWeight: 600, color: "#595959" }}>
              {t("reports.override_title", language)}
            </span>
            <span style={{ marginLeft: 8 }}>
              {t("reports.override_desc", language)}
            </span>
          </div>
          <Row gutter={[12, 10]} style={{ marginTop: 10 }}>
            <Col xs={24} sm={8}>
              <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
                <span style={{ fontSize: 12, color: "#8C8C8C" }}>
                  {t("reports.discount_rate_override", language)}
                </span>
                <Input
                  size="small"
                  value={discountRateOverride}
                  onChange={(e) => onDiscountRateOverrideChange(e.target.value)}
                  placeholder={t("reports.override_placeholder", language)}
                  allowClear
                />
              </div>
            </Col>
            <Col xs={24} sm={8}>
              <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
                <span style={{ fontSize: 12, color: "#8C8C8C" }}>
                  {t("reports.report_currency", language)}
                </span>
                <Select
                  size="small"
                  value={reportCurrency || undefined}
                  onChange={(value) => onReportCurrencyChange(value || "")}
                  style={{ width: "100%" }}
                  placeholder={t("reports.select_currency", language)}
                  allowClear
                  options={[
                    { value: "CNY", label: t("reports.option.currency_cny", language) },
                    { value: "USD", label: t("reports.option.currency_usd", language) },
                    { value: "HKD", label: t("reports.option.currency_hkd", language) },
                    { value: "EUR", label: t("reports.option.currency_eur", language) },
                  ]}
                />
              </div>
            </Col>
            <Col xs={24} sm={8}>
              <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
                <span style={{ fontSize: 12, color: "#8C8C8C" }}>
                  {t("reports.exchange_rate", language)}
                </span>
                <Input
                  size="small"
                  value={exchangeRate}
                  onChange={(e) => onExchangeRateChange(e.target.value)}
                  placeholder={t("reports.exchange_rate_placeholder", language)}
                  allowClear
                />
              </div>
            </Col>
          </Row>
        </>
      )}

      {(discountRateOverride || reportCurrency) && (
        <div style={{ marginTop: 16 }}>
          {discountRateOverride && (
            <span
              style={{
                display: "inline-block",
                marginRight: 8,
              }}
            >
              <span
                style={{
                  fontSize: 12,
                  padding: "2px 10px",
                  border: "1px solid #D9D9D9",
                  borderRadius: 6,
                }}
              >
                {t("reports.discount_rate_override", language)}: {Number(discountRateOverride).toFixed(2)}%
              </span>
            </span>
          )}
          {reportCurrency && (
            <span
              style={{
                display: "inline-block",
              }}
            >
              <span
                style={{
                  fontSize: 12,
                  padding: "2px 10px",
                  border: "1px solid #D9D9D9",
                  borderRadius: 6,
                }}
              >
                {t("reports.report_currency", language)}: {reportCurrency}
                {exchangeRate ? ` @ ${Number(exchangeRate).toFixed(2)}` : ""}
              </span>
            </span>
          )}
        </div>
      )}
    </Card>
  );
}
