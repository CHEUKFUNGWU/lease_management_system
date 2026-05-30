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

// ─── Fade Variants ─────────────────────────────────────────────

export const fadeIn = {
  initial: { opacity: 0 },
  animate: { opacity: 1 },
  transition: { duration: 0.2, ease: "easeOut" },
};

export const fadeInUp = {
  initial: { opacity: 0, y: 4 },
  animate: { opacity: 1, y: 0 },
  transition: { duration: 0.25, ease: "easeOut" },
};

export const fadeInDown = {
  initial: { opacity: 0, y: -4 },
  animate: { opacity: 1, y: 0 },
  transition: { duration: 0.2, ease: "easeOut" },
};

export const fadeInScale = {
  initial: { opacity: 0, scale: 0.98 },
  animate: { opacity: 1, scale: 1 },
  transition: { duration: 0.2, ease: "easeOut" },
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

// ─── Card / Modal ──────────────────────────────────────────────

export const cardHover = {
  rest: {
    boxShadow: "0 0 0 1px rgba(0, 0, 0, 0.04)",
    y: 0,
  },
  hover: {
    boxShadow: "0 0 0 1px rgba(0, 0, 0, 0.06), 0 2px 8px rgba(0, 0, 0, 0.04)",
    y: -1,
    transition: { duration: 0.15, ease: [0.4, 0, 0.2, 1] },
  },
};

export const modalOverlay = {
  initial: { opacity: 0 },
  animate: { opacity: 1 },
  exit: { opacity: 0 },
  transition: { duration: 0.2 },
};

export const modalContent = {
  initial: { opacity: 0, scale: 0.98, y: 8 },
  animate: { opacity: 1, scale: 1, y: 0 },
  exit: { opacity: 0, scale: 0.98, y: 8 },
  transition: { duration: 0.25, ease: [0.4, 0, 0.2, 1] },
};

// ─── List Items ────────────────────────────────────────────────

export const listItem = {
  initial: { opacity: 0, x: -4 },
  animate: { opacity: 1, x: 0 },
  exit: { opacity: 0, x: 4 },
  transition: { duration: 0.15 },
};

// ─── Skeleton Shimmer ──────────────────────────────────────────

export const shimmerAnimation = {
  background: "linear-gradient(90deg, #F0F0F0 25%, #F7F7F7 50%, #F0F0F0 75%)",
  backgroundSize: "200% 100%",
  animation: "shimmer 1.5s infinite linear",
};

// ─── Toast / Notification ──────────────────────────────────────

export const toastSlide = {
  initial: { opacity: 0, y: -8, x: "-50%" },
  animate: { opacity: 1, y: 0, x: "-50%" },
  exit: { opacity: 0, y: -8, x: "-50%" },
  transition: { duration: 0.2, ease: "easeOut" },
};

// ─── Dropdown / Popover ────────────────────────────────────────

export const dropdownMenu = {
  initial: { opacity: 0, y: -2, scale: 0.98 },
  animate: { opacity: 1, y: 0, scale: 1 },
  exit: { opacity: 0, y: -2, scale: 0.98 },
  transition: { duration: 0.15, ease: "easeOut" },
};

// ─── Button Press ──────────────────────────────────────────────

export const buttonTap = {
  scale: 0.98,
  transition: { duration: 0.05 },
};

// ─── Status Change ─────────────────────────────────────────────

export const statusPulse = {
  animate: {
    scale: [1, 1.05, 1],
    transition: { duration: 0.3, ease: "easeOut" },
  },
};

// ─── Number Counter ────────────────────────────────────────────

export const countUp = {
  initial: { opacity: 0, y: 8 },
  animate: { opacity: 1, y: 0 },
  transition: { duration: 0.4, ease: [0.4, 0, 0.2, 1] },
};

// ─── Scroll Reveal ─────────────────────────────────────────────

export const scrollReveal = {
  initial: { opacity: 0, y: 12 },
  whileInView: { opacity: 1, y: 0 },
  viewport: { once: true, margin: "-40px" },
  transition: { duration: 0.3, ease: "easeOut" },
};
