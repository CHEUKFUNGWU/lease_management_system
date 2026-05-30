/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "./app/**/*.{js,ts,jsx,tsx,mdx}",
    "./components/**/*.{js,ts,jsx,tsx,mdx}",
  ],
  theme: {
    extend: {
      colors: {
        // Monochrome scale: 12 levels from #000 to #fff
        // Designed for maximum hierarchy clarity in a no-color system
        mono: {
          0: "#000000",   // Primary text, key actions
          5: "#0A0A0A",   // Deep surface (modal overlay)
          10: "#141414",  // Primary buttons, active states
          20: "#262626",  // Secondary text, hover on dark
          30: "#434343",  // Strong borders, icons default
          40: "#595959",  // Body text, descriptions
          50: "#737373",  // Muted text, placeholders
          60: "#8C8C8C",  // Disabled text, subtle icons
          70: "#A6A6A6",  // Borders on dark backgrounds
          80: "#BFBFBF",  // Light borders, dividers
          90: "#D9D9D9",  // Hover borders, active borders
          95: "#F0F0F0",  // Secondary backgrounds, table headers
          98: "#F7F7F7",  // Card backgrounds, subtle surfaces
          100: "#FFFFFF", // Page background, input backgrounds
        },
        // Semantic colors — extremely desaturated, almost monochrome
        // Used only when state MUST be distinguished (error forms, success toast)
        success: "#1A1A1A",   // Black — minimal success indication
        warning: "#333333",   // Dark gray — warning is important but not alarming
        error: "#000000",     // Black — errors are critical, use bold weight + icon
        info: "#595959",      // Medium gray — informational
      },
      fontFamily: {
        sans: [
          '"Inter"',
          "-apple-system",
          "BlinkMacSystemFont",
          '"Segoe UI"',
          "Roboto",
          '"Noto Sans SC"',
          '"PingFang SC"',
          '"Microsoft YaHei"',
          "sans-serif",
        ],
        mono: [
          '"JetBrains Mono"',
          '"Fira Code"',
          "ui-monospace",
          "SFMono-Regular",
          "monospace",
        ],
      },
      fontSize: {
        // Tight, precise scale for data-heavy enterprise UI
        "2xs": ["11px", { lineHeight: "14px", letterSpacing: "0.02em" }],
        xs: ["12px", { lineHeight: "16px", letterSpacing: "0.01em" }],
        sm: ["13px", { lineHeight: "18px", letterSpacing: "0" }],
        base: ["14px", { lineHeight: "22px", letterSpacing: "0" }],
        md: ["15px", { lineHeight: "24px", letterSpacing: "0" }],
        lg: ["16px", { lineHeight: "26px", letterSpacing: "-0.01em" }],
        xl: ["18px", { lineHeight: "28px", letterSpacing: "-0.02em" }],
        "2xl": ["20px", { lineHeight: "30px", letterSpacing: "-0.02em" }],
        "3xl": ["24px", { lineHeight: "32px", letterSpacing: "-0.03em" }],
        "4xl": ["28px", { lineHeight: "36px", letterSpacing: "-0.03em" }],
        "5xl": ["32px", { lineHeight: "40px", letterSpacing: "-0.04em" }],
        "6xl": ["40px", { lineHeight: "48px", letterSpacing: "-0.04em" }],
      },
      spacing: {
        // 4px base grid, with 2px micro-adjustments
        0.5: "2px",
        1: "4px",
        1.5: "6px",
        2: "8px",
        2.5: "10px",
        3: "12px",
        3.5: "14px",
        4: "16px",
        5: "20px",
        6: "24px",
        7: "28px",
        8: "32px",
        9: "36px",
        10: "40px",
        11: "44px",
        12: "48px",
        14: "56px",
        16: "64px",
        18: "72px",
        20: "80px",
        24: "96px",
      },
      borderRadius: {
        none: "0px",
        sm: "4px",
        DEFAULT: "6px",
        md: "8px",
        lg: "10px",
        xl: "12px",
        "2xl": "14px",
        "3xl": "16px",
        full: "9999px",
      },
      boxShadow: {
        // Extremely subtle — minimalism means shadows are barely perceptible
        none: "none",
        // Static: barely visible, defines boundary without weight
        static: "0 0 0 1px rgba(0, 0, 0, 0.04)",
        // Hover: slight lift, 1px vertical offset only
        hover: "0 1px 2px rgba(0, 0, 0, 0.06), 0 0 0 1px rgba(0, 0, 0, 0.04)",
        // Card: soft ambient presence
        card: "0 0 0 1px rgba(0, 0, 0, 0.04), 0 2px 8px rgba(0, 0, 0, 0.04)",
        // Modal: focused attention, slightly more presence
        modal: "0 0 0 1px rgba(0, 0, 0, 0.06), 0 8px 24px rgba(0, 0, 0, 0.08)",
        // Dropdown: subtle directional hint
        dropdown: "0 0 0 1px rgba(0, 0, 0, 0.04), 0 4px 12px rgba(0, 0, 0, 0.06)",
        // Focus: inner glow for accessibility
        focus: "0 0 0 2px rgba(0, 0, 0, 0.08), inset 0 0 0 1px rgba(0, 0, 0, 0.04)",
      },
      transitionTimingFunction: {
        // Custom easing curves for refined micro-interactions
        "micro": "cubic-bezier(0.4, 0, 0.2, 1)",
        "enter": "cubic-bezier(0, 0, 0.2, 1)",
        "exit": "cubic-bezier(0.4, 0, 1, 1)",
        "bounce-subtle": "cubic-bezier(0.34, 1.56, 0.64, 1)",
      },
      transitionDuration: {
        "instant": "0ms",
        "fast": "100ms",
        "normal": "150ms",
        "slow": "250ms",
        "slower": "350ms",
        "slowest": "500ms",
      },
      keyframes: {
        "fade-in": {
          "0%": { opacity: "0" },
          "100%": { opacity: "1" },
        },
        "fade-in-up": {
          "0%": { opacity: "0", transform: "translateY(4px)" },
          "100%": { opacity: "1", transform: "translateY(0)" },
        },
        "fade-in-down": {
          "0%": { opacity: "0", transform: "translateY(-4px)" },
          "100%": { opacity: "1", transform: "translateY(0)" },
        },
        "scale-in": {
          "0%": { opacity: "0", transform: "scale(0.98)" },
          "100%": { opacity: "1", transform: "scale(1)" },
        },
        "slide-in-right": {
          "0%": { opacity: "0", transform: "translateX(8px)" },
          "100%": { opacity: "1", transform: "translateX(0)" },
        },
        "pulse-subtle": {
          "0%, 100%": { opacity: "1" },
          "50%": { opacity: "0.6" },
        },
        "shimmer": {
          "0%": { backgroundPosition: "-200% 0" },
          "100%": { backgroundPosition: "200% 0" },
        },
      },
      animation: {
        "fade-in": "fade-in 0.2s ease-out forwards",
        "fade-in-up": "fade-in-up 0.25s ease-out forwards",
        "fade-in-down": "fade-in-down 0.2s ease-out forwards",
        "scale-in": "scale-in 0.2s ease-out forwards",
        "slide-in-right": "slide-in-right 0.2s ease-out forwards",
        "pulse-subtle": "pulse-subtle 2s ease-in-out infinite",
        "shimmer": "shimmer 2s linear infinite",
      },
    },
  },
  plugins: [],
};
