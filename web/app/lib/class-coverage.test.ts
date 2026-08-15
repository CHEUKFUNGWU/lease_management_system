/**
 * FIX-034: every className the app hands to a plain element must resolve to a
 * rule somewhere.
 *
 * STY-005 removed 44 static inline styles and replaced them with class names —
 * and never wrote the classes. `enforce-design` only checks that inline styles
 * are gone, the 279 tests never render, so the suite stayed green while both
 * trend charts vanished (a height-less parent gives ResponsiveContainer nothing
 * to lay out against) and every card gap collapsed to zero.
 *
 * The guard is deliberately dumb: a class name in source, a matching selector
 * in a stylesheet. It cannot tell you the rule is *correct* — that is what the
 * measured checks in chatLayout / kpi-card-height do — but it makes "the
 * replacement was never written" impossible to ship again.
 */
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

const appDir = join(import.meta.dirname, "..");

function walk(dir: string): string[] {
  return readdirSync(dir).flatMap((entry) => {
    const full = join(dir, entry);
    if (entry === "node_modules" || entry === ".next") return [];
    if (statSync(full).isDirectory()) return walk(full);
    return /\.tsx$/.test(entry) ? [full] : [];
  });
}

/** Styles we do not own: AntD's own classes and CSS-module output. */
function isForeign(cls: string): boolean {
  return cls.startsWith("ant-") || cls.startsWith("css-") || cls.includes("_");
}

/**
 * Pre-existing style-less classes, listed rather than ignored.
 *
 * These predate FIX-034 and are structural markers or wrappers that inherit
 * everything from a parent rule — none of them is the "replacement was never
 * written" failure this guard exists for. They are named here so the list can
 * only shrink: adding to it should require saying why in review.
 */
const KNOWN_MARKERS = new Set([
  "is-assistant",             // modifier; only .is-user carries styling
  "ai-confidence-text",
  "ai-thinking-trace",
  "chat-input-wrapper",
  "help-section",
  "help-flow-diagram",
  "home-band-attention-signal",
  "scenario-workbench-page",
]);

describe("FIX-034: class names resolve to rules", () => {
  const css = readFileSync(join(appDir, "globals.css"), "utf8");

  it("no className in the app is left without a stylesheet rule", () => {
    const missing = new Map<string, string>();
    for (const file of walk(appDir)) {
      if (/\.test\.tsx$/.test(file)) continue; // tests assert on class names, they do not render
      const source = readFileSync(file, "utf8");
      for (const match of Array.from(source.matchAll(/className="([a-zA-Z0-9_ -]+)"/g))) {
        for (const cls of match[1].split(/\s+/).filter(Boolean)) {
          if (isForeign(cls) || KNOWN_MARKERS.has(cls)) continue;
          if (new RegExp(`\\.${cls}(?![\\w-])`).test(css)) continue;
          if (!missing.has(cls)) missing.set(cls, file.slice(appDir.length + 1));
        }
      }
    }
    const report = Array.from(missing).map(([cls, file]) => `  ${cls}  (${file})`).join("\n");
    expect(missing.size, `these classes have no rule — the element renders unstyled:\n${report}`).toBe(0);
  });
});
