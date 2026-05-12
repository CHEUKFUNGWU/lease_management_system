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
} from "antd";
import {
  SendOutlined,
  UserOutlined,
  RobotOutlined,
  FileTextOutlined,
} from "@ant-design/icons";
import AppLayout from "../components/AppLayout";
import ProtectedRoute from "../components/ProtectedRoute";

const { TextArea } = Input;
const { Text } = Typography;

interface Message {
  id: string;
  role: "user" | "assistant";
  content: string;
  timestamp: Date;
  sources?: string[];
}

export default function AIChatPage() {
  const [messages, setMessages] = useState<Message[]>([
    {
      id: "welcome",
      role: "assistant",
      content:
        "你好！我是 IFRS 16 AI 助手。我可以帮你：\n\n1. 识别和解析租赁合同\n2. 提取付款计划\n3. 判断合同变更类型\n4. 回答 IFRS 16 相关问题\n\n请直接描述你的需求或上传文件。",
      timestamp: new Date(),
    },
  ]);
  const [input, setInput] = useState("");
  const [loading, setLoading] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  };

  useEffect(() => {
    scrollToBottom();
  }, [messages]);

  const handleSend = async () => {
    if (!input.trim()) return;

    const userMessage: Message = {
      id: Date.now().toString(),
      role: "user",
      content: input,
      timestamp: new Date(),
    };

    setMessages((prev) => [...prev, userMessage]);
    setInput("");
    setLoading(true);

    // TODO: 调用 AI Service API
    setTimeout(() => {
      const aiMessage: Message = {
        id: (Date.now() + 1).toString(),
        role: "assistant",
        content: `收到你的问题："${input}"\n\n我正在处理中。实际实现时会调用 AI Service 接口。`,
        timestamp: new Date(),
      };
      setMessages((prev) => [...prev, aiMessage]);
      setLoading(false);
    }, 1000);
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
              <RobotOutlined style={{ color: "#1890ff" }} />
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
                          msg.role === "user" ? "#87d068" : "#1890ff",
                      }}
                    />
                    <div
                      style={{
                        maxWidth: 600,
                        padding: 12,
                        borderRadius: 8,
                        backgroundColor:
                          msg.role === "user" ? "#f6ffed" : "#f0f5ff",
                      }}
                    >
                      <Text style={{ whiteSpace: "pre-wrap" }}>
                        {msg.content}
                      </Text>
                      {msg.sources && (
                        <div style={{ marginTop: 8 }}>
                          <Text type="secondary" style={{ fontSize: 12 }}>
                            引用来源:
                          </Text>
                          {msg.sources.map((source, idx) => (
                            <Tag
                              key={idx}
                              size="small"
                              icon={<FileTextOutlined />}
                            >
                              {source}
                            </Tag>
                          ))}
                        </div>
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
            }}
          >
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
