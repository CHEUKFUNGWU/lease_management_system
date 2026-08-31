/**
 * Design Tokens — Monochrome System
 *
 * The entire visual language is built on a grayscale-first system.
 * Semantic color is reserved for state tags and is always paired with an icon
 * and text so color is never the only signal. Hierarchy is created through:
 * 1. Contrast (darkness levels)
 * 2. Typography (weight, size, tracking)
 * 3. Spacing (density, breathing room)
 * 4. Depth (subtle borders vs shadows)
 * 5. Motion (timing, easing)
 *
 * This file is the single source of truth for design tokens: Ant Design reads
 * it through theme.ts -> ThemeProvider, so the values have to live in TS.
 * The `:root` block in globals.css must mirror it. Changing one side only is
 * how the two drifted apart in the first place — see DESIGN.md section 1 for
 * the divergences that still need reconciling.
 *
 * Rules that govern how these tokens may be used: DESIGN.md
 */

// ─── Color Tokens ──────────────────────────────────────────────

export const colors = {
  // Neutral grayscale palette — quiet, editorial, and deliberately low-chroma.
  background: {
    page: "#FFFFFF",
    surface: "#FFFFFF",
    elevated: "#FFFFFF",
    inset: "#F7F7F7",
    code: "#FBFBFB",
    // Identity surface: dark enough to anchor the login mark without pure black.
    brandSlab: "#111111",
    onBrandSlab: "#FFFFFF",
  },

  foreground: {
    primary: "#111111",
    secondary: "#2F3437",
    tertiary: "#5C605D",
    muted: "#787774",
    inverse: "#FFFFFF",
  },

  border: {
    default: "#EAEAEA",
    strong: "#A4A6A2",
    subtle: "#F1F1F1",
    inverse: "rgba(255,255,255,0.12)",
  },

  // Semantic accents stay small: status dots, icons, and restrained labels.
  state: {
    success: "#5C7863",
    warning: "#826A38",
    error: "#9A5F5F",
    info: "#5A6F87",
  },

  status: {
    success: { bg: "#EEF3EE", text: "#45604C", border: "#CCD9CD" },
    processing: { bg: "#EEF1F4", text: "#465A6C", border: "#CCD6DF" },
    warning: { bg: "#FFFFFF", text: "#6B5A39", border: "#E2D7BB" },
    error: { bg: "#F6EEEE", text: "#744C4C", border: "#E3D0D0" },
    neutral: { bg: "#F7F7F7", text: "#5C605D", border: "#EAEAEA" },
  },

  // Legacy names stay for compatibility; their values follow the new neutral system.
  morandi: {
    slate: "#111111",
    cream: "#F7F7F7",
    sand: "#5C7863",
    greige: "#787774",
    terracotta: "#8A5D5D",
  },

  // Chart series — neutral first, with restrained positive / negative accents.
  chart: {
    blue: "#2F3437",
    purple: "#5C605D",
    primary: "#111111",
    accent: "#5C7863",
    secondary: "#787774",
    negative: "#8A5D5D",
    fill: "#EAEAEA",
  },

  // Brand mark values are graphic-specific, not UI semantics.
  brand: {
    frame: "#111111",
    bar: "#2F3437",
    arrow: "#787774",
    arrowHighlight: "#A4A6A2",
    ring: "#8D918C",
    hub: "#5C605D",
    node: "#2F3437",
    inverse: {
      frame: "#FFFFFF",
      bar: "#E5E7EB",
      arrow: "#F3F4F6",
      arrowHighlight: "#FFFFFF",
      ring: "#9CA3AF",
      hub: "#D1D5DB",
      node: "#FFFFFF",
    },
    // White-alpha overlays on the dark brand panel (login screen) — the
    // brand panel is a fixed dark surface in both themes, so these stay
    // white-based rather than following the theme's text colours.
    overlay: {
      badge: "rgba(255, 255, 255, 0.08)",
      badgeBorder: "rgba(255, 255, 255, 0.15)",
      point: "rgba(255, 255, 255, 0.38)",
      text: "rgba(255, 255, 255, 0.72)",
    },
  },

  // Dark surfaces remain explicit so admin and code areas can be themed independently.
  surface: {
    admin: "#2F3437",
    code: "#2F3437",
  },
} as const;

// ─── Typography Tokens ─────────────────────────────────────────

