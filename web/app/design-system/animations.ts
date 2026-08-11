/**
 * Animation Utilities — Framer Motion variants and CSS animation helpers
 * for the monochrome IFRS 16 system.
 *
 * All animations are subtle and purposeful:
 * - No bounce unless explicitly playful
 * - No long durations (max 500ms)
 * - Easing curves are smooth, not flashy
 */

// ─── Page Transition ───────────────────────────────────────────

export const pageTransition = {
  initial: { opacity: 0, y: 4 },
  animate: { opacity: 1, y: 0 },
  exit: { opacity: 0, y: -4 },
  transition: {
    duration: 0.25,
    ease: [0.4, 0, 0.2, 1],
  },
};

// ─── Stagger Children ──────────────────────────────────────────

export const staggerContainer = {
  animate: {
    transition: {
      staggerChildren: 0.04,
      delayChildren: 0.05,
    },
  },
};

export const staggerItem = {
  initial: { opacity: 0, y: 4 },
  animate: { opacity: 1, y: 0 },
  transition: { duration: 0.2, ease: "easeOut" },
};
