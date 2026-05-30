/**
 * Design Tokens — IFRS 16 Monochrome System
 *
 * The entire visual language is built on a 12-level grayscale.
 * In a no-color system, hierarchy is created through:
 * 1. Contrast (darkness levels)
 * 2. Typography (weight, size, tracking)
 * 3. Spacing (density, breathing room)
 * 4. Depth (subtle borders vs shadows)
 * 5. Motion (timing, easing)
 */

// ─── Color Tokens ──────────────────────────────────────────────

export const colors = {
  // Monochrome scale — semantic names for clarity
  background: {
    page: "#FFFFFF",        // Page canvas
    surface: "#F7F7F7",     // Cards, panels, subtle elevation
    elevated: "#FFFFFF",    // Modal, dropdown — same as page but with shadow
    inset: "#F0F0F0",       // Table headers, secondary panels
    code: "#F7F7F7",        // Code blocks, diff backgrounds
  },

  foreground: {
    primary: "#000000",     // Headlines, primary actions, key data
    secondary: "#262626",   // Body text, important labels
    tertiary: "#595959",    // Descriptions, metadata
    muted: "#8C8C8C",       // Placeholders, disabled, hints
    inverse: "#FFFFFF",     // Text on dark backgrounds
  },

  border: {
    default: "#E5E5E5",     // Standard dividers, card borders
    strong: "#D9D9D9",      // Hover states, active borders
    subtle: "#F0F0F0",      // Internal dividers, table rows
    inverse: "rgba(255,255,255,0.1)", // Borders on dark elements
  },

  // State colors — extremely minimal, almost monochrome
  state: {
    success: "#000000",     // Use with check icon, bold weight
    warning: "#333333",     // Use with alert icon
    error: "#000000",       // Use with x icon, bold weight + underline
    info: "#595959",        // Use with info icon
  },
} as const;

// ─── Typography Tokens ─────────────────────────────────────────

export const typography = {
  fontFamily: {
    sans: '"Inter", -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Noto Sans SC", "PingFang SC", "Microsoft YaHei", sans-serif',
    mono: '"JetBrains Mono", "Fira Code", ui-monospace, SFMono-Regular, monospace',
  },

  // Scale: 8 levels, tight tracking for headings
  sizes: {
    display: { size: 32, lineHeight: 40, weight: 700, tracking: -0.04 },    // Page titles
    h1: { size: 24, lineHeight: 32, weight: 700, tracking: -0.03 },         // Section headers
    h2: { size: 18, lineHeight: 28, weight: 600, tracking: -0.02 },         // Card titles, tabs
    h3: { size: 15, lineHeight: 24, weight: 600, tracking: -0.01 },         // Subsection, form groups
    body: { size: 14, lineHeight: 22, weight: 400, tracking: 0 },           // Primary body
    bodySmall: { size: 13, lineHeight: 20, weight: 400, tracking: 0 },      // Secondary body
    caption: { size: 12, lineHeight: 16, weight: 500, tracking: 0.01 },     // Labels, badges
    micro: { size: 11, lineHeight: 14, weight: 500, tracking: 0.02 },       // Timestamps, metadata
  },

  // Weights used in the system
  weights: {
    normal: 400,
    medium: 500,
    semibold: 600,
    bold: 700,
  },
} as const;

// ─── Spacing Tokens ────────────────────────────────────────────

export const spacing = {
  // Base unit: 4px
  unit: 4,

  // Semantic spacing
  xs: 4,      // Micro gaps (icon + text)
  sm: 8,      // Tight gaps (inline elements)
  md: 12,     // Standard gaps (form fields, list items)
  lg: 16,     // Section padding (card internal)
  xl: 24,     // Component gaps (between cards)
  "2xl": 32,  // Section gaps (page sections)
  "3xl": 48,  // Major sections
  "4xl": 64,  // Page-level padding
} as const;

// ─── Depth / Elevation Tokens ──────────────────────────────────

