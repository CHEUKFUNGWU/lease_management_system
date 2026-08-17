"use client";

import { StatusTag, statusKindFromAntColor } from "../components/StatusTag";

import { useState, useEffect, useMemo } from "react";
import {
  Alert, Card, Typography, Table, Spin, Statistic, Row, Col, Input, InputNumber,
  Button, Space, Tag, Modal, Empty, message, Select,
} from "antd";
import {
  SearchOutlined, CopyOutlined, EyeOutlined, BarChartOutlined,
  TagOutlined, SettingOutlined, FileTextOutlined, ReloadOutlined, LogoutOutlined,
} from "@ant-design/icons";
import AppLayout from "../components/AppLayout";
import PageHeader from "../components/PageHeader";
import ProtectedRoute from "../components/ProtectedRoute";
import { StateBlock } from "../components/StateBlock";
import { ApiError, authApi, exchangeRateApi, reportApi, settingsApi } from "../lib/api";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { t } from "../lib/i18n";
import { useRouter } from "next/navigation";
import { classifyDataState } from "../lib/dataState";
import { notifyError } from "../lib/notify";

const { Title, Text, Paragraph } = Typography;

interface ContractRef {
  contract_id: string;
  contract_number: string;
  contract_name?: string;
}

interface TagSummaryRow {
  tag: string;
  contract_count: number;
  contracts?: ContractRef[];
}

interface AuthSession {
  id: string;
  created_at: string;
  expires_at: string;
  revoked_at?: string | null;
  ip_address?: string;
  user_agent?: string;
  active: boolean;
}

