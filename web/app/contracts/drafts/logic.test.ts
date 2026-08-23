import { describe, expect, it } from "vitest";
import {
  assembleDraftFieldRows,
  classificationLabelKey,
  formatDraftValue,
  lowConfidenceBlockers,
  type DraftFieldRow,
} from "./logic";

const language = "zh-CN" as const;

describe("draftreview field assembly", () => {
  const detail = {
    ai_values: {
      contract_number: "CT-2026-001",
      lessee_name: "承租方甲",
      area_sqm: 120.5,
      renewal_option: true,
      commencement_date: "2026-01-01T00:00:00Z",
      fixed_rent_amount: "",
      custom_extra: "x",
    },
    human_values: { lessee_name: "承租方甲（更正）" },
    confirmed_fields: ["lessee_name"],
    confidence_scores: { contract_number: 0.95, lessee_name: 0.42 },
  };

  it("orders known fields first, unknown fields after, drops human_edits", () => {
    const rows = assembleDraftFieldRows(
      { ...detail, ai_values: { ...detail.ai_values, human_edits: { lessee_name: {} } } },
      language,
    );
    expect(rows[0].field).toBe("contract_number");
    expect(rows.map((row) => row.field)).not.toContain("human_edits");
    // unknown keys sort to the back: custom_extra < renewal_option
    expect(rows[rows.length - 1].field).toBe("renewal_option");
  });

  it("carries AI value and human value side by side (D-B9), plus confirmation", () => {
    const rows = assembleDraftFieldRows(detail, language);
    const lessee = rows.find((row) => row.field === "lessee_name");
    expect(lessee?.aiValue).toBe("承租方甲");
    expect(lessee?.humanValue).toBe("承租方甲（更正）");
    expect(lessee?.confirmed).toBe(true);
  });

  it("labels come from i18n when available, raw key otherwise", () => {
    const rows = assembleDraftFieldRows(detail, language);
    expect(rows.find((row) => row.field === "contract_number")?.label).toBe("合同编号");
    expect(rows.find((row) => row.field === "custom_extra")?.label).toBe("custom_extra");
  });
});

describe("formatDraftValue", () => {
  it("renders missing as an em dash, never as 0 (DESIGN §13-9)", () => {
    expect(formatDraftValue("area_sqm", null, language)).toBe("—");
    expect(formatDraftValue("area_sqm", undefined, language)).toBe("—");
    expect(formatDraftValue("store_address", "", language)).toBe("—");
  });

  it("truncates dates to the day part", () => {
    expect(formatDraftValue("commencement_date", "2026-01-01T00:00:00Z", language)).toBe("2026-01-01");
  });

  it("booleans localize, numbers stringify", () => {
    expect(formatDraftValue("renewal_option", true, language)).toBe("是");
    expect(formatDraftValue("renewal_option", false, "en")).toBe("No");
    expect(formatDraftValue("fixed_rent_amount", 1200, language)).toBe("1200");
  });
});

describe("lowConfidenceBlockers (hint layer; server Decide is the control)", () => {
  const rows: DraftFieldRow[] = [
    { field: "a", label: "甲字段", aiValue: "x", confidence: 0.42, confirmed: false },
    { field: "b", label: "乙字段", aiValue: "x", confidence: 0.55, confirmed: true },
    { field: "c", label: "丙字段", aiValue: "x", confidence: 0.95, confirmed: false },
    { field: "d", label: "丁字段", aiValue: "x", confidence: undefined, confirmed: false },
  ];

  it("blocks only low-confidence AND unconfirmed fields", () => {
    expect(lowConfidenceBlockers(rows)).toEqual(["甲字段"]);
  });

  it("empty list when everything is confirmed or confident", () => {
    const unblocked = rows.map((row) => ({ ...row, confidence: 0.99 }));
    expect(lowConfidenceBlockers(unblocked)).toEqual([]);
  });
});

describe("classification labels", () => {
  it("maps all three classifications, defaulting unknowns to production wording", () => {
    expect(classificationLabelKey("production")).toBe("trust.classification_production");
    expect(classificationLabelKey("simulated")).toBe("trust.classification_simulated");
    expect(classificationLabelKey("mixed")).toBe("trust.classification_mixed");
    expect(classificationLabelKey(undefined)).toBe("trust.classification_production");
  });
});
