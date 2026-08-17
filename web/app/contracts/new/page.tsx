"use client";

import { useState, useEffect } from "react";
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
import PageHeader from "../../components/PageHeader";
import ProtectedRoute from "../../components/ProtectedRoute";
import { contractApi, legalEntityApi, masterDataApi, settingsApi } from "../../lib/api";
import { useAuth } from "../../context/AuthContext";
import { useLanguage } from "../../context/LanguageContext";
import { t } from "../../lib/i18n";
import { normalizeTagValues, DEFAULT_TAG_SUGGESTIONS } from "../../lib/tags";
import { notifyError } from "../../lib/notify";

interface LegalEntityOption {
  id: string;
  code: string;
  name: string;
}

interface StoreOption {
  id: string;
  code: string;
  name: string;
}

interface LandlordOption {
  id: string;
  code: string;
  name: string;
}

export default function NewContractPage() {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [drMissing, setDrMissing] = useState(false);
  const [legalEntities, setLegalEntities] = useState<LegalEntityOption[]>([]);
  const [stores, setStores] = useState<StoreOption[]>([]);
  const [landlords, setLandlords] = useState<LandlordOption[]>([]);
  const [masterDataLoading, setMasterDataLoading] = useState(false);
  const [storesLoading, setStoresLoading] = useState(false);
  const router = useRouter();
  const { token, user } = useAuth();
  const { language } = useLanguage();
  const selectedLegalEntityId = Form.useWatch("legal_entity_id", form);

  /* ---- load global discount rate as default ---- */
  useEffect(() => {
    if (!token) return;
    settingsApi
      .getGlobal(token)
      .then((res) => {
        const raw = Number(res.global_discount_rate);
        if (Number.isFinite(raw) && raw > 0) {
          const percent = raw > 1 ? raw : raw * 100;
          form.setFieldsValue({ discount_rate_value: percent });
        }
      })
      .catch(() => {
        setDrMissing(true);
      });
  }, [token, form]);

  useEffect(() => {
    if (!token) return;
    setMasterDataLoading(true);
    Promise.all([legalEntityApi.list(token), masterDataApi.listLandlords(token)])
      .then(([entitiesResponse, landlordsResponse]) => {
        setLegalEntities(entitiesResponse.legal_entities || []);
        setLandlords(landlordsResponse.landlords || []);
        if (user?.legal_entity_id) {
          form.setFieldsValue({ legal_entity_id: user.legal_entity_id });
        }
      })
      .catch((error: Error) => {
        notifyError(error.message || t("contract_new.create_failed", language));
      })
      .finally(() => setMasterDataLoading(false));
  }, [token, user?.legal_entity_id, form, language]);

  useEffect(() => {
    if (!token || !selectedLegalEntityId) {
      setStores([]);
      form.setFieldsValue({ store_id: undefined });
      return;
    }
    setStoresLoading(true);
    masterDataApi
      .listStores(token, selectedLegalEntityId)
      .then((response) => {
        const nextStores: StoreOption[] = response.stores || [];
        setStores(nextStores);
        const currentStoreId = form.getFieldValue("store_id");
        if (currentStoreId && !nextStores.some((store) => store.id === currentStoreId)) {
          form.setFieldsValue({ store_id: undefined });
        }
      })
      .catch((error: Error) => {
        setStores([]);
        notifyError(error.message || t("contract_new.create_failed", language));
      })
      .finally(() => setStoresLoading(false));
  }, [token, selectedLegalEntityId, form, language]);

  const handleSubmit = async (values: any) => {
    if (!token) {
      notifyError(t("contract_new.please_login", language));
      return;
    }

    const hasDiscountRateValue =
      typeof values.discount_rate_value === "number" && Number.isFinite(values.discount_rate_value) && values.discount_rate_value > 0;

    if (!hasDiscountRateValue) {
      setDrMissing(true);
      message.warning(t("contract_new.discount_rate_empty_warning", language));
    } else {
      setDrMissing(false);
    }

    setLoading(true);
    try {
      const data = {
        contract_number: values.contract_number,
        contract_name: values.contract_name,
        legal_entity_id: values.legal_entity_id,
        store_id: values.store_id,
        landlord_id: values.landlord_id,
        currency: values.currency,
        asset_type: values.asset_type,
        area_sqm: values.area_sqm ?? null,
        commencement_date: values.commencement_date?.format("YYYY-MM-DD"),
        lease_start_date: values.lease_start_date?.format("YYYY-MM-DD"),
        lease_end_date: values.lease_end_date?.format("YYYY-MM-DD"),
        asset_category: values.asset_category,
        property_category: values.property_category,
        discount_rate_type: values.discount_rate_type,
        discount_rate_version: values.discount_rate_version,
        discount_rate_value: values.discount_rate_value ?? null,
        lease_scope: values.lease_scope,
        exemption_reason: values.exemption_reason || null,
        scope_source: "manual",
        tags: normalizeTagValues(values.tags),
        discount_rate_missing: !hasDiscountRateValue,
      };

      await contractApi.create(data, token);
      message.success(t("contract_new.create_success", language));
      router.push("/contracts");
    } catch (error: any) {
      notifyError(error.message || t("contract_new.create_failed", language));
    } finally {
      setLoading(false);
    }
  };

  return (
    <ProtectedRoute>
      <AppLayout>
        <PageHeader
          title={t("contract_new.title", language)}

          primaryAction={
            <Button
              icon={<ArrowLeftOutlined />}
              onClick={() => router.push("/contracts")}
            >
              {t("contract_new.back", language)}
            </Button>
          }
        />

        <Card>
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
              <Select
                placeholder={t("contract_new.select_legal_entity", language)}
                loading={masterDataLoading}
                disabled={masterDataLoading || !!user?.legal_entity_id}
                options={legalEntities.map((entity) => ({
                  value: entity.id,
                  label: `${entity.code} - ${entity.name}`,
                }))}
              />
            </Form.Item>

            <Form.Item
              label={t("contract_new.store", language)}
              name="store_id"
              rules={[{ required: true, message: t("contract_new.please_select_store", language) }]}
            >
              <Select
                placeholder={t("contract_new.select_store", language)}
                loading={storesLoading}
                disabled={!selectedLegalEntityId}
                options={stores.map((store) => ({
                  value: store.id,
                  label: `${store.code} - ${store.name}`,
                }))}
              />
            </Form.Item>

            <Form.Item
              label={t("contract_new.lessor", language)}
              name="landlord_id"
              rules={[{ required: true, message: t("contract_new.please_select_lessor", language) }]}
            >
              <Select
                placeholder={t("contract_new.select_lessor", language)}
                loading={masterDataLoading}
                options={landlords.map((landlord) => ({
                  value: landlord.id,
                  label: `${landlord.code} - ${landlord.name}`,
                }))}
              />
            </Form.Item>

            <Form.Item
              label={t("contract_new.currency", language)}
              name="currency"
              rules={[{ required: true }]}
            >
              <Input placeholder="CNY" />
            </Form.Item>

            <Form.Item label="资产类型" name="asset_type" rules={[{ required: true, message: "请选择资产类型" }]}>
              <Select>
                <Select.Option value="real_estate">不动产</Select.Option>
                <Select.Option value="vehicle">车辆</Select.Option>
                <Select.Option value="it_equipment">IT 设备</Select.Option>
                <Select.Option value="machinery">机器设备</Select.Option>
                <Select.Option value="other">其他</Select.Option>
              </Select>
            </Form.Item>

            <Form.Item
              label={t("contract_new.area_sqm", language)}
              name="area_sqm"
              tooltip={t("contract_new.area_sqm_help", language)}
            >
              <InputNumber
                min={0}
                step={10}
                style={{ width: "100%" }}
                placeholder={t("contract_new.area_sqm_placeholder", language)}
                addonAfter="㎡"
              />
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

            <Card title="IFRS 16 范围判定" size="small" style={{ marginBottom: 16 }}>
              <Space direction="vertical" style={{ width: "100%" }}>
                <Alert
                  message="范围闸门"
                  description="先判断合同是否需要资本化。短期租赁、低价值资产和非租赁合同不会生成 ROU 与租赁负债。"
                  type="info"
                  showIcon
                />
                <Form.Item
                  label="计量范围"
                  name="lease_scope"
                  rules={[{ required: true, message: "请选择 IFRS 16 计量范围" }]}
                >
                  <Select>
                    <Select.Option value="in_scope">资本化租赁（ROU + 租赁负债）</Select.Option>
                    <Select.Option value="short_term_exempt">短期租赁豁免（直线法费用化）</Select.Option>
                    <Select.Option value="low_value_exempt">低价值资产豁免（直线法费用化）</Select.Option>
                    <Select.Option value="not_a_lease">非租赁合同（不入 IFRS 16）</Select.Option>
                  </Select>
                </Form.Item>
                <Form.Item label="豁免/排除依据" name="exemption_reason">
                  <Input.TextArea rows={2} placeholder="例如：租期 10 个月且无续租意图；未识别特定资产；低价值 IT 设备" />
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