export default function SettingsPage() {
  const { token } = useAuth();
  const { language } = useLanguage();
  const router = useRouter();

  const [loading, setLoading] = useState(false);
  const [data, setData] = useState<TagSummaryRow[]>([]);

  /* ---- device sessions ---- */
  const [authSessions, setAuthSessions] = useState<AuthSession[]>([]);
  const [authSessionsLoading, setAuthSessionsLoading] = useState(false);
  const [revokingSessionId, setRevokingSessionId] = useState<string | null>(null);
  const [loggingOutAll, setLoggingOutAll] = useState(false);

  /* ---- global discount rate ---- */
  const [discountRateLoading, setDiscountRateLoading] = useState(false);
  const [discountRatePercent, setDiscountRatePercent] = useState<number | null>(null);
  const [discountRateSaving, setDiscountRateSaving] = useState(false);
  const [effectiveRate, setEffectiveRate] = useState<number | null>(null);
  const [healthyCeiling, setHealthyCeiling] = useState<number | null>(null);
  const [warningCeiling, setWarningCeiling] = useState<number | null>(null);
  const [ratioSaving, setRatioSaving] = useState(false);
  const [varianceMateriality, setVarianceMateriality] = useState<number | null>(null);
  const [tieOutTolerance, setTieOutTolerance] = useState<number | null>(null);
  const [journalMateriality, setJournalMateriality] = useState<number | null>(null);
  const [variancePolicySaving, setVariancePolicySaving] = useState(false);

  /* ---- exchange rates ---- */
  const [rates, setRates] = useState<any[]>([]);
  const [ratesLoading, setRatesLoading] = useState(false);
  const [rateForm, setRateForm] = useState({
    from_currency: "",
    to_currency: "",
    rate_date: "",
    rate_type: "closing" as "closing" | "average",
    rate: null as number | null,
  });
  const [rateSaving, setRateSaving] = useState(false);

  const loadRates = async () => {
    if (!token) return;
    setRatesLoading(true);
    try {
      const res = await exchangeRateApi.list(token);
      setRates(res.data || []);
    } catch {
      // Rates are optional master data; a read failure must not block settings.
    } finally {
      setRatesLoading(false);
    }
  };

  const loadAuthSessions = async () => {
    if (!token) return;
    setAuthSessionsLoading(true);
    try {
      const response = await authApi.listSessions(token);
      setAuthSessions(response.sessions || []);
    } catch (error: any) {
      notifyError(error?.message || t("settings.sessions_load_failed", language));
    } finally {
      setAuthSessionsLoading(false);
    }
  };

  const revokeAuthSession = async (sessionId: string) => {
    if (!token) return;
    setRevokingSessionId(sessionId);
    try {
      await authApi.revokeSession(sessionId, token);
      message.success(t("settings.session_revoked", language));
      await loadAuthSessions();
    } catch (error: any) {
      notifyError(error?.message || t("settings.session_revoke_failed", language));
    } finally {
      setRevokingSessionId(null);
    }
  };

  const revokeAllAuthSessions = async () => {
    if (!token) return;
    setLoggingOutAll(true);
    try {
      await authApi.logoutAll(token);
      message.success(t("settings.sessions_revoked", language));
      await loadAuthSessions();
    } catch (error: any) {
      notifyError(error?.message || t("settings.sessions_revoke_failed", language));
    } finally {
      setLoggingOutAll(false);
    }
  };

  useEffect(() => {
    loadAuthSessions();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [token]);

  useEffect(() => {
    loadRates();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [token]);

  const saveRate = async () => {
    if (!token) return;
    if (!rateForm.rate_date || !rateForm.rate) {
      message.warning(t("settings.fx_incomplete", language));
      return;
    }
    setRateSaving(true);
    try {
      await exchangeRateApi.upsert({ ...rateForm, rate: rateForm.rate }, token);
      message.success(t("settings.fx_saved", language));
      setRateForm({ ...rateForm, rate: null });
      loadRates();
    } catch (error: any) {
      notifyError(error?.message || t("settings.fx_save_failed", language));
    } finally {
      setRateSaving(false);
    }
  };

  /* ---- fetch global discount rate ---- */
  useEffect(() => {
    if (!token) return;
    setDiscountRateLoading(true);
    settingsApi
      .getGlobal(token)
      .then((res) => {
        const raw: number = res.global_discount_rate ?? 0;
        const percent = raw > 1 ? raw : raw * 100;
        setEffectiveRate(percent);
        setDiscountRatePercent(percent);
        setHealthyCeiling(res.rent_to_sales_healthy_ceiling || null);
        setWarningCeiling(res.rent_to_sales_warning_ceiling || null);
        setVarianceMateriality(res.budget_variance_materiality_threshold || null);
        setTieOutTolerance(res.budget_tie_out_tolerance || null);
        setJournalMateriality(res.journal_entry_materiality_threshold || null);
      })
      .catch(() => notifyError(t("settings.load_failed", language)))
      .finally(() => setDiscountRateLoading(false));
  }, [token]);

  const handleSaveDiscountRate = async () => {
    if (discountRatePercent == null || !token) return;
    setDiscountRateSaving(true);
    try {
      await settingsApi.updateGlobal(
        { global_discount_rate: discountRatePercent },
        token,
      );
      setEffectiveRate(discountRatePercent);
      message.success(t("settings.save_success", language));
    } catch {
      notifyError(t("settings.save_failed", language));
    } finally {
      setDiscountRateSaving(false);
    }
  };

  const handleSaveRatioPolicy = async () => {
    if (!token || healthyCeiling == null || warningCeiling == null) return;
    if (healthyCeiling <= 0 || warningCeiling <= 0 || warningCeiling < healthyCeiling || warningCeiling > 100) {
      message.warning(t("settings.ratio_policy_invalid", language));
      return;
    }
    setRatioSaving(true);
    try {
      await settingsApi.updateGlobal({
        rent_to_sales_healthy_ceiling: healthyCeiling,
        rent_to_sales_warning_ceiling: warningCeiling,
      }, token);
      message.success(t("settings.ratio_policy_saved", language));
    } catch (error: any) {
      notifyError(error?.message || t("settings.save_failed", language));
    } finally {
      setRatioSaving(false);
    }
  };

  const handleSaveVariancePolicy = async () => {
    if (!token || varianceMateriality == null || tieOutTolerance == null || varianceMateriality <= 0 || tieOutTolerance <= 0) {
      message.warning(t("settings.variance_policy_invalid", language));
      return;
    }
    setVariancePolicySaving(true);
    try {
      await settingsApi.updateGlobal({
        budget_variance_materiality_threshold: varianceMateriality,
        budget_tie_out_tolerance: tieOutTolerance,
      }, token);
      message.success(t("settings.variance_policy_saved", language));
    } catch (error: any) {
      notifyError(error?.message || t("settings.save_failed", language));
    } finally {
      setVariancePolicySaving(false);
    }
  };

  const handleSaveJournalPolicy = async () => {
    if (!token || journalMateriality == null || journalMateriality <= 0) {
      message.warning(t("settings.journal_policy_invalid", language));
      return;
    }
    setVariancePolicySaving(true);
    try {
      await settingsApi.updateGlobal({ journal_entry_materiality_threshold: journalMateriality }, token);
      message.success(t("settings.journal_policy_saved", language));
    } catch (error: any) {
      notifyError(error?.message || t("settings.save_failed", language));
    } finally {
      setVariancePolicySaving(false);
    }
  };

  /* ---- filters ---- */
  const [searchText, setSearchText] = useState("");
  const [minContractCount, setMinContractCount] = useState<number | null>(null);

  /* ---- modal ---- */
  const [modalVisible, setModalVisible] = useState(false);
  const [modalTag, setModalTag] = useState("");
  const [modalContracts, setModalContracts] = useState<ContractRef[]>([]);

  /* ---- fetch ---- */
  const [tagLoadError, setTagLoadError] = useState<string | null>(null);
  useEffect(() => {
    if (!token) return;
    setLoading(true);
    setTagLoadError(null);
    reportApi
      .tagSummary(token)
      .then((res) => setData(res.data || []))
      .catch((err) => {
        // STATE-001：标签统计被折现率缺失阻塞（422 data_unavailable）是
        // 「用户能自己解决」——inline 呈现下一步，而不是红色 toast。
        const state = classifyDataState({
          error: err,
          data: null,
          actionFor: (e) => {
            if (!(e instanceof ApiError) || e.status !== 422 || e.code !== "data_unavailable") return null;
            const contracts = (e.detail as { details?: { contracts?: string[] } })?.details?.contracts;
            const list = contracts && contracts.length > 0 ? contracts.join("、") : "";
            return { message: t("settings.tags_actionable", language, { contracts: list }), actionLabel: "" };
          },
        });
        if (state.kind === "actionable" && state.message) setTagLoadError(state.message);
        else notifyError(t("settings.load_tags_failed", language));
      })
      .finally(() => setLoading(false));
  }, [token, language]);

  /* ---- derived ---- */
  const filtered = useMemo(() => {
    let rows = data;
    if (searchText.trim()) {
      const s = searchText.trim().toLowerCase();
      rows = rows.filter((r) => r.tag.toLowerCase().includes(s));
    }
    if (minContractCount != null) {
      rows = rows.filter((r) => r.contract_count >= minContractCount);
    }
    return rows;
  }, [data, searchText, minContractCount]);

  const stats = useMemo(() => {
    const totalTags = data.length;
    const uniqueContractIds = new Set<string>();
    let totalContracts = 0;
    data.forEach((r) => {
      totalContracts += r.contract_count;
      r.contracts?.forEach((c) => uniqueContractIds.add(c.contract_id));
    });
    const taggedContracts = uniqueContractIds.size;
    const avgPerTag = totalTags > 0 ? Math.round(totalContracts / totalTags) : 0;
    return { totalTags, taggedContracts, avgPerTag };
  }, [data]);

  /* ---- actions ---- */
  const handleCopyTag = async (tag: string) => {
    try {
      await navigator.clipboard.writeText(tag);
      message.success(t("settings.tag_copied", language));
    } catch {
      message.warning(t("settings.copy_failed", language));
    }
  };

  const handleViewContracts = (row: TagSummaryRow) => {
    setModalTag(row.tag);
    setModalContracts(row.contracts || []);
    setModalVisible(true);
  };

  const handleGotoReport = (tag: string) => {
    const q = new URLSearchParams({ tab: "amortization", view: "tag", tags: tag });
    router.push(`/reports?${q.toString()}`);
  };

  const columns = [
    {
      title: t("settings.col_tag", language),
      dataIndex: "tag",
      width: 200,
      render: (tag: string) => (
        <StatusTag kind="processing" style={{ fontSize: 13, padding: "1px 8px" }}>
          {tag}
        </StatusTag>
      ),
    },
    {
      title: t("settings.col_contract_count", language),
      dataIndex: "contract_count",
      width: 100,
      align: "center" as const,
    },
    {
      title: t("settings.col_example_contract", language),
      dataIndex: "contracts",
      width: 260,
      render: (contracts: ContractRef[] | undefined) => {
        if (!contracts || !contracts.length) return <Text type="secondary">—</Text>;
        const show = contracts.slice(0, 3);
        const rest = contracts.length - 3;
        return (
          <Space size={4} wrap>
            {show.map((c) => (
              <StatusTag key={c.contract_id} kind="neutral">
                {c.contract_number}
              </StatusTag>
            ))}
            {rest > 0 && <Text type="secondary">+{rest}</Text>}
          </Space>
        );
      },
    },
    {
      title: t("settings.col_action", language),
      width: 280,
      render: (_: any, row: TagSummaryRow) => (
        <Space size={4} wrap>
          <Button
            size="small"
            icon={<CopyOutlined />}
            onClick={() => handleCopyTag(row.tag)}
          >
            {t("settings.action_copy_tag", language)}
          </Button>
          <Button
            size="small"
            icon={<EyeOutlined />}
            onClick={() => handleViewContracts(row)}
          >
            {t("settings.action_view_contracts", language)}
          </Button>
          <Button
            size="small"
            icon={<BarChartOutlined />}
            onClick={() => handleGotoReport(row.tag)}
          >
            {t("settings.action_view_reports", language)}
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <ProtectedRoute>
      <AppLayout>
        <PageHeader
          title={<><SettingOutlined style={{ marginRight: 8 }} />{t("settings.title", language)}</>}
        />

        <Card
          title={t("settings.group_device_sessions", language)}
          style={{ marginBottom: 24 }}
          extra={
            <Space>
              <Button
                icon={<ReloadOutlined />}
                loading={authSessionsLoading}
                onClick={loadAuthSessions}
              >
                {t("settings.sessions_refresh", language)}
              </Button>
              <Button
                danger
                icon={<LogoutOutlined />}
                loading={loggingOutAll}
                onClick={revokeAllAuthSessions}
              >
                {t("settings.sessions_logout_all", language)}
              </Button>
            </Space>
          }
        >
          <Paragraph type="secondary">
            {t("settings.device_sessions_desc", language)}
          </Paragraph>
          <Table<AuthSession>
            size="small"
            loading={authSessionsLoading}
            rowKey="id"
            dataSource={authSessions}
            locale={{ emptyText: t("settings.sessions_empty", language) }}
            pagination={{ pageSize: 5, hideOnSinglePage: true }}
            columns={[
              {
                title: t("settings.session_created", language),
                dataIndex: "created_at",
                render: (value: string) => new Date(value).toLocaleString(),
              },
              {
                title: t("settings.session_device", language),
                dataIndex: "user_agent",
                render: (value: string) => value || "-",
              },
              {
                title: t("settings.session_ip", language),
                dataIndex: "ip_address",
                render: (value: string) => value || "-",
              },
              {
                title: t("settings.session_status", language),
                dataIndex: "active",
                render: (active: boolean) => (
                  <StatusTag kind={statusKindFromAntColor(active ? "green" : "default")}>
                    {active
                      ? t("settings.session_active", language)
                      : t("settings.session_revoked_status", language)}
                  </StatusTag>
                ),
              },
              {
                title: t("settings.col_action", language),
                render: (_: unknown, session: AuthSession) => (
                  <Button
                    size="small"
                    danger
                    disabled={!session.active}
                    loading={revokingSessionId === session.id}
                    onClick={() => revokeAuthSession(session.id)}
                  >
                    {t("settings.session_revoke", language)}
                  </Button>
                ),
              },
            ]}
          />
        </Card>

        {/* global discount rate card */}
        <Card
          title={t("settings.group_discount_rate", language)}
          style={{ marginBottom: 24 }}
          extra={
            effectiveRate != null ? (
              <StatusTag kind="processing">{t("settings.current_effective", language)}{effectiveRate.toFixed(2)}%</StatusTag>
            ) : null
          }
        >
          <Paragraph type="secondary">
            {t("settings.discount_rate_desc", language)}
          </Paragraph>
          <Spin spinning={discountRateLoading}>
            <Space align="center" size={12}>
              <Text strong>{t("settings.default_discount_rate", language)}</Text>
              <InputNumber
                value={discountRatePercent}
                onChange={(v) => setDiscountRatePercent(v)}
                step={0.01}
                min={0}
                placeholder={t("settings.discount_rate_placeholder", language)}
                style={{ width: 180 }}
              />
              <Button
                type="primary"
                loading={discountRateSaving}
                onClick={handleSaveDiscountRate}
              >
                {t("settings.save_discount_rate", language)}
              </Button>
            </Space>
          </Spin>
        </Card>

        <Card
          title={t("settings.group_rent_to_sales", language)}
          style={{ marginBottom: 24 }}
        >
          <Paragraph type="secondary">{t("settings.ratio_policy_desc", language)}</Paragraph>
          <Space align="center" size={12} wrap>
            <Text strong>{t("settings.ratio_healthy", language)}</Text>
            <InputNumber value={healthyCeiling} onChange={(value) => setHealthyCeiling(value)} min={0.01} max={100} step={0.5} addonAfter="%" />
            <Text strong>{t("settings.ratio_warning", language)}</Text>
            <InputNumber value={warningCeiling} onChange={(value) => setWarningCeiling(value)} min={0.01} max={100} step={0.5} addonAfter="%" />
            <Button type="primary" loading={ratioSaving} onClick={handleSaveRatioPolicy}>{t("settings.save_ratio_policy", language)}</Button>
          </Space>
        </Card>

        <Card
          title={t("settings.group_variance_policy", language)}
          style={{ marginBottom: 24 }}
        >
          <Paragraph type="secondary">{t("settings.variance_policy_desc", language)}</Paragraph>
          <Space align="center" size={12} wrap>
            <Text strong>{t("settings.variance_materiality", language)}</Text>
            <InputNumber value={varianceMateriality} onChange={(value) => setVarianceMateriality(value)} min={0.000001} step={0.01} />
            <Text strong>{t("settings.tie_out_tolerance", language)}</Text>
            <InputNumber value={tieOutTolerance} onChange={(value) => setTieOutTolerance(value)} min={0.000001} step={0.01} />
            <Button type="primary" loading={variancePolicySaving} onClick={handleSaveVariancePolicy}>{t("settings.save_variance_policy", language)}</Button>
          </Space>
        </Card>

        <Card
          title={t("settings.group_journal_policy", language)}
          style={{ marginBottom: 24 }}
        >
          <Paragraph type="secondary">{t("settings.journal_policy_desc", language)}</Paragraph>
          <Space align="center" size={12} wrap>
            <Text strong>{t("settings.journal_materiality", language)}</Text>
            <InputNumber value={journalMateriality} onChange={(value) => setJournalMateriality(value)} min={0.000001} step={0.01} />
            <Button type="primary" loading={variancePolicySaving} onClick={handleSaveJournalPolicy}>{t("settings.save_journal_policy", language)}</Button>
          </Space>
        </Card>

        {/* summary cards */}
        <Spin spinning={loading}>
          {tagLoadError && <Alert type="warning" showIcon className="settings-tags-actionable" message={tagLoadError} />}
          <Row gutter={16} style={{ marginBottom: 24 }}>
            <Col span={8}>
              <Card size="small">
                <Statistic
                  title={t("settings.stat_total_tags", language)}
                  value={stats.totalTags}
                  prefix={<TagOutlined />}
                />
              </Card>
            </Col>
            <Col span={8}>
              <Card size="small">
                <Statistic
                  title={t("settings.stat_tagged_contracts", language)}
                  value={stats.taggedContracts}
                  prefix={<FileTextOutlined />}
                />
              </Card>
            </Col>
            <Col span={8}>
              <Card size="small">
                <Statistic
                  title={t("settings.stat_avg_contracts_per_tag", language)}
                  value={stats.avgPerTag}
                />
              </Card>
            </Col>
          </Row>
        </Spin>

        {/* exchange rates */}
        <Card title={t("settings.fx_title", language)} style={{ marginBottom: 16 }}>
          <Paragraph type="secondary" style={{ marginBottom: 16 }}>
            {t("settings.fx_desc", language)}
          </Paragraph>
          <Space wrap style={{ marginBottom: 16 }}>
            <Input
              style={{ width: 90 }}
              value={rateForm.from_currency}
              onChange={(e) => setRateForm({ ...rateForm, from_currency: e.target.value.toUpperCase() })}
              placeholder="CNY"
            />
            <span>→</span>
            <Input
              style={{ width: 90 }}
              value={rateForm.to_currency}
              onChange={(e) => setRateForm({ ...rateForm, to_currency: e.target.value.toUpperCase() })}
              placeholder="CNY"
            />
            <Input
              style={{ width: 140 }}
              value={rateForm.rate_date}
              onChange={(e) => setRateForm({ ...rateForm, rate_date: e.target.value })}
              placeholder="YYYY-MM-DD"
            />
            <Select
              style={{ width: 150 }}
              value={rateForm.rate_type}
              onChange={(value) => setRateForm({ ...rateForm, rate_type: value })}
              options={[
                { value: "closing", label: t("settings.fx_type_closing", language) },
                { value: "average", label: t("settings.fx_type_average", language) },
              ]}
            />
            <InputNumber
              style={{ width: 160 }}
              min={0}
              step={0.0001}
              value={rateForm.rate}
              onChange={(value) => setRateForm({ ...rateForm, rate: value })}
              placeholder={t("settings.fx_rate_placeholder", language)}
            />
            <Button type="primary" loading={rateSaving} onClick={saveRate}>
              {t("settings.fx_save", language)}
            </Button>
          </Space>
          <Table
            loading={ratesLoading}
            dataSource={rates}
            rowKey="id"
            size="small"
            pagination={{ pageSize: 8 }}
            locale={{ emptyText: t("settings.fx_empty", language) }}
            columns={[
              {
                title: t("settings.fx_pair", language),
                key: "pair",
                render: (_: unknown, row: any) => `${row.from_currency} → ${row.to_currency}`,
              },
              {
                title: t("settings.fx_date", language),
                dataIndex: "rate_date",
                render: (value: string) => (value || "").slice(0, 10),
              },
              {
                title: t("settings.fx_type", language),
                dataIndex: "rate_type",
                render: (value: string) => (
                  <StatusTag kind={statusKindFromAntColor(value === "closing" ? "blue" : "gold")}>
                    {value === "closing"
                      ? t("settings.fx_type_closing", language)
                      : t("settings.fx_type_average", language)}
                  </StatusTag>
                ),
              },
              { title: t("settings.fx_rate", language), dataIndex: "rate", align: "right" as const },
              { title: t("settings.fx_source", language), dataIndex: "source", render: (v: string) => v || "-" },
            ]}
          />
        </Card>

        {/* filters */}
        <Card style={{ marginBottom: 16 }}>
          <Row gutter={[16, 12]} align="middle">
            <Col>
              <Space>
                <Text>{t("settings.search_tag", language)}</Text>
                <Input
                  value={searchText}
                  onChange={(e) => setSearchText(e.target.value)}
                  placeholder={t("settings.search_tag_placeholder", language)}
                  prefix={<SearchOutlined />}
                  allowClear
                  style={{ width: 220 }}
                />
              </Space>
            </Col>
            <Col>
              <Space>
                <Text>{t("settings.min_contract_count", language)}</Text>
                <InputNumber
                  value={minContractCount}
                  onChange={(v) => setMinContractCount(v)}
                  placeholder={t("settings.min_contract_all", language)}
                  min={0}
                  style={{ width: 100 }}
                />
              </Space>
            </Col>
          </Row>
        </Card>

        {/* table */}
        <Card>
          <Spin spinning={loading}>
            {!loading && !data.length ? (
              <StateBlock state={{ kind: "empty", reason: t("settings.empty_no_tags", language) }} language={language} />
            ) : (
              <Table
                columns={columns}
                dataSource={filtered}
                rowKey="tag"
                pagination={{ pageSize: 20, showSizeChanger: true }}
                locale={{ emptyText: t("settings.empty_no_match", language) }}
                size="middle"
              />
            )}
          </Spin>
        </Card>

        {/* contract details modal */}
        <Modal
          title={
            <span>
              {t("settings.modal_tag_contracts", language, { tag: modalTag })}
            </span>
          }
          open={modalVisible}
          onCancel={() => setModalVisible(false)}
          footer={null}
          width={560}
        >
          {modalContracts.length === 0 ? (
            <Empty description={t("settings.modal_no_contracts", language)} />
          ) : (
            <Table
              columns={[
                { title: t("settings.modal_contract_number", language), dataIndex: "contract_number", width: 160 },
                {
                  title: t("settings.modal_contract_name", language),
                  dataIndex: "contract_name",
                  ellipsis: true,
                  render: (v: string | undefined) => v || "—",
                },
              ]}
              dataSource={modalContracts}
              rowKey="contract_id"
              pagination={false}
              size="small"
            />
          )}
        </Modal>
      </AppLayout>
    </ProtectedRoute>
  );
}