export const typography = {
  fontFamily: {
    display: '"Instrument Serif", "Iowan Old Style", "Baskerville", "Noto Serif SC", Georgia, serif',
    sans: '-apple-system, BlinkMacSystemFont, "Helvetica Neue", "SF Pro Display", "PingFang SC", "Noto Sans SC", "Microsoft YaHei", sans-serif',
    mono: '"SF Mono", "JetBrains Mono", ui-monospace, SFMono-Regular, Menlo, monospace',
  },

  // Scale: 8 levels, tight tracking for headings
  sizes: {
    display: { size: 28, lineHeight: 36, weight: 400, tracking: -0.02 },    // Editorial page titles
    h1: { size: 24, lineHeight: 32, weight: 600, tracking: -0.03 },         // Section headers
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
  },
} as const;

// ─── Depth / Elevation Tokens ──────────────────────────────────

export const depth = {
  // In a monochrome system, depth is achieved through:
  // 1. Borders (static, always present)
  // 2. Background shifts (subtle gray steps)
  // 3. Shadows (extremely subtle, only for floating elements)

  static: {
    shadow: "0 0 0 1px rgba(0, 0, 0, 0.04)",
    background: "#FFFFFF",
  },

  hover: {
    shadow: "0 1px 2px rgba(0, 0, 0, 0.06)",
  },

  card: {
    shadow: "0 0 0 1px rgba(0, 0, 0, 0.04), 0 2px 8px rgba(0, 0, 0, 0.04)",
  },

  dropdown: {
    shadow: "0 0 0 1px rgba(0, 0, 0, 0.04), 0 4px 12px rgba(0, 0, 0, 0.06)",
  },

  modal: {
    shadow: "0 0 0 1px rgba(0, 0, 0, 0.06), 0 8px 24px rgba(0, 0, 0, 0.08)",
    overlay: "rgba(0, 0, 0, 0.4)",
  },

  tooltip: {
    shadow: "0 0 0 1px rgba(0, 0, 0, 0.06), 0 4px 12px rgba(0, 0, 0, 0.08)",
  },

  // Halo behind a focused control. Keep the ring quiet; the shared CSS focus
  // token provides the visible keyboard affordance.
  focus: {
    outline: "rgba(0, 0, 0, 0.08)",
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

// ─── Dark Theme Colors (DARK-001) ───────────────────────────────
//
// Same semantic slots as `colors`, dark values. Every text/status/chart
// pair below was verified against WCAG 2.1 AA (>= 4.5:1) on its expected
// background — see the delivery report's contrast table. Charts get their
// own brighter values because dark mode does not invert them.
export const darkColors = {
  background: {
    page: "#141414",
    surface: "#1E1E1E",
    elevated: "#1E1E1E",
    inset: "#262626",
    code: "#1E1E1E",
    brandSlab: "#111111",
    onBrandSlab: "#FFFFFF",
  },
  foreground: {
    primary: "#FFFFFF",
    secondary: "#D9D9D9",
    tertiary: "#A6A6A6",
    muted: "#8C8C8C",
    inverse: "#141414",
  },
  border: {
    default: "#3A3A3A",
    strong: "#595959",
    subtle: "#2E2E2E",
    inverse: "rgba(255,255,255,0.1)",
  },
  state: {
    success: "#66BB6A",
    warning: "#FFB74D",
    error: "#F0625C",
    info: "#64B5F6",
  },
  status: {
    success: { bg: "#1B3A22", text: "#66BB6A", border: "#2E5A38" },
    processing: { bg: "#12293A", text: "#64B5F6", border: "#1E4A6A" },
    warning: { bg: "#3A2E10", text: "#FFB74D", border: "#5A4A1E" },
    error: { bg: "#3A1616", text: "#F0625C", border: "#5A2626" },
    neutral: { bg: "#262626", text: "#A6A6A6", border: "#3A3A3A" },
  },
  morandi: {
    slate: "#D1D5DB",
    cream: "#262626",
    sand: "#E5C896",
    greige: "#9CA3AF",
    terracotta: "#D98E73",
  },
  chart: {
    blue: "#7EA2D6",
    purple: "#9D8BC9",
    primary: "#7BA4D0",
    accent: "#4E9B7E",
    secondary: "#A0AEC0",
    negative: "#C96868",
    fill: "#475569",
  },
  // Brand mark stays the same in both themes; only the inverse variant
  // flips. Surface slots (admin sider / code) are the same dark values.
  surface: {
    admin: "#2F3437",
    code: "#1E1E1E",
  },
} as const;
