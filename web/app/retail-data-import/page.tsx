"use client";

import { useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { Alert, Button, Card, DatePicker, Descriptions, Flex, Input, Row, Col, Space, Spin, Steps, Table, Tag, Tooltip, Typography, Upload, message } from "antd";
import { CheckCircleOutlined, InfoCircleOutlined, ArrowRightOutlined } from "@ant-design/icons";
import dayjs from "dayjs";
import AppLayout from "../components/AppLayout";
import PageHeader from "../components/PageHeader";
import ProtectedRoute from "../components/ProtectedRoute";
import { StateBlock } from "../components/StateBlock";
import { StatusTag } from "../components/StatusTag";
import { UploadGlyph, DownloadGlyph, SourceCircleGlyph } from "../components/MonochromeGlyphs";
import { apiErrorMessage, fpnaPlanImportApi, operatingFactsApi, retailIngestApi, trialBalanceApi, type RetailIngestPreviewResponse, type RetailIngestCommitResponse } from "../lib/api";
import { tableScrollX } from "../lib/tableScroll";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { t, type Language } from "../lib/i18n";

const REQUIRED_FIELDS = ["store", "business_date", "currency", "revenue"];

function FieldHint({ text }: { text: string }) {
  return (
    <Tooltip title={text}>
      <InfoCircleOutlined className="retail-import-field-hint" style={{ color: "var(--text-secondary, #8C8C8C)", cursor: "pointer" }} />
    </Tooltip>
  );
}

function fieldLabel(field: string, language: Language): string {
  const label = t(`retail_import.field_label.${field}`, language);
  return label || field;
}

export default function RetailDataImportPage() {
  const router = useRouter();
  const { token } = useAuth();
  const { language } = useLanguage();
  const [currentStep, setCurrentStep] = useState(0);
  const [file, setFile] = useState<File | null>(null);
  const [sourceSystem, setSourceSystem] = useState("pos");
  const [asOf, setAsOf] = useState(dayjs().format("YYYY-MM-DD"));
  const [preview, setPreview] = useState<RetailIngestPreviewResponse | null>(null);
  const [mapping, setMapping] = useState<Record<string, string>>({});
  const [previewing, setPreviewing] = useState(false);
  const [previewError, setPreviewError] = useState<string | null>(null);
  const [committing, setCommitting] = useState(false);
  const [commitResult, setCommitResult] = useState<RetailIngestCommitResponse | null>(null);
  const [templateDownloading, setTemplateDownloading] = useState(false);
  const commitKeyRef = useRef<string>("");

  const downloadStoreTemplate = async () => {
    if (!token) return;
    setTemplateDownloading(true);
    try {
      const blob = await operatingFactsApi.downloadStoreCSVTemplate(token);
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement("a");
      anchor.href = url;
      anchor.download = "store_facts_template.csv";
      anchor.click();
      URL.revokeObjectURL(url);
    } catch (error) {
      message.error(apiErrorMessage(error));
    } finally {
      setTemplateDownloading(false);
    }
  };

  const runPreview = async (nextFile: File, nextMapping: Record<string, string> | null, nextSource?: string) => {
    if (!token) return;
    setPreviewing(true);
    setPreviewError(null);
    setCommitResult(null);
    try {
      const result = await retailIngestApi.preview(nextFile, nextSource ?? sourceSystem, nextMapping, token);
      setPreview(result);
      setMapping(result.mapping);
      setCurrentStep(1);
    } catch (err) {
      setPreview(null);
      setPreviewError(apiErrorMessage(err));
    } finally {
      setPreviewing(false);
    }
  };

  const onFileSelected = async (selected: File) => {
    setFile(selected);
    commitKeyRef.current = "";
    await runPreview(selected, null);
  };

  const headerForField = (field: string): string => {
    for (const [header, target] of Object.entries(mapping)) {
      if (target === field) return header;
    }
    return "";
  };

  const changeMapping = (field: string, header: string) => {
    const next: Record<string, string> = {};
    for (const [existingHeader, target] of Object.entries(mapping)) {
      if (target !== field && existingHeader !== header) next[existingHeader] = target;
    }
    if (header) next[header] = field;
    setMapping(next);
  };

  // FP&A Plan
  const [planFile, setPlanFile] = useState<File | null>(null);
  const [planName, setPlanName] = useState("");
  const [planType, setPlanType] = useState("budget");
  const [planSource, setPlanSource] = useState("excel-import");
  const [planFrom, setPlanFrom] = useState(dayjs().subtract(6, "month").format("YYYY-MM"));
  const [planTo, setPlanTo] = useState(dayjs().add(6, "month").format("YYYY-MM"));
  const [planAsOf, setPlanAsOf] = useState(dayjs().subtract(1, "month").format("YYYY-MM"));
  const [planOfficial, setPlanOfficial] = useState(false);
  const [planImporting, setPlanImporting] = useState(false);
  const [planResult, setPlanResult] = useState<string | null>(null);

  const importPlanVersion = async () => {
    if (!token || !planFile || !planName.trim()) return;
    setPlanImporting(true);
    try {
      const result = await fpnaPlanImportApi.importPlanVersion(planFile, {
        name: planName.trim(), version_type: planType, source: planSource.trim(),
        as_of_period: planAsOf, from_period: planFrom, to_period: planTo, is_official: planOfficial,
      }, token);
      setPlanResult(result.idempotent_replay
        ? t("plan_import.replay", language)
        : `${t("plan_import.done", language)} · ${result.version.name} · ${result.accepted_rows}/${result.accepted_rows + result.rejected_rows}${result.rejected_rows > 0 ? ` · ${result.rejected_rows} 行失败` : ""}`);
      message.success(result.idempotent_replay ? t("plan_import.replay", language) : t("plan_import.done", language));
    } catch (err) {
      message.error(apiErrorMessage(err));
    } finally {
      setPlanImporting(false);
    }
  };

  // GL Trial Balance
  const [tbFile, setTbFile] = useState<File | null>(null);
  const [tbName, setTbName] = useState("");
  const [tbSource, setTbSource] = useState("gl-export");
  const [tbPeriod, setTbPeriod] = useState(dayjs().subtract(1, "month").format("YYYY-MM"));
  const [tbCurrency, setTbCurrency] = useState("CNY");
  const [tbImporting, setTbImporting] = useState(false);
  const [tbResult, setTbResult] = useState<string | null>(null);

  const importTrialBalance = async () => {
    if (!token || !tbFile || !tbName.trim()) return;
    setTbImporting(true);
    try {
      const result = await trialBalanceApi.importTB(tbFile, {
        name: tbName.trim(), source_system: tbSource.trim(), period: tbPeriod, functional_currency: tbCurrency.toUpperCase(),
      }, token);
      const balanced = result.balanced ? t("tb_import.balanced", language) : t("tb_import.unbalanced", language);
      setTbResult(result.idempotent_replay ? t("tb_import.replay", language) : `${t("tb_import.done", language)} · ${result.accepted_rows} 行 · ${balanced}`);
      message.success(result.idempotent_replay ? t("tb_import.replay", language) : t("tb_import.done", language));
    } catch (err) {
      message.error(apiErrorMessage(err));
    } finally {
      setTbImporting(false);
    }
  };

  const commit = async () => {
    if (!token || !file) return;
    if (!commitKeyRef.current) {
      commitKeyRef.current = typeof crypto !== "undefined" && "randomUUID" in crypto
        ? crypto.randomUUID()
        : `retail-import-${Date.now()}-${Math.random().toString(16).slice(2)}`;
    }
    setCommitting(true);
    try {
      const result = await retailIngestApi.commit(file, sourceSystem, asOf, mapping, commitKeyRef.current, token);
      setCommitResult(result);
      setCurrentStep(2);
      message.success(t("retail_import.commit_success", language));
    } catch (err) {
      message.error(apiErrorMessage(err));
    } finally {
      setCommitting(false);
    }
  };

  const report = preview?.report;
  const missingRequired = (report?.missing_fields || []).filter((field) => REQUIRED_FIELDS.includes(field));
  const commitBlocked = !preview || !report || report.valid_rows === 0 || missingRequired.length > 0 || (report.ambiguous_mappings || []).length > 0;
  const rowErrors = report?.errors || [];

  return (
    <ProtectedRoute>
      <AppLayout>
        <div className="retail-import-page">
          <PageHeader
            title={t("retail_import.title", language)}
            meta={t("retail_import.scope_note", language)}
            primaryAction={
              <Button
                type="primary"
                icon={<UploadGlyph size={13} />}
                loading={committing}
                disabled={commitBlocked || previewing}
                onClick={commit}
              >
                {committing ? t("retail_import.committing", language) : t("retail_import.commit", language)}
              </Button>
            }
            secondaryAction={
              <Button disabled={!file || previewing} onClick={() => file && runPreview(file, mapping)}>
                {t("retail_import.revalidate", language)}
              </Button>
            }
          />

          {/* 3步导入向导导航栏 */}
          <Card size="small" style={{ marginBottom: 16 }}>
            <Steps
              current={currentStep}
              size="small"
              items={[
                { title: "下载模板与准备数据", description: "获取标准格式" },
                { title: "文件上传与智能映射", description: "校验必填项与字段" },
                { title: "预审报告与安全入库", description: "幂等入账与数据治理" },
              ]}
            />
          </Card>

          {/* 第一步：门店日粒度经营事实标准上载 */}
          <Card
            size="small"
            title={<Space><SourceCircleGlyph size={14} /><span>门店日粒度经营事实导入 (Store-Day Facts)</span></Space>}
            className="retail-import-block-gap"
            extra={
              <Button
                icon={<DownloadGlyph size={13} />}
                loading={templateDownloading}
                onClick={() => { void downloadStoreTemplate(); }}
              >
                {t("import.download_template", language)}
              </Button>
            }
          >
            <div style={{ display: "flex", gap: 16, flexWrap: "wrap", alignItems: "center", marginBottom: 12 }}>
              <div className="precision-filter-group">
                <span className="precision-filter-label">{t("retail_import.source_system", language)}:</span>
                <Input
                  size="small"
                  className="retail-import-source-input"
                  style={{ width: 140 }}
                  value={sourceSystem}
                  onChange={(event) => setSourceSystem(event.target.value.trim())}
                  onPressEnter={() => file && runPreview(file, null, sourceSystem || "pos")}
                />
              </div>
              <div className="precision-filter-group">
                <span className="precision-filter-label">{t("retail_import.as_of", language)}:</span>
                <DatePicker
                  size="small"
                  allowClear={false}
                  value={dayjs(asOf)}
                  onChange={(value) => value && setAsOf(value.format("YYYY-MM-DD"))}
                />
              </div>
            </div>

            {/* 拖拽上传区域 */}
            <Upload.Dragger
              accept=".csv,.xlsx"
              maxCount={1}
              showUploadList={false}
              beforeUpload={(selected) => { void onFileSelected(selected); return false; }}
              style={{ padding: "16px 0", background: "var(--bg-subtle, #FAFAFA)", border: "1px dashed var(--border-default, #D9D9D9)", borderRadius: 6 }}
            >
              <p className="ant-upload-drag-icon" style={{ marginBottom: 8 }}>
                <UploadGlyph size={28} />
              </p>
              <p className="ant-upload-text" style={{ fontSize: 14, fontWeight: 500, margin: "0 0 4px" }}>
                {file ? file.name : "点击或将门店经营事实文件 (.csv / .xlsx) 拖拽至此处"}
              </p>
              <p className="ant-upload-hint" style={{ fontSize: 12, color: "var(--text-secondary, #8C8C8C)", margin: 0 }}>
                {t("retail_import.upload_hint", language)}
              </p>
            </Upload.Dragger>
          </Card>

          {previewError && <Alert className="retail-import-block-gap" type="error" showIcon message={t("retail_import.report_title", language)} description={previewError} />}
          {previewing && <Card className="retail-import-block-gap"><Flex justify="center"><Spin tip={t("retail_import.previewing", language)} /></Flex></Card>}

          {!preview && !previewing && !previewError && (
            <div className="retail-import-block-gap">
              <StateBlock state={{ kind: "empty", message: t("retail_import.empty_title", language), reason: t("retail_import.empty_desc", language) }} language={language} />
            </div>
          )}

          {commitResult && (
            <Alert
              className="retail-import-block-gap"
              type="success"
              showIcon
              message={<Space><CheckCircleOutlined />{t("retail_import.commit_success", language)}</Space>}
              description={
                <Space direction="vertical">
                  <span>{commitResult.report.accepted_rows} / {commitResult.report.total_rows} · 批次编号: {commitResult.report.batch_id.slice(0, 8)}… · {commitResult.report.new_store_days} 条新增 / {commitResult.report.superseded_store_days} 条覆盖更新{commitResult.idempotent_replay ? " · 拦截重复提交 (幂等安全)" : ""}</span>
                  <Button type="link" onClick={() => router.push("/operating-pulse?data_classification=production")}>
                    {t("retail_import.go_pulse", language)} <ArrowRightOutlined />
                  </Button>
                </Space>
              }
            />
          )}

          {preview && report && (
            <>
              {missingRequired.length > 0 && <Alert className="retail-import-block-gap" type="warning" showIcon message={t("retail_import.missing_fields", language) + missingRequired.map((field) => fieldLabel(field, language)).join("、")} />}
              {(report.ambiguous_mappings || []).length > 0 && <Alert className="retail-import-block-gap" type="warning" showIcon message={t("retail_import.ambiguous", language) + (report.ambiguous_mappings || []).join("; ")} />}
              {(report.unmatched_stores || []).length > 0 && <Alert className="retail-import-block-gap" type="warning" showIcon message={t("retail_import.unmatched_stores", language)} description={<Space wrap>{(report.unmatched_stores || []).map((store) => <Tag key={store}>{store}</Tag>)}</Space>} />}

              <Card
                title={
                  <Space>
                    <span>{t("retail_import.mapping_title", language)}</span>
                    <StatusTag kind={preview.suggested_mapping_source === "ai" ? "processing" : "neutral"}>
                      {preview.suggested_mapping_source === "ai" ? t("retail_import.mapping_source_ai", language) : t("retail_import.mapping_source_rule", language)}
                    </StatusTag>
                  </Space>
                }
                className="retail-import-block-gap"
                size="small"
              >
                <Table
                  size="small"
                  pagination={false}
                  rowKey="field"
                  dataSource={preview.standard_fields.map((field) => ({ field, required: REQUIRED_FIELDS.includes(field) }))}
                  scroll={tableScrollX(preview.standard_fields.length, 640)}
                  columns={[
                    {
                      title: t("retail_import.field", language),
                      render: (_: unknown, row: { field: string; required: boolean }) => (
                        <Space>
                          {row.required ? <StatusTag kind="error">{t("retail_import.required", language)}</StatusTag> : null}
                          {fieldLabel(row.field, language)}
                        </Space>
                      ),
                    },
                    {
                      title: t("retail_import.file_column", language),
                      render: (_: unknown, row: { field: string }) => (
                        <select
                          className="retail-import-mapping-select"
                          value={headerForField(row.field)}
                          onChange={(event) => changeMapping(row.field, event.target.value)}
                        >
                          <option value="">{t("retail_import.unmapped", language)}</option>
                          {preview.headers.map((header) => <option key={header} value={header}>{header}</option>)}
                        </select>
                      ),
                    },
                  ]}
                />
              </Card>

              <Card title={t("retail_import.report_title", language)} className="retail-import-block-gap" size="small">
                <Descriptions size="small" column={{ xs: 1, sm: 2, lg: 4 }} className="retail-import-block-gap-sm">
                  <Descriptions.Item label={t("retail_import.total_rows", language)}>{report.total_rows}</Descriptions.Item>
                  <Descriptions.Item label={t("retail_import.valid_rows", language)}>{report.valid_rows}</Descriptions.Item>
                  <Descriptions.Item label={t("retail_import.store_count", language)}>{report.coverage.store_count}</Descriptions.Item>
                  <Descriptions.Item label="基准期间">{report.coverage.date_from || "—"} ~ {report.coverage.date_to || "—"}</Descriptions.Item>
                  <Descriptions.Item label={t("retail_import.overlap", language)}>{report.coverage.overlap_store_days}</Descriptions.Item>
                  <Descriptions.Item label={t("retail_import.new_days", language)}>{report.coverage.new_store_days}</Descriptions.Item>
                </Descriptions>
                {rowErrors.length > 0 && (
                  <Table
                    size="small"
                    pagination={{ pageSize: 10 }}
                    rowKey={(row) => `${row.row}-${row.code}-${row.column || ""}`}
                    dataSource={rowErrors}
                    scroll={tableScrollX(rowErrors.length, 720)}
                    columns={[
                      { title: t("retail_import.error_row", language), dataIndex: "row", width: 80 },
                      { title: t("retail_import.error_code", language), dataIndex: "code", width: 180, render: (code: string) => <StatusTag kind="warning">{code}</StatusTag> },
                      { title: t("retail_import.error_message", language), dataIndex: "message" },
                    ]}
                  />
                )}
              </Card>
            </>
          )}

          {/* 第二部分：FP&A 计划版本导入 */}
          <Card size="small" className="retail-import-block-gap" title={t("plan_import.title", language)}>
            <Flex gap={12} wrap="wrap" align="center">
              <Input aria-label={t("plan_import.name", language)} className="retail-import-source-input" style={{ width: 140 }} value={planName} onChange={(event) => setPlanName(event.target.value)} placeholder={t("plan_import.name", language)} />
              <select aria-label={t("plan_import.version_type", language)} className="retail-import-type-select" value={planType} onChange={(event) => setPlanType(event.target.value)}>
                <option value="budget">预算版本 (Budget)</option>
                <option value="forecast">滚动预测 (Forecast)</option>
                <option value="scenario">测算方案 (Scenario)</option>
              </select>
              <Input aria-label={t("plan_import.source", language)} className="retail-import-source-input" style={{ width: 120 }} value={planSource} onChange={(event) => setPlanSource(event.target.value)} />
              <span>{t("plan_import.from_period", language)}</span>
              <DatePicker picker="month" allowClear={false} value={dayjs(planFrom)} onChange={(value) => value && setPlanFrom(value.format("YYYY-MM"))} />
              <span>{t("plan_import.to_period", language)}</span>
              <DatePicker picker="month" allowClear={false} value={dayjs(planTo)} onChange={(value) => value && setPlanTo(value.format("YYYY-MM"))} />
              <span>{t("plan_import.as_of_period", language)}</span>
              <DatePicker picker="month" allowClear={false} value={dayjs(planAsOf)} onChange={(value) => value && setPlanAsOf(value.format("YYYY-MM"))} />
              <Upload accept=".csv,.xlsx" maxCount={1} showUploadList={false} beforeUpload={(selected) => { setPlanFile(selected); return false; }}>
                <Button icon={<UploadGlyph size={13} />}>{planFile ? planFile.name : t("plan_import.title", language)}</Button>
              </Upload>
              <span><input type="checkbox" checked={planOfficial} onChange={(event) => setPlanOfficial(event.target.checked)} /> {t("plan_import.official", language)}</span>
              <Button type="primary" loading={planImporting} disabled={!planFile || !planName.trim()} onClick={() => { void importPlanVersion(); }}>{t("plan_import.commit", language)}</Button>
            </Flex>
            <Typography.Text type="secondary" className="retail-import-hint">{t("plan_import.hint_plain", language)} <FieldHint text={t("plan_import.hint", language)} /></Typography.Text>
            {planResult && <Alert className="retail-import-block-gap-sm" type="success" showIcon message={planResult} />}
          </Card>

          {/* 第三部分：总账试算平衡表导入 */}
          <Card size="small" className="retail-import-block-gap" title={t("tb_import.title", language)}>
            <Flex gap={12} wrap="wrap" align="center">
              <Input aria-label={t("plan_import.name", language)} className="retail-import-source-input" style={{ width: 140 }} value={tbName} onChange={(event) => setTbName(event.target.value)} placeholder={t("plan_import.name", language)} />
              <Input aria-label={t("tb_import.source_system", language)} className="retail-import-source-input" style={{ width: 120 }} value={tbSource} onChange={(event) => setTbSource(event.target.value)} />
              <span>{t("tb_import.period", language)}</span>
              <DatePicker picker="month" allowClear={false} value={dayjs(tbPeriod)} onChange={(value) => value && setTbPeriod(value.format("YYYY-MM"))} />
              <span>{t("tb_import.currency", language)}</span>
              <Input aria-label={t("tb_import.currency", language)} className="retail-import-source-input" style={{ width: 80 }} value={tbCurrency} onChange={(event) => setTbCurrency(event.target.value.toUpperCase())} maxLength={3} />
              <Upload accept=".csv" maxCount={1} showUploadList={false} beforeUpload={(selected) => { setTbFile(selected); return false; }}>
                <Button icon={<UploadGlyph size={13} />}>{tbFile ? tbFile.name : t("tb_import.title", language)}</Button>
              </Upload>
              <Button type="primary" loading={tbImporting} disabled={!tbFile || !tbName.trim()} onClick={() => { void importTrialBalance(); }}>{t("tb_import.commit", language)}</Button>
            </Flex>
            <Typography.Text type="secondary" className="retail-import-hint">{t("tb_import.hint_plain", language)} <FieldHint text={t("tb_import.hint", language)} /></Typography.Text>
            {tbResult && <Alert className="retail-import-block-gap-sm" type="success" showIcon message={tbResult} />}
          </Card>
        </div>
      </AppLayout>
    </ProtectedRoute>
  );
}
