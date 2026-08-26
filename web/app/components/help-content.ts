import { t, type Language } from "../lib/i18n";
import type { HelpContent } from "./HelpDrawer";

/**
 * HELP-001: structured tutorial content for the sample pages. Lives outside
 * the page code so content edits never touch page logic. Compliance wording
 * deliberately stays out — those sentences are pinned on the page meta.
 */
export function pulseHelpContent(language: Language): HelpContent {
  return {
    title: t("help.pulse.title", language),
    flow: [
      { key: "classification", label: t("help.pulse.flow.classification", language) },
      { key: "window", label: t("help.pulse.flow.window", language) },
      { key: "read", label: t("help.pulse.flow.read", language) },
      { key: "drill", label: t("help.pulse.flow.drill", language) },
    ],
    sections: [
      {
        key: "classification",
        heading: t("help.pulse.s1.heading", language),
        body: t("help.pulse.s1.body", language),
      },
      {
        key: "window",
        heading: t("help.pulse.s2.heading", language),
        body: t("help.pulse.s2.body", language),
      },
      {
        key: "read",
        heading: t("help.pulse.s3.heading", language),
        body: t("help.pulse.s3.body", language),
      },
    ],
  };
}

export function scenarioHelpContent(language: Language): HelpContent {
  return {
    title: t("help.scenario.title", language),
    flow: [
      { key: "pick", label: t("help.scenario.flow.pick", language) },
      { key: "assume", label: t("help.scenario.flow.assume", language) },
      { key: "result", label: t("help.scenario.flow.result", language) },
      { key: "act", label: t("help.scenario.flow.act", language) },
    ],
    sections: [
      {
        key: "pick",
        heading: t("help.scenario.s1.heading", language),
        body: t("help.scenario.s1.body", language),
      },
      {
        key: "assume",
        heading: t("help.scenario.s2.heading", language),
        body: t("help.scenario.s2.body", language),
      },
      {
        key: "act",
        heading: t("help.scenario.s3.heading", language),
        body: t("help.scenario.s3.body", language),
      },
    ],
  };
}

/**
 * HELP-002: tutorial content for the store 360 page. Same shape as the
 * sample pages; compliance wording stays on the page meta slot.
 */
export function store360HelpContent(language: Language): HelpContent {
  return {
    title: t("help.store360.title", language),
    flow: [
      { key: "pick", label: t("help.store360.flow.pick", language) },
      { key: "read", label: t("help.store360.flow.read", language) },
      { key: "bridge", label: t("help.store360.flow.bridge", language) },
      { key: "act", label: t("help.store360.flow.act", language) },
    ],
    sections: [
      {
        key: "pick",
        heading: t("help.store360.s1.heading", language),
        body: t("help.store360.s1.body", language),
      },
      {
        key: "read",
        heading: t("help.store360.s2.heading", language),
        body: t("help.store360.s2.body", language),
      },
      {
        key: "bridge",
        heading: t("help.store360.s3.heading", language),
        body: t("help.store360.s3.body", language),
      },
    ],
  };
}

/**
 * HELP-002: tutorial content for the performance cockpit page.
 */
export function performanceHelpContent(language: Language): HelpContent {
  return {
    title: t("help.performance.title", language),
    flow: [
      { key: "period", label: t("help.performance.flow.period", language) },
      { key: "overview", label: t("help.performance.flow.overview", language) },
      { key: "actions", label: t("help.performance.flow.actions", language) },
      { key: "ask", label: t("help.performance.flow.ask", language) },
    ],
    sections: [
      {
        key: "period",
        heading: t("help.performance.s1.heading", language),
        body: t("help.performance.s1.body", language),
      },
      {
        key: "overview",
        heading: t("help.performance.s2.heading", language),
        body: t("help.performance.s2.body", language),
      },
      {
        key: "actions",
        heading: t("help.performance.s3.heading", language),
        body: t("help.performance.s3.body", language),
      },
    ],
  };
}

/**
 * HELP-002: tutorial content for the portfolio analysis page.
 */
export function portfolioHelpContent(language: Language): HelpContent {
  return {
    title: t("help.portfolio.title", language),
    flow: [
      { key: "mode", label: t("help.portfolio.flow.mode", language) },
      { key: "group", label: t("help.portfolio.flow.group", language) },
      { key: "read", label: t("help.portfolio.flow.read", language) },
      { key: "export", label: t("help.portfolio.flow.export", language) },
    ],
    sections: [
      {
        key: "mode",
        heading: t("help.portfolio.s1.heading", language),
        body: t("help.portfolio.s1.body", language),
      },
      {
        key: "group",
        heading: t("help.portfolio.s2.heading", language),
        body: t("help.portfolio.s2.body", language),
      },
      {
        key: "read",
        heading: t("help.portfolio.s3.heading", language),
        body: t("help.portfolio.s3.body", language),
      },
    ],
  };
}

/**
 * HELP-003: tutorial content for the FP&A performance workbench page.
 */
