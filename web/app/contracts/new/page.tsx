"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import {
  Form,
  Input,
  Button,
  Card,
  DatePicker,
  Select,
  message,
  InputNumber,
} from "antd";
import { ArrowLeftOutlined, SaveOutlined } from "@ant-design/icons";
import AppLayout from "../components/AppLayout";
import ProtectedRoute from "../components/ProtectedRoute";
import { contractApi } from "../lib/api";
import { useAuth } from "../context/AuthContext";

export default function NewContractPage() {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const router = useRouter();
  const { token } = useAuth();

  const handleSubmit = async (values: any) => {
    if (!token) {
      message.error("请先登录");
      return;
    }

    setLoading(true);
    try {
      const data = {
        contract_number: values.contract_number,
        contract_name: values.contract_name,
        legal_entity_id: values.legal_entity_id,
        store_id: values.store_id,
        landlord_id: values.landlord_id,
        currency: values.currency || "CNY",
        commencement_date: values.commencement_date?.format("YYYY-MM-DD"),
        lease_start_date: values.lease_start_date?.format("YYYY-MM-DD"),
        lease_end_date: values.lease_end_date?.format("YYYY-MM-DD"),
        asset_category: values.asset_category,
        property_category: values.property_category,
      };

      await contractApi.create(data, token);
      message.success("合同创建成功！");
      router.push("/contracts");
    } catch (error: any) {
      message.error(error.message || "创建失败");
    } finally {
      setLoading(false);
    }
  };

  return (
    <ProtectedRoute>
      <AppLayout>
        <div style={{ marginBottom: 16 }}>
          <Button
            icon={<ArrowLeftOutlined />}
            onClick={() => router.push("/contracts")}
          >
            返回
          </Button>
        </div>

        <Card title="新增合同">
          <Form
            form={form}
            layout="vertical"
            onFinish={handleSubmit}
            initialValues={{ currency: "CNY" }}
          >
            <Form.Item
              label="合同编号"
              name="contract_number"
              rules={[{ required: true, message: "请输入合同编号" }]}
            >
              <Input placeholder="例如：LEASE-2024-001" />
            </Form.Item>

            <Form.Item
              label="合同名称"
              name="contract_name"
              rules={[{ required: true, message: "请输入合同名称" }]}
            >
              <Input placeholder="例如：南京东路旗舰店租赁合同" />
            </Form.Item>

            <Form.Item
              label="法人主体"
              name="legal_entity_id"
              rules={[{ required: true, message: "请选择法人主体" }]}
            >
              <Select placeholder="选择法人主体">
                <Select.Option value="a1b2c3d4-e5f6-7890-abcd-ef1234567890">
                  零售集团总公司
                </Select.Option>
                <Select.Option value="b2c3d4e5-f6a7-8901-bcde-f12345678901">
                  零售集团上海公司
                </Select.Option>
              </Select>
            </Form.Item>

            <Form.Item
              label="门店"
              name="store_id"
              rules={[{ required: true, message: "请选择门店" }]}
            >
              <Select placeholder="选择门店">
                <Select.Option value="c3d4e5f6-a7b8-9012-cdef-123456789012">
                  南京东路旗舰店
                </Select.Option>
                <Select.Option value="d4e5f6a7-b8c9-0123-def1-234567890123">
                  淮海路店
                </Select.Option>
              </Select>
            </Form.Item>

            <Form.Item
              label="出租方"
              name="landlord_id"
              rules={[{ required: true, message: "请选择出租方" }]}
            >
              <Select placeholder="选择出租方">
                <Select.Option value="e5f6a7b8-c9d0-1234-ef12-345678901234">
                  上海商业地产集团
                </Select.Option>
                <Select.Option value="f6a7b8c9-d0e1-2345-f123-456789012345">
                  北京购物中心管理
                </Select.Option>
              </Select>
            </Form.Item>

            <Form.Item
              label="币种"
              name="currency"
              rules={[{ required: true }]}
            >
              <Select>
                <Select.Option value="CNY">人民币 (CNY)</Select.Option>
                <Select.Option value="USD">美元 (USD)</Select.Option>
                <Select.Option value="EUR">欧元 (EUR)</Select.Option>
              </Select>
            </Form.Item>

            <Form.Item
              label="租赁起始日 (Commencement Date)"
              name="commencement_date"
              rules={[{ required: true, message: "请选择租赁起始日" }]}
            >
              <DatePicker style={{ width: "100%" }} />
            </Form.Item>

            <Form.Item
              label="租赁开始日 (Lease Start Date)"
              name="lease_start_date"
              rules={[{ required: true, message: "请选择租赁开始日" }]}
            >
              <DatePicker style={{ width: "100%" }} />
            </Form.Item>

            <Form.Item
              label="租赁结束日 (Lease End Date)"
              name="lease_end_date"
              rules={[{ required: true, message: "请选择租赁结束日" }]}
            >
              <DatePicker style={{ width: "100%" }} />
            </Form.Item>

            <Form.Item>
              <Button
                type="primary"
                htmlType="submit"
                icon={<SaveOutlined />}
                loading={loading}
                size="large"
              >
                创建合同
              </Button>
            </Form.Item>
          </Form>
        </Card>
      </AppLayout>
    </ProtectedRoute>
  );
}
