"use client";

import { Col, Form, Input, InputNumber, Modal, Row, Select } from "antd";
import type { FormInstance } from "antd";
import type { ObligationFormValues } from "./types";

interface ObligationModalProps {
  open: boolean;
  loading: boolean;
  form: FormInstance<ObligationFormValues>;
  onCancel: () => void;
  onSubmit: (values: ObligationFormValues) => void;
}

export function ObligationModal({
  open,
  loading,
  form,
  onCancel,
  onSubmit,
}: ObligationModalProps) {
  return (
    <Modal
      title="新增条款义务"
      open={open}
      onCancel={onCancel}
      onOk={() => form.submit()}
      confirmLoading={loading}
      width={720}
    >
      <Form
        form={form}
        layout="vertical"
        onFinish={onSubmit}
        initialValues={{ obligation_type: "notice", responsible_party: "lessee" }}
      >
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item label="义务类型" name="obligation_type" rules={[{ required: true, message: "请选择类型" }]}>
              <Select>
                <Select.Option value="maintenance">维修维护</Select.Option>
                <Select.Option value="cam">CAM / 管理费</Select.Option>
                <Select.Option value="insurance">保险</Select.Option>
                <Select.Option value="index_adjustment">指数调整</Select.Option>
                <Select.Option value="restoration">复原义务</Select.Option>
                <Select.Option value="security_deposit">押金</Select.Option>
                <Select.Option value="notice">通知义务</Select.Option>
                <Select.Option value="other">其他</Select.Option>
              </Select>
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item label="责任方" name="responsible_party" rules={[{ required: true, message: "请选择责任方" }]}>
              <Select>
                <Select.Option value="lessee">承租方</Select.Option>
                <Select.Option value="lessor">出租方</Select.Option>
                <Select.Option value="shared">双方共同</Select.Option>
                <Select.Option value="third_party">第三方</Select.Option>
              </Select>
            </Form.Item>
          </Col>
        </Row>
        <Form.Item label="标题" name="title" rules={[{ required: true, message: "请输入标题" }]}>
          <Input placeholder="例如：提前 6 个月提交续租通知" />
        </Form.Item>
        <Form.Item label="说明" name="description">
          <Input.TextArea rows={3} placeholder="记录义务内容、触发条件、管理动作或财务影响" />
        </Form.Item>
        <Row gutter={16}>
          <Col span={8}>
            <Form.Item label="来源页码" name="source_page">
              <InputNumber min={1} style={{ width: "100%" }} />
            </Form.Item>
          </Col>
          <Col span={16}>
            <Form.Item label="原文条款摘录" name="source_clause">
              <Input placeholder="粘贴合同条款原文，便于审计追溯" />
            </Form.Item>
          </Col>
        </Row>
      </Form>
    </Modal>
  );
}
