import { describe, expect, it } from "vitest";
import { safeInternalAIURL } from "./retailAI";

describe("retail AI evidence URL guard", () => {
  it("keeps server-created same-site paths only", () => {
    expect(safeInternalAIURL("/store-360?store_id=x")).toBe("/store-360?store_id=x");
    expect(safeInternalAIURL("/api/v1/retail/kpis/store-days?x=1")).toBe("/api/v1/retail/kpis/store-days?x=1");
    expect(safeInternalAIURL("https://evil.example")).toBeUndefined();
    expect(safeInternalAIURL("//evil.example")).toBeUndefined();
    expect(safeInternalAIURL("/javascript:alert(1)")).toBeUndefined();
    expect(safeInternalAIURL("/contracts?x=1")).toBeUndefined();
    expect(safeInternalAIURL(undefined)).toBeUndefined();
  });
});
