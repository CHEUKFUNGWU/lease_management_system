"use client";

/**
 * F1 科目树编辑器（/financial-model 内）。
 *
 * 边界：
 *  - 预置科目（16 保留键）不可删、键不可改；删除按钮以机械原因解释
 *    （D-F1），绝不以权限措辞。
 *  - 新增科目必须选父小计（D-F3）；父为保留存量键时存量/流量必填、无默认
 *    （D-F4）。
 *  - 名称命中既有维度值时提示用维度表达，只提示不阻止（D-F6）。
 *  - 公式校验一律走 /templates/validate（D-F8），本组件与 helpers 均无本地
 *    解析逻辑（coa-editor.test.tsx 源码层守卫锁定）。
 *  - 用户主张的零走「确认为零」标记（D-F2），与缺失（— + Gap）在数据层
 *    分离。
 */

import React, { useCallback, useEffect, useMemo, useState } from "react";
import { Alert, Button, Form, Input, Radio, Select, Space, Table, Tag, Typography } from "antd";
import { useLanguage } from "../context/LanguageContext";
import { t } from "../lib/i18n";
import {
  RESERVED_KEYS_REASON_I18N_KEY,
  RESERVED_SHEET_KEYS,
  appendAccount,
  dimensionHint,
  parentRequiresFold,
  validateNewAccount,
  type CoaRow,
} from "./coa-helpers";

const { Text } = Typography;

export interface CoaEditorProps {
  /** 测试注入口：初始新增科目名称（生产代码不传）。 */
  defaultNewLabel?: string;
  rows: CoaRow[];
  templateName: string;
  templateVersion: number;
  status: string;
  source?: string;
  /** fact.* 中文名（唯一真相源 retailkpi，经 /retail/kpis/definitions 取得）。 */
  kpiNames: Record<string, string>;
  /** 既有维度值（区域/品牌），用于 D-F6 提示。 */
  dimensionValues?: readonly string[];
  /** 用户主张的零（D-F2）：rowKey → 确认人。数据层与缺失分离。 */
  humanZeros: Record<string, { confirmedBy: string; confirmedAt?: string }>;
  onToggleHumanZero: (rowKey: string) => void;
  onSave: (rows: CoaRow[], nextVersion: number) => Promise<void>;
  saving?: boolean;
}

