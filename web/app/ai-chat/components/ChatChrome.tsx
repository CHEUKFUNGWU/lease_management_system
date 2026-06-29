"use client";

import { ReactNode } from "react";
import { motion } from "framer-motion";
import { Button, Dropdown, Tag, Tooltip, Typography } from "antd";
import { DownOutlined, PlusOutlined, RobotOutlined } from "@ant-design/icons";
import { t, type Language } from "../../lib/i18n";
import type { AIPageContext } from "../../lib/types/ai-chat";

const { Text } = Typography;

export function ChatHeader({
  title,
  selectedModel,
  modelOptions,
  language,
  onSelectModel,
  onNewSession,
}: {
  title?: string;
  selectedModel: string;
  modelOptions: Array<{ value: string; label: string }>;
  language: Language;
  onSelectModel: (model: string) => void;
  onNewSession: () => void;
}) {
  return (
    <div
      style={{
        height: 56,
        borderBottom: "1px solid #E5E5E5",
        display: "flex",
        alignItems: "center",
        justifyContent: "space-between",
        padding: "0 20px",
        background: "#fff",
        flexShrink: 0,
      }}
    >
      <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
        <RobotOutlined style={{ fontSize: 18, color: "#000" }} />
        <span style={{ fontSize: 15, fontWeight: 600, color: "#000" }}>
          {title || t("ai.assistant_name", language)}
        </span>
      </div>

      <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
        <Dropdown
          menu={{
            items: modelOptions.map((model) => ({
              key: model.value,
              label: model.label,
              onClick: () => onSelectModel(model.value),
            })),
          }}
        >
          <Button type="text" style={{ fontSize: 13, color: "#595959" }}>
            {modelOptions.find((model) => model.value === selectedModel)?.label || selectedModel}
            <DownOutlined style={{ fontSize: 10, marginLeft: 4 }} />
          </Button>
        </Dropdown>

        <Tooltip title={t("ai.new_session_btn", language)}>
          <Button type="text" icon={<PlusOutlined />} onClick={onNewSession} style={{ color: "#000" }} />
        </Tooltip>
      </div>
    </div>
  );
}

export function ChatContextStrip({
  pageContext,
  language,
}: {
  pageContext?: AIPageContext;
  language: Language;
}) {
  if (!pageContext) return null;

  return (
    <motion.div
      initial={{ opacity: 0, y: -4 }}
      animate={{ opacity: 1, y: 0 }}
      style={{
        padding: "8px 12px",
        borderRadius: 8,
        background: "#F7F7F7",
        border: "1px solid #E5E5E5",
        marginBottom: 16,
        display: "flex",
        alignItems: "center",
        gap: 8,
        flexWrap: "wrap",
      }}
    >
      <Text type="secondary" style={{ fontSize: 12, fontWeight: 500 }}>
        {t("ai.context", language)}
      </Text>
      <Tag style={{ borderRadius: 4 }}>{pageContext.title || pageContext.page}</Tag>
      {pageContext.contract_id && <Tag style={{ borderRadius: 4 }}>{pageContext.contract_id}</Tag>}
      {pageContext.period && <Tag style={{ borderRadius: 4 }}>{pageContext.period}</Tag>}
    </motion.div>
  );
}

export function ConversationStarters({
  visible,
  loading,
  chips,
  skillStarters,
  language,
  renderSkillIcon,
  onSelectSkillPrompt,
  onSelectChip,
}: {
  visible: boolean;
  loading: boolean;
  chips: string[];
  skillStarters: Array<{ key: string; labelKey: string; promptKey: string; icon: string }>;
  language: Language;
  renderSkillIcon: (icon: string) => ReactNode;
  onSelectSkillPrompt: (prompt: string) => void;
  onSelectChip: (question: string) => void;
}) {
  if (!visible) return null;

  return (
    <>
      <motion.div
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        style={{
          display: "flex",
          flexWrap: "wrap",
          gap: 8,
          marginBottom: 12,
          alignItems: "center",
        }}
      >
        <Text type="secondary" style={{ fontSize: 12, marginRight: 4 }}>
          {t("ai.agent_skills", language)}
        </Text>
        {skillStarters.map((skill) => (
          <Button
            key={skill.key}
            type="default"
            icon={renderSkillIcon(skill.icon)}
            onClick={() => onSelectSkillPrompt(t(skill.promptKey, language))}
            disabled={loading}
            style={{
              fontSize: 12,
              borderRadius: 6,
              borderColor: "#D9D9D9",
              color: "#262626",
            }}
          >
            {t(skill.labelKey, language)}
          </Button>
        ))}
      </motion.div>

      <motion.div
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        style={{
          display: "flex",
          flexWrap: "wrap",
          gap: 8,
          marginBottom: 20,
          alignItems: "center",
        }}
      >
        <Text type="secondary" style={{ fontSize: 12, marginRight: 4 }}>
          {t("ai.quick_questions", language)}
        </Text>
        {chips.map((chipKey, idx) => (
          <Button
            key={idx}
            type="default"
            onClick={() => onSelectChip(t(chipKey, language))}
            disabled={loading}
            style={{
              fontSize: 12,
              borderRadius: 9999,
              borderColor: "#E5E5E5",
              color: "#595959",
            }}
          >
            {t(chipKey, language)}
          </Button>
        ))}
      </motion.div>
    </>
  );
}
