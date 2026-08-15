import { describe, expect, it } from "vitest";
import { getHomeResponsiveState, HOME_RIGHT_DRAWER_BREAKPOINT } from "./responsive";

// HOME-001 H3: 1440×900 renders three columns; 390×844 degrades to a
// single brief column plus a to-dos Drawer. DESIGN.md §5.2 pins the mobile
// breakpoint at 768 — below it is the Drawer, at and above it is inline.
describe("getHomeResponsiveState", () => {
  it("renders three columns at desktop widths", () => {
    for (const width of [1440, 1280, 1024, 768]) {
      const state = getHomeResponsiveState(width);
      expect(state.threeColumn).toBe(true);
      expect(state.rightAsDrawer).toBe(false);
    }
  });

  it("degrades to a single column plus to-dos Drawer below 768", () => {
    for (const width of [767, 640, 390, 320]) {
      const state = getHomeResponsiveState(width);
      expect(state.threeColumn).toBe(false);
      expect(state.rightAsDrawer).toBe(true);
    }
  });

  it("treats the 768 boundary itself as desktop per DESIGN.md §5.2", () => {
    const below = getHomeResponsiveState(HOME_RIGHT_DRAWER_BREAKPOINT - 1);
    const at = getHomeResponsiveState(HOME_RIGHT_DRAWER_BREAKPOINT);
    expect(below.rightAsDrawer).toBe(true);
    expect(at.threeColumn).toBe(true);
  });

  it("never crashes on a non-finite width", () => {
    const state = getHomeResponsiveState(Number.NaN);
    expect(state.threeColumn).toBe(false);
    expect(state.rightAsDrawer).toBe(true);
  });
});
