"use client";

import { Card, Form, Input, Button, message } from "antd";
import AppLayout from "../components/AppLayout";
import ProtectedRoute from "../components/ProtectedRoute";

export default function SettingsPage() {
  const [form] = Form.useForm();

  const handleSave = () => {
    message.success("设置已保存");
  };

  return (
    <ProtectedRoute>
      <AppLayout>
        <h1 style={{ marginBottom: 24 }}>系统设置</h1>

        <Card title="基本设置">
          <Form form={form} layout="vertical" onFinish={handleSave}>
            <Form.Item label="系统名称" name="systemName" initialValue="IFRS 16 租赁管理系统">
              <Input />
            </Form.Item>

            <Form.Item label="默认币种" name="defaultCurrency" initialValue="CNY">
              <Input />
            </Form.Item>

            <Form.Item label="联系方式" name="contact">
              <Input placeholder="技术支持联系方式" />
            </Form.Item>

            <Form.Item>
              <Button type="primary" htmlType="submit">
                保存设置
              </Button>
            </Form.Item>
          </Form>
        </Card>
      </AppLayout>
    </ProtectedRoute>
  );
}
