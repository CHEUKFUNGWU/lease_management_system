import { describe, expect, it } from "vitest";

import { buildSchedulePayload } from "./forms";

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
