"use client";

import { useState, useRef, useEffect, useMemo, Suspense } from "react";
import { useSearchParams } from "next/navigation";
import {
  Card,
  Input,
  Button,
  List,
  Avatar,
  Typography,
  Space,
  Tag,
  Spin,
  Upload,
  message,
  Tooltip,
} from "antd";
import {
  SendOutlined,
  UserOutlined,
  RobotOutlined,
  FileTextOutlined,
  PaperClipOutlined,
  FilePdfOutlined,
  FileExcelOutlined,
  FileImageOutlined,
} from "@ant-design/icons";
import AppLayout from "../components/AppLayout";
import ProtectedRoute from "../components/ProtectedRoute";
import { aiChatApi } from "../lib/api";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { t } from "../lib/i18n";

const { TextArea } = Input;
const { Text } = Typography;

interface UploadedFile {
  file_id: string;
  original_name: string;
  content_type: string;
  object_name?: string;
}

interface Message {
  id: string;
  role: "user" | "assistant";
  content: string;
  timestamp: Date;
  sources?: string[];
  attachments?: UploadedFile[];
  model?: string;
}

const contextChipMap: Record<string, string[]> = {
  "contract-detail": [
    "这份合同有什么风险？",
    "为什么不能计算？",
    "这份合同的关键会计影响是什么？",
  ],
  reports: [
    "帮我解释这张报表的结果",
    "这次查询用了什么口径？",
    "有哪些异常值得关注？",
  ],
  "monthly-closing": [
    "当前期间还有哪些阻塞项？",
    "这些分录主要来自什么？",
    "下一步建议做什么？",
  ],
};

const defaultChips = [
  "列出折现率缺失合同",
  "有哪些待审批事项？",
  "哪些合同即将到期？",
];