export function fpnaWorkbenchHelpContent(language: Language): HelpContent {
  return {
    title: t("help.fpna.title", language),
    flow: [
      { key: "lineage", label: t("help.fpna.flow.lineage", language) },
      { key: "compare", label: t("help.fpna.flow.compare", language) },
      { key: "forecast", label: t("help.fpna.flow.forecast", language) },
      { key: "quality", label: t("help.fpna.flow.quality", language) },
      { key: "governance", label: t("help.fpna.flow.governance", language) },
    ],
    sections: [
      {
        key: "lineage",
        heading: t("help.fpna.s1.heading", language),
        body: t("help.fpna.s1.body", language),
      },
      {
        key: "compare",
        heading: t("help.fpna.s2.heading", language),
        body: t("help.fpna.s2.body", language),
      },
      {
        key: "forecast",
        heading: t("help.fpna.s3.heading", language),
        body: t("help.fpna.s3.body", language),
      },
      {
        key: "quality",
        heading: t("help.fpna.s4.heading", language),
        body: t("help.fpna.s4.body", language),
      },
      {
        key: "dual_basis",
        heading: t("help.fpna.s5.heading", language),
        body: t("help.fpna.s5.body", language),
      },
    ],
  };
}

/**
 * F2-1（任务指令）：三表财务模型页帮助。flow 直接复用页面卡片自身的
 * ①–⑤ 步骤键（finmodel.step_*），不另造一套编号；sections 回答财务用户
 * 的三个真实疑问：三道闸各拦什么、勾稽不过下一步做什么、发布的计划版本
 * 谁能看到。
 */
export function financialModelHelpContent(language: Language): HelpContent {
  return {
    title: t("help.finmodel.title", language),
    flow: [
      { key: "select_def", label: t("finmodel.step_select_def", language) },
      { key: "assumptions", label: t("finmodel.step_assumptions", language) },
      { key: "opening", label: t("finmodel.step_opening", language) },
      { key: "run", label: t("finmodel.step_run", language) },
      { key: "publish", label: t("finmodel.step_publish_export", language) },
    ],
    sections: [
      {
        key: "gates",
        heading: t("help.finmodel.s1.heading", language),
        body: t("help.finmodel.s1.body", language),
      },
      {
        key: "tie_out_next",
        heading: t("help.finmodel.s2.heading", language),
        body: t("help.finmodel.s2.body", language),
      },
      {
        key: "publish_audience",
        heading: t("help.finmodel.s3.heading", language),
        body: t("help.finmodel.s3.body", language),
      },
    ],
  };
}

/**
 * F2-1：单店利润表页帮助。回答两个口径为什么不一样、以及 Decision Ready
 * 不满足时这张表还能不能用。
 */
export function storePnlHelpContent(language: Language): HelpContent {
  return {
    title: t("help.storepnl.title", language),
    flow: [
      { key: "pick_store", label: t("help.storepnl.flow.pick_store", language) },
      { key: "pick_compare", label: t("help.storepnl.flow.pick_compare", language) },
      { key: "read_bases", label: t("help.storepnl.flow.read_bases", language) },
      { key: "drill", label: t("help.storepnl.flow.drill", language) },
    ],
    sections: [
      {
        key: "two_bases",
        heading: t("help.storepnl.s1.heading", language),
        body: t("help.storepnl.s1.body", language),
      },
      {
        key: "decision_ready",
        heading: t("help.storepnl.s2.heading", language),
        body: t("help.storepnl.s2.body", language),
      },
      {
        key: "provenance",
        heading: t("help.storepnl.s3.heading", language),
        body: t("help.storepnl.s3.body", language),
      },
    ],
  };
}

/**
 * F2-1：月结中心帮助。回答本页动作与总账的关系、哪些动作不可逆。
 */
export function monthlyClosingHelpContent(language: Language): HelpContent {
  return {
    title: t("help.monthly.title", language),
    flow: [
      { key: "period", label: t("help.monthly.flow.period", language) },
      { key: "entries", label: t("help.monthly.flow.entries", language) },
      { key: "review", label: t("help.monthly.flow.review", language) },
      { key: "post_lock", label: t("help.monthly.flow.post_lock", language) },
    ],
    sections: [
      {
        key: "gl_relation",
        heading: t("help.monthly.s1.heading", language),
        body: t("help.monthly.s1.body", language),
      },
      {
        key: "irreversible",
        heading: t("help.monthly.s2.heading", language),
        body: t("help.monthly.s2.body", language),
      },
      {
        key: "scope",
        heading: t("help.monthly.s3.heading", language),
        body: t("help.monthly.s3.body", language),
      },
    ],
  };
}


export function ecomHelpContent(language: Language): HelpContent {
  return {
    title: t("ecom.pulse.title", language),
    flow: [
      { key: "classification", label: t("ecom.common.classification", language) },
      { key: "window", label: t("ecom.common.window_days", language) },
      { key: "read", label: t("ecom.pulse.subtitle", language) },
    ],
    sections: [
      {
        key: "classification",
        heading: t("ecom.common.classification", language),
        body: t("ecom.common.no_data_reason", language),
      },
      {
        key: "window",
        heading: t("ecom.common.window_days", language),
        body: t("ecom.pulse.subtitle", language),
      },
      {
        key: "read",
        heading: t("ecom.common.site", language),
        body: t("ecom.common.currency", language),
      },
    ],
  };
}