export function CoaEditor({
  defaultNewLabel,
  rows,
  templateName,
  templateVersion,
  status,
  source,
  kpiNames,
  dimensionValues,
  humanZeros,
  onToggleHumanZero,
  onSave,
  saving,
}: CoaEditorProps) {
  const { language } = useLanguage();
  const [form] = Form.useForm();
  const [foldChoice, setFoldChoice] = useState<"" | "stock" | "flow">("");
  const [parentKey, setParentKey] = useState<string>("");
  const [newLabel, setNewLabel] = useState(defaultNewLabel ?? "");
  const [newKey, setNewKey] = useState("");
  const [rejection, setRejection] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);

  const subtotals = useMemo(() => rows.filter((r) => r.kind === "subtotal"), [rows]);
  const reservedSet = useMemo(() => new Set(RESERVED_SHEET_KEYS), []);

  // 结构化拒绝码 → 文案（§13-7：文案走 t()）。
  const rejectionMessage = (problem: NonNullable<ReturnType<typeof validateNewAccount>>): string => {
    const detail = problem.detail ? `（${problem.detail}）` : "";
    switch (problem.code) {
      case "missing_key":
        return `${t("finmodel.coa_err_missing_key", language)}${detail}`;
      case "missing_label":
        return t("finmodel.coa_err_missing_label", language);
      case "reserved_key_conflict":
        return `${t("finmodel.coa_err_reserved_conflict", language)}${detail}：${t(RESERVED_KEYS_REASON_I18N_KEY, language)}`;
      case "duplicate":
        return `${t("finmodel.coa_err_duplicate", language)}${detail}`;
      case "parent_required":
        return t("finmodel.coa_err_parent_required", language);
      case "parent_not_subtotal":
        return t("finmodel.coa_err_parent_not_subtotal", language);
      case "fold_required":
        return t("finmodel.coa_fold_required", language);
    }
  };

  const submitAdd = () => {
    const input = { key: newKey, label: newLabel, parentKey, fold: foldChoice, rows };
    const problem = validateNewAccount(input);
    if (problem) {
      setRejection(rejectionMessage(problem));
      return;
    }
    setRejection(null);
    onSave(appendAccount(rows, input), templateVersion + 1);
    setNewLabel("");
    setNewKey("");
    setFoldChoice("");
    setParentKey("");
    form.resetFields();
  };

  // F1 D-F6：输入即提示，只提示不阻止。
  const dimensionNotice = dimensionHint(newLabel, dimensionValues ?? []);

  const deleteRow = (row: CoaRow) => {
    if (reservedSet.has(row.key)) {
      // D-F1：拒绝并说明机械原因。绝不说「无权限」。
      setRejection(`${row.label}（${row.key}）：${t("finmodel.coa_err_delete_reserved", language)} ${t(RESERVED_KEYS_REASON_I18N_KEY, language)}`);
      return;
    }
    setRejection(null);
    const next = rows
      .map((candidate) =>
        candidate.kind === "subtotal"
          ? { ...candidate, children: (candidate.children ?? []).filter((child) => child !== row.key) }
          : candidate,
      )
      .filter((candidate) => candidate.key !== row.key);
    onSave(next, templateVersion + 1);
  };

  const columns = [
    {
      title: t("finmodel.coa_key", language) ?? "科目键",
      dataIndex: "key",
      render: (key: string) =>
        reservedSet.has(key) ? (
          <Space size={4}>
            <span>{key}</span>
            <Tag>{t("finmodel.coa_reserved", language) ?? "预置"}</Tag>
          </Space>
        ) : (
          key
        ),
    },
    { title: t("finmodel.coa_col_label", language), dataIndex: "label" },
    {
      title: t("finmodel.col_source", language),
      dataIndex: "source",
      render: (source: string | undefined, row: CoaRow) => {
        if (row.kind === "formula") return `formula: ${row.formula ?? ""}`;
        if (row.kind === "input") {
          return humanZeros[row.key]
            ? `${t("finmodel.coa_zero", language)}（${humanZeros[row.key].confirmedBy}）`
            : t("finmodel.coa_kind_manual", language);
        }
        if (row.kind === "link") {
          // 中文名唯一真相源 retailkpi；非 fact 族显示绑定键本身。
          const code = (source ?? "").replace(/^fact\./, "");
          return source?.startsWith("fact.") && kpiNames[code] ? `${kpiNames[code]}（${source}）` : source;
        }
        if (row.kind === "subtotal") return `${(row.children ?? []).length} ${t("finmodel.coa_kind_subtotal_items", language)}`;
        return "";
      },
    },
    {
      title: t("finmodel.col_stock_flow", language),
      dataIndex: "fold",
      render: (_: unknown, row: CoaRow) => {
        if (row.fold === "stock") return t("finmodel.fold_stock", language);
        if (row.fold === "flow") return t("finmodel.fold_flow", language);
        return RESERVED_SHEET_KEYS.includes(row.key) ? t("finmodel.fold_stock", language) : "—";
      },
    },
    {
      title: t("finmodel.col_actions", language),
      render: (_: unknown, row: CoaRow) => (
        <Space size={4}>
          <Button
            size="small"
            danger={reservedSet.has(row.key)}
            disabled={reservedSet.has(row.key)}
            title={reservedSet.has(row.key) ? t(RESERVED_KEYS_REASON_I18N_KEY, language) : undefined}
            onClick={() => deleteRow(row)}
          >
            {t("finmodel.coa_delete", language)}
          </Button>
          {!reservedSet.has(row.key) && row.kind !== "subtotal" && (
            <Button
              size="small"
              onClick={() => onToggleHumanZero(row.key)}
              
            >
              {humanZeros[row.key]
                ? t("finmodel.coa_unmark_zero", language)
                : t("finmodel.coa_mark_zero", language)}
            </Button>
          )}
        </Space>
      ),
    },
  ];

  return (
    <Space direction="vertical" className="finmodel-full-width" data-testid="coa-editor">
      <Space size={8} wrap>
        <Text strong>{templateName}</Text>
        <Text type="secondary">v{templateVersion}</Text>
        <Tag>{status}</Tag>
        {source === "ai_suggestion" && (
          <Tag>{t("finmodel.coa_ai_source", language)}</Tag>
        )}
      </Space>

      {/* F1 D-F2: an asserted zero is a HumanInput cell (explicit 0); an
          unmarked empty value renders as em-dash plus named gap. */}
      {Object.keys(humanZeros).length > 0 && (
        <Alert
          type="info"
          showIcon
          message={`${t("finmodel.coa_zero_note", language) ?? "已确认为零的科目"}：${Object.keys(humanZeros).join(", ")}`}
        />
      )}

      {dimensionNotice && (
        <Alert type="warning" showIcon message={t("finmodel.coa_dimension_hint", language).replace("{value}", dimensionNotice)} />
      )}

      {rejection && (
        <Alert type="error" showIcon message={rejection} closable afterClose={() => setRejection(null)} />
      )}
      {saveError && <Alert type="error" showIcon message={saveError} />}

      <Table
        size="small"
        rowKey="key"
        columns={columns as never}
        dataSource={rows}
        pagination={false}
      />

      <Form form={form} layout="inline" onFinish={submitAdd}>
        <Form.Item label={t("finmodel.coa_new_key", language) ?? "科目键"}>
          <Input value={newKey} onChange={(e) => setNewKey(e.target.value)} placeholder="subscription_revenue" />
        </Form.Item>
        <Form.Item label={t("finmodel.coa_new_label", language)}>
          <Input value={newLabel} onChange={(e) => setNewLabel(e.target.value)} placeholder="subscription_revenue" />
        </Form.Item>
        <Form.Item label={t("finmodel.coa_parent", language)}>
          <Select
            value={parentKey || undefined}
            onChange={(value: string) => setParentKey(value)}
            placeholder={t("finmodel.coa_parent_placeholder", language) ?? "选择子计行"}
            options={subtotals.map((s) => ({ value: s.key, label: s.label }))}
          />
        </Form.Item>
        <Form.Item label={t("finmodel.coa_fold", language) ?? "存量/流量"}>
          <Radio.Group
            value={foldChoice || undefined}
            onChange={(e) => setFoldChoice(e.target.value)}
            options={[
              { value: "stock", label: t("finmodel.fold_stock", language) },
              { value: "flow", label: t("finmodel.fold_flow", language) },
            ]}
          />
        </Form.Item>
        <Form.Item>
          <Button type="primary" htmlType="submit">
            {t("finmodel.coa_add", language) ?? "新增科目"}
          </Button>
        </Form.Item>
      </Form>
      {parentRequiresFold(parentKey) && !foldChoice && (
        <Alert
          type="info"
          showIcon
          message={t("finmodel.coa_fold_required", language) ?? "该父小计属于资产负债表科目：必须声明存量或流量（不给默认值）"}
        />
      )}

      <Text type="secondary">{t("finmodel.coa_footer", language)}</Text>
    </Space>
  );
}

