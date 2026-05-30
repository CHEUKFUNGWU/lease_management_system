/**
 * Theme Configuration — Maps our monochrome design tokens to Ant Design's theme system.
 *
 * This ensures every Ant Design component inherits our black-white-gray
 * visual language without individual style overrides.
 */

import { colors, typography, radius, depth } from "./tokens";

export const antdTheme = {
  token: {
    // ── Core Colors ──
    colorPrimary: colors.foreground.primary,      // #000000
    colorInfo: colors.foreground.tertiary,        // #595959
    colorSuccess: colors.foreground.primary,      // Black — success is conveyed through icons + weight
    colorWarning: colors.foreground.secondary,    // Dark gray
    colorError: colors.foreground.primary,        // Black — errors are bold + icon

    // ── Base ──
    colorBgBase: colors.background.page,          // #FFFFFF
    colorTextBase: colors.foreground.primary,     // #000000
    borderRadius: radius.lg,                      // 8px
    wireframe: false,
    fontFamily: typography.fontFamily.sans,

    // ── Borders ──
    colorBorder: colors.border.default,           // #E5E5E5
    colorBorderSecondary: colors.border.subtle,   // #F0F0F0

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
      fontWeight: typography.weights.semibold,
      defaultBg: colors.background.page,
      defaultBorderColor: colors.border.default,
      defaultColor: colors.foreground.primary,
      defaultHoverBg: colors.background.surface,
      defaultHoverBorderColor: colors.border.strong,
      defaultHoverColor: colors.foreground.primary,
      defaultActiveBg: colors.background.inset,
      defaultActiveBorderColor: colors.foreground.primary,
      primaryShadow: "none",
      dangerShadow: "none",
    },

    // ── Card ──
    Card: {
      borderRadiusLG: radius.xl,
      borderRadiusSM: radius.md,
      colorBorderSecondary: colors.border.default,
      headerBg: colors.background.page,
      headerFontSize: typography.sizes.h2.size,
      headerHeight: 52,
      boxShadow: "none",
      boxShadowTertiary: "none",
    },

    // ── Menu ──
    Menu: {
      itemBorderRadius: radius.md,
      activeBarBorderWidth: 0,
      itemSelectedBg: colors.background.inset,
      itemSelectedColor: colors.foreground.primary,
      itemHoverBg: colors.background.surface,
      itemHoverColor: colors.foreground.secondary,
      itemColor: colors.foreground.tertiary,
      itemHeight: 40,
      subMenuItemBg: colors.background.page,
      groupTitleColor: colors.foreground.muted,
      groupTitleFontSize: typography.sizes.caption.size,
    },

    // ── Layout ──
    Layout: {
      bodyBg: colors.background.page,
      headerBg: colors.background.page,
      siderBg: colors.background.page,
      footerBg: colors.background.page,
      headerHeight: 60,
      headerPadding: "0 32px",
    },

    // ── Table ──
    Table: {
      headerBg: colors.background.inset,
      headerBorderRadius: radius.lg,
      headerColor: colors.foreground.tertiary,
      borderColor: colors.border.subtle,
      rowHoverBg: colors.background.surface,
      rowSelectedBg: colors.background.inset,
      rowSelectedHoverBg: colors.background.inset,
      cellPaddingBlock: 12,
      cellPaddingInline: 16,
      cellFontSize: typography.sizes.body.size,
      headerSplitColor: colors.border.subtle,
    },

    // ── Input / Select / DatePicker ──
    Input: {
      borderRadius: radius.lg,
      activeBorderColor: colors.foreground.primary,
      hoverBorderColor: colors.border.strong,
      colorBgContainer: colors.background.page,
      colorTextPlaceholder: colors.foreground.muted,
      controlHeight: 36,
      controlHeightLG: 44,
      controlHeightSM: 28,
    },
    Select: {
      borderRadius: radius.lg,
      controlHeight: 36,
      optionSelectedBg: colors.background.inset,
      optionActiveBg: colors.background.surface,
      optionSelectedColor: colors.foreground.primary,
    },
    DatePicker: {
      borderRadius: radius.lg,
      controlHeight: 36,
    },

    // ── Modal ──
    Modal: {
      borderRadiusLG: radius["2xl"],
      titleFontSize: typography.sizes.h1.size,
      titleLineHeight: typography.sizes.h1.lineHeight,
      headerBg: colors.background.page,
      contentBg: colors.background.page,
      footerBg: colors.background.page,
      headerPadding: "20px 24px",
      contentPadding: "0 24px 24px",
      footerPadding: "16px 24px",
      boxShadow: "0 0 0 1px rgba(0, 0, 0, 0.06), 0 8px 24px rgba(0, 0, 0, 0.08)",
    },

    // ── Tag ──
    Tag: {
      borderRadiusSM: radius.sm,
      defaultBg: colors.background.inset,
      defaultColor: colors.foreground.secondary,
      lineHeight: 18,
    },

    // ── Descriptions ──
    Descriptions: {
      borderRadiusLG: radius.lg,
      colorSplit: colors.border.subtle,
      labelColor: colors.foreground.tertiary,
      contentColor: colors.foreground.secondary,
    },

    // ── Timeline ──
    Timeline: {
      dotBorderWidth: 2,
      dotBg: colors.background.page,
      itemPaddingBottom: 24,
    },

    // ── Statistic ──
    Statistic: {
      titleFontSize: typography.sizes.caption.size,
      contentFontSize: typography.sizes.h1.size,
      contentFontWeight: typography.weights.bold,
    },

    // ── Tabs ──
    Tabs: {
      cardBg: colors.background.inset,
      cardHeight: 40,
      itemColor: colors.foreground.tertiary,
      itemHoverColor: colors.foreground.secondary,
      itemSelectedColor: colors.foreground.primary,
      itemActiveColor: colors.foreground.primary,
      inkBarColor: colors.foreground.primary,
      horizontalItemGutter: 24,
    },

    // ── Pagination ──
    Pagination: {
      borderRadius: radius.md,
      itemSize: 32,
      itemSizeSM: 24,
      itemActiveBg: colors.foreground.primary,
      itemActiveColor: colors.foreground.inverse,
      itemActiveBgDisabled: colors.border.subtle,
    },

    // ── Dropdown ──
    Dropdown: {
      borderRadius: radius.lg,
      controlItemBgHover: colors.background.surface,
      controlItemBgActive: colors.background.inset,
    },

    // ── Tooltip ──
    Tooltip: {
      borderRadius: radius.md,
      colorBgSpotlight: colors.foreground.secondary,
      colorTextLightSolid: colors.foreground.inverse,
    },

    // ── Popover ──
    Popover: {
      borderRadius: radius.xl,
      colorBgElevated: colors.background.elevated,
    },

    // ── Notification ──
    Notification: {
      borderRadius: radius.xl,
      borderRadiusLG: radius.xl,
    },

    // ── Breadcrumb ──
    Breadcrumb: {
      lastItemColor: colors.foreground.primary,
      linkColor: colors.foreground.tertiary,
      linkHoverColor: colors.foreground.secondary,
      separatorColor: colors.border.strong,
      itemColor: colors.foreground.tertiary,
    },

    // ── Steps ──
    Steps: {
      colorPrimary: colors.foreground.primary,
      colorText: colors.foreground.tertiary,
      colorTextDescription: colors.foreground.muted,
      iconFontSize: 14,
      iconSize: 32,
    },

    // ── Checkbox / Radio ──
    Checkbox: {
      borderRadius: radius.sm,
      colorPrimary: colors.foreground.primary,
    },
    Radio: {
      borderRadius: radius.full,
      colorPrimary: colors.foreground.primary,
      buttonSolidCheckedActiveBg: colors.foreground.primary,
      buttonSolidCheckedBg: colors.foreground.primary,
      buttonSolidCheckedHoverBg: colors.foreground.secondary,
    },

    // ── Switch ──
    Switch: {
      colorPrimary: colors.foreground.primary,
      colorPrimaryHover: colors.foreground.secondary,
    },

    // ── Slider ──
    Slider: {
      trackBg: colors.foreground.primary,
      trackHoverBg: colors.foreground.secondary,
      railBg: colors.border.default,
      handleColor: colors.foreground.primary,
    },

    // ── Progress ──
    Progress: {
      defaultColor: colors.foreground.primary,
      remainingColor: colors.border.default,
    },

    // ── Badge ──
    Badge: {
      colorError: colors.foreground.primary,
      colorWarning: colors.foreground.secondary,
    },

    // ── Avatar ──
    Avatar: {
      borderRadius: radius.full,
      colorBg: colors.background.inset,
      colorText: colors.foreground.secondary,
    },

    // ── Segmented ──
    Segmented: {
      borderRadius: radius.md,
      itemColor: colors.foreground.tertiary,
      itemHoverColor: colors.foreground.secondary,
      itemSelectedColor: colors.foreground.primary,
      itemSelectedBg: colors.background.page,
      trackBg: colors.background.inset,
    },

    // ── Collapse ──
    Collapse: {
      borderRadius: radius.lg,
      headerBg: colors.background.page,
      contentBg: colors.background.page,
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
      emptyTextColor: colors.foreground.muted,
    },

    // ── Empty ──
    Empty: {
      colorText: colors.foreground.muted,
      colorTextDescription: colors.foreground.muted,
    },

    // ── Result ──
    Result: {
      iconFontSize: 64,
      titleFontSize: typography.sizes.h1.size,
      subtitleFontSize: typography.sizes.body.size,
      colorError: colors.foreground.primary,
      colorSuccess: colors.foreground.primary,
      colorWarning: colors.foreground.secondary,
      colorInfo: colors.foreground.tertiary,
    },

    // ── Skeleton ──
    Skeleton: {
      color: colors.background.inset,
      colorGradientEnd: colors.background.surface,
      paragraphLiHeight: 22,
      titleHeight: 16,
    },
  },
} as const;

