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
