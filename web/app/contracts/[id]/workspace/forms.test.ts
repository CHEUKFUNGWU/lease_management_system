import { describe, expect, it } from "vitest";

import { buildRevisionParameters, buildSchedulePayload } from "./forms";

const date = (value: string) => ({ format: () => value });

describe("contract workspace form mapping", () => {
  it("requires explicit payment timing instead of guessing accounting treatment", () => {
    expect(() => buildSchedulePayload("contract-1", {
      due_date: date("2026-01-01"),
      amount: 1000,
      payment_timing: "",
    })).toThrow("payment timing");
  });

  it("preserves explicit schedule accounting fields", () => {
    expect(buildSchedulePayload("contract-1", {
      effective_start_date: date("2026-01-01"),
      effective_end_date: date("2026-01-31"),
      coverage_start_date: date("2026-01-01"),
      coverage_end_date: date("2026-01-31"),
      due_date: date("2026-01-01"),
      amount: 1000,
      payment_timing: "prepaid",
      amount_type: "fixed_rent",
      currency: "CNY",
      is_fixed: true,
      is_lease_component: true,
      included_in_liability_pv: true,
    })).toMatchObject({
      contract_id: "contract-1",
      due_date: "2026-01-01",
      amount: 1000,
      payment_timing: "prepaid",
      is_fixed: true,
      is_lease_component: true,
      included_in_liability_pv: true,
    });
  });
});

describe("lease event clause mapping", () => {
  it("sends no clause when none was stated", () => {
    expect(buildRevisionParameters({ event_type: "rent_change" })).toBeUndefined();
  });

  it("sends only the fields belonging to the chosen clause", () => {
    // A stray percentage left over from switching clause type must not travel
    // with an index clause — it would be ambiguous which one the landlord wrote.
    const clause = buildRevisionParameters({
      revision_kind: "index",
      revision_base_index: "102.4",
      revision_new_index: "105.1",
      revision_cap: "2",
      revision_percentage: "5",
      revision_amount: "9999",
    });
    expect(clause).toEqual({
      kind: "index",
      base_index: 102.4,
      new_index: 105.1,
      cap_percentage: 2,
    });
  });

  it("omits an unstated cap rather than sending zero", () => {
    // "capped at 0%" is a real clause and a very different one, so an empty
    // field must not become a bound.
    const clause = buildRevisionParameters({
      revision_kind: "index",
      revision_base_index: 100,
      revision_new_index: 103,
      revision_cap: "",
    });
    expect(clause).not.toHaveProperty("cap_percentage");
    expect(clause).not.toHaveProperty("floor_percentage");
  });

  it("keeps a stated cap of zero", () => {
    const clause = buildRevisionParameters({
      revision_kind: "index",
      revision_base_index: 100,
      revision_new_index: 103,
      revision_cap: 0,
    });
    expect(clause).toMatchObject({ cap_percentage: 0 });
  });

  it("sends clause dates at the start of the day in UTC", () => {
    const clause = buildRevisionParameters({
      revision_kind: "percentage",
      revision_percentage: 5,
      revision_applies_from: { format: () => "2026-07-01" },
    });
    expect(clause).toMatchObject({ applies_from: "2026-07-01T00:00:00Z" });
  });

  it("drops incomplete steps instead of sending a half-written ladder", () => {
    const clause = buildRevisionParameters({
      revision_kind: "stepped",
      revision_steps: [
        { from_date: { format: () => "2026-07-01" }, amount: "12000" },
        { from_date: null, amount: "13000" },
        undefined,
      ],
    });
    expect(clause).toEqual({
      kind: "stepped",
      steps: [{ from_date: "2026-07-01T00:00:00Z", amount: 12000 }],
    });
  });
});