export const depth = {
  // In a monochrome system, depth is achieved through:
  // 1. Borders (static, always present)
  // 2. Background shifts (subtle gray steps)
  // 3. Shadows (extremely subtle, only for floating elements)

  static: {
    border: "1px solid #E5E5E5",
    background: "#FFFFFF",
  },

  hover: {
    border: "1px solid #D9D9D9",
    shadow: "0 1px 2px rgba(0, 0, 0, 0.06)",
  },

  card: {
    border: "1px solid #E5E5E5",
    shadow: "0 0 0 1px rgba(0, 0, 0, 0.04), 0 2px 8px rgba(0, 0, 0, 0.04)",
  },

  dropdown: {
    border: "1px solid #E5E5E5",
    shadow: "0 0 0 1px rgba(0, 0, 0, 0.04), 0 4px 12px rgba(0, 0, 0, 0.06)",
  },

  modal: {
    border: "1px solid #E5E5E5",
    shadow: "0 0 0 1px rgba(0, 0, 0, 0.06), 0 8px 24px rgba(0, 0, 0, 0.08)",
    overlay: "rgba(0, 0, 0, 0.4)",
  },

  tooltip: {
    shadow: "0 0 0 1px rgba(0, 0, 0, 0.06), 0 4px 12px rgba(0, 0, 0, 0.08)",
  },
} as const;

// ─── Motion Tokens ─────────────────────────────────────────────

export const motion = {
  // Durations
  instant: 0,
  fast: 100,      // Hover states, color changes
  normal: 150,    // Standard transitions
  slow: 250,      // Layout changes, page transitions
  slower: 350,    // Modal open/close
  slowest: 500,   // Major state changes

  // Easing curves
  easing: {
    micro: "cubic-bezier(0.4, 0, 0.2, 1)",      // Standard UI transitions
    enter: "cubic-bezier(0, 0, 0.2, 1)",        // Elements appearing
    exit: "cubic-bezier(0.4, 0, 1, 1)",          // Elements disappearing
    bounce: "cubic-bezier(0.34, 1.56, 0.64, 1)", // Playful micro-interactions
    linear: "linear",                             // Progress, skeletons
  },

  // Preset animations (for framer-motion or CSS)
  presets: {
    fadeIn: { opacity: [0, 1], transition: { duration: 0.2, ease: "easeOut" } },
    fadeInUp: { opacity: [0, 1], y: [4, 0], transition: { duration: 0.25, ease: "easeOut" } },
    scaleIn: { opacity: [0, 1], scale: [0.98, 1], transition: { duration: 0.2, ease: "easeOut" } },
    slideInRight: { opacity: [0, 1], x: [8, 0], transition: { duration: 0.2, ease: "easeOut" } },
  },
} as const;

// ─── Border Radius Tokens ──────────────────────────────────────

export const radius = {
  none: 0,
  sm: 4,      // Tags, small buttons
  md: 6,      // Inputs, small cards
  lg: 8,      // Buttons, standard cards
  xl: 10,     // Modals, large cards
  "2xl": 12,  // Feature cards
  "3xl": 16,  // Hero elements
  full: 9999, // Pills, avatars
} as const;

// ─── Z-Index Scale ─────────────────────────────────────────────

export const zIndex = {
  base: 0,
  dropdown: 100,
  sticky: 200,
  overlay: 300,
  modal: 400,
  toast: 500,
  tooltip: 600,
} as const;

// ─── Layout Tokens ─────────────────────────────────────────────

export const layout = {
  sidebar: {
    width: 240,
    collapsedWidth: 64,
  },
  header: {
    height: 60,
  },
  content: {
    maxWidth: 1440,
    padding: 32,
  },
  breakpoints: {
    sm: 640,
    md: 768,
    lg: 1024,
    xl: 1280,
    "2xl": 1440,
    "3xl": 1920,
  },
} as const;
