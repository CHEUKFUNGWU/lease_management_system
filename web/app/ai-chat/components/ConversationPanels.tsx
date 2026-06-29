"use client";

import { ReactNode } from "react";
import { AnimatePresence, motion } from "framer-motion";
import { Avatar, Button, Input, Space, Spin, Tag, Tooltip, Typography, Upload } from "antd";
import {
  CloseCircleOutlined,
  MessageOutlined,
  PaperClipOutlined,
  RobotOutlined,
  SendOutlined,
  ToolOutlined,
  UserOutlined,
} from "@ant-design/icons";
import {
  AgentReviewPanel,
  AgentTracePanel,
  MessageContent,
  ReviewActionHistoryPanel,
  TypewriterMessage,
} from "./MessageRenderers";
import { t, type Language } from "../../lib/i18n";
import type { ChatMessage, RuntimeReviewAction, UploadedFile } from "../../lib/types/ai-chat";

const { TextArea } = Input;
const { Text } = Typography;

export function ConversationMessageList({
  messages,
  typingMessageId,
  loading,
  language,
  serverSessionId,
  renderAttachmentIcon,
  renderDraftPanels,
  onContinueFromAction,
  onContinueFromMessage,
  onContinueFromRun,
  onContinueFromArtifact,
}: {
  messages: ChatMessage[];
  typingMessageId: string | null;
  loading: boolean;
  language: Language;
  serverSessionId?: string;
  renderAttachmentIcon: (contentType: string) => ReactNode;
  renderDraftPanels: (message: ChatMessage) => ReactNode;
  onContinueFromAction: (message: ChatMessage, action: RuntimeReviewAction) => void;
  onContinueFromMessage: (messageId: string) => void;
  onContinueFromRun: (runId: string) => void;
  onContinueFromArtifact: (artifactId: string) => void;
}) {
  return (
    <AnimatePresence>
      {messages.map((message, index) => (
        <motion.div
          key={message.id}
          initial={{ opacity: 0, y: 8 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.25, delay: index === messages.length - 1 ? 0 : 0 }}
          style={{
            display: "flex",
            justifyContent: message.role === "user" ? "flex-end" : "flex-start",
            marginBottom: 20,
          }}
        >
          <Space
            align="start"
            style={{
              flexDirection: message.role === "user" ? "row-reverse" : "row",
              maxWidth: "85%",
            }}
          >
            <Avatar
              icon={message.role === "user" ? <UserOutlined /> : <RobotOutlined />}
              style={{
                backgroundColor: message.role === "user" ? "#262626" : "#000",
                flexShrink: 0,
              }}
              size={32}
            />

            <div
              style={{
                padding: "12px 16px",
                borderRadius: message.role === "user" ? "16px 16px 4px 16px" : "16px 16px 16px 4px",
                backgroundColor: message.role === "user" ? "#000" : "#F7F7F7",
                border: message.role === "user" ? "none" : "1px solid #E5E5E5",
                color: message.role === "user" ? "#fff" : "#262626",
              }}
            >
              {message.role === "assistant" && typingMessageId === message.id ? (
                <TypewriterMessage
                  content={message.content}
                  sources={message.sources}
                  model={message.model}
                  thinking={message.thinking}
                  i18nLang={language}
                />
              ) : (
                <MessageContent
                  content={message.content}
                  sources={message.sources}
                  model={message.model}
                  thinking={message.thinking}
                  i18nLang={language}
                  role={message.role}
                />
              )}

              {message.role === "assistant" && message.agentMode && (
                <>
                  <AgentTracePanel plan={message.agentPlan} toolCalls={message.toolCalls} language={language} />
                  <AgentReviewPanel prompts={message.reviewPrompts} language={language} />
                </>
              )}

              {message.role === "assistant" && (
                <ReviewActionHistoryPanel
                  actions={message.reviewActions}
                  language={language}
                  onContinue={(action) => onContinueFromAction(message, action)}
                />
              )}

              {message.attachments && message.attachments.length > 0 && (
                <div style={{ marginTop: 8, display: "flex", gap: 6, flexWrap: "wrap" }}>
                  {message.attachments.map((attachment, idx) => (
                    <Tag
                      key={idx}
                      icon={renderAttachmentIcon(attachment.content_type)}
                      style={{
                        borderRadius: 4,
                        background: message.role === "user" ? "rgba(255,255,255,0.1)" : "#fff",
                        border: message.role === "user" ? "1px solid rgba(255,255,255,0.2)" : "1px solid #E5E5E5",
                        color: message.role === "user" ? "#fff" : "#262626",
                      }}
                    >
                      {attachment.original_name}
                    </Tag>
                  ))}
                </div>
              )}

              {renderDraftPanels(message)}

              {message.id !== "welcome" && serverSessionId && (
                <div
                  style={{
                    marginTop: 12,
                    paddingTop: 10,
                    borderTop: message.role === "user" ? "1px solid rgba(255,255,255,0.15)" : "1px solid #EAEAEA",
                    display: "flex",
                    gap: 8,
                    flexWrap: "wrap",
                  }}
                >
                  <Button
                    type="text"
                    size="small"
                    icon={<MessageOutlined />}
                    disabled={loading}
                    onClick={() => onContinueFromMessage(message.id)}
                    style={{
                      paddingInline: 8,
                      color: message.role === "user" ? "rgba(255,255,255,0.88)" : "#595959",
                    }}
                  >
                    {t("ai.continue_from_message", language)}
                  </Button>

                  {message.runId && (
                    <Button
                      type="text"
                      size="small"
                      icon={<ToolOutlined />}
                      disabled={loading}
                      onClick={() => onContinueFromRun(message.runId!)}
                      style={{
                        paddingInline: 8,
                        color: message.role === "user" ? "rgba(255,255,255,0.88)" : "#595959",
                      }}
                    >
                      {t("ai.continue_from_run", language)}
                    </Button>
                  )}

                  {(message.contractDraftArtifactId || message.paymentScheduleArtifactId) && (
                    <Button
                      type="text"
                      size="small"
                      disabled={loading}
                      onClick={() =>
                        onContinueFromArtifact(message.contractDraftArtifactId || message.paymentScheduleArtifactId!)
                      }
                      style={{
                        paddingInline: 8,
                        color: message.role === "user" ? "rgba(255,255,255,0.88)" : "#595959",
                      }}
                    >
                      {t("ai.continue_from_artifact", language)}
                    </Button>
                  )}
                </div>
              )}
            </div>
          </Space>
        </motion.div>
      ))}
    </AnimatePresence>
  );
}

