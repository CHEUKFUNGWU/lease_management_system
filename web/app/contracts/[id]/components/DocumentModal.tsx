"use client";

import { Col, Form, Input, Modal, Row, Select } from "antd";
import type { FormInstance } from "antd";
import type { DocumentFormValues } from "./types";

interface DocumentModalProps {
  open: boolean;
  loading: boolean;
  form: FormInstance<DocumentFormValues>;
  onCancel: () => void;
  onSubmit: (values: DocumentFormValues) => void;
}

export function DocumentModal({
  open,
  loading,
  form,
  onCancel,
  onSubmit,
}: DocumentModalProps) {
  return (
    <Modal
      title="新增文档记录"
      open={open}
      onCancel={onCancel}
      onOk={() => form.submit()}
      confirmLoading={loading}
      width={600}
    >
      <Form
        form={form}
        layout="vertical"
        onFinish={onSubmit}
        initialValues={{ document_type: "main_contract" }}
      >
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item label="文档类型" name="document_type">
              <Select>
                <Select.Option value="main_contract">主合同</Select.Option>
                <Select.Option value="amendment">补充协议</Select.Option>
                <Select.Option value="side_letter">Side Letter</Select.Option>
                <Select.Option value="invoice">发票/账单</Select.Option>
                <Select.Option value="other">其他</Select.Option>
              </Select>
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item label="版本" name="document_version">
              <Input placeholder="例如：v1 / 2024-签署版" />
            </Form.Item>
          </Col>
        </Row>
        <Form.Item label="文件名" name="file_name" rules={[{ required: true, message: "请输入文件名" }]}>
          <Input placeholder="例如：LEASE-2024-001 主合同.pdf" />
        </Form.Item>
        <Form.Item label="文件类型" name="file_type">
          <Input placeholder="application/pdf" />
        </Form.Item>
        <Form.Item label="备注" name="notes">
          <Input.TextArea rows={3} placeholder="记录文件来源、关键条款页码或归档说明" />
        </Form.Item>
      </Form>
    </Modal>
  );
}
