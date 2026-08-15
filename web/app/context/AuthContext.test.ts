/**
 * FIX-022: corrupted localStorage user must never leave the app stuck on
 * "loading…". The provider treats parse failure as logged out.
 *
 * Source-level contract (no jsdom in this repo): the parse sits in a
 * try/catch whose catch clears both keys — regressing to a bare
 * JSON.parse(storedUser) fails this test.
 */
import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import path from "node:path";

const source = readFileSync(path.join(import.meta.dirname, "AuthContext.tsx"), "utf8");

describe("AuthContext localStorage recovery (FIX-022)", () => {
  it("wraps the user parse in try/catch", () => {
    const block = source.slice(source.indexOf("const storedToken"));
    expect(block).toMatch(/try \{[\s\S]*?JSON\.parse\(storedUser\)[\s\S]*?\} catch \{/);
  });

  it("the catch clears both keys instead of swallowing silently", () => {
    const catchBlock = source.slice(source.indexOf("} catch {"), source.indexOf("} catch {") + 600);
    expect(catchBlock).toContain('localStorage.removeItem("token")');
    expect(catchBlock).toContain('localStorage.removeItem("refresh_token")');
    expect(catchBlock).toContain('localStorage.removeItem("user")');
    // A silent swallow would leave the corrupted user in storage.
    expect(catchBlock).not.toContain("// no-op");
  });
});
