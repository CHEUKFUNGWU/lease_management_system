/**
 * M3 platform export — the web half (design doc §5). The Go side owns the
 * column/formula descriptor and the authoritative CSV; this module consumes
 * THE SAME descriptor (fetched from /retail/exports/descriptors) plus the
 * page's controlled response to build:
 *   - XLSX via ExcelJS: formula + cached result, native Excel Table with
 *     totalsRow and filter buttons, structured-reference formulas,
 *     = + - @ cell escaping, sanitized table/column names
 *   - PPTX via pptxgenjs: native editable text + table + chart objects
 *   - CSV client-side (scenario, whose source read is a POST): same
 *     provenance header and escaping as the server renderer
 * There is deliberately NO second column list here — every column comes from
 * the descriptor (CONTRACT-001 shape, pinned by retail-export.test.ts).
 */
import { API_BASE_URL, type RetailPulseResponse, type RetailStoreDiagnosticsResponse, type RetailScenarioResponse } from "./api";

export type ExportFormulaSpec = { kind: "sum" | "delta" | "ratio"; source?: string[]; scale?: number };
export type ExportColumnSpec = { key: string; header: string; formula?: ExportFormulaSpec; sum?: boolean };
export type ExportDescriptor = { kind: string; title: string; columns: ExportColumnSpec[] };

export type ExportEnvelope = {
  basis: string;
  dataClassification: string;
  datasetVersion: string;
  periodLabel: string;
  asOf: string;
  formulaVersion: string;
  sourceSystems: string[];
  generatedAt: string;
};

export type ExportRow = Record<string, string | number | null | undefined>;

let descriptorCache: Record<string, ExportDescriptor> | null = null;

export async function fetchExportDescriptors(token: string): Promise<Record<string, ExportDescriptor>> {
  if (descriptorCache) return descriptorCache;
  const baseUrl = (API_BASE_URL || "").replace(/\/$/, "");
  const response = await fetch(`${baseUrl}/api/v1/retail/exports/descriptors`, { headers: { Authorization: `Bearer ${token}` } });
  if (!response.ok) throw new Error(`export descriptors unavailable (${response.status})`);
  const payload = (await response.json()) as { data: Record<string, ExportDescriptor> };
  descriptorCache = payload.data;
  return descriptorCache;
}

/** Test hook — injects descriptors so the builders can run offline. */
export function setExportDescriptorCacheForTests(descriptors: Record<string, ExportDescriptor> | null) {
  descriptorCache = descriptors;
}

export function provenanceLines(envelope: ExportEnvelope): string[] {
  const classes = envelope.dataClassification || "unspecified";
  const dataset = envelope.datasetVersion || "—";
  const sources = envelope.sourceSystems.length ? envelope.sourceSystems.join(", ") : "—";
  const mark = classes === "simulated" ? "模拟数据（simulated）— 不得作为正式口径对外"
    : classes === "production" ? "Working（未经关账审计）"
    : "mixed — 口径混合，注意区分";
  return [
    `${envelope.basis} · ${mark}`,
    `data_classification=${classes} · dataset=${dataset} · period=${envelope.periodLabel} · as_of=${envelope.asOf}`,
    `formula=${envelope.formulaVersion} · sources=${sources} · generated_at=${envelope.generatedAt}`,
  ];
}

export function escapeCell(value: string | number | null | undefined): string {
  const text = value == null ? "" : String(value);
  const trimmed = text.replace(/^[ \t]+/, "");
  if (trimmed && ["=", "+", "-", "@"].includes(trimmed[0])) return `'${text}`;
  return text;
}

export function exportFilename(kind: string, classification: string, periodLabel: string, extension: string): string {
  const mark = classification === "simulated" ? "simulated" : "working";
  const period = (periodLabel || "period").replace(/[ /:近天]/g, (c) => (c === "近" || c === "天" ? "" : "-")).replace(/-+/g, "-");
  const stamp = new Date().toISOString().replace(/[-:]/g, "").replace(/\.\d{3}Z$/, "Z");
  return `${kind}_${mark}_${period || "period"}_${stamp}.${extension}`;
}

/** Client-side CSV with the same provenance header, BOM and escaping as the
 * server renderer (used where the source read is a POST, e.g. scenario). */
