/**
 * Theme Configuration — Maps our monochrome design tokens to Ant Design's theme system.
 *
 * This ensures every Ant Design component inherits our black-white-gray
 * visual language without individual style overrides.
 */

import { colors, darkColors, typography, radius, depth } from "./tokens";

export function buildTheme(palette: typeof colors | typeof darkColors) {
  return {
  token: {
    // ── Core Colors ──
    colorPrimary: palette.foreground.primary,      // palette.foreground.primary
    // STY-007: primary-button hover used to be painted foreground.secondary by CSS; antd
    // derives the hover fill from colorPrimaryHover.
    colorPrimaryHover: palette.foreground.secondary,
    // DARK-003: the text antd paints on a solid primary surface. It defaults to
    // white, which is right while colorPrimary is black — but the dark theme's
    // primary IS white, so the login button rendered white-on-#DCDCDC at 1.37:1.
    // Tying it to the page canvas keeps it the inverse of the primary surface in
    // both themes.
    colorTextLightSolid: palette.background.page,
    colorInfo: palette.state.info,
    // STY-007: antd's Statistic title renders with colorTextDescription;
    // the old CSS forced foreground.tertiary — carry the rendered value.
    colorTextDescription: palette.foreground.tertiary,
    colorInfoBg: palette.status.processing.bg,
    colorInfoBorder: palette.status.processing.border,
    colorInfoText: palette.status.processing.text,
    colorSuccess: palette.state.success,
    colorSuccessBg: palette.status.success.bg,
    colorSuccessBorder: palette.status.success.border,
    colorSuccessText: palette.status.success.text,
    colorWarning: palette.state.warning,
    colorWarningBg: palette.status.warning.bg,
    colorWarningBorder: palette.status.warning.border,
    colorWarningText: palette.status.warning.text,
    colorError: palette.state.error,
    colorErrorBg: palette.status.error.bg,
    colorErrorBorder: palette.status.error.border,
    colorErrorText: palette.status.error.text,

    // ── Base ──
    colorBgBase: palette.background.page,          // palette.background.page
    colorTextBase: palette.foreground.primary,     // palette.foreground.primary
    borderRadius: radius.lg,                      // 8px
    wireframe: false,
    fontFamily: typography.fontFamily.sans,

    // ── Borders ──
    colorBorder: palette.border.default,           // palette.border.default
    colorBorderSecondary: palette.border.subtle,   // palette.border.subtle

    // ── Focus ──
    controlOutline: depth.focus.outline,          // rgba(0, 0, 0, 0.08)
    controlOutlineWidth: 3,

    // ── Typography ──
    fontSize: typography.sizes.body.size,
    lineHeight: typography.sizes.body.lineHeight / typography.sizes.body.size,

    // ── Spacing ──
    paddingXS: 4,
    paddingSM: 8,
    padding: 12,
    paddingMD: 16,
    paddingLG: 24,
    paddingXL: 32,

    // ── Motion ──
    motionDurationFast: "0.1s",
    motionDurationMid: "0.15s",
    motionDurationSlow: "0.25s",
    motionEaseInOut: "cubic-bezier(0.4, 0, 0.2, 1)",
    motionEaseOut: "cubic-bezier(0, 0, 0.2, 1)",
    motionEaseIn: "cubic-bezier(0.4, 0, 1, 1)",
  },

  components: {
    // ── Button ──
    Button: {
      borderRadius: radius.full,
      controlHeight: 36,
      controlHeightSM: 28,
      controlHeightLG: 44,
      // STY-007: the global override used to force 500 via CSS; the token
      // must match what the UI actually rendered, not the old 600 intent.
      fontWeight: typography.weights.medium,
      defaultBg: palette.background.page,
      defaultBorderColor: palette.border.default,
      defaultColor: palette.foreground.primary,
      defaultHoverBg: palette.background.surface,
      // STY-007: the override painted the hover border with the primary
      // foreground; keep that rendered value.
      defaultHoverBorderColor: palette.foreground.primary,
      defaultHoverColor: palette.foreground.primary,
      defaultActiveBg: palette.background.inset,
      defaultActiveBorderColor: palette.foreground.primary,
      primaryShadow: "none",
      dangerShadow: "none",
    },

    // ── Card ──
    Card: {
      borderRadiusLG: radius.xl,
      borderRadiusSM: radius.md,
      colorBorderSecondary: palette.border.default,
      headerBg: palette.background.page,
      headerFontSize: typography.sizes.h2.size,
      headerHeight: 52,
      // STY-007: the card-body override used to force 20px; antd's default
      // body padding is 24px (12px for size="small"), so both tokens must
      // carry the value the override rendered.
      bodyPadding: 20,
      bodyPaddingSM: 20,
      boxShadow: "none",
      boxShadowTertiary: "none",
    },

    // ── Menu ──
    Menu: {
      itemBorderRadius: radius.md,
      // STY-007: submenu titles carried the same 6px radius via CSS.
      subMenuItemBorderRadius: radius.md,
      activeBarBorderWidth: 0,
      itemSelectedBg: palette.background.inset,
      itemSelectedColor: palette.foreground.primary,
      itemHoverBg: palette.background.surface,
      itemHoverColor: palette.foreground.secondary,
      itemColor: palette.foreground.tertiary,
      itemHeight: 40,
      subMenuItemBg: palette.background.page,
      groupTitleColor: palette.foreground.muted,
      groupTitleFontSize: typography.sizes.caption.size,
    },

    // ── Layout ──
    Layout: {
      bodyBg: palette.background.page,
      headerBg: palette.background.page,
      siderBg: palette.background.page,
      footerBg: palette.background.page,
      headerHeight: 60,
      headerPadding: "0 32px",
    },

    // ── Table ──
    Table: {
      headerBg: palette.background.inset,
      headerBorderRadius: radius.lg,
      headerColor: palette.foreground.tertiary,
      borderColor: palette.border.subtle,
      rowHoverBg: palette.background.surface,
      rowSelectedBg: palette.background.inset,
      rowSelectedHoverBg: palette.background.inset,
      cellPaddingBlock: 12,
      cellPaddingInline: 16,
      // STY-007: size="small" tables used to be forced to the same 12/16
      // padding by a CSS override flagged important; without it antd's small
      // defaults (4px inline) change every small table's look. Pin the small
      // tokens to the same values so deleting the override does not change
      // rendered output.
      cellPaddingBlockSM: 12,
      cellPaddingInlineSM: 16,
      cellFontSize: typography.sizes.body.size,
      headerSplitColor: palette.border.subtle,
    },

    // ── Input / Select / DatePicker ──
    Input: {
      borderRadius: radius.lg,
      activeBorderColor: palette.foreground.primary,
      hoverBorderColor: palette.border.strong,
      colorBgContainer: palette.background.page,
      colorTextPlaceholder: palette.foreground.muted,
      controlHeight: 36,
      controlHeightLG: 44,
      controlHeightSM: 28,
    },
    Select: {
      borderRadius: radius.lg,
      controlHeight: 36,
      optionSelectedBg: palette.background.inset,
      optionActiveBg: palette.background.surface,
      optionSelectedColor: palette.foreground.primary,
    },
    DatePicker: {
      borderRadius: radius.lg,
      controlHeight: 36,
    },

    // ── Modal ──
    Modal: {
      borderRadiusLG: radius["2xl"],
      titleFontSize: typography.sizes.h1.size,
      // FIX-019: AntD consumes titleLineHeight as a ratio (line-height
      // multiplier), not pixels — 32 here rendered line-height: 768px on a
      // 24px title (32 × 24). The global token below (line 48) already does
      // the division; this one missed it.
      titleLineHeight: typography.sizes.h1.lineHeight / typography.sizes.h1.size,
      headerBg: palette.background.page,
      contentBg: palette.background.page,
      footerBg: palette.background.page,
      headerPadding: "20px 24px",
      contentPadding: "0 24px 24px",
      footerPadding: "16px 24px",
      boxShadow: "0 0 0 1px rgba(0, 0, 0, 0.06), 0 8px 24px rgba(0, 0, 0, 0.08)",
    },

    // ── Tag ──
    Tag: {
      borderRadiusSM: radius.sm,
      defaultBg: palette.background.inset,
      defaultColor: palette.foreground.secondary,
      lineHeight: 18,
    },

    // ── Descriptions ──
    Descriptions: {
      borderRadiusLG: radius.lg,
      colorSplit: palette.border.subtle,
      labelColor: palette.foreground.tertiary,
      contentColor: palette.foreground.secondary,
    },

    // ── Timeline ──
    Timeline: {
      dotBorderWidth: 2,
      dotBg: palette.background.page,
      itemPaddingBottom: 24,
    },

    // ── Statistic ──
    Statistic: {
      titleFontSize: typography.sizes.caption.size,
      // STY-007: the content-value override forced 28px; the token used to
      // say 24 (h1) — align with what the UI actually rendered.
      contentFontSize: typography.sizes.display.size,
      // Note: contentFontWeight is NOT consumed by antd's Statistic styles
      // (verified against 5.29 source) — the 600 weight is carried by CSS.
    },

    // ── Tabs ──
    Tabs: {
      cardBg: palette.background.inset,
      cardHeight: 40,
      itemColor: palette.foreground.tertiary,
      itemHoverColor: palette.foreground.secondary,
      itemSelectedColor: palette.foreground.primary,
      itemActiveColor: palette.foreground.primary,
      inkBarColor: palette.foreground.primary,
      horizontalItemGutter: 24,
    },

    // ── Pagination ──
    Pagination: {
      borderRadius: radius.md,
      itemSize: 32,
      itemSizeSM: 24,
      itemActiveBg: palette.foreground.primary,
      itemActiveColor: palette.foreground.inverse,
      itemActiveBgDisabled: palette.border.subtle,
    },

    // ── Dropdown ──
    Dropdown: {
      borderRadius: radius.lg,
      controlItemBgHover: palette.background.surface,
      controlItemBgActive: palette.background.inset,
    },

    // ── Tooltip ──
    Tooltip: {
      borderRadius: radius.md,
      colorBgSpotlight: palette.foreground.secondary,
      colorTextLightSolid: palette.foreground.inverse,
    },

    // ── Popover ──
    Popover: {
      borderRadius: radius.xl,
      colorBgElevated: palette.background.elevated,
    },

    // ── Notification ──
    Notification: {
      borderRadius: radius.xl,
      borderRadiusLG: radius.xl,
    },

    // ── Breadcrumb ──
    Breadcrumb: {
      lastItemColor: palette.foreground.primary,
      linkColor: palette.foreground.tertiary,
      linkHoverColor: palette.foreground.secondary,
      separatorColor: palette.border.strong,
      itemColor: palette.foreground.tertiary,
    },

    // ── Steps ──
    Steps: {
      colorPrimary: palette.foreground.primary,
      colorText: palette.foreground.tertiary,
      colorTextDescription: palette.foreground.muted,
      iconFontSize: 14,
      iconSize: 32,
    },

    // ── Checkbox / Radio ──
    Checkbox: {
      borderRadius: radius.sm,
      colorPrimary: palette.foreground.primary,
    },
    Radio: {
      borderRadius: radius.full,
      colorPrimary: palette.foreground.primary,
      buttonSolidCheckedActiveBg: palette.foreground.primary,
      buttonSolidCheckedBg: palette.foreground.primary,
      buttonSolidCheckedHoverBg: palette.foreground.secondary,
    },

    // ── Switch ──
    Switch: {
      colorPrimary: palette.foreground.primary,
      colorPrimaryHover: palette.foreground.secondary,
    },

    // ── Slider ──
    Slider: {
      trackBg: palette.foreground.primary,
      trackHoverBg: palette.foreground.secondary,
      railBg: palette.border.default,
      handleColor: palette.foreground.primary,
    },

    // ── Progress ──
    Progress: {
      defaultColor: palette.foreground.primary,
      remainingColor: palette.border.default,
    },

    // ── Badge ──
    Badge: {
      colorError: palette.foreground.primary,
      colorWarning: palette.foreground.secondary,
      // STY-007: the badge-count override forced 10px/600/16px; antd's
      // defaults are 12px and 20px — pin the tokens to the rendered values.
      textFontSize: 10,
      textFontWeight: typography.weights.semibold,
      indicatorHeight: 16,
      indicatorHeightSM: 16,
    },

    // ── Avatar ──
    Avatar: {
      borderRadius: radius.full,
      colorBg: palette.background.inset,
      colorText: palette.foreground.secondary,
    },

    // ── Segmented ──
    Segmented: {
      borderRadius: radius.md,
      itemColor: palette.foreground.tertiary,
      itemHoverColor: palette.foreground.secondary,
      itemSelectedColor: palette.foreground.primary,
      itemSelectedBg: palette.background.page,
      trackBg: palette.background.inset,
    },

    // ── Collapse ──
    Collapse: {
      borderRadius: radius.lg,
      headerBg: palette.background.page,
      contentBg: palette.background.page,
      headerPadding: "12px 16px",
    },

    // ── Drawer ──
    Drawer: {
      borderRadius: 0,
      footerPaddingBlock: 16,
      footerPaddingInline: 24,
      headerPadding: "16px 24px",
    },

    // ── List ──
    List: {
      borderRadius: radius.lg,
      itemPadding: "12px 16px",
      itemPaddingSM: "8px 12px",
      itemPaddingLG: "16px 24px",
      emptyTextColor: palette.foreground.muted,
    },

    // ── Empty ──
    Empty: {
      colorText: palette.foreground.muted,
      colorTextDescription: palette.foreground.muted,
    },

    // ── Result ──
    Result: {
      iconFontSize: 64,
      titleFontSize: typography.sizes.h1.size,
      subtitleFontSize: typography.sizes.body.size,
      colorError: palette.foreground.primary,
      colorSuccess: palette.foreground.primary,
      colorWarning: palette.foreground.secondary,
      colorInfo: palette.foreground.tertiary,
    },

    // ── Skeleton ──
    Skeleton: {
      gradientFromColor: palette.background.inset,
      gradientToColor: palette.background.surface,
      paragraphLiHeight: 22,
      titleHeight: 16,
    },
  },
} as const;
}


export const antdTheme = buildTheme(colors);
export const antdDarkTheme = buildTheme(darkColors);
