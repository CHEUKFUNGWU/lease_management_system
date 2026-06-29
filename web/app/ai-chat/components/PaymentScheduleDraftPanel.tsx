"use client";

import { useState } from "react";
import { Button, Input, Tag, Typography, message } from "antd";
import { t, type Language } from "../../lib/i18n";
import type { PaymentScheduleDraftItem, PaymentScheduleParseSummary } from "../../lib/types/ai-chat";

const { Text } = Typography;

interface PaymentScheduleDraftPanelProps {
  schedules: PaymentScheduleDraftItem[];
  summary: PaymentScheduleParseSummary;
  onConfirm: (selectedSchedules: PaymentScheduleDraftItem[]) => Promise<void> | void;
  onSkip: () => void;
  language: Language;
}

export function PaymentScheduleDraftPanel({
  schedules,
  summary,
  onConfirm,
  onSkip,
  language,
}: PaymentScheduleDraftPanelProps) {
  const [editedSchedules, setEditedSchedules] = useState<PaymentScheduleDraftItem[]>(
    schedules.map((schedule) => ({ ...schedule }))
  );
  const [selectedIndices, setSelectedIndices] = useState<Set<number>>(
    new Set(schedules.map((_, index) => index))
  );
  const [creating, setCreating] = useState(false);

  const toggleSelect = (index: number) => {
    setSelectedIndices((prev) => {
      const next = new Set(prev);
      if (next.has(index)) {
        next.delete(index);
      } else {
        next.add(index);
      }
      return next;
    });
  };

  const toggleSelectAll = () => {
    if (selectedIndices.size === editedSchedules.length) {
      setSelectedIndices(new Set());
      return;
    }
    setSelectedIndices(new Set(editedSchedules.map((_, index) => index)));
  };

  const updateSchedule = <K extends keyof PaymentScheduleDraftItem>(
    index: number,
    field: K,
    value: PaymentScheduleDraftItem[K]
  ) => {
    setEditedSchedules((prev) => {
      const next = [...prev];
      next[index] = { ...next[index], [field]: value };
      return next;
    });
  };

  const handleConfirm = async () => {
    const selected = Array.from(selectedIndices).map((index) => editedSchedules[index]);
    if (selected.length === 0) {
      message.warning(t("ai.draft_select_at_least_one", language));
      return;
    }
    if (!summary.can_import || !summary.contract_id) {
      message.warning(t("ai.schedule_bind_contract_first", language));
      return;
    }

    setCreating(true);
    try {
      await onConfirm(selected);
    } finally {
      setCreating(false);
    }
  };

  return (
    <div style={{ marginTop: 12, border: "1px solid #E5E5E5", borderRadius: 12, overflow: "hidden" }}>
      <div style={{ padding: "12px 16px", background: "#FAFAFA", borderBottom: "1px solid #E5E5E5" }}>
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12 }}>
          <div>
            <Text strong style={{ fontSize: 14 }}>{t("ai.schedule_panel_title", language)}</Text>
            <Text type="secondary" style={{ fontSize: 12, marginLeft: 8 }}>
              {t("ai.draft_panel_subtitle", language, { total: String(summary.total_count), confidence: String((summary.overall_confidence * 100).toFixed(0)) })}
            </Text>
          </div>
          <div style={{ display: "flex", gap: 8 }}>
            <Button size="small" onClick={toggleSelectAll}>
              {selectedIndices.size === editedSchedules.length ? t("ai.deselect_all", language) : t("ai.select_all", language)}
            </Button>
            <Button size="small" danger onClick={onSkip}>
              {t("ai.skip", language)}
            </Button>
          </div>
        </div>
        {!summary.can_import && (
          <div style={{ marginTop: 8, padding: "8px 12px", background: "#FFF1F0", borderRadius: 6, border: "1px solid #FFA39E" }}>
            <Text style={{ fontSize: 12, color: "#CF1322" }}>{t("ai.schedule_bind_contract_first", language)}</Text>
          </div>
        )}
        {(summary.requires_human_confirmation || summary.warnings.length > 0) && (
          <div style={{ marginTop: 8, padding: "8px 12px", background: "#FFF7E6", borderRadius: 6, border: "1px solid #FFD591" }}>
            <Text style={{ fontSize: 12, color: "#D46B08" }}>{t("ai.schedule_review_warning", language)}</Text>
          </div>
        )}
      </div>

      <div style={{ maxHeight: 360, overflowY: "auto" }}>
        {editedSchedules.map((schedule, index) => (
          <div
            key={index}
            style={{
              padding: "12px 16px",
              borderBottom: "1px solid #F0F0F0",
              background: selectedIndices.has(index) ? "#F6FFED" : "#fff",
              opacity: selectedIndices.has(index) ? 1 : 0.6,
            }}
          >
            <div style={{ display: "flex", alignItems: "flex-start", gap: 12 }}>
              <input type="checkbox" checked={selectedIndices.has(index)} onChange={() => toggleSelect(index)} style={{ marginTop: 4 }} />
              <div style={{ flex: 1 }}>
                <div style={{ display: "flex", gap: 8, flexWrap: "wrap", marginBottom: 8 }}>
                  <div style={{ flex: "1 1 120px" }}>
                    <Text type="secondary" style={{ fontSize: 11 }}>{t("ai.schedule_period_start", language)}</Text>
                    <Input size="small" value={schedule.period_start} onChange={(e) => updateSchedule(index, "period_start", e.target.value)} />
                  </div>
                  <div style={{ flex: "1 1 120px" }}>
                    <Text type="secondary" style={{ fontSize: 11 }}>{t("ai.schedule_period_end", language)}</Text>
                    <Input size="small" value={schedule.period_end} onChange={(e) => updateSchedule(index, "period_end", e.target.value)} />
                  </div>
                  <div style={{ flex: "1 1 120px" }}>
                    <Text type="secondary" style={{ fontSize: 11 }}>{t("ai.schedule_due_date", language)}</Text>
                    <Input size="small" value={schedule.due_date} onChange={(e) => updateSchedule(index, "due_date", e.target.value)} />
                  </div>
                  <div style={{ flex: "1 1 110px" }}>
                    <Text type="secondary" style={{ fontSize: 11 }}>{t("ai.schedule_amount", language)}</Text>
                    <Input size="small" value={schedule.amount} onChange={(e) => updateSchedule(index, "amount", parseFloat(e.target.value) || 0)} status={schedule.amount <= 0 ? "error" : ""} />
                  </div>
                </div>
                <div style={{ display: "flex", gap: 8, flexWrap: "wrap", marginBottom: 8 }}>
                  <div style={{ flex: "1 1 110px" }}>
                    <Text type="secondary" style={{ fontSize: 11 }}>{t("ai.draft_currency", language)}</Text>
                    <Input size="small" value={schedule.currency || ""} onChange={(e) => updateSchedule(index, "currency", e.target.value)} status={!schedule.currency ? "warning" : ""} />
                  </div>
                  <div style={{ flex: "1 1 120px" }}>
                    <Text type="secondary" style={{ fontSize: 11 }}>{t("ai.draft_payment_timing", language)}</Text>
                    <Input size="small" value={schedule.payment_timing} onChange={(e) => updateSchedule(index, "payment_timing", e.target.value)} />
                  </div>
                  <div style={{ flex: "1 1 140px" }}>
                    <Text type="secondary" style={{ fontSize: 11 }}>{t("ai.schedule_amount_type", language)}</Text>
                    <Input size="small" value={schedule.amount_type} onChange={(e) => updateSchedule(index, "amount_type", e.target.value)} />
                  </div>
                  <div style={{ flex: "1 1 130px" }}>
                    <Text type="secondary" style={{ fontSize: 11 }}>{t("ai.schedule_is_fixed", language)}</Text>
                    <Input size="small" value={schedule.is_fixed ? "true" : "false"} onChange={(e) => updateSchedule(index, "is_fixed", e.target.value === "true")} />
                  </div>
                  <div style={{ flex: "1 1 150px" }}>
                    <Text type="secondary" style={{ fontSize: 11 }}>{t("ai.schedule_is_lease_component", language)}</Text>
                    <Input size="small" value={schedule.is_lease_component ? "true" : "false"} onChange={(e) => updateSchedule(index, "is_lease_component", e.target.value === "true")} />
                  </div>
                </div>
                <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
                  <Tag color={schedule.confidence < 0.8 ? "warning" : "green"} style={{ fontSize: 11 }}>
                    {t("ai.draft_confidence", language, { value: String((schedule.confidence * 100).toFixed(0)) })}
                  </Tag>
                  {!schedule.is_fixed && <Tag color="orange" style={{ fontSize: 11 }}>{t("ai.schedule_variable_rent", language)}</Tag>}
                  {!schedule.is_lease_component && <Tag color="orange" style={{ fontSize: 11 }}>{t("ai.schedule_non_lease_component", language)}</Tag>}
                </div>
              </div>
            </div>
          </div>
        ))}
      </div>

      <div style={{ padding: "12px 16px", background: "#FAFAFA", borderTop: "1px solid #E5E5E5", display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <Text type="secondary" style={{ fontSize: 12 }}>
          {t("ai.draft_selected_count", language, { selected: String(selectedIndices.size), total: String(editedSchedules.length) })}
        </Text>
        <Button type="primary" loading={creating} disabled={selectedIndices.size === 0 || !summary.can_import} onClick={handleConfirm} style={{ background: "#000", borderColor: "#000" }}>
          {t("ai.schedule_confirm_import", language)}
        </Button>
      </div>
    </div>
  );
}
