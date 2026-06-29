"use client";

import { Card, Radio, Space } from "antd";
import { FileTextOutlined, SafetyOutlined } from "@ant-design/icons";
import { t, type Language } from "../../lib/i18n";

interface ReportModeSelectorProps {
  reportMode: "working" | "official";
  language: Language;
  onChange: (mode: "working" | "official") => void;
}

export function ReportModeSelector({
  reportMode,
  language,
  onChange,
}: ReportModeSelectorProps) {
  return (
    <Card style={{ marginBottom: 16 }}>
      <div
        style={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          flexWrap: "wrap",
          gap: 12,
        }}
      >
        <Space>
          <span
            style={{
              fontWeight: 600,
              fontSize: 14,
              color: "#000",
            }}
          >
            {t("reports.mode", language)}
          </span>
          <Radio.Group
            value={reportMode}
            onChange={(e) => onChange(e.target.value)}
            buttonStyle="solid"
          >
            <Radio.Button value="working">
              <FileTextOutlined /> {t("reports.working", language)}
            </Radio.Button>
            <Radio.Button value="official">
              <SafetyOutlined /> {t("reports.official", language)}
            </Radio.Button>
          </Radio.Group>
        </Space>

        <span
          style={{
            fontSize: 12,
            color: "#8C8C8C",
            display: "flex",
            alignItems: "center",
            gap: 4,
          }}
        >
          {reportMode === "working" ? (
            <>
              <span style={{ opacity: 0.5 }}>⚠</span>
              {t("reports.working_hint", language)}
            </>
          ) : (
            <>
              <span style={{ opacity: 0.5 }}>✓</span>
              {t("reports.official_hint", language)}
            </>
          )}
        </span>
      </div>
    </Card>
  );
}
