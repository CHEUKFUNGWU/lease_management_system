"use client";

import { Button, Typography } from "antd";

const { Title, Paragraph } = Typography;

export default function Home() {
  return (
    <main style={{ padding: 24, maxWidth: 1200, margin: "0 auto" }}>
      <Title>IFRS 16 租赁管理系统</Title>
      <Paragraph>
        欢迎使用 IFRS 16 租赁管理系统。系统支持合同管理、租赁计量、
        事件变更、会计分录、披露报表及 AI Agent 智能录入。
      </Paragraph>
      <Button type="primary">开始使用</Button>
    </main>
  );
}
