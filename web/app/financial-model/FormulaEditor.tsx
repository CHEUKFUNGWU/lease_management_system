"use client";

// R2-4（RH6，D-R16）：公式编辑器。前端零本地校验——包括括号配对这种
// 「显然安全」的检查也一律走后端 /templates/validate。本组件里不允许
// 出现任何解析逻辑（括号计数、token 切分、正则校验公式内容）；
// variance-attribution.test 同款守卫在 formula-editor.test.tsx 锁这一点。
//
// 交互：科目键 chips 点选插入 rows.<key>；输入停顿 300ms 调后端；
// 错误按 kind 渲染（循环引用展示完整链路），不 split 后端文本。

import React, { useEffect, useRef, useState } from "react";
import { Alert, Button, Input, Space, Spin, Tag, Typography } from "antd";
import { useLanguage } from "../context/LanguageContext";
import { useAuth } from "../context/AuthContext";
import { t } from "../lib/i18n";
import { finModelTemplatesApi, type TemplateValidationResult } from "../lib/api";

const { Text } = Typography;

export interface FormulaEditorProps {
  language?: Parameters<typeof t>[1];
  /** 测试注入口：直接给定校验结果绕过网络；生产代码不传。 */
  __testResult?: TemplateValidationResult | null;
}

export function FormulaEditor({ language: langProp, __testResult }: FormulaEditorProps) {
  const { language } = useLanguage();
  const { token } = useAuth();
  const lang = langProp ?? language;

  const [rowKeysText, setRowKeysText] = useState("");
  const [formula, setFormula] = useState("rows.");
  const [validating, setValidating] = useState(false);
  const [netValidation, setNetValidation] = useState<TemplateValidationResult | null>(null);
  const validation = __testResult !== undefined ? __testResult : netValidation;
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const seq = useRef(0);

  const rowKeys = rowKeysText
    .split(/[,\s]+/)
    .map((k) => k.trim())
    .filter(Boolean);

  // 输入停顿后调校验端点：debounce 是交互节奏，不是本地校验。
  useEffect(() => {
    if (__testResult !== undefined) return;
    if (!token) return;
    const id = ++seq.current;
    setValidating(true);
    const timer = setTimeout(async () => {
      try {
        const def = {
          name: "formula-editor-draft",
          version: 1,
          rows: [
            ...rowKeys.map((key) => ({ key, label: key, kind: "input", basis: "shared" })),
            { key: "__editor_formula__", label: "formula", kind: "formula", basis: "shared", formula },
          ],
        };
        const res = await finModelTemplatesApi.validate(def, token!);
        if (id === seq.current) {
          setNetValidation(res);
          setValidating(false);
        }
      } catch {
        if (id === seq.current) {
          setNetValidation(null);
          setValidating(false);
        }
      }
    }, 300);
    return () => clearTimeout(timer);
  }, [formula, rowKeysText, token]);

  const insertKey = (key: string) => {
    const el = textareaRef.current;
    if (!el) return;
    const pos = el.selectionStart ?? formula.length;
    const tokenText = `rows.${key}`;
    const next = formula.slice(0, pos) + tokenText + formula.slice(el.selectionEnd ?? pos);
    setFormula(next);
    requestAnimationFrame(() => {
      el.focus();
      const caret = pos + tokenText.length;
      el.setSelectionRange(caret, caret);
    });
  };

  const errorOfKind = (kind: string) => validation?.errors?.find((e) => e.kind === kind);

  const renderFeedback = () => {
    if (validating && !validation) {
      return <Spin size="small" />;
    }
    if (!validation) return null;
    if (validation.valid) {
      return <Tag>{t("finmodel.formula.valid", lang)}</Tag>;
    }
    const cycle = errorOfKind("circular_reference");
    const unknown = errorOfKind("unknown_reference");
    const syntax = errorOfKind("syntax");
    if (cycle?.cycle_path?.length) {
      return (
        <Alert
          type="error"
          showIcon
          className="formula-err formula-err-cycle"
          message={t("finmodel.formula.err_cycle", lang).replace("{path}", cycle.cycle_path.join(" → "))}
        />
      );
    }
    if (unknown) {
      return (
        <Alert
          type="error"
          showIcon
          className="formula-err formula-err-unknown"
          message={t("finmodel.formula.err_unknown", lang).replace("{key}", unknown.ref_key || "—")}
        />
      );
    }
    if (syntax) {
      return (
        <Alert
          type="error"
          showIcon
          className="formula-err formula-err-syntax"
          message={t("finmodel.formula.err_syntax", lang)
            .replace("{pos}", syntax.position != null ? String(syntax.position + 1) : "?")
            .replace("{detail}", syntax.message)}
        />
      );
    }
    return null;
  };

  return (
    <Space direction="vertical" size={8} className="formula-editor-body">
      <div>
        <div className="formula-editor-label">{t("finmodel.formula.rows_hint", lang)}</div>
        <Input
          value={rowKeysText}
          onChange={(e) => setRowKeysText(e.target.value)}
          placeholder="rev, cost"
          className="formula-editor-keys-input"
        />
      </div>
      <Space wrap size={4}>
        {rowKeys.map((key) => (
          <Button key={key} size="small" onClick={() => insertKey(key)} title={`rows.${key}`}>
            {key}
          </Button>
        ))}
      </Space>
      <div>
        <div className="formula-editor-label">{t("finmodel.formula.insert_hint", lang)}</div>
        <Input.TextArea
          ref={textareaRef as never}
          value={formula}
          onChange={(e) => setFormula(e.target.value)}
          rows={2}
          className="formula-editor-textarea font-tabular"
        />
        <div className="formula-editor-label">{t("finmodel.formula.lag_hint", lang)}</div>
      </div>
      <Space size={8} align="center">
        {renderFeedback()}
        {validating && validation ? <span className="formula-validating">{t("finmodel.formula.validating", lang)}</span> : null}
      </Space>
    </Space>
  );
}