function AIChatPageContent() {
  const { token } = useAuth();
  const { language } = useLanguage();
  const searchParams = useSearchParams();

  const pageContext = useMemo(() => {
    const page = searchParams.get("page");
    if (!page) return undefined;
    return {
      page,
      title: searchParams.get("title") || undefined,
      contract_id: searchParams.get("contract_id") || undefined,
      period: searchParams.get("period") || undefined,
      report_view: searchParams.get("report_view") || undefined,
      tags: searchParams.getAll("tags"),
      summary: searchParams.get("summary") || undefined,
    };
  }, [searchParams]);

  const chips = pageContext ? (contextChipMap[pageContext.page!] || defaultChips) : defaultChips;

  const [messages, setMessages] = useState<Message[]>([
    {
      id: "welcome",
      role: "assistant",
      content: t("ai.welcome", language),
      timestamp: new Date(),
    },
  ]);
  const [input, setInput] = useState("");
  const [loading, setLoading] = useState(false);
  const [lastUploadedFile, setLastUploadedFile] = useState<UploadedFile | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  };

  useEffect(() => {
    scrollToBottom();
  }, [messages]);

  const getFileIcon = (type: string) => {
    if (type.includes("pdf")) return <FilePdfOutlined style={{ color: "#EF4444" }} />;
    if (type.includes("excel") || type.includes("sheet"))
      return <FileExcelOutlined style={{ color: "#10B981" }} />;
    return <FileImageOutlined style={{ color: "#666" }} />;
  };

  const handleFileUpload = async (options: any) => {
    const { file, onSuccess, onError, onProgress } = options;
    const formData = new FormData();
    formData.append("file", file);
    formData.append("file_type", "contract");

    try {
      const response = await fetch(`${window.location.origin}/api/ai/files/upload`, {
        method: "POST",
        headers: {
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
        body: formData,
      });

      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}));
        throw new Error(errorData.detail || `上传失败: ${response.status}`);
      }

      const data = await response.json();
      
      const uploadedFile: UploadedFile = {
        file_id: data.file_id,
        original_name: data.original_name,
        content_type: data.content_type,
        object_name: data.object_name,
      };

      setLastUploadedFile(uploadedFile);

      // Add a message showing the uploaded file
      const userMessage: Message = {
        id: Date.now().toString(),
        role: "user",
        content: `${t("ai.file_uploaded", language)}${data.original_name}`,
        timestamp: new Date(),
        attachments: [uploadedFile],
      };

      setMessages((prev) => [...prev, userMessage]);
      message.success(`${data.original_name} ${t("ai.upload_success", language)}`);

      onSuccess(data, file);
    } catch (err: any) {
      onError(err);
      message.error(`${t("ai.upload_failed", language)}: ${err.message}`);
    }
  };

  const handleSend = async (messageOverride?: string) => {
    const msg = messageOverride ?? input;
    if (!msg.trim() || !token) return;

    const userMessage: Message = {
      id: Date.now().toString(),
      role: "user",
      content: msg,
      timestamp: new Date(),
    };

    // Build conversation history from existing messages (last 10, excluding welcome)
    const existingMessages = [...messages];
    const history = existingMessages
      .filter(m => m.id !== "welcome")
      .slice(-10)
      .map(m => ({ role: m.role, content: m.content }));

    setMessages((prev) => [...prev, userMessage]);
    setInput("");
    setLoading(true);

    try {
      const chatData: any = { message: msg, history, language };
      if (lastUploadedFile) {
        chatData.file_id = lastUploadedFile.file_id;
        chatData.object_name = lastUploadedFile.object_name;
        chatData.content_type = lastUploadedFile.content_type;
        // Clear the uploaded file after sending
        setLastUploadedFile(null);
      }
      if (pageContext) {
        chatData.page_context = {
          page: pageContext.page,
          title: pageContext.title,
          contract_id: pageContext.contract_id,
          period: pageContext.period,
          report_view: pageContext.report_view,
          summary: pageContext.summary,
        };
      }
      const contractIdFromUrl = searchParams.get("contract_id");
      if (contractIdFromUrl) {
        chatData.contract_id = contractIdFromUrl;
      }

      const data = await aiChatApi.chat(chatData, token);
      const aiMessage: Message = {
        id: (Date.now() + 1).toString(),
        role: "assistant",
        content: data.answer || (language === "en" ? "Sorry, I couldn't understand your question." : "抱歉，我无法理解您的问题。"),
        timestamp: new Date(),
        sources: data.sources?.map((s: any) => s.title || s.type),
        model: data.model,
      };
      setMessages((prev) => [...prev, aiMessage]);
    } catch (error: any) {
      const errorMessage: Message = {
        id: (Date.now() + 1).toString(),
        role: "assistant",
        content: language === "en" ? `Request failed: ${error.message || "Unknown error"}` : `请求失败：${error.message || "未知错误"}`,
        timestamp: new Date(),
      };
      setMessages((prev) => [...prev, errorMessage]);
    } finally {
      setLoading(false);
    }
  };

  const handleChipClick = (question: string) => {
    handleSend(question);
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  return (
    <ProtectedRoute>
      <AppLayout>
        <Card
          title={
            <Space>
              <RobotOutlined style={{ color: "#000" }} />
              <span>{t("nav.ai_chat", language)}</span>
            </Space>
          }
          style={{ height: "calc(100vh - 140px)" }}
          bodyStyle={{ height: "calc(100% - 57px)", padding: 0 }}
        >
          {/* Context strip */}
          {pageContext && (
            <div
              style={{
                padding: "8px 16px",
                borderBottom: "1px solid #f0f0f0",
                background: "#FAFAFA",
              }}
            >
              <Space wrap size={[4, 4]}>
                <Text type="secondary" style={{ fontSize: 12, fontWeight: 500 }}>
                  {t("ai.context", language)}
                </Text>
                <Tag color="blue">{pageContext.title || pageContext.page}</Tag>
                {pageContext.contract_id && (
                  <Tag color="geekblue">{pageContext.contract_id}</Tag>
                )}
                {pageContext.period && (
                  <Tag color="purple">{pageContext.period}</Tag>
                )}
                {pageContext.report_view && (
                  <Tag color="cyan">{pageContext.report_view}</Tag>
                )}
                {pageContext.tags && pageContext.tags.length > 0 &&
                  pageContext.tags.map((t: string, i: number) => (
                    <Tag key={i} color="green">{t}</Tag>
                  ))
                }
              </Space>
            </div>
          )}

          {/* Quick question chips */}
          <div
            style={{
              padding: "8px 16px",
              borderBottom: "1px solid #f0f0f0",
              display: "flex",
              flexWrap: "wrap",
              gap: 6,
              alignItems: "center",
            }}
          >
            <Text type="secondary" style={{ fontSize: 12, whiteSpace: "nowrap", marginRight: 2 }}>
              {t("ai.quick_questions", language)}
            </Text>
            {chips.map((chip, idx) => (
              <Button
                key={idx}
                size="small"
                type="default"
                ghost
                onClick={() => handleChipClick(chip)}
                disabled={loading}
                style={{ fontSize: 12, borderRadius: 12 }}
              >
                {chip}
              </Button>
            ))}
          </div>

          <div
            style={{
              height: "calc(100% - 80px)",
              overflowY: "auto",
              padding: 16,
            }}
          >
            <List
              dataSource={messages}
              renderItem={(msg) => (
                <List.Item
                  style={{
                    justifyContent:
                      msg.role === "user" ? "flex-end" : "flex-start",
                    borderBottom: "none",
                    padding: "8px 0",
                  }}
                >
                  <Space
                    align="start"
                    style={{
                      flexDirection:
                        msg.role === "user" ? "row-reverse" : "row",
                    }}
                  >
                    <Avatar
                      icon={
                        msg.role === "user" ? <UserOutlined /> : <RobotOutlined />
                      }
                      style={{
                        backgroundColor:
                          msg.role === "user" ? "#666" : "#000",
                      }}
                    />
                    <div
                      style={{
                        maxWidth: 600,
                        padding: 12,
                        borderRadius: 8,
                        backgroundColor:
                          msg.role === "user" ? "#F5F5F5" : "#FAFAFA",
                      }}
                    >
                      <Text style={{ whiteSpace: "pre-wrap" }}>
                        {msg.content}
                      </Text>
                      {msg.attachments && msg.attachments.length > 0 && (
                        <div style={{ marginTop: 8 }}>
                          {msg.attachments.map((att, idx) => (
                            <Tag key={idx} icon={getFileIcon(att.content_type)}>
                              {att.original_name}
                            </Tag>
                          ))}
                        </div>
                      )}
                      {msg.sources && (
                        <div style={{ marginTop: 8 }}>
                          <Text type="secondary" style={{ fontSize: 12 }}>
                            {t("ai.sources", language)}
                          </Text>
                          {msg.sources.map((source, idx) => (
                            <Tag
                              key={idx}
                              icon={<FileTextOutlined />}
                            >
                              {source}
                            </Tag>
                          ))}
                        </div>
                      )}
                      {msg.model && (
                        <Text type="secondary" style={{ fontSize: 11, marginTop: 4, display: 'block' }}>
                          {t("ai.model", language)}{msg.model}
                        </Text>
                      )}
                    </div>
                  </Space>
                </List.Item>
              )}
            />
            {loading && (
              <div style={{ textAlign: "center", padding: 16 }}>
                <Spin tip={t("ai.thinking", language)} />
              </div>
            )}
            <div ref={messagesEndRef} />
          </div>

          <div
            style={{
              padding: 16,
              borderTop: "1px solid #f0f0f0",
              display: "flex",
              gap: 8,
              alignItems: "flex-end",
            }}
          >
            <Upload
              customRequest={handleFileUpload}
              showUploadList={false}
              beforeUpload={(file) => {
                const allowedTypes = [
                  "application/pdf",
                  "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
                  "application/vnd.ms-excel",
                  "image/jpeg",
                  "image/png",
                  "image/tiff",
                ];
                if (!allowedTypes.includes(file.type)) {
                  message.error(t("ai.unsupported_file", language));
                  return Upload.LIST_IGNORE;
                }
                const isLt50M = file.size / 1024 / 1024 < 50;
                if (!isLt50M) {
                  message.error(t("ai.file_too_large", language));
                  return Upload.LIST_IGNORE;
                }
                return true;
              }}
            >
              <Tooltip title="上传文件">
                <Button icon={<PaperClipOutlined />} />
              </Tooltip>
            </Upload>
            <TextArea
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder={t("ai.placeholder", language)}
              autoSize={{ minRows: 1, maxRows: 4 }}
              style={{ flex: 1 }}
            />
            <Button
              type="primary"
              icon={<SendOutlined />}
              onClick={() => handleSend()}
              loading={loading}
            >
              {t("ai.send", language)}
            </Button>
          </div>
        </Card>
      </AppLayout>
    </ProtectedRoute>
  );
}

export default function AIChatPage() {
  return (
    <Suspense fallback={<Spin size="large" style={{ display: "block", padding: 120, textAlign: "center" }} />}>
      <AIChatPageContent />
    </Suspense>
  );
}
