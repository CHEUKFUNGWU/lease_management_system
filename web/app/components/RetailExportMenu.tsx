"use client";

import { useState } from "react";
import { Button, Dropdown, message } from "antd";
import { DownloadOutlined } from "@ant-design/icons";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { t } from "../lib/i18n";
import { apiErrorMessage } from "../lib/api";
import {
  buildCSV, buildPPTX, buildXLSX, downloadExportFile, exportFilename,
  fetchExportDescriptors, type ExportDescriptor, type ExportEnvelope, type ExportRow,
} from "../lib/retail-export";

/**
 * M3 export menu (design §5): one dropdown per page — CSV (server
 * authoritative when the source read is a GET), formula-bearing XLSX
 * (ExcelJS) and editable PPTX (pptxgenjs) built from the page's controlled
 * response plus the shared descriptor. No page owns a column list.
 */
export function RetailExportMenu({ kind, disabled, envelope, rows, csvDownload, buttonLabel }: {
  kind: "operating_pulse" | "store_diagnostics" | "scenario";
  disabled: boolean;
  envelope: ExportEnvelope | null;
  rows: () => ExportRow[];
  /** Server CSV download for GET-based reads; omit for POST-based (scenario)
   * where CSV is built client-side from the same response. */
  csvDownload?: () => Promise<{ filename: string; blob: Blob }>;
  buttonLabel?: string;
}) {
  const { token } = useAuth();
  const { language } = useLanguage();
  const [busy, setBusy] = useState(false);

  const run = async (format: "csv" | "xlsx" | "pptx") => {
    if (!token) return;
    setBusy(true);
    try {
      if (format === "csv" && csvDownload) {
        const { filename, blob } = await csvDownload();
        const text = await blob.text();
        downloadExportFile(filename, text, "text/csv; charset=utf-8");
        return;
      }
      const descriptors = await fetchExportDescriptors(token);
      const descriptor: ExportDescriptor | undefined = descriptors[kind];
      if (!descriptor || !envelope) throw new Error("export descriptor unavailable");
      const exportRows = rows();
      if (format === "csv") {
        downloadExportFile(exportFilename(kind, envelope.dataClassification, envelope.periodLabel, "csv"), buildCSV(descriptor, envelope, exportRows), "text/csv; charset=utf-8");
      } else if (format === "xlsx") {
        const buffer = await buildXLSX(descriptor, envelope, exportRows);
        downloadExportFile(exportFilename(kind, envelope.dataClassification, envelope.periodLabel, "xlsx"), buffer, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet");
      } else {
        const buffer = await buildPPTX(descriptor, envelope, exportRows);
        downloadExportFile(exportFilename(kind, envelope.dataClassification, envelope.periodLabel, "pptx"), buffer, "application/vnd.openxmlformats-officedocument.presentationml.presentation");
      }
    } catch (err) {
      message.error(apiErrorMessage(err));
    } finally {
      setBusy(false);
    }
  };

  return <Dropdown
    trigger={["click"]}
    menu={{ items: [
      { key: "csv", label: csvDownload ? t("retail_export.csv_server", language) : t("retail_export.csv", language) },
      { key: "xlsx", label: t("retail_export.xlsx", language) },
      { key: "pptx", label: t("retail_export.pptx", language) },
    ], onClick: ({ key }) => { void run(key as "csv" | "xlsx" | "pptx"); } }}
    disabled={disabled || busy}
  >
    <Button icon={<DownloadOutlined />} loading={busy} disabled={disabled} className="retail-export-menu">{buttonLabel || t("retail_export.menu", language)}</Button>
  </Dropdown>;
}
