"use client";

import { useState } from "react";
import { Button, Input, Tag, Typography, message } from "antd";
import { t, type Language } from "../../lib/i18n";
import type { BatchParseSummary, ContractDraftItem } from "../../lib/types/ai-chat";

const { Text } = Typography;

interface DraftConfirmationPanelProps {
  contracts: ContractDraftItem[];
  summary: BatchParseSummary;
  onConfirm: (selectedContracts: ContractDraftItem[]) => Promise<void> | void;
  onSkip: () => void;
  language: Language;
}

export function DraftConfirmationPanel({
  contracts,
  summary,
  onConfirm,
  onSkip,
  language,
}: DraftConfirmationPanelProps) {
  const [editedContracts, setEditedContracts] = useState<ContractDraftItem[]>(
    contracts.map((contract) => ({ ...contract }))
  );
  const [selectedIndices, setSelectedIndices] = useState<Set<number>>(
    new Set(contracts.map((_, index) => index))
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
    if (selectedIndices.size === editedContracts.length) {
      setSelectedIndices(new Set());
      return;
    }
    setSelectedIndices(new Set(editedContracts.map((_, index) => index)));
  };

  const updateContract = <K extends keyof ContractDraftItem>(
    index: number,
    field: K,
    value: ContractDraftItem[K]
  ) => {
    setEditedContracts((prev) => {
      const next = [...prev];
      next[index] = { ...next[index], [field]: value };
      return next;
    });
  };

  const handleConfirm = async () => {
    const selected = Array.from(selectedIndices).map((index) => editedContracts[index]);
    if (selected.length === 0) {
      message.warning(t("ai.draft_select_at_least_one", language));
      return;
    }

    setCreating(true);
    try {
      await onConfirm(selected);
    } finally {
      setCreating(false);
    }
  };

  const hasLowConfidence = (contract: ContractDraftItem) =>
    contract.confidence < 0.8 ||
    (contract.scope_confidence ?? 1) < 0.8 ||
    contract.missing_fields.length > 0;

  return (
    <div style={{ marginTop: 12, border: "1px solid #E5E5E5", borderRadius: 12, overflow: "hidden" }}>
      <div style={{ padding: "12px 16px", background: "#FAFAFA", borderBottom: "1px solid #E5E5E5" }}>
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
          <div>
            <Text strong style={{ fontSize: 14 }}>
              {t("ai.draft_panel_title", language)}
            </Text>
            <Text type="secondary" style={{ fontSize: 12, marginLeft: 8 }}>
              {t("ai.draft_panel_subtitle", language, {
                total: String(summary.total_count),
                confidence: String((summary.overall_confidence * 100).toFixed(0)),
              })}
            </Text>
          </div>
          <div style={{ display: "flex", gap: 8 }}>
            <Button size="small" onClick={toggleSelectAll}>
              {selectedIndices.size === editedContracts.length
                ? t("ai.deselect_all", language)
                : t("ai.select_all", language)}
            </Button>
            <Button size="small" danger onClick={onSkip}>
              {t("ai.skip", language)}
            </Button>
          </div>
        </div>
        {summary.requires_human_confirmation && (
          <div
            style={{
              marginTop: 8,
              padding: "8px 12px",
              background: "#FFF7E6",
              borderRadius: 6,
              border: "1px solid #FFD591",
            }}
          >
            <Text style={{ fontSize: 12, color: "#D46B08" }}>⚠️ {t("ai.draft_warning", language)}</Text>
          </div>
        )}
        {summary.warnings.length > 0 && (
          <div style={{ marginTop: 8 }}>
            {summary.warnings.slice(0, 3).map((warning, index) => (
              <Text key={index} style={{ fontSize: 11, color: "#CF1322", display: "block" }}>
                • {warning}
              </Text>
            ))}
          </div>
        )}
      </div>

      <div style={{ maxHeight: 400, overflowY: "auto" }}>
        {editedContracts.map((contract, index) => (
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
              <input
                type="checkbox"
                checked={selectedIndices.has(index)}
                onChange={() => toggleSelect(index)}
                style={{ marginTop: 4 }}
              />
              <div style={{ flex: 1 }}>
                <div style={{ display: "flex", gap: 8, flexWrap: "wrap", marginBottom: 8 }}>
                  <div style={{ flex: "1 1 200px" }}>
                    <Text type="secondary" style={{ fontSize: 11 }}>{t("ai.draft_contract_number", language)}</Text>
                    <Input size="small" value={contract.contract_number} onChange={(e) => updateContract(index, "contract_number", e.target.value)} style={{ fontSize: 13 }} />
                  </div>
                  <div style={{ flex: "1 1 200px" }}>
                    <Text type="secondary" style={{ fontSize: 11 }}>{t("ai.draft_contract_name", language)}</Text>
                    <Input size="small" value={contract.contract_name} onChange={(e) => updateContract(index, "contract_name", e.target.value)} style={{ fontSize: 13 }} />
                  </div>
                  <div style={{ flex: "1 1 150px" }}>
                    <Text type="secondary" style={{ fontSize: 11 }}>{t("ai.draft_currency", language)}</Text>
                    <Input size="small" value={contract.currency} onChange={(e) => updateContract(index, "currency", e.target.value)} style={{ fontSize: 13 }} status={!contract.currency ? "error" : ""} />
                  </div>
                </div>

                <div style={{ display: "flex", gap: 8, flexWrap: "wrap", marginBottom: 8 }}>
                  <div style={{ flex: "1 1 200px" }}>
                    <Text type="secondary" style={{ fontSize: 11 }}>{t("ai.draft_lessee", language)}</Text>
                    <Input size="small" value={contract.lessee} onChange={(e) => updateContract(index, "lessee", e.target.value)} style={{ fontSize: 13 }} />
                  </div>
                  <div style={{ flex: "1 1 200px" }}>
                    <Text type="secondary" style={{ fontSize: 11 }}>{t("ai.draft_lessor", language)}</Text>
                    <Input size="small" value={contract.lessor} onChange={(e) => updateContract(index, "lessor", e.target.value)} style={{ fontSize: 13 }} />
                  </div>
                  <div style={{ flex: "1 1 150px" }}>
                    <Text type="secondary" style={{ fontSize: 11 }}>{t("ai.draft_store", language)}</Text>
                    <Input size="small" value={contract.store_name} onChange={(e) => updateContract(index, "store_name", e.target.value)} style={{ fontSize: 13 }} />
                  </div>
                </div>

                <div style={{ display: "flex", gap: 8, flexWrap: "wrap", marginBottom: 8 }}>
                  <div style={{ flex: "1 1 150px" }}>
                    <Text type="secondary" style={{ fontSize: 11 }}>{t("ai.draft_commencement_date", language)}</Text>
                    <Input size="small" value={contract.commencement_date} onChange={(e) => updateContract(index, "commencement_date", e.target.value)} style={{ fontSize: 13 }} />
                  </div>
                  <div style={{ flex: "1 1 150px" }}>
                    <Text type="secondary" style={{ fontSize: 11 }}>{t("ai.draft_lease_start", language)}</Text>
                    <Input size="small" value={contract.lease_start_date} onChange={(e) => updateContract(index, "lease_start_date", e.target.value)} style={{ fontSize: 13 }} />
                  </div>
                  <div style={{ flex: "1 1 150px" }}>
                    <Text type="secondary" style={{ fontSize: 11 }}>{t("ai.draft_lease_end", language)}</Text>
                    <Input size="small" value={contract.lease_end_date} onChange={(e) => updateContract(index, "lease_end_date", e.target.value)} style={{ fontSize: 13 }} />
                  </div>
                </div>

                <div style={{ display: "flex", gap: 8, flexWrap: "wrap", marginBottom: 8 }}>
                  <div style={{ flex: "1 1 120px" }}>
                    <Text type="secondary" style={{ fontSize: 11 }}>{t("ai.draft_fixed_rent", language)}</Text>
                    <Input size="small" value={contract.fixed_rent_amount} onChange={(e) => updateContract(index, "fixed_rent_amount", parseFloat(e.target.value) || 0)} style={{ fontSize: 13 }} />
                  </div>
                  <div style={{ flex: "1 1 120px" }}>
                    <Text type="secondary" style={{ fontSize: 11 }}>{t("ai.draft_payment_timing", language)}</Text>
                    <Input size="small" value={contract.payment_timing} onChange={(e) => updateContract(index, "payment_timing", e.target.value)} style={{ fontSize: 13 }} />
                  </div>
                  <div style={{ flex: "1 1 120px" }}>
                    <Text type="secondary" style={{ fontSize: 11 }}>{t("ai.draft_discount_rate", language)}</Text>
                    <Input size="small" value={contract.discount_rate} onChange={(e) => updateContract(index, "discount_rate", parseFloat(e.target.value) || 0)} style={{ fontSize: 13 }} status={!contract.discount_rate ? "warning" : ""} />
                  </div>
                </div>

                <div style={{ display: "flex", gap: 8, flexWrap: "wrap", marginBottom: 8 }}>
                  <div style={{ flex: "1 1 140px" }}>
                    <Text type="secondary" style={{ fontSize: 11 }}>资产类型</Text>
                    <Input size="small" value={contract.asset_type || "real_estate"} onChange={(e) => updateContract(index, "asset_type", e.target.value)} style={{ fontSize: 13 }} />
                  </div>
                  <div style={{ flex: "1 1 160px" }}>
                    <Text type="secondary" style={{ fontSize: 11 }}>租赁范围</Text>
                    <Input size="small" value={contract.lease_scope || contract.suggested_scope || "in_scope"} onChange={(e) => updateContract(index, "lease_scope", e.target.value)} style={{ fontSize: 13 }} status={!contract.lease_scope && !contract.suggested_scope ? "warning" : ""} />
                  </div>
                  <div style={{ flex: "1 1 140px" }}>
                    <Text type="secondary" style={{ fontSize: 11 }}>范围置信度</Text>
                    <Input size="small" value={contract.scope_confidence ?? ""} onChange={(e) => updateContract(index, "scope_confidence", parseFloat(e.target.value) || 0)} style={{ fontSize: 13 }} status={(contract.scope_confidence ?? 1) < 0.8 ? "warning" : ""} />
                  </div>
                  <div style={{ flex: "1 1 180px" }}>
                    <Text type="secondary" style={{ fontSize: 11 }}>豁免/排除原因</Text>
                    <Input size="small" value={contract.exemption_reason || ""} onChange={(e) => updateContract(index, "exemption_reason", e.target.value)} style={{ fontSize: 13 }} />
                  </div>
                </div>

                <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
                  {hasLowConfidence(contract) && (
                    <Tag color="warning" style={{ fontSize: 11 }}>
                      {t("ai.draft_confidence", language, { value: String((contract.confidence * 100).toFixed(0)) })}
                    </Tag>
                  )}
                  {contract.lease_scope && (
                    <Tag color={contract.lease_scope === "in_scope" ? "blue" : "orange"} style={{ fontSize: 11 }}>
                      Scope: {contract.lease_scope}
                    </Tag>
                  )}
                  {contract.missing_fields.length > 0 && (
                    <Tag color="error" style={{ fontSize: 11 }}>
                      {t("ai.draft_missing_fields", language, { fields: contract.missing_fields.join(", ") })}
                    </Tag>
                  )}
                  {contract.warnings.slice(0, 2).map((warning, warningIndex) => (
                    <Text key={warningIndex} style={{ fontSize: 11, color: "#CF1322" }}>
                      {warning}
                    </Text>
                  ))}
                </div>
              </div>
            </div>
          </div>
        ))}
      </div>

      <div style={{ padding: "12px 16px", background: "#FAFAFA", borderTop: "1px solid #E5E5E5", display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <Text type="secondary" style={{ fontSize: 12 }}>
          {t("ai.draft_selected_count", language, { selected: String(selectedIndices.size), total: String(editedContracts.length) })}
        </Text>
        <Button type="primary" loading={creating} disabled={selectedIndices.size === 0} onClick={handleConfirm} style={{ background: "#000", borderColor: "#000" }}>
          {t("ai.draft_confirm_import", language)}
        </Button>
      </div>
    </div>
  );
}
