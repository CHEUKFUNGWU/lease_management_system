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
