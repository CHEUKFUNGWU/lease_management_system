export const AI_CHAT_MOBILE_BREAKPOINT = 767;
export const AI_CHAT_SESSION_ITEM_CLASS = "ai-chat-session-item";
export const AI_CHAT_SESSION_ROW_CLASS = "ai-chat-session-row";
export const AI_CHAT_SESSION_MORE_CLASS = "session-more-btn";

export function getAIChatSessionButtonProps(active: boolean) {
  return { type: "button" as const, "aria-pressed": active };
}

export function getAIChatSessionRowProps(active: boolean) {
  return {
    className: `${AI_CHAT_SESSION_ROW_CLASS}${active ? " is-active" : ""}`,
    "data-active": active ? "true" : "false",
  };
}

/** Responsive contract shared by the page and boundary tests. */
export function isAIChatMobileViewport(width: number): boolean {
  return Number.isFinite(width) && width <= AI_CHAT_MOBILE_BREAKPOINT;
}

export type AIChatResponsiveState = {
  isMobile: boolean;
  showDesktopSidebar: boolean;
  showMobileSessionTrigger: boolean;
};

/**
 * Keep the page's responsive branches as one small, testable contract.  The
 * Desktop keeps the original sidebar while mobile exposes the session trigger.
 */
export function getAIChatResponsiveState(width: number): AIChatResponsiveState {
  const isMobile = isAIChatMobileViewport(width);
  return {
    isMobile,
    showDesktopSidebar: !isMobile,
    showMobileSessionTrigger: isMobile,
  };
}

export type AIChatDrawerEvent = "open" | "selection" | "new" | "close";

export type AIChatDrawerTransition = {
  open: boolean;
  restoreFocus: boolean;
};

/** State transition used by the page and keyboard-focused drawer tests. */
export function transitionAIChatDrawer(
  currentOpen: boolean,
  event: AIChatDrawerEvent,
): AIChatDrawerTransition {
  if (event === "open") return { open: true, restoreFocus: false };
  if (event === "selection" || event === "new" || event === "close") {
    return { open: false, restoreFocus: currentOpen };
  }
  return { open: currentOpen, restoreFocus: false };
}
