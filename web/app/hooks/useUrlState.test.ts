import { describe, expect, it } from "vitest";
import { updateUrlStateBuffer } from "./useUrlState";

describe("useUrlState shared query buffer", () => {
  it("composes setters for different keys fired in the same tick", () => {
    const pathname = "/contracts-test-shared-buffer";
    const initialQuery = "";

    updateUrlStateBuffer(pathname, initialQuery, "status", "", "approved");
    const nextUrl = updateUrlStateBuffer(pathname, initialQuery, "risk", "", "discount_rate_missing");

    const params = new URL(nextUrl, "http://localhost").searchParams;
    expect(params.get("status")).toBe("approved");
    expect(params.get("risk")).toBe("discount_rate_missing");
  });

  it("clears all filters when consecutive setters share one snapshot", () => {
    const pathname = "/contracts-test-clear";
    const initialQuery = "status=approved&risk=discount_rate_missing&lease_scope=in_scope";

    updateUrlStateBuffer(pathname, initialQuery, "status", "", "");
    updateUrlStateBuffer(pathname, initialQuery, "risk", "", "");
    const nextUrl = updateUrlStateBuffer(pathname, initialQuery, "lease_scope", "", "");

    expect(nextUrl).toBe(pathname);
  });
});
