"use client";

import { useState, useRef, useEffect } from "react";
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

export default function AIChatPage() {
  const { token } = useAuth();
  const [messages, setMessages] = useState<Message[]>([
    {
      id: "welcome",
      role: "assistant",
      content:
        "你好！我是 IFRS 16 AI 助手。我可以帮你：\n\n1. 查询合同台账信息\n2. 查看 IFRS 16 计量结果\n3. 了解审批状态\n4. 回答 IFRS 16 会计问题\n\n你还可以上传合同文件或租金表，我会帮你解析其中的关键信息。",
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
        content: `已上传文件: ${data.original_name}`,
        timestamp: new Date(),
        attachments: [uploadedFile],
      };

      setMessages((prev) => [...prev, userMessage]);
      message.success(`${data.original_name} 上传成功`);
      
      onSuccess(data, file);
    } catch (err: any) {
      onError(err);
      message.error(`上传失败: ${err.message}`);
    }
  };

  const handleSend = async () => {
    if (!input.trim() || !token) return;

    const userMessage: Message = {
      id: Date.now().toString(),
      role: "user",
      content: input,
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
      const chatData: any = { message: input, history };
      if (lastUploadedFile) {
        chatData.file_id = lastUploadedFile.file_id;
        chatData.object_name = lastUploadedFile.object_name;
        chatData.content_type = lastUploadedFile.content_type;
        // Clear the uploaded file after sending
        setLastUploadedFile(null);
      }

      const data = await aiChatApi.chat(chatData, token);
      const aiMessage: Message = {
        id: (Date.now() + 1).toString(),
        role: "assistant",
        content: data.answer || "抱歉，我无法理解您的问题。",
        timestamp: new Date(),
        sources: data.sources?.map((s: any) => s.title || s.type),
        model: data.model,
      };
      setMessages((prev) => [...prev, aiMessage]);
    } catch (error: any) {
      const errorMessage: Message = {
        id: (Date.now() + 1).toString(),
        role: "assistant",
        content: `请求失败：${error.message || "未知错误"}`,
        timestamp: new Date(),
      };
      setMessages((prev) => [...prev, errorMessage]);
    } finally {
      setLoading(false);
    }
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
              <span>AI 助手</span>
            </Space>
          }
          style={{ height: "calc(100vh - 140px)" }}
          bodyStyle={{ height: "calc(100% - 57px)", padding: 0 }}
        >
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
                            引用来源:
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
                          模型: {msg.model}
                        </Text>
                      )}
                    </div>
                  </Space>
                </List.Item>
              )}
            />
            {loading && (
              <div style={{ textAlign: "center", padding: 16 }}>
                <Spin tip="AI 思考中..." />
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
                  message.error("不支持的文件类型，请上传 PDF、Excel 或图片文件");
                  return Upload.LIST_IGNORE;
                }
                const isLt50M = file.size / 1024 / 1024 < 50;
                if (!isLt50M) {
                  message.error("文件大小不能超过 50MB");
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
              placeholder="输入问题，按 Enter 发送，Shift+Enter 换行..."
              autoSize={{ minRows: 1, maxRows: 4 }}
              style={{ flex: 1 }}
            />
            <Button
              type="primary"
              icon={<SendOutlined />}
              onClick={handleSend}
              loading={loading}
            >
              发送
            </Button>
          </div>
        </Card>
      </AppLayout>
    </ProtectedRoute>
  );
}
