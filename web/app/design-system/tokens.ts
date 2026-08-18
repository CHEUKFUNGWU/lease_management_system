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
  // Coastal Navy & Emerald Palette (Professional High-Trust SaaS)
  background: {
    page: "#F8FAFC",        // Cool slate canvas (clean, crisp, modern foundation)
    surface: "#FFFFFF",     // Crisp pure white card surface
    elevated: "#FFFFFF",    // Modal, dropdown
    inset: "#F1F5F9",       // Table headers, secondary panels
    code: "#F8FAFC",        // Code blocks, diff backgrounds
    // DARK-003: the login brand slab is an identity surface, not a foreground.
    // It used var(--fg-primary), so the dark theme flipped it white — the black
    // slab is the brand, and it stays black in both themes. Its text is pinned
    // the same way: `--fg-inverse` follows the theme and went dark-on-black.
    brandSlab: "#000000",
    onBrandSlab: "#FFFFFF",
  },

  foreground: {
    primary: "#0F172A",     // Deep Coastal Obsidian Navy (rich high-contrast authority)
    secondary: "#1E293B",   // Slate 800 (high contrast body text, important labels)
    tertiary: "#334155",    // Slate 700 (descriptions, metadata, labels)
    muted: "#64748B",       // Slate 500 (hints, placeholders, comparison baseline)
    inverse: "#FFFFFF",     // Text on dark backgrounds
  },

  border: {
    default: "#E2E8F0",     // Standard dividers, card borders (--mono-90)
    strong: "#64748B",      // Hover states, active borders (--mono-70)
    subtle: "#F1F5F9",      // Internal dividers, table rows (--mono-95)
    inverse: "rgba(255,255,255,0.12)", // Borders on dark elements
  },

  // State colors — crisp, vibrant semantic accents with high contrast
  state: {
    success: "#059669",     // Crisp Emerald Green
    warning: "#D97706",     // Amber Ochre
    error: "#E11D48",       // Ruby Rose Red
    info: "#2563EB",        // Royal Cobalt Blue
  },

  status: {
    success: { bg: "#ECFDF5", text: "#065F46", border: "#A7F3D0" },
    processing: { bg: "#EFF6FF", text: "#1E40AF", border: "#BFDBFE" },
    warning: { bg: "#FFFBEB", text: "#92400E", border: "#FDE68A" },
    error: { bg: "#FFF1F2", text: "#9F1239", border: "#FECDD3" },
    neutral: { bg: "#F1F5F9", text: "#475569", border: "#E2E8F0" },
  },

  // Morandi legacy mappings preserved for backwards compatibility
  morandi: {
    slate: "#0F172A",
    cream: "#EFF6FF",
    sand: "#10B981",
    greige: "#64748B",
    terracotta: "#E11D48",
  },

  // Chart series — Ascetic & high-clarity financial data palette
  chart: {
    blue: "#1E293B",        // Deep Charcoal Slate / Midnight Blue-Black
    purple: "#334155",      // Dark Slate
    primary: "#0F172A",     // Obsidian Navy
    accent: "#2D4B46",      // Deep Ascetic Pine
    secondary: "#64748B",   // Cool Slate
    negative: "#7F473E",    // Deep Muted Rust
    fill: "#E2E8F0",        // Subtle Gray Slate
  },

  // Brand mark (BrandIcon) — graphic-specific values, not UI semantics.
  // Dark mode keeps the same mark; only the inverse variant flips.
  brand: {
    frame: "#111827",
    bar: "#1F2937",
    arrow: "#4B5563",
    arrowHighlight: "#9CA3AF",
    ring: "#6B7280",
    hub: "#374151",
    node: "#1F2937",
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

  // Dark-only surfaces (admin sider, code blocks) — kept as explicit
  // semantic slots so a theme can override them independently.
  surface: {
    admin: "#001529",
    code: "#1E1E1E",
  },
} as const;

// ─── Typography Tokens ─────────────────────────────────────────

export const typography = {
  fontFamily: {
    sans: 'var(--font-inter), -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Noto Sans SC", "PingFang SC", "Microsoft YaHei", sans-serif',
    mono: 'ui-monospace, SFMono-Regular, Menlo, monospace',
  },

  // Scale: 8 levels, tight tracking for headings
  sizes: {
    display: { size: 28, lineHeight: 36, weight: 600, tracking: -0.04 },    // Page titles
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

  // Halo behind a focused control. Ant Design derives this from colorPrimary,
  // which is pure black here — left alone it paints a 75%-black slab around
  // every focused input.
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
    brandSlab: "#000000",
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
    admin: "#001529",
    code: "#1E1E1E",
  },
} as const;
