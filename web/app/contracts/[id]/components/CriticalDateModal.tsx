"use client";

import { Col, DatePicker, Form, Input, InputNumber, Modal, Row, Select } from "antd";
import type { FormInstance } from "antd";
import type { CriticalDateFormValues } from "./types";

interface CriticalDateModalProps {
  open: boolean;
  loading: boolean;
  form: FormInstance<CriticalDateFormValues>;
  onCancel: () => void;
  onSubmit: (values: CriticalDateFormValues) => void;
}

export function CriticalDateModal({
  open,
  loading,
  form,
  onCancel,
  onSubmit,
}: CriticalDateModalProps) {
  return (
    <Modal
      title="新增关键日期"
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
        initialValues={{ date_type: "renewal_deadline", reminder_days: 30 }}
      >
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item label="类型" name="date_type" rules={[{ required: true, message: "请选择类型" }]}>
              <Select>
                <Select.Option value="renewal_deadline">续租截止</Select.Option>
                <Select.Option value="break_notice">Break 通知</Select.Option>
                <Select.Option value="rent_review">租金 Review</Select.Option>
                <Select.Option value="lease_expiry">租约到期</Select.Option>
                <Select.Option value="insurance_renewal">保险续保</Select.Option>
                <Select.Option value="other">其他</Select.Option>
              </Select>
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item label="目标日期" name="target_date" rules={[{ required: true, message: "请选择目标日期" }]}>
              <DatePicker style={{ width: "100%" }} />
            </Form.Item>
          </Col>
        </Row>
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item label="提前提醒天数" name="reminder_days">
              <InputNumber min={0} max={365} style={{ width: "100%" }} />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item label="标题" name="title" rules={[{ required: true, message: "请输入标题" }]}>
              <Input placeholder="例如：续租通知截止日" />
            </Form.Item>
          </Col>
        </Row>
        <Form.Item label="说明" name="description">
          <Input.TextArea rows={3} placeholder="记录条款依据、通知期、责任人或操作建议" />
        </Form.Item>
      </Form>
    </Modal>
  );
}
