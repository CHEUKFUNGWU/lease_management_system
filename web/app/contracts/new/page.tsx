"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import {
  Form,
  Input,
  InputNumber,
  Button,
  Card,
  DatePicker,
  Select,
  message,
  Alert,
  Tag,
  Space,
} from "antd";
import { ArrowLeftOutlined, SaveOutlined, ExclamationCircleOutlined } from "@ant-design/icons";
import AppLayout from "../../components/AppLayout";
import ProtectedRoute from "../../components/ProtectedRoute";
import { contractApi } from "../../lib/api";
import { useAuth } from "../../context/AuthContext";
import { useLanguage } from "../../context/LanguageContext";
import { t } from "../../lib/i18n";
import { normalizeTagValues, DEFAULT_TAG_SUGGESTIONS } from "../../lib/tags";

export default function NewContractPage() {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [drMissing, setDrMissing] = useState(false);
  const router = useRouter();
  const { token } = useAuth();
  const { language } = useLanguage();

  const handleSubmit = async (values: any) => {
    if (!token) {
      message.error(t("contract_new.please_login", language));
      return;
    }

    // Check discount rate
    if (!values.discount_rate_type) {
      setDrMissing(true);
      message.warning(t("contract_new.discount_rate_empty_warning", language));
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
        discount_rate_type: values.discount_rate_type,
        discount_rate_version: values.discount_rate_version,
        discount_rate_value: values.discount_rate_value ?? null,
        tags: normalizeTagValues(values.tags),
        discount_rate_missing: !values.discount_rate_type,
      };

      await contractApi.create(data, token);
      message.success(t("contract_new.create_success", language));
      router.push("/contracts");
    } catch (error: any) {
      message.error(error.message || t("contract_new.create_failed", language));
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
            {t("contract_new.back", language)}
          </Button>
        </div>

        <Card title={t("contract_new.title", language)}>
          {drMissing && (
            <Alert
              message={t("contract_new.discount_rate_missing_title", language)}
              description={t("contract_new.discount_rate_missing_desc", language)}
              type="warning"
              showIcon
              icon={<ExclamationCircleOutlined />}
              style={{ marginBottom: 16 }}
            />
          )}

          <Form
            form={form}
            layout="vertical"
            onFinish={handleSubmit}
            initialValues={{ currency: "CNY" }}
          >
            <Form.Item
              label={t("contract_new.contract_number", language)}
              name="contract_number"
              rules={[{ required: true, message: t("contract_new.please_enter_number", language) }]}
            >
              <Input placeholder={t("contract_new.contract_number_placeholder", language)} />
            </Form.Item>

            <Form.Item
              label={t("contract_new.contract_name", language)}
              name="contract_name"
              rules={[{ required: true, message: t("contract_new.please_enter_name", language) }]}
            >
              <Input placeholder={t("contract_new.contract_name_placeholder", language)} />
            </Form.Item>

            <Form.Item
              label={t("contract_new.legal_entity", language)}
              name="legal_entity_id"
              rules={[{ required: true, message: t("contract_new.please_select_entity", language) }]}
            >
              <Select placeholder={t("contract_new.select_legal_entity", language)}>
                <Select.Option value="a1b2c3d4-e5f6-7890-abcd-ef1234567890">
                  {t("contract_new.entity_le001", language)}
                </Select.Option>
                <Select.Option value="b2c3d4e5-f6a7-8901-bcde-f12345678901">
                  {t("contract_new.entity_le002", language)}
                </Select.Option>
              </Select>
            </Form.Item>

            <Form.Item
              label={t("contract_new.store", language)}
              name="store_id"
              rules={[{ required: true, message: t("contract_new.please_select_store", language) }]}
            >
              <Select placeholder={t("contract_new.select_store", language)}>
                <Select.Option value="c3d4e5f6-a7b8-9012-cdef-123456789012">
                  {t("contract_new.store_nanjing", language)}
                </Select.Option>
                <Select.Option value="d4e5f6a7-b8c9-0123-def1-234567890123">
                  {t("contract_new.store_huaihai", language)}
                </Select.Option>
              </Select>
            </Form.Item>

            <Form.Item
              label={t("contract_new.lessor", language)}
              name="landlord_id"
              rules={[{ required: true, message: t("contract_new.please_select_lessor", language) }]}
            >
              <Select placeholder={t("contract_new.select_lessor", language)}>
                <Select.Option value="e5f6a7b8-c9d0-1234-ef12-345678901234">
                  {t("contract_new.lessor_shanghai", language)}
                </Select.Option>
                <Select.Option value="f6a7b8c9-d0e1-2345-f123-456789012345">
                  {t("contract_new.lessor_beijing", language)}
                </Select.Option>
              </Select>
            </Form.Item>

            <Form.Item
              label={t("contract_new.currency", language)}
              name="currency"
              rules={[{ required: true }]}
            >
              <Select>
                <Select.Option value="CNY">{t("contract_new.currency_cny", language)}</Select.Option>
                <Select.Option value="USD">{t("contract_new.currency_usd", language)}</Select.Option>
                <Select.Option value="EUR">{t("contract_new.currency_eur", language)}</Select.Option>
              </Select>
            </Form.Item>

            <Form.Item
              label={t("contract_new.tags", language)}
              name="tags"
              tooltip={t("contract_new.tags_help", language)}
            >
              <Select
                mode="tags"
                tokenSeparators={[",", "，", ";", "；", " ", "|"]}
                placeholder={t("contract_new.tags_placeholder", language)}
                options={DEFAULT_TAG_SUGGESTIONS.map((tag) => ({
                  value: tag,
                  label: tag,
                }))}
              />
            </Form.Item>

            <Form.Item
              label={t("contract_new.commencement_date", language)}
              name="commencement_date"
              rules={[{ required: true, message: t("contract_new.please_select_date", language) }]}
            >
              <DatePicker style={{ width: "100%" }} />
            </Form.Item>

            <Form.Item
              label={t("contract_new.lease_start_date", language)}
              name="lease_start_date"
              rules={[{ required: true, message: t("contract_new.please_select_start", language) }]}
            >
              <DatePicker style={{ width: "100%" }} />
            </Form.Item>

            <Form.Item
              label={t("contract_new.lease_end_date", language)}
              name="lease_end_date"
              rules={[{ required: true, message: t("contract_new.please_select_end", language) }]}
            >
              <DatePicker style={{ width: "100%" }} />
            </Form.Item>

            <Card title={t("contract_new.discount_rate_section", language)} size="small" style={{ marginBottom: 16 }}>
              <Space direction="vertical" style={{ width: "100%" }}>
                <Alert
                  message={t("contract_new.discount_rate_tip", language)}
                  description={t("contract_new.discount_rate_tip_desc", language)}
                  type="info"
                  showIcon
                />

                <Form.Item
                  label={t("contract_new.discount_rate_value", language)}
                  name="discount_rate_value"
                  help={t("contract_new.discount_rate_help", language)}
                >
                  <InputNumber
                    style={{ width: "100%" }}
                    min={0}
                    step={0.01}
                    placeholder={t("contract_new.discount_rate_placeholder", language)}
                  />
                </Form.Item>
                
                <Form.Item
                  label={t("contract_new.discount_rate_type", language)}
                  name="discount_rate_type"
                >
                  <Select placeholder={t("contract_new.select_discount_rate_type", language)} allowClear>
                    <Select.Option value="ibr">{t("contract_new.rate_type_group_ibr", language)}</Select.Option>
                    <Select.Option value="entity_specific">{t("contract_new.rate_type_entity", language)}</Select.Option>
                    <Select.Option value="contract_specific">{t("contract_new.rate_type_contract", language)}</Select.Option>
                    <Select.Option value="implicit_rate">{t("contract_new.rate_type_implicit", language)}</Select.Option>
                  </Select>
                </Form.Item>

                <Form.Item
                  label={t("contract_new.discount_rate_version", language)}
                  name="discount_rate_version"
                >
                  <Input placeholder={t("contract_new.discount_rate_version_placeholder", language)} />
                </Form.Item>
              </Space>
            </Card>

            <Form.Item>
              <Button
                type="primary"
                htmlType="submit"
                icon={<SaveOutlined />}
                loading={loading}
                size="large"
              >
                {t("contract_new.create_button", language)}
              </Button>
            </Form.Item>
          </Form>
        </Card>
      </AppLayout>
    </ProtectedRoute>
  );
}
