"use client";

import { useState } from "react";
import { Button, Drawer, Space, Typography } from "antd";
import { QuestionCircleOutlined } from "@ant-design/icons";
import { t, type Language } from "../lib/i18n";
import HelpFlowDiagram, { type FlowStep } from "./HelpFlowDiagram";

/**
 * HELP-001: page usage tutorial panel. A quiet question-mark entry in the
 * PageHeader help slot opens a Drawer (not a Modal — the tutorial can sit
 * beside the page, like RetailAIDrawer). Content is structured and lives
 * outside the page code; all copy is trilingual.
 *
 * Red line: compliance wording never goes here — it stays with the relevant
 * data or workflow section; the page header intentionally stays title-only.
 */

export interface HelpSection {
  key: string;
  heading: string;
  body: string;
}

export interface HelpContent {
  title: string;
  flow: FlowStep[];
  sections: HelpSection[];
}

export function HelpTrigger({ content, language }: { content: HelpContent; language: Language }) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <Button
        type="text"
        className="page-header-help-trigger"
        icon={<QuestionCircleOutlined />}
        aria-label={t("help.open_tutorial", language)}
        onClick={() => setOpen(true)}
      />
      <Drawer
        open={open}
        onClose={() => setOpen(false)}
        title={content.title}
        placement="right"
        width={420}
        classNames={{ body: "app-drawer-body" }}
      >
        <Space direction="vertical" size={16} className="help-drawer-body">
          <HelpFlowDiagram steps={content.flow} />
          {content.sections.map((section) => (
            <section key={section.key} className="help-section">
              <Typography.Title level={5} className="help-section-heading">
                {section.heading}
              </Typography.Title>
              <Typography.Paragraph type="secondary" className="help-section-body">
                {section.body}
              </Typography.Paragraph>
            </section>
          ))}
        </Space>
      </Drawer>
    </>
  );
}
