/**
 * HOME-001: responsive contract for the three-column home page.
 *
 * DESIGN.md §5.2 — the mobile breakpoint is 768: below it the layout
 * degrades to a single brief column plus a to-dos Drawer; at and above it
 * the three columns (nav | brief | to-dos) are shown.
 */
export const HOME_RIGHT_DRAWER_BREAKPOINT = 768;

export type HomeResponsiveState = {
  /** >= 768: nav | brief | to-dos columns all render inline. */
  threeColumn: boolean;
  /** < 768: the to-dos column lives in a Drawer, not inline. */
  rightAsDrawer: boolean;
};

export function getHomeResponsiveState(width: number): HomeResponsiveState {
  const threeColumn = Number.isFinite(width) && width >= HOME_RIGHT_DRAWER_BREAKPOINT;
  return { threeColumn, rightAsDrawer: !threeColumn };
}
