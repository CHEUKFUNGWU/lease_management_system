/**
 * M3 web export tests — the design doc's test face: workbook round-trip
 * (formula + cached result + native table), PPTX unpacked as native objects
 * (never images), CSV escaping parity with the server renderer, and the
 * CONTRACT-001 pin that extractor row keys match the Go descriptor columns.
 */
import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import path from "node:path";
import JSZip from "jszip";
import ExcelJS from "exceljs";
import {
  buildCSV, buildPPTX, buildXLSX, escapeCell, exportFilename, provenanceLines,
  setExportDescriptorCacheForTests, diagnosticsRowsFromResponse, pulseRowsFromResponse,
  scenarioRowsFromResponse, type ExportDescriptor, type ExportEnvelope,
} from "./retail-export";

const repoRoot = path.join(import.meta.dirname, "../../../");
const goExport = readFileSync(path.join(repoRoot, "core-service/internal/services/retailexport/retailexport.go"), "utf8");

const diagnosticsDescriptor: ExportDescriptor = {
  kind: "store_diagnostics",
  title: "门店360·指标摘要",
  columns: [
    { key: "metric", header: "指标" },
    { key: "unit", header: "单位" },
    { key: "current", header: "本期" },
    { key: "comparison", header: "对比期" },
    { key: "change", header: "变化", formula: { kind: "delta", source: ["current", "comparison"] } },
    { key: "status", header: "状态", sum: true },
  ],
};

const envelope: ExportEnvelope = {
  basis: "Working", dataClassification: "simulated", datasetVersion: "planA-v1",
  periodLabel: "近 14 天", asOf: "2026-08-16", formulaVersion: "retail-kpi-v1",
  sourceSystems: ["retail_simulator"], generatedAt: "2026-08-16T08:00:00Z",
};

const numericRows = [
  { metric: "营业额", unit: "CNY", current: 120, comparison: 80, change: null, status: "=SUM(A1:A9)" },
  { metric: "毛利", unit: "CNY", current: 36, comparison: 24, change: null, status: "+evil()" },
  { metric: "客流", unit: "count", current: 300, comparison: 400, change: null, status: "ok" },
];