export function buildCSV(descriptor: ExportDescriptor, envelope: ExportEnvelope, rows: ExportRow[]): string {
  const lines: string[] = [];
  const quote = (value: string | number | null | undefined) => {
    const text = escapeCell(value);
    return /[",\n\r]/.test(text) ? `"${text.replace(/"/g, '""')}"` : text;
  };
  for (const line of provenanceLines(envelope)) lines.push(quote(line));
  lines.push(descriptor.columns.map((column) => quote(column.header)).join(","));
  for (const row of rows) lines.push(descriptor.columns.map((column) => quote(row[column.key])).join(","));
  return `\uFEFF${lines.join("\r\n")}\r\n`;
}

// Excel structured references escape these inside column names.
function escapeTableColumnName(name: string): string {
  return name.replace(/'/g, "''").replace(/([\[\]#])/g, "'$1");
}

function sanitizeTableName(kind: string): string {
  const cleaned = kind.replace(/[^A-Za-z0-9_]/g, "_").replace(/^_+|_+$/g, "");
  return `T${cleaned || "Export"}`;
}

function isNumericLike(value: string | number | null | undefined): value is number {
  return typeof value === "number" && Number.isFinite(value);
}

export async function buildXLSX(descriptor: ExportDescriptor, envelope: ExportEnvelope, rows: ExportRow[]): Promise<ArrayBuffer> {
  let ExcelJS: any;
  try {
    const excelMod = await import("exceljs");
    ExcelJS = excelMod.default || excelMod;
  } catch {
    const excelDist = await import("exceljs/dist/exceljs.min.js");
    ExcelJS = excelDist.default || excelDist;
  }
  const workbook = new ExcelJS.Workbook();
  workbook.creator = "retail workstation";
  const sheet = workbook.addWorksheet(descriptor.title.slice(0, 28) || "Export");

  // Provenance header rows span the table width as merged banner cells.
  const width = Math.max(descriptor.columns.length, 1);
  provenanceLines(envelope).forEach((line, index) => {
    const row = sheet.addRow([line]);
    sheet.mergeCells(row.number, 1, row.number, width);
    const cell = sheet.getCell(row.number, 1);
    cell.font = { bold: index === 0, size: 10, color: { argb: "FF4B5563" } };
  });
  sheet.addRow([]);

  // The native table owns its whole region (header + data + totals row);
  // ExcelJS writes those cells itself, so values go through addTable —
  // never pre-written and clobbered.
  const headerRowIndex = provenanceLines(envelope).length + 2;
  const tableName = sanitizeTableName(descriptor.kind);
  const cellValue = (value: string | number | null | undefined) => (isNumericLike(value) ? value : escapeCell(value));
  sheet.addTable({
    name: tableName,
    ref: `A${headerRowIndex}`,
    headerRow: true,
    totalsRow: descriptor.columns.some((column) => column.sum),
    style: { theme: "TableStyleMedium2", showRowStripes: true },
    columns: descriptor.columns.map((column) => ({
      name: column.header,
      filterButton: true,
      totalsRowFunction: column.sum ? "sum" : undefined,
    })),
    rows: rows.map((row) => descriptor.columns.map((column) => cellValue(row[column.key]))),
  });

  // Computed columns are overwritten into the table's data cells as
  // {formula, result}: structured references against the table, cached
  // results from the controlled response (ExcelJS never evaluates).
  const columnByName = new Map(descriptor.columns.map((column) => [column.key, column]));
  for (const column of descriptor.columns) {
    if (!column.formula) continue;
    const columnIndex = descriptor.columns.indexOf(column) + 1;
    const ref = (key: string) => `${tableName}[[#This Row],[${escapeTableColumnName(columnByName.get(key)?.header || key)}]]`;
    rows.forEach((row, index) => {
      const operands = (column.formula!.source || []).map((key) => Number(row[key]));
      const cell = sheet.getRow(headerRowIndex + 1 + index).getCell(columnIndex);
      if (column.formula!.kind === "delta" && operands.length === 2 && operands.every(Number.isFinite)) {
        cell.value = { formula: `${ref(column.formula!.source![0])}-${ref(column.formula!.source![1])}`, result: operands[0] - operands[1] };
      } else if (column.formula!.kind === "ratio" && operands.length === 2 && operands.every(Number.isFinite) && operands[1] !== 0) {
        const scale = column.formula!.scale || 1;
        const ratio = `(${ref(column.formula!.source![0])}/${ref(column.formula!.source![1])})${scale !== 1 ? `*${scale}` : ""}`;
        cell.value = { formula: `IF(ISERROR(${ref(column.formula!.source![0])}/${ref(column.formula!.source![1])}),"",${ratio})`, result: (operands[0] / operands[1]) * scale };
      } else {
        cell.value = null;
      }
    });
  }

  sheet.views = [{ state: "frozen", ySplit: headerRowIndex }];
  const buffer = await workbook.xlsx.writeBuffer();
  return buffer as ArrayBuffer;
}

function rowAt(rows: ExportRow[], index: number): ExportRow | undefined {
  return rows[index];
}

export async function buildPPTX(descriptor: ExportDescriptor, envelope: ExportEnvelope, rows: ExportRow[]): Promise<ArrayBuffer> {
  const pptxgen = (await import("pptxgenjs")).default;
  const pptx = new pptxgen();
  const slide = pptx.addSlide();
  slide.addText(descriptor.title, { x: 0.4, y: 0.3, fontSize: 22, bold: true });
  slide.addText(provenanceLines(envelope).join("\n"), { x: 0.4, y: 0.95, fontSize: 10, color: "555555" });

  // Native table — editable in PowerPoint, never a screenshot.
  const tableRows: Array<Array<{ text: string; options?: Record<string, unknown> }>> = [
    descriptor.columns.map((column) => ({ text: column.header, options: { bold: true, fill: { color: "E8EDF3" } } })),
    ...rows.slice(0, 14).map((row) => descriptor.columns.map((column) => ({ text: escapeCell(row[column.key]) || "—" }))),
  ];
  slide.addTable(tableRows, { x: 0.4, y: 1.5, w: 9.2, fontSize: 10, border: { type: "solid", pt: 0.5, color: "CCCCCC" } } as Parameters<typeof slide.addTable>[1]);

  // Native chart when at least two rows carry a numeric column (pptxgenjs
  // series shape: {name, labels, values}).
  const labelKey = descriptor.columns.find((column) => !column.formula && !rows.slice(0, 12).some((row) => isNumericLike(row[column.key])))?.key || descriptor.columns[0].key;
  const numericColumn = descriptor.columns.find((column) => rows.slice(0, 14).some((row) => isNumericLike(row[column.key])));
  if (numericColumn && rows.length >= 2) {
    const chartRows = rows.slice(0, 12);
    slide.addChart(pptx.ChartType.bar, [{
      name: numericColumn.header,
      labels: chartRows.map((row) => String(row[labelKey] ?? "")),
      values: chartRows.map((row) => Number(row[numericColumn.key] ?? 0)),
    }], { x: 0.4, y: 4.4, w: 9.2, h: 2.6, showLegend: false, showTitle: true, title: numericColumn.header } as Parameters<typeof slide.addChart>[2]);
  }

  const output = await pptx.write({ outputType: "arraybuffer" });
  return output as ArrayBuffer;
}

export function downloadExportFile(filename: string, content: ArrayBuffer | string, mimeType: string) {
  let blob: Blob;
  if (content instanceof ArrayBuffer) {
    blob = new Blob([content], { type: mimeType });
  } else {
    // For CSV files, prepend UTF-8 BOM so Excel opens with proper Chinese encoding
    const isCSV = mimeType.includes("csv") || filename.toLowerCase().endsWith(".csv");
    const data = isCSV && !content.startsWith("\uFEFF") ? `\uFEFF${content}` : content;
    blob = new Blob([data], { type: mimeType });
  }
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = filename;
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  URL.revokeObjectURL(url);
}

// ── Per-page row extractors (the only response-shape knowledge on the web
// side; the column list itself stays in the descriptor) ──

export function pulseRowsFromResponse(response: RetailPulseResponse): ExportRow[] {
  const rows: ExportRow[] = [];
  const partitions = response.partitions && response.partitions.length ? response.partitions : [{ attention: response.attention || [], currency: response.currency || "" }];
  for (const partition of partitions) {
    for (const item of partition.attention || []) {
      const grouped = item.group_by === "region" || item.group_by === "brand";
      rows.push({
        rank: item.rank,
        identity: grouped ? item.group_label : `${item.store_code} ${item.store_name}`,
        brand_region: grouped ? (item.group_by === "region" ? "按区域" : "按品牌") : `${item.brand} · ${item.region}`,
        signals: (item.observed_signals || []).map((signal) => signal.signal_code).join("、"),
        score: item.score,
        severity: item.severity,
        revenue: item.current_kpis?.revenue?.value ?? null,
        revenue_change: changePercent(item.current_kpis?.revenue?.value, item.comparison_kpis?.revenue?.value),
        store_contribution: item.current_kpis?.store_contribution?.value ?? null,
        contribution_change: changePercent(item.current_kpis?.store_contribution?.value, item.comparison_kpis?.store_contribution?.value),
        source_systems: (item.evidence?.source_systems || []).join(","),
        currency: item.currency,
      });
    }
  }
  return rows;
}

function changePercent(current?: number | null, comparison?: number | null): number | null {
  if (current == null || comparison == null || comparison === 0) return null;
  return ((current - comparison) / Math.abs(comparison)) * 100;
}

export function diagnosticsRowsFromResponse(response: RetailStoreDiagnosticsResponse): ExportRow[] {
  return Object.keys(response.summary || {}).sort().map((code) => {
    const metric = response.summary[code];
    return {
      metric: code,
      unit: metric.current?.unit || "",
      current: metric.current?.value ?? null,
      comparison: metric.comparison?.value ?? null,
      change: metric.change_value ?? null,
      status: metric.status,
    };
  });
}

export function scenarioRowsFromResponse(response: RetailScenarioResponse, selectedKey: string): ExportRow[] {
  const scenario = (response.scenarios || []).find((item) => item.key === selectedKey) || (response.scenarios || [])[0];
  if (!scenario) return [];
  const metrics = response.baseline?.metrics ? Object.keys(response.baseline.metrics) : [];
  return metrics.map((code) => {
    const baseline = response.baseline.metrics[code];
    const plan = scenario.metrics[code];
    const baselineValue = baseline?.result ?? null;
    const planValue = plan?.result ?? null;
    return {
      metric: code,
      unit: baseline?.unit || plan?.unit || "",
      baseline: baselineValue,
      plan: planValue,
      delta: plan?.delta ?? null,
      // attainment feeds the ratio formula with the IF-guard zero protection;
      // null when the denominator is missing or zero.
      attainment: typeof planValue === "number" && typeof baselineValue === "number" && baselineValue !== 0 ? (planValue / baselineValue) * 100 : null,
      status: plan?.status || "",
    };
  });
}

export function envelopeFromPulse(response: RetailPulseResponse): ExportEnvelope {
  return {
    basis: response.basis || "Working",
    dataClassification: response.data_classification || "",
    datasetVersion: response.dataset_version || "",
    periodLabel: response.period_label || `${response.current?.date_from || ""}~${response.current?.date_to || ""}`,
    asOf: response.current?.date_to || "",
    formulaVersion: response.formula_version || "",
    sourceSystems: response.source_systems || [],
    generatedAt: response.generated_at || "",
  };
}

export function envelopeFromDiagnostics(response: RetailStoreDiagnosticsResponse): ExportEnvelope {
  return {
    basis: response.basis || "Working",
    dataClassification: response.data_classification || "",
    datasetVersion: response.dataset_version || "",
    periodLabel: `${response.current?.date_from || ""}~${response.current?.date_to || ""}`,
    asOf: response.current?.date_to || "",
    formulaVersion: response.formula_version || "",
    sourceSystems: response.source_systems || [],
    generatedAt: response.generated_at || "",
  };
}

export function envelopeFromScenario(response: RetailScenarioResponse): ExportEnvelope {
  return {
    basis: "Working",
    dataClassification: response.data_classification || "",
    datasetVersion: response.dataset_version || "",
    periodLabel: `${response.evidence?.current?.date_from || ""}~${response.evidence?.current?.date_to || ""}`,
    asOf: response.evidence?.current?.date_to || "",
    formulaVersion: response.formula_version || "",
    sourceSystems: response.evidence?.source_systems || [],
    generatedAt: "",
  };
}