export function ConversationLoadingState({ visible, language }: { visible: boolean; language: Language }) {
  if (!visible) return null;

  return (
    <motion.div
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      style={{ display: "flex", alignItems: "center", gap: 8, padding: "8px 0" }}
    >
      <Avatar icon={<RobotOutlined />} style={{ background: "#000", flexShrink: 0 }} size={32} />
      <div
        style={{
          padding: "12px 16px",
          borderRadius: "16px 16px 16px 4px",
          background: "#F7F7F7",
          border: "1px solid #E5E5E5",
        }}
      >
        <Spin />
        <span style={{ marginLeft: 8, fontSize: 13, color: "#8C8C8C" }}>{t("ai.thinking", language)}</span>
      </div>
    </motion.div>
  );
}

export function ChatComposer({
  input,
  loading,
  pendingUpload,
  language,
  onInputChange,
  onKeyDown,
  onFileUpload,
  onBeforeUpload,
  onSend,
  onClearUpload,
}: {
  input: string;
  loading: boolean;
  pendingUpload: UploadedFile | null;
  language: Language;
  onInputChange: (value: string) => void;
  onKeyDown: React.KeyboardEventHandler<HTMLTextAreaElement>;
  onFileUpload: (options: any) => Promise<void>;
  onBeforeUpload: (file: any) => any;
  onSend: () => void;
  onClearUpload: () => void;
}) {
  return (
    <div
      style={{
        padding: "16px 20%",
        borderTop: "1px solid #E5E5E5",
        background: "#fff",
        flexShrink: 0,
      }}
    >
      {pendingUpload && (
        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: 8,
            marginBottom: 8,
            padding: "6px 12px",
            background: "#F0F0F0",
            borderRadius: 8,
            fontSize: 13,
            color: "#595959",
          }}
        >
          <PaperClipOutlined style={{ fontSize: 14 }} />
          <span style={{ flex: 1, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
            {pendingUpload.original_name}
          </span>
          <Button
            type="text"
            size="small"
            icon={<CloseCircleOutlined style={{ fontSize: 14, color: "#8C8C8C" }} />}
            onClick={onClearUpload}
            style={{ padding: 0, height: 22, width: 22 }}
          />
        </div>
      )}

      <div
        style={{
          display: "flex",
          alignItems: "flex-end",
          gap: 8,
          background: "#F7F7F7",
          borderRadius: 24,
          padding: "8px 16px",
          border: "1px solid #E5E5E5",
          transition: "border-color 0.15s",
        }}
        className="chat-input-wrapper"
      >
        <Upload customRequest={onFileUpload} showUploadList={false} disabled={loading} beforeUpload={onBeforeUpload}>
          <Tooltip title={t("ai.upload_file_tooltip", language)}>
            <Button
              type="text"
              icon={<PaperClipOutlined style={{ fontSize: 18, color: "#8C8C8C" }} />}
              disabled={loading}
              style={{ height: 36, width: 36, padding: 0 }}
            />
          </Tooltip>
        </Upload>

        <TextArea
          value={input}
          onChange={(e) => onInputChange(e.target.value)}
          onKeyDown={onKeyDown}
          placeholder={t("ai.placeholder", language)}
          autoSize={{ minRows: 1, maxRows: 6 }}
          style={{
            flex: 1,
            background: "transparent",
            border: "none",
            boxShadow: "none",
            resize: "none",
            fontSize: 14,
            lineHeight: 1.6,
            padding: "6px 0",
          }}
          onFocus={(e) => {
            e.currentTarget.parentElement!.parentElement!.style.borderColor = "#000";
          }}
          onBlur={(e) => {
            e.currentTarget.parentElement!.parentElement!.style.borderColor = "#E5E5E5";
          }}
        />

        <Button
          type="primary"
          shape="circle"
          icon={<SendOutlined />}
          onClick={onSend}
          loading={loading}
          disabled={loading || (!input.trim() && !pendingUpload)}
          style={{
            width: 36,
            height: 36,
            flexShrink: 0,
            background: input.trim() || pendingUpload ? "#000" : "#D9D9D9",
            borderColor: input.trim() || pendingUpload ? "#000" : "#D9D9D9",
          }}
        />
      </div>

      <Text
        type="secondary"
        style={{
          fontSize: 11,
          textAlign: "center",
          display: "block",
          marginTop: 8,
          color: "#BFBFBF",
        }}
      >
        {t("ai.disclaimer", language)}
      </Text>
    </div>
  );
}