// ── 页面接线包装：自取模板列表、管草稿状态与保存（版本自动 +1）。────────

import { finModelTemplatesApi, type FinStatementTemplateRow } from "../lib/api";
import { useAuth } from "../context/AuthContext";

export function CoaEditorPanel({ language: langProp }: { language?: Parameters<typeof t>[1] }) {
  const { language } = useLanguage();
  const { token } = useAuth();
  const [templates, setTemplates] = useState<FinStatementTemplateRow[] | null>(null);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [rows, setRows] = useState<CoaRow[] | null>(null);
  const [humanZeros, setHumanZeros] = useState<Record<string, { confirmedBy: string; confirmedAt?: string }>>({});
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!token) return;
    try {
      const res = await finModelTemplatesApi.list(token, { status: "draft" });
      const drafts = res.templates ?? [];
      setTemplates(drafts);
      const first = drafts[0];
      if (first) {
        setSelectedId(first.id);
        setRows((first.rows?.rows ?? []) as CoaRow[]);
      } else {
        setRows([]);
        setSelectedId(null);
      }
    } catch {
      setError(t("finmodel.coa_load_failed", language) ?? "科目树加载失败");
    }
  }, [token, language]);

  useEffect(() => {
    void load();
  }, [load]);

  if (!token) {
    return <Alert type="info" showIcon message={t("finmodel.coa_login", language) ?? "登录后使用科目树编辑器"} />;
  }
  if (error) return <Alert type="error" showIcon message={error} />;
  if (rows === null) {
    return <Alert type="info" showIcon message={t("finmodel.coa_loading", language) ?? "科目树加载中"} />;
  }

  const selected = templates?.find((tpl) => tpl.id === selectedId) ?? null;

  const handleSave = async (nextRows: CoaRow[], nextVersion: number) => {
    if (!token || !selected) {
      setError(t("finmodel.coa_no_draft_selected", language) ?? "没有可保存的草稿模板");
      return;
    }
    setSaving(true);
    try {
      await finModelTemplatesApi.create(
        { name: selected.name, version: nextVersion, rows: nextRows },
        token,
        selected.legal_entity_id ? "shared" : "personal",
      );
      await load();
    } catch {
      setError(t("finmodel.coa_save_failed", language) ?? "科目树保存失败");
    } finally {
      setSaving(false);
    }
  };

  if (!selected) {
    return (
      <Alert
        type="info"
        showIcon
        message={t("finmodel.coa_empty", language) ?? "暂无 draft 模板：先在上方创建或复制一份科目树草稿"}
      />
    );
  }

  return (
    <CoaEditor
      rows={rows}
      templateName={selected.name}
      templateVersion={selected.version}
      status={selected.status}
      source={selected.rows?.source}
      kpiNames={{}}
      humanZeros={humanZeros}
      onToggleHumanZero={(rowKey) =>
        setHumanZeros((prev) => {
          const next = { ...prev };
          if (next[rowKey]) delete next[rowKey];
          else next[rowKey] = { confirmedBy: "current-user", confirmedAt: new Date().toISOString() };
          return next;
        })
      }
      onSave={handleSave}
      saving={saving}
    />
  );
}
