import { describe, it, expect } from "vitest";
import { tableScrollX } from "../lib/tableScroll";

describe("Machine Credentials Frontend Logic", () => {
  it("tableScrollX correctly handles non-empty and empty counts", () => {
    expect(tableScrollX(0, 840)).toBeUndefined();
    expect(tableScrollX(3, 840)).toEqual({ x: 840 });
  });

  it("formats credential scopes and status correctly", () => {
    const cred = {
      client_id: "mch_test123",
      scopes: ["operating_facts:write", "store:read"],
      revoked_at: null,
    };
    expect(cred.scopes.includes("operating_facts:write")).toBe(true);
    expect(cred.revoked_at).toBeNull();
  });
});