describe("M3 web export", () => {
  it("CSV carries BOM, provenance and escapes =+-@ like the server", () => {
    const csv = buildCSV(diagnosticsDescriptor, envelope, numericRows);
    expect(csv.startsWith("﻿")).toBe(true);
    expect(csv).toContain("模拟数据（simulated）");
    expect(csv).toContain("period=近 14 天");
    expect(csv).toContain("'=SUM(A1:A9)");
    expect(csv).toContain("'+evil()");
    expect(csv).not.toContain(",ok\r\n'=");
    expect(escapeCell("-1+1")).toBe("'-1+1");
    expect(escapeCell("normal")).toBe("normal");
  });

  it("XLSX round-trip: formula + cached result, native table, escaping", async () => {
    const buffer = await buildXLSX(diagnosticsDescriptor, envelope, numericRows);
    const workbook = new ExcelJS.Workbook();
    await workbook.xlsx.load(buffer);
    const sheet = workbook.worksheets[0];
    // Provenance banner
    expect(sheet.getCell("A1").value).toContain("模拟数据（simulated）");
    // Header row at the frozen boundary (3 banner rows + 1 blank)
    expect(sheet.getCell("A5").value).toBe("指标");
    // Delta formula column carries structured references AND cached result
    const deltaCell = sheet.getCell("E6");
    expect(String((deltaCell.value as ExcelJS.CellFormulaValue).formula)).toContain("Tstore_diagnostics[[#This Row],[本期]]");
    expect((deltaCell.value as ExcelJS.CellFormulaValue).result).toBe(40);
    // Injection cells escaped
    expect(sheet.getCell("F6").value).toBe("'=SUM(A1:A9)");
    // Native table registered; the totals row lands right after the data
    // as a SUBTOTAL cell over the sum column.
    const table = (sheet as unknown as { tables: Record<string, { ref: string; totalsRowCount?: number }> }).tables[`Tstore_diagnostics`];
    expect(table).toBeDefined();
    const totalsCell = sheet.getCell("F9");
    expect(String((totalsCell.value as ExcelJS.CellFormulaValue).formula)).toContain("SUBTOTAL(109");
    expect(sheet.views[0]).toMatchObject({ state: "frozen" });
  });

  it("PPTX unzips to native table/chart XML, never an image", async () => {
    const buffer = await buildPPTX(diagnosticsDescriptor, envelope, numericRows);
    const zip = await JSZip.loadAsync(buffer);
    const slideXML = await zip.file("ppt/slides/slide1.xml")!.async("string");
    expect(slideXML).toContain("<a:tbl>"); // native editable table
    expect(slideXML).toContain("graphicFrame");
    const media = Object.keys(zip.files).filter((name) => name.startsWith("ppt/media/") && !zip.files[name].dir);
    expect(media).toEqual([]); // no screenshots
  });

  it("filenames carry the working/simulated mark", () => {
    expect(exportFilename("store_diagnostics", "simulated", "2026-07", "csv")).toMatch(/^store_diagnostics_simulated_2026-07_/);
    expect(exportFilename("operating_pulse", "production", "近 14 天", "xlsx")).toMatch(/^operating_pulse_working_/);
  });

  it("provenance lines distinguish classifications", () => {
    expect(provenanceLines({ ...envelope, dataClassification: "production" })[0]).toContain("Working");
    expect(provenanceLines({ ...envelope, dataClassification: "mixed" })[0]).toContain("mixed");
  });

  it("P0-3: scenario attainment ratio formula + IF zero guard + cached result", async () => {
    const scenarioDescriptor: ExportDescriptor = {
      kind: "scenario", title: "租金谈判测算·情景对比",
      columns: [
        { key: "metric", header: "指标" }, { key: "unit", header: "单位" },
        { key: "baseline", header: "Baseline", sum: true }, { key: "plan", header: "方案", sum: true },
        { key: "delta", header: "差异", formula: { kind: "delta", source: ["plan", "baseline"] } },
        { key: "attainment", header: "达成率%", formula: { kind: "ratio", source: ["plan", "baseline"], scale: 100 } },
        { key: "status", header: "状态" },
      ],
    };
    const scenarioRows = scenarioRowsFromResponse({
      baseline: { metrics: { revenue: { result: 100, unit: "currency" }, margin: { result: 0, unit: "percent" } } },
      scenarios: [{ key: "plan", metrics: { revenue: { result: 120, unit: "currency" }, margin: { result: 5, unit: "percent" } } }],
    } as never, "plan");
    const buffer = await buildXLSX(scenarioDescriptor, { ...envelope, dataClassification: "production" }, scenarioRows);
    const workbook = new ExcelJS.Workbook();
    await workbook.xlsx.load(buffer);
    const sheet = workbook.worksheets[0];
    // attainment cell: formula carries the IF guard AND the *100 scale,
    // cached result = 120/100*100 = 120.
    const attainment = sheet.getCell("F6").value as ExcelJS.CellFormulaValue;
    expect(String(attainment.formula)).toContain("IF(ISERROR(");
    expect(String(attainment.formula)).toContain("*100");
    expect(attainment.result).toBe(120);
    // zero baseline: cached result must be null — the guard refused it.
    expect(sheet.getCell("F7").value).toBeNull();
  });

  it("CONTRACT-001: extractor row keys ⊆ Go descriptor columns (no second list)", async () => {
    setExportDescriptorCacheForTests({ store_diagnostics: diagnosticsDescriptor });
    // Diagnostics extractor keys from a mocked response
    const diagnosticsRows = diagnosticsRowsFromResponse({
      summary: {
        revenue: { current: { value: 120, unit: "CNY" }, comparison: { value: 80, unit: "CNY" }, change_value: 40, change_type: "percent", status: "complete" },
      },
    } as never);
    const allowed = new Set(diagnosticsDescriptor.columns.map((column) => column.key));
    for (const row of diagnosticsRows) {
      for (const key of Object.keys(row)) expect(allowed.has(key)).toBe(true);
    }
    // Pulse extractor keys ⊆ the Go operating_pulse descriptor whitelist
    const goColumns = Array.from(goExport.matchAll(/Key:\s+"([a-z_]+)"/g), (m) => m[1]);
    expect(goColumns.length).toBeGreaterThanOrEqual(12);
    const pulseKeys = ["rank", "identity", "brand_region", "signals", "score", "severity", "revenue", "revenue_change", "store_contribution", "contribution_change", "source_systems", "currency"];
    for (const key of pulseKeys) expect(goColumns, `Go pulse descriptor covers ${key}`).toContain(key);
    // Pulse extractor actually produces those keys
    const pulseRows = pulseRowsFromResponse({
      partitions: [],
      attention: [{
        rank: 1, store_id: "s1", store_code: "S1", store_name: "One", brand: "B", region: "R", currency: "CNY",
        score: 2, severity: "medium", observed_signals: [], current_kpis: { revenue: { value: 10 } }, comparison_kpis: { revenue: { value: 5 } },
        evidence: { source_systems: ["pos"] },
      }],
    } as never);
    expect(Object.keys(pulseRows[0]).sort()).toEqual([...pulseKeys].sort());
    setExportDescriptorCacheForTests(null);
  });
});
