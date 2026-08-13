import { describe, expect, it } from "vitest";

import { AI_CHAT_SESSION_ITEM_CLASS, AI_CHAT_SESSION_MORE_CLASS, AI_CHAT_SESSION_ROW_CLASS, getAIChatResponsiveState, getAIChatSessionButtonProps, getAIChatSessionRowProps, isAIChatMobileViewport, transitionAIChatDrawer } from "./responsive";

describe("AI Chat responsive contract", () => {
  it.each([390, 767])("uses the mobile drawer at %ipx", (width) => {
    expect(isAIChatMobileViewport(width)).toBe(true);
  });

  it.each([768, 1440])("keeps the desktop sidebar at %ipx", (width) => {
    expect(isAIChatMobileViewport(width)).toBe(false);
  });

  it("does not classify non-finite measurements as mobile", () => {
    expect(isAIChatMobileViewport(Number.NaN)).toBe(false);
    expect(isAIChatMobileViewport(Number.POSITIVE_INFINITY)).toBe(false);
  });

  it.each([
    [390, true],
    [767, true],
    [768, false],
    [1440, false],
  ])("publishes one sidebar/trigger contract at %ipx", (width, mobile) => {
    const state = getAIChatResponsiveState(width);
    expect(state.isMobile).toBe(mobile);
    expect(state.showDesktopSidebar).toBe(!mobile);
    expect(state.showMobileSessionTrigger).toBe(mobile);
  });

  it.each(["selection", "new", "close"] as const)("closes and requests focus return after %s", (event) => {
    expect(transitionAIChatDrawer(true, event)).toEqual({ open: false, restoreFocus: true });
  });

  it("opens without moving focus when the trigger opens the drawer", () => {
    expect(transitionAIChatDrawer(false, "open")).toEqual({ open: true, restoreFocus: false });
  });

  it.each([true, false])( "binds active=%s session rows and controls to the rendered contract", (active) => {
    expect(AI_CHAT_SESSION_ITEM_CLASS).toBe("ai-chat-session-item");
    expect(AI_CHAT_SESSION_ROW_CLASS).toBe("ai-chat-session-row");
    expect(AI_CHAT_SESSION_MORE_CLASS).toBe("session-more-btn");
    expect(getAIChatSessionRowProps(active)).toEqual({
      className: active ? "ai-chat-session-row is-active" : "ai-chat-session-row",
      "data-active": active ? "true" : "false",
    });
    expect({ ...getAIChatSessionButtonProps(true), selector: AI_CHAT_SESSION_ITEM_CLASS }).toEqual({
      type: "button",
      "aria-pressed": true,
      selector: "ai-chat-session-item",
    });
    expect({ type: "button", selector: AI_CHAT_SESSION_MORE_CLASS }).toEqual({
      type: "button",
      selector: "session-more-btn",
    });
  });
});
