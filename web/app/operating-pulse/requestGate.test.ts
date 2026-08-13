import { describe, expect, it } from "vitest";
import { createLatestRequestGate } from "./requestGate";

describe("operating pulse latest request gate", () => {
  it("commits the newer deferred response and drops the older late response", async () => {
    const gate = createLatestRequestGate();
    let state = "initial";
    const oldRequestID = gate.begin();
    let resolveOld!: () => void;
    const oldResponse = new Promise<void>((resolve) => { resolveOld = resolve; });
    const newRequestID = gate.begin();
    let resolveNew!: () => void;
    const newResponse = new Promise<void>((resolve) => { resolveNew = resolve; });

    // The page's loadPulse path uses this same gate: the new response commits
    // first, then the deliberately late old response attempts to commit.
    resolveNew();
    await newResponse;
    expect(gate.commit(newRequestID, () => { state = "new-7"; })).toBe(true);
    resolveOld();
    await oldResponse;
    expect(gate.commit(oldRequestID, () => { state = "old-28"; })).toBe(false);
    expect(state).toBe("new-7");
  });
});
