"use client";

import { useState, useCallback } from "react";
import { useRouter } from "next/navigation";
import dayjs, { Dayjs } from "dayjs";
import {
  Upload,
  Button,
  Card,
  Form,
  Input,
  InputNumber,
  DatePicker,
  Select,
  Tag,
  Alert,
  Steps,
  Spin,
  message,
  Space,
  Row,
  Col,
  Typography,
  Divider,
  Result,
  Tooltip,
  Table,
  Modal,
  Progress,
} from "antd";
import type { ColumnsType } from "antd/es/table";
import {
  UploadOutlined,
  FilePdfOutlined,
  FileExcelOutlined,
  FileImageOutlined,
  LoadingOutlined,
  CheckCircleOutlined,
  EditOutlined,
  ArrowRightOutlined,
  ReloadOutlined,
  WarningOutlined,
  InfoCircleOutlined,
  FileOutlined,
  ExclamationCircleOutlined,
  CloseCircleOutlined,
  ThunderboltOutlined,
  CloudUploadOutlined,
  OrderedListOutlined,
} from "@ant-design/icons";
import AppLayout from "../components/AppLayout";
import ProtectedRoute from "../components/ProtectedRoute";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { t } from "../lib/i18n";
import { aiRequest, contractApi } from "../lib/api";

const { Dragger } = Upload;
const { Text, Title, Paragraph } = Typography;

const AI_BASE_URL =
  process.env.NEXT_PUBLIC_AI_URL || "http://localhost:8081";

// ── Helpers ──
function confidenceColor(score: number | undefined): string {
  if (score === undefined || score === null) return "default";
  if (score >= 0.8) return "green";
  if (score >= 0.6) return "gold";
  return "red";
}

function getFileIcon(type: string) {
  if (!type) return <FileOutlined />;
  if (type.includes("pdf"))
    return <FilePdfOutlined style={{ color: "#EF4444" }} />;
  if (type.includes("excel") || type.includes("sheet"))
    return <FileExcelOutlined style={{ color: "#10B981" }} />;
  return <FileImageOutlined style={{ color: "#666" }} />;
}

// ── Types ──
interface BatchItem {
  file_id: string;
  object_name: string;
  original_name: string;
  content_type: string;
  file_size: number;
  status: "uploading" | "uploaded" | "parsing" | "parsed" | "error";
  parseResult?: any;
  error?: string;
  formData?: Record<string, any>;
  createdContract?: any;
}

// ── Form field defaults (used when parseResult is missing values) ──
function defaultFormData(): Record<string, any> {
  return { currency: "CNY" };
}

// ── Component ──
export default function UploadPage() {
  const { token } = useAuth();
  const { language } = useLanguage();
  const router = useRouter();

  // step: 0 = upload, 1 = parsing, 2 = batch review, 3 = complete
  const [step, setStep] = useState(0);
  const [batchItems, setBatchItems] = useState<BatchItem[]>([]);
  const [parsingIndex, setParsingIndex] = useState(0);
  const [parsingTotal, setParsingTotal] = useState(0);
  const [selectedKeys, setSelectedKeys] = useState<React.Key[]>([]);
  const [editingIndex, setEditingIndex] = useState<number | null>(null);
  const [creating, setCreating] = useState(false);
  const [createResult, setCreateResult] = useState<{
    success: number;
    failed: number;
    contracts: any[];
  } | null>(null);

  const [editForm] = Form.useForm();

  // ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  // Step 0: Upload multiple files
  // ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  const handleUploadFile = async (file: File): Promise<void> => {
    const tempId = `temp-${Date.now()}-${Math.random().toString(36).slice(2)}`;

    // Add item with "uploading" status
    const newItem: BatchItem = {
      file_id: tempId,
      object_name: "",
      original_name: file.name,
      content_type: file.type,
      file_size: file.size,
      status: "uploading",
    };
    setBatchItems((prev) => [...prev, newItem]);

    const formData = new FormData();
    formData.append("file", file);
    formData.append("task_type", "contract");

    try {
      const response = await fetch(`${AI_BASE_URL}/api/v1/files/upload`, {
        method: "POST",
        headers: {
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
        body: formData,
      });

      if (!response.ok) {
        const err = await response.json().catch(() => ({}));
        throw new Error(err.detail || `Upload failed: ${response.status}`);
      }

      const data = await response.json();

      // Update item with server response, mark as "uploaded"
      setBatchItems((prev) =>
        prev.map((it) =>
          it.file_id === tempId
            ? {
                ...it,
                file_id: data.file_id,
                object_name: data.object_name,
                content_type: data.content_type || file.type,
                file_size: data.file_size || file.size,
                status: "uploaded",
              }
            : it
        )
      );
      message.success(t("upload.upload_success", language, { name: file.name }));
    } catch (err: any) {
      setBatchItems((prev) =>
        prev.map((it) =>
          it.file_id === tempId
            ? {
                ...it,
                status: "error",
                error: err.message || t("upload.status_failed", language),
              }
            : it
        )
      );
      message.error(t("upload.upload_failed", language, { name: file.name }) + `: ${err.message}`);
    }
  };

  const uploadProps = {
    name: "file",
    multiple: true,
    showUploadList: false, // we render our own list
    customRequest: async (options: any) => {
      const { file, onSuccess } = options;
      await handleUploadFile(file);
      onSuccess(null, file);
    },
    beforeUpload(file: File) {
      const allowedTypes = [
        "application/pdf",
        "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
        "application/vnd.ms-excel",
        "image/jpeg",
        "image/png",
        "image/tiff",
      ];
      if (!allowedTypes.includes(file.type)) {
        message.error(t("upload.unsupported_file", language));
        return Upload.LIST_IGNORE;
      }
      const isLt50M = file.size / 1024 / 1024 < 50;
      if (!isLt50M) {
        message.error(t("upload.file_too_large", language));
        return Upload.LIST_IGNORE;
      }
      return true;
    },
  };

  const handleRemoveFile = (fileId: string) => {
    setBatchItems((prev) => prev.filter((it) => it.file_id !== fileId));
    setSelectedKeys((prev) => prev.filter((k) => k !== fileId));
  };

  // ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  // Step 1: Parse all files
  // ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  const handleStartParse = useCallback(async () => {
    if (!token) {
      message.error(t("upload.please_login", language));
      return;
    }

    const toParse = batchItems.filter((it) => it.status === "uploaded");
    if (toParse.length === 0) {
      message.warning(t("upload.no_files", language));
      return;
    }

    setStep(1);
    setParsingTotal(toParse.length);

    // Mark all as "parsing"
    setBatchItems((prev) =>
      prev.map((it) =>
        it.status === "uploaded" ? { ...it, status: "parsing" as const } : it
      )
    );

    for (let i = 0; i < toParse.length; i++) {
      const item = toParse[i];
      setParsingIndex(i + 1);

      try {
        const result = await aiRequest("/api/v1/parse/contract", {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          } as Record<string, string>,
          body: JSON.stringify({
            file_id: item.file_id,
            object_name: item.object_name,
            content_type: item.content_type,
          }),
          token: token!,
        });

        // Build formData from extracted_data
        const ed = result.extracted_data || {};
        const fd: Record<string, any> = {
          contract_number: ed.contract_number || undefined,
          contract_name: ed.contract_name || undefined,
          lessee: ed.lessee || undefined,
          lessor: ed.lessor || undefined,
          store_name: ed.store_name || undefined,
          store_address: ed.store_address || undefined,
          currency:
            ed.currency && ["CNY", "USD", "EUR"].includes(ed.currency)
              ? ed.currency
              : "CNY",
          commencement_date: ed.commencement_date
            ? dayjs(ed.commencement_date)
            : undefined,
          lease_start_date: ed.lease_start_date
            ? dayjs(ed.lease_start_date)
            : undefined,
          lease_end_date: ed.lease_end_date
            ? dayjs(ed.lease_end_date)
            : undefined,
          fixed_rent_amount: ed.fixed_rent_amount
            ? Number(ed.fixed_rent_amount)
            : undefined,
          payment_timing:
            ed.payment_timing &&
            ["prepaid", "postpaid"].includes(ed.payment_timing)
              ? ed.payment_timing
              : undefined,
          payment_frequency:
            ed.payment_frequency &&
            ["monthly", "quarterly", "yearly"].includes(ed.payment_frequency)
              ? ed.payment_frequency
              : undefined,
          discount_rate_type: ed.discount_rate_type || undefined,
          discount_rate: ed.discount_rate
            ? Number(ed.discount_rate)
            : undefined,
          cam_amount: ed.cam_amount ? Number(ed.cam_amount) : undefined,
          service_fee: ed.service_fee ? Number(ed.service_fee) : undefined,
          tags: ed.tags || undefined,
        };

        setBatchItems((prev) =>
          prev.map((it) =>
            it.file_id === item.file_id
              ? {
                  ...it,
                  status: "parsed",
                  parseResult: result,
                  formData: fd,
                }
              : it
          )
        );
      } catch (err: any) {
        setBatchItems((prev) =>
          prev.map((it) =>
            it.file_id === item.file_id
              ? {
                  ...it,
                  status: "error",
                  error: err.message || t("upload.parse_failed", language),
                  parseResult: null,
                }
              : it
          )
        );
        message.error(`${item.original_name} ${t("upload.parse_failed", language)}: ${err.message}`);
      }
    }

    setStep(2);
  }, [batchItems, token]);

  // ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  // Step 2: Edit modal
  // ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  const openEditModal = useCallback(
    (index: number) => {
      if (index < 0 || index >= batchItems.length) return;
      const item = batchItems[index];
      if (item.status !== "parsed") {
        message.warning(t("upload.edit_only_parsed", language));
        return;
      }
      setEditingIndex(index);
      // Pre-fill form with item's formData (or parseResult extracted_data)
      const fd = item.formData || item.parseResult?.extracted_data || {};
      editForm.setFieldsValue({
        contract_number: fd.contract_number || undefined,
        contract_name: fd.contract_name || undefined,
        lessee: fd.lessee || undefined,
        lessor: fd.lessor || undefined,
        store_name: fd.store_name || undefined,
        store_address: fd.store_address || undefined,
        currency: fd.currency || "CNY",
        commencement_date: fd.commencement_date
          ? dayjs.isDayjs(fd.commencement_date)
            ? fd.commencement_date
            : dayjs(fd.commencement_date)
          : undefined,
        lease_start_date: fd.lease_start_date
          ? dayjs.isDayjs(fd.lease_start_date)
            ? fd.lease_start_date
            : dayjs(fd.lease_start_date)
          : undefined,
        lease_end_date: fd.lease_end_date
          ? dayjs.isDayjs(fd.lease_end_date)
            ? fd.lease_end_date
            : dayjs(fd.lease_end_date)
          : undefined,
        fixed_rent_amount: fd.fixed_rent_amount ?? undefined,
        payment_timing: fd.payment_timing || undefined,
        payment_frequency: fd.payment_frequency || undefined,
        discount_rate_type: fd.discount_rate_type || undefined,
        discount_rate: fd.discount_rate ?? undefined,
        cam_amount: fd.cam_amount ?? undefined,
        service_fee: fd.service_fee ?? undefined,
        tags: fd.tags || undefined,
      });
    },
    [batchItems, editForm]
  );

  const handleSaveEdit = useCallback(() => {
    editForm
      .validateFields()
      .then((values) => {
        if (editingIndex === null) return;

        // Convert dates to dayjs for storage
        const fd: Record<string, any> = { ...values };
        // The form already has dayjs objects from DatePicker

        setBatchItems((prev) =>
          prev.map((it, idx) =>
            idx === editingIndex ? { ...it, formData: fd } : it
          )
        );
        setEditingIndex(null);
        message.success(t("upload.modify_saved", language));
      })
      .catch(() => {
        // validation failed
      });
  }, [editForm, editingIndex]);

  const editingItem =
    editingIndex !== null ? batchItems[editingIndex] : null;

  // ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  // Step 3: Batch create contracts
  // ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  const handleBatchCreate = useCallback(async () => {
    if (!token) {
      message.error(t("upload.please_login", language));
      return;
    }
    if (selectedKeys.length === 0) {
      message.warning(t("upload.select_at_least_one", language));
      return;
    }

    setCreating(true);
    setStep(3);

    const selected = batchItems.filter((it) =>
      selectedKeys.includes(it.file_id)
    );
    const contracts: any[] = [];
    let successCount = 0;
    let failedCount = 0;

    for (let i = 0; i < selected.length; i++) {
      const item = selected[i];
      const fd = item.formData || {};
      setParsingIndex(i + 1);
      setParsingTotal(selected.length);

      const v = (key: string) =>
        fd[key] !== undefined && fd[key] !== null ? fd[key] : undefined;

      const payload: Record<string, any> = {
        contract_number: v("contract_number"),
        contract_name: v("contract_name"),
        lessee_name: v("lessee"),
        lessor_name: v("lessor"),
        store_name: v("store_name") || undefined,
        store_address: v("store_address") || undefined,
        currency: v("currency"),
        asset_type: v("asset_type") || "real_estate",
        commencement_date: v("commencement_date")
          ? dayjs.isDayjs(v("commencement_date"))
            ? (v("commencement_date") as Dayjs).format("YYYY-MM-DD")
            : v("commencement_date")
          : undefined,
        lease_start_date: v("lease_start_date")
          ? dayjs.isDayjs(v("lease_start_date"))
            ? (v("lease_start_date") as Dayjs).format("YYYY-MM-DD")
            : v("lease_start_date")
          : undefined,
        lease_end_date: v("lease_end_date")
          ? dayjs.isDayjs(v("lease_end_date"))
            ? (v("lease_end_date") as Dayjs).format("YYYY-MM-DD")
            : v("lease_end_date")
          : undefined,
        tags: v("tags") || undefined,
        discount_rate_type: v("discount_rate_type") || undefined,
        lease_scope: v("lease_scope") || v("suggested_scope") || "in_scope",
        exemption_reason: v("exemption_reason") || undefined,
        scope_source: v("scope_source") || "ai_suggested",
        scope_confidence: v("scope_confidence") ?? undefined,
      };

      try {
        const created = await contractApi.create(payload, token);
        contracts.push(created);
        successCount++;
        setBatchItems((prev) =>
          prev.map((it) =>
            it.file_id === item.file_id
              ? { ...it, createdContract: created, status: "parsed" }
              : it
          )
        );
      } catch (err: any) {
        failedCount++;
        message.error(
          `${item.original_name}${t("upload.create_failed_divider", language)}${err.message || t("upload.unknown_error", language)}`
        );
      }
    }

    setCreateResult({ success: successCount, failed: failedCount, contracts });
    setCreating(false);
  }, [batchItems, selectedKeys, token]);

  // ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  // Reset
  // ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  const handleReset = () => {
    setBatchItems([]);
    setSelectedKeys([]);
    setEditingIndex(null);
    setCreateResult(null);
    setParsingIndex(0);
    setParsingTotal(0);
    setStep(0);
    editForm.resetFields();
  };

  // ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  // Confidence label helper (for edit modal)
  // ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  const confidenceLabel = (
    label: string,
    fieldName: string
  ): React.ReactNode => {
    const scores = editingItem?.parseResult?.confidence_scores;
    const score = scores?.[fieldName];
    if (score === undefined || score === null) return label;

    return (
      <Space size={4}>
        <Text>{label}</Text>
        <Tooltip title={t("upload.ai_confidence", language, { score: (score * 100).toFixed(0) })}>
          <Tag
            color={confidenceColor(score)}
            style={{ fontSize: 11, lineHeight: "16px", padding: "0 4px" }}
          >
            {(score * 100).toFixed(0)}%
          </Tag>
        </Tooltip>
      </Space>
    );
  };

  // ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  // Render helper: Confidence tag for table cell
  // ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  const renderConfidenceCell = (
    record: BatchItem,
    fieldName: string,
    displayValue: any
  ) => {
    const score = record.parseResult?.confidence_scores?.[fieldName];
    const label = displayValue != null ? String(displayValue) : "-";

    return (
      <Space size={4}>
        <Text>{label}</Text>
        {score !== undefined && score !== null && (
          <Tag
            color={confidenceColor(score)}
            style={{ fontSize: 10, lineHeight: "14px", padding: "0 3px" }}
          >
            {(score * 100).toFixed(0)}%
          </Tag>
        )}
      </Space>
    );
  };

  // ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  // Table columns
  // ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  const columns: ColumnsType<BatchItem> = [
    {
      title: t("upload.col_file", language),
      dataIndex: "original_name",
      key: "file",
      width: 200,
      render: (name: string, record: BatchItem) => (
        <Space>
          {getFileIcon(record.content_type)}
          <Text ellipsis style={{ maxWidth: 140 }}>
            {name}
          </Text>
        </Space>
      ),
    },
    {
      title: t("upload.col_contract_number", language),
      key: "contract_number",
      width: 160,
      render: (_: any, record: BatchItem) => {
        const fd = record.formData;
        const val = fd?.contract_number || record.parseResult?.extracted_data?.contract_number;
        return renderConfidenceCell(record, "contract_number", val);
      },
    },
    {
      title: t("upload.col_lessee", language),
      key: "lessee",
      width: 140,
      render: (_: any, record: BatchItem) => {
        const fd = record.formData;
        const val = fd?.lessee || record.parseResult?.extracted_data?.lessee;
        return renderConfidenceCell(record, "lessee", val);
      },
    },
    {
      title: t("upload.col_lessor", language),
      key: "lessor",
      width: 140,
      render: (_: any, record: BatchItem) => {
        const fd = record.formData;
        const val = fd?.lessor || record.parseResult?.extracted_data?.lessor;
        return renderConfidenceCell(record, "lessor", val);
      },
    },
    {
      title: t("upload.col_store", language),
      key: "store_name",
      width: 120,
      render: (_: any, record: BatchItem) => {
        const fd = record.formData;
        const val = fd?.store_name || record.parseResult?.extracted_data?.store_name;
        return renderConfidenceCell(record, "store_name", val);
      },
    },
    {
      title: t("upload.col_start_date", language),
      key: "commencement_date",
      width: 110,
      render: (_: any, record: BatchItem) => {
        const fd = record.formData;
        const val =
          fd?.commencement_date ||
          record.parseResult?.extracted_data?.commencement_date;
        const display = val
          ? dayjs.isDayjs(val)
            ? (val as Dayjs).format("YYYY-MM-DD")
            : String(val)
          : "-";
        return renderConfidenceCell(record, "commencement_date", display);
      },
    },
    {
      title: t("upload.col_end_date", language),
      key: "lease_end_date",
      width: 110,
      render: (_: any, record: BatchItem) => {
        const fd = record.formData;
        const val =
          fd?.lease_end_date ||
          record.parseResult?.extracted_data?.lease_end_date;
        const display = val
          ? dayjs.isDayjs(val)
            ? (val as Dayjs).format("YYYY-MM-DD")
            : String(val)
          : "-";
        return renderConfidenceCell(record, "lease_end_date", display);
      },
    },
    {
      title: t("upload.col_rent", language),
      key: "fixed_rent_amount",
      width: 100,
      render: (_: any, record: BatchItem) => {
        const fd = record.formData;
        const val =
          fd?.fixed_rent_amount ??
          record.parseResult?.extracted_data?.fixed_rent_amount;
        const display = val != null ? `¥${Number(val).toLocaleString()}` : "-";
        return renderConfidenceCell(record, "fixed_rent_amount", display);
      },
    },
    {
      title: t("upload.col_status", language),
      key: "status",
      width: 90,
      fixed: "right",
      render: (_: any, record: BatchItem) => {
        const statusMap: Record<string, { color: string; label: string }> = {
          uploading: { color: "processing", label: t("upload.status_uploading", language) },
          uploaded: { color: "blue", label: t("upload.status_uploaded", language) },
          parsing: { color: "processing", label: t("upload.status_parsing", language) },
          parsed: { color: "green", label: t("upload.status_parsed", language) },
          error: { color: "red", label: t("upload.status_failed", language) },
        };
        const s = statusMap[record.status] || { color: "default", label: record.status };
        return <Tag color={s.color}>{s.label}</Tag>;
      },
    },
    {
      title: t("upload.col_action", language),
      key: "actions",
      width: 80,
      fixed: "right",
      render: (_: any, record: BatchItem, index: number) => (
        <Button
          size="small"
          icon={<EditOutlined />}
          disabled={record.status !== "parsed"}
          onClick={() => openEditModal(index)}
        >
          {t("upload.action_edit", language)}
        </Button>
      ),
    },
  ];

  // ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  // Derived counts
  // ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  const uploadedCount = batchItems.filter(
    (it) => it.status === "uploaded"
  ).length;
  const parsedCount = batchItems.filter(
    (it) => it.status === "parsed"
  ).length;
  const errorCount = batchItems.filter(
    (it) => it.status === "error"
  ).length;

  const canStartParse = uploadedCount > 0 && step === 0;
  const canBatchCreate = selectedKeys.length > 0 && step === 2;

  // ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  // Render
  // ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  return (
    <ProtectedRoute>
      <AppLayout>
        <Title level={3} style={{ marginBottom: 24 }}>
          {t("upload.title", language)}
        </Title>

        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 24 }}
          message="推荐使用 AI 录入主入口"
          description="上传合同或台账文件后，系统会直接在聊天中生成可编辑的合同草稿卡片；本页继续保留为批量上传和逐步复核备用入口。"
          action={
            <Button size="small" type="primary" icon={<ArrowRightOutlined />} onClick={() => router.push("/ai-chat")}>
              前往 AI 录入
            </Button>
          }
        />

        {/* ── Steps indicator ── */}
        <Card style={{ marginBottom: 24 }}>
          <Steps
            current={step}
            size="small"
            items={[
              {
                title: t("upload.step_upload", language),
                icon:
                  step === 0 ? <CloudUploadOutlined /> : <CheckCircleOutlined />,
              },
              {
                title: t("upload.step_parse", language),
                icon:
                  step === 1 ? (
                    <LoadingOutlined spin />
                  ) : step > 1 ? (
                    <CheckCircleOutlined />
                  ) : (
                    <ThunderboltOutlined />
                  ),
              },
              {
                title: t("upload.step_confirm", language),
                icon:
                  step === 2 ? (
                    <EditOutlined />
                  ) : step > 2 ? (
                    <CheckCircleOutlined />
                  ) : (
                    <OrderedListOutlined />
                  ),
              },
              {
                title: t("upload.step_complete", language),
                icon:
                  step === 3 && !creating ? (
                    <CheckCircleOutlined />
                  ) : step === 3 && creating ? (
                    <LoadingOutlined spin />
                  ) : undefined,
              },
            ]}
          />
        </Card>

        {/* ════════════════════════════════════════════════════
            Step 0: Upload area
            ════════════════════════════════════════════════════ */}
        {(step === 0 || step === 1) && (
          <>
            <Card
              title={
                <Space>
                  <CloudUploadOutlined />
                  <Text>{t("upload.upload_card_title", language)}</Text>
                  {batchItems.length > 0 && (
                    <Tag color="blue">{t("upload.file_count", language, { n: String(batchItems.length) })}</Tag>
                  )}
                </Space>
              }
              extra={
                batchItems.length > 0 && step === 0 ? (
                  <Button
                    onClick={handleReset}
                    icon={<ReloadOutlined />}
                    size="small"
                  >
                    {t("upload.reupload", language)}
                  </Button>
                ) : undefined
              }
              style={{ marginBottom: 24 }}
            >
              <Dragger
                {...uploadProps}
                disabled={step === 1}
                style={{ padding: batchItems.length > 0 ? "12px 0" : undefined }}
              >
                <p className="ant-upload-drag-icon">
                  <UploadOutlined />
                </p>
                <p className="ant-upload-text">
                  {t("upload.drag_hint", language)}
                </p>
                <p className="ant-upload-hint">
                  {t("upload.support_formats", language)}
                </p>
              </Dragger>
            </Card>

            {/* ── Uploaded files list ── */}
            {batchItems.length > 0 && (
              <Card
                title={t("upload.uploaded_files", language)}
                extra={
                  step === 0 && (
                    <Button
                      type="primary"
                      icon={<ThunderboltOutlined />}
                      onClick={handleStartParse}
                      disabled={!canStartParse}
                    >
                      {t("upload.start_parse", language, { n: String(uploadedCount) })}
                    </Button>
                  )
                }
                style={{ marginBottom: 24 }}
              >
                <div
                  style={{
                    display: "flex",
                    flexDirection: "column",
                    gap: 8,
                    maxHeight: 300,
                    overflow: "auto",
                  }}
                >
                  {batchItems.map((item) => (
                    <div
                      key={item.file_id}
                      style={{
                        display: "flex",
                        alignItems: "center",
                        justifyContent: "space-between",
                        padding: "8px 12px",
                        background: "#fafafa",
                        borderRadius: 6,
                        border: "1px solid #f0f0f0",
                      }}
                    >
                      <Space>
                        {item.status === "uploading" ? (
                          <LoadingOutlined spin />
                        ) : item.status === "uploaded" ? (
                          <CheckCircleOutlined style={{ color: "#10B981" }} />
                        ) : item.status === "parsing" ? (
                          <LoadingOutlined spin />
                        ) : item.status === "parsed" ? (
                          <CheckCircleOutlined style={{ color: "#10B981" }} />
                        ) : item.status === "error" ? (
                          <CloseCircleOutlined style={{ color: "#EF4444" }} />
                        ) : null}
                        {getFileIcon(item.content_type)}
                        <Text>{item.original_name}</Text>
                        <Text type="secondary" style={{ fontSize: 12 }}>
                          ({(item.file_size / 1024).toFixed(1)} KB)
                        </Text>
                      </Space>
                      <Space>
                        <Tag
                          color={
                            item.status === "error"
                              ? "red"
                              : item.status === "parsed"
                              ? "green"
                              : item.status === "uploaded"
                              ? "blue"
                              : item.status === "parsing" ||
                                item.status === "uploading"
                              ? "processing"
                              : "default"
                          }
                        >
                          {item.status === "uploading"
                            ? t("upload.status_uploading", language)
                            : item.status === "uploaded"
                            ? t("upload.status_uploaded", language)
                            : item.status === "parsing"
                            ? t("upload.status_parsing", language)
                            : item.status === "parsed"
                            ? t("upload.status_parsed", language)
                            : item.status === "error"
                            ? t("upload.status_failed", language)
                            : item.status}
                        </Tag>
                        {item.status === "error" && item.error && (
                          <Tooltip title={item.error}>
                            <WarningOutlined style={{ color: "#EF4444" }} />
                          </Tooltip>
                        )}
                        {step === 0 && (
                          <Button
                            type="link"
                            size="small"
                            danger
                            onClick={() => handleRemoveFile(item.file_id)}
                          >
                            {t("upload.remove", language)}
                          </Button>
                        )}
                      </Space>
                    </div>
                  ))}
                </div>
              </Card>
            )}
          </>
        )}

        {/* ════════════════════════════════════════════════════
            Step 1: Parsing progress
            ════════════════════════════════════════════════════ */}
        {step === 1 && (
          <Card>
            <div style={{ textAlign: "center", padding: "40px 0" }}>
              <Spin
                indicator={<LoadingOutlined style={{ fontSize: 48 }} spin />}
              />
              <Paragraph
                style={{ marginTop: 24, fontSize: 16, color: "#666" }}
              >
                {t("upload.parsing_progress", language, { n: String(parsingIndex), total: String(parsingTotal) })}
              </Paragraph>
              <Progress
                percent={Math.round((parsingIndex / parsingTotal) * 100)}
                status="active"
                style={{ maxWidth: 400, margin: "16px auto" }}
              />
              <Paragraph type="secondary">
                {t("upload.parsing_detail", language)}
              </Paragraph>
            </div>
          </Card>
        )}

        {/* ════════════════════════════════════════════════════
            Step 2: Batch review table
            ════════════════════════════════════════════════════ */}
        {step === 2 && batchItems.length > 0 && (
          <Card
            title={
              <Space>
                <EditOutlined />
                <span>{t("upload.confirm_title", language)}</span>
                <Tag color="green">{t("upload.parsed_count", language, { n: String(parsedCount) })}</Tag>
                {errorCount > 0 && (
                  <Tag color="red">{t("upload.failed_count", language, { n: String(errorCount) })}</Tag>
                )}
              </Space>
            }
            extra={
              <Space>
                <Button icon={<ReloadOutlined />} onClick={handleReset}>
                  {t("upload.reupload_btn", language)}
                </Button>
                <Button
                  type="primary"
                  size="large"
                  icon={<ArrowRightOutlined />}
                  disabled={!canBatchCreate}
                  onClick={handleBatchCreate}
                >
                  {t("upload.batch_create", language, { n: String(selectedKeys.length > 0 ? selectedKeys.length : 0) })}
                </Button>
              </Space>
            }
          >
            {errorCount > 0 && (
              <Alert
                message={t("upload.parse_failed_alert", language, { n: String(errorCount) })}
                type="warning"
                showIcon
                style={{ marginBottom: 16 }}
                closable
              />
            )}

            <Table
              rowKey="file_id"
              rowSelection={{
                type: "checkbox",
                selectedRowKeys: selectedKeys,
                onChange: (keys) => setSelectedKeys(keys),
                getCheckboxProps: (record: BatchItem) => ({
                  disabled: record.status !== "parsed",
                }),
              }}
              columns={columns}
              dataSource={batchItems.filter(
                (it) =>
                  it.status === "parsed" || it.status === "error"
              )}
              size="small"
              scroll={{ x: 1400 }}
              pagination={false}
              style={{ marginBottom: 16 }}
            />
          </Card>
        )}

        {/* ════════════════════════════════════════════════════
            Step 3: Complete
            ════════════════════════════════════════════════════ */}
        {step === 3 && (
          <Card>
            {creating ? (
              <div style={{ textAlign: "center", padding: "40px 0" }}>
                <Spin
                  indicator={<LoadingOutlined style={{ fontSize: 48 }} spin />}
                />
                <Paragraph
                  style={{ marginTop: 24, fontSize: 16, color: "#666" }}
                >
                  {t("upload.creating_progress", language, { n: String(parsingIndex), total: String(parsingTotal) })}
                </Paragraph>
                <Progress
                  percent={Math.round(
                    (parsingIndex / Math.max(parsingTotal, 1)) * 100
                  )}
                  status="active"
                  style={{ maxWidth: 400, margin: "16px auto" }}
                />
              </div>
            ) : createResult ? (
              <Result
                status={createResult.failed > 0 ? "warning" : "success"}
                title={
                  createResult.failed > 0
                    ? t("upload.partial_success", language)
                    : t("upload.all_success", language)
                }
                subTitle={
                  <Space direction="vertical">
                    <Text>
                      {t("upload.success_count", language, { n: String(createResult.success) })}
                      {createResult.failed > 0 &&
                        `，${t("upload.failed_count_result", language, { n: String(createResult.failed) })}`}
                    </Text>
                    {createResult.contracts.length > 0 && (
                      <div style={{ textAlign: "left", maxHeight: 200, overflow: "auto" }}>
                        {createResult.contracts.map((c, i) => (
                          <div key={c.id || i} style={{ marginBottom: 4 }}>
                            <Text code>{c.contract_number}</Text>
                            <Text type="secondary" style={{ marginLeft: 8 }}>
                              ID: {c.id}
                            </Text>
                          </div>
                        ))}
                      </div>
                    )}
                  </Space>
                }
                extra={[
                  <Button
                    type="primary"
                    key="list"
                    icon={<ArrowRightOutlined />}
                    onClick={() => router.push("/contracts")}
                  >
                    {t("upload.view_contracts", language)}
                  </Button>,
                  <Button
                    key="reset"
                    icon={<ReloadOutlined />}
                    onClick={handleReset}
                  >
                    {t("upload.reupload_btn", language)}
                  </Button>,
                ]}
              />
            ) : (
              <div style={{ textAlign: "center", padding: "24px", color: "#999" }}>
                {t("upload.processing", language)}
              </div>
            )}
          </Card>
        )}

        {/* ════════════════════════════════════════════════════
            Edit Modal (Step 2)
            ════════════════════════════════════════════════════ */}
        <Modal
          title={
            <Space>
              <EditOutlined />
              <span>{t("upload.edit_modal_title", language)}</span>
              {editingItem?.original_name && (
                <Tag color="blue">{editingItem.original_name}</Tag>
              )}
            </Space>
          }
          open={editingIndex !== null}
          onCancel={() => setEditingIndex(null)}
          onOk={handleSaveEdit}
          width={900}
          okText={t("upload.save", language)}
          cancelText={t("upload.cancel", language)}
          destroyOnClose={false}
          maskClosable={false}
          styles={{ body: { maxHeight: "65vh", overflow: "auto" } }}
        >
          {editingItem && (
            <>
              {/* ── Warnings ── */}
              {editingItem.parseResult?.warnings &&
                editingItem.parseResult.warnings.length > 0 && (
                  <Alert
                    message={
                      <Space>
                        <WarningOutlined />
                        <span>
                          {t("upload.ai_tips", language, { n: String(editingItem.parseResult.warnings.length) })}
                        </span>
                      </Space>
                    }
                    description={
                      <ul style={{ margin: "8px 0 0", paddingLeft: 20 }}>
                        {editingItem.parseResult.warnings.map(
                          (w: string, i: number) => (
                            <li key={i}>{w}</li>
                          )
                        )}
                      </ul>
                    }
                    type={
                      editingItem.parseResult.requires_human_confirmation
                        ? "warning"
                        : "info"
                    }
                    showIcon
                    closable
                    style={{ marginBottom: 16 }}
                  />
                )}

              {/* ── Missing fields alert ── */}
              {editingItem.parseResult?.missing_fields &&
                editingItem.parseResult.missing_fields.length > 0 && (
                  <Alert
                    message={
                      <Space>
                        <ExclamationCircleOutlined />
                        <span>
                          {t("upload.missing_fields_title", language, { count: String(editingItem.parseResult.missing_fields.length) })}
                        </span>
                      </Space>
                    }
                    description={editingItem.parseResult.missing_fields
                      .map((f: string) => {
                        const labels: Record<string, string> = {
                          contract_number: t("upload.missing_field_label.contract_number", language),
                          contract_name: t("upload.missing_field_label.contract_name", language),
                          lessee: t("upload.missing_field_label.lessee", language),
                          lessor: t("upload.missing_field_label.lessor", language),
                          commencement_date: t("upload.missing_field_label.commencement_date", language),
                          lease_start_date: t("upload.missing_field_label.lease_start_date", language),
                          lease_end_date: t("upload.missing_field_label.lease_end_date", language),
                          currency: t("upload.missing_field_label.currency", language),
                          fixed_rent_amount: t("upload.missing_field_label.fixed_rent_amount", language),
                          payment_timing: t("upload.missing_field_label.payment_timing", language),
                          discount_rate: t("upload.missing_field_label.discount_rate", language),
                        };
                        return labels[f] || f;
                      })
                      .join("、")}
                    type="error"
                    showIcon
                    style={{ marginBottom: 16 }}
                  />
                )}

              <Form
                form={editForm}
                layout="vertical"
                initialValues={defaultFormData()}
                scrollToFirstError
              >
                {/* ── 合同基本信息 ── */}
                <Divider orientation="left" plain>
                  {t("upload.section_basic", language)}
                </Divider>

                <Row gutter={24}>
                  <Col xs={24} md={12}>
                    <Form.Item
                      label={confidenceLabel(t("upload.field.contract_number", language), "contract_number")}
                      name="contract_number"
                      rules={[{ required: true, message: t("upload.validation.contract_number_required", language) }]}
                    >
                      <Input placeholder={t("upload.placeholder.contract_number", language)} />
                    </Form.Item>
                  </Col>
                  <Col xs={24} md={12}>
                    <Form.Item
                      label={confidenceLabel(t("upload.field.contract_name", language), "contract_name")}
                      name="contract_name"
                      rules={[{ required: true, message: t("upload.validation.contract_name_required", language) }]}
                    >
                      <Input placeholder={t("upload.placeholder.contract_name", language)} />
                    </Form.Item>
                  </Col>
                </Row>

                <Row gutter={24}>
                  <Col xs={24} md={12}>
                    <Form.Item
                      label={confidenceLabel(t("upload.field.lessee", language), "lessee")}
                      name="lessee"
                      rules={[{ required: true, message: t("upload.validation.lessee_required", language) }]}
                    >
                      <Input placeholder={t("upload.placeholder.lessee", language)} />
                    </Form.Item>
                  </Col>
                  <Col xs={24} md={12}>
                    <Form.Item
                      label={confidenceLabel(t("upload.field.lessor", language), "lessor")}
                      name="lessor"
                      rules={[{ required: true, message: t("upload.validation.lessor_required", language) }]}
                    >
                      <Input placeholder={t("upload.placeholder.lessor", language)} />
                    </Form.Item>
                  </Col>
                </Row>

                <Row gutter={24}>
                  <Col xs={24} md={12}>
                    <Form.Item
                      label={confidenceLabel(t("upload.field.store_name", language), "store_name")}
                      name="store_name"
                    >
                      <Input placeholder={t("upload.placeholder.store_name", language)} />
                    </Form.Item>
                  </Col>
                  <Col xs={24} md={12}>
                    <Form.Item
                      label={confidenceLabel(t("upload.field.store_address", language), "store_address")}
                      name="store_address"
                    >
                      <Input placeholder={t("upload.placeholder.store_address", language)} />
                    </Form.Item>
                  </Col>
                </Row>

                {/* ── 日期与金额 ── */}
                <Divider orientation="left" plain>
                  {t("upload.section_dates", language)}
                </Divider>

                <Row gutter={24}>
                  <Col xs={24} md={8}>
                    <Form.Item
                      label={confidenceLabel(
                        t("upload.field.commencement_date", language),
                        "commencement_date"
                      )}
                      name="commencement_date"
                      rules={[
                        { required: true, message: t("upload.validation.commencement_date_required", language) },
                      ]}
                    >
                      <DatePicker style={{ width: "100%" }} />
                    </Form.Item>
                  </Col>
                  <Col xs={24} md={8}>
                    <Form.Item
                      label={confidenceLabel(
                        t("upload.field.lease_start_date", language),
                        "lease_start_date"
                      )}
                      name="lease_start_date"
                      rules={[
                        { required: true, message: t("upload.validation.lease_start_date_required", language) },
                      ]}
                    >
                      <DatePicker style={{ width: "100%" }} />
                    </Form.Item>
                  </Col>
                  <Col xs={24} md={8}>
                    <Form.Item
                      label={confidenceLabel(
                        t("upload.field.lease_end_date", language),
                        "lease_end_date"
                      )}
                      name="lease_end_date"
                      rules={[
                        { required: true, message: t("upload.validation.lease_end_date_required", language) },
                      ]}
                    >
                      <DatePicker style={{ width: "100%" }} />
                    </Form.Item>
                  </Col>
                </Row>

                <Row gutter={24}>
                  <Col xs={24} md={6}>
                    <Form.Item
                      label={confidenceLabel(t("upload.field.currency", language), "currency")}
                      name="currency"
                      rules={[{ required: true, message: t("upload.validation.currency_required", language) }]}
                    >
                      <Select>
                        <Select.Option value="CNY">{t("upload.option.currency_cny", language)}</Select.Option>
                        <Select.Option value="USD">{t("upload.option.currency_usd", language)}</Select.Option>
                        <Select.Option value="EUR">{t("upload.option.currency_eur", language)}</Select.Option>
                      </Select>
                    </Form.Item>
                  </Col>
                  <Col xs={24} md={6}>
                    <Form.Item
                      label={confidenceLabel(
                        t("upload.field.fixed_rent_amount", language),
                        "fixed_rent_amount"
                      )}
                      name="fixed_rent_amount"
                    >
                      <InputNumber
                        style={{ width: "100%" }}
                        placeholder={t("upload.placeholder.fixed_rent_amount", language)}
                        min={0}
                        precision={2}
                        addonAfter={t("upload.addon_after.monthly", language)}
                      />
                    </Form.Item>
                  </Col>
                  <Col xs={24} md={6}>
                    <Form.Item
                      label={confidenceLabel(t("upload.field.payment_timing", language), "payment_timing")}
                      name="payment_timing"
                    >
                      <Select placeholder={t("upload.placeholder.select", language)} allowClear>
                        <Select.Option value="prepaid">
                          {t("upload.option.payment_prepaid", language)}
                        </Select.Option>
                        <Select.Option value="postpaid">
                          {t("upload.option.payment_postpaid", language)}
                        </Select.Option>
                      </Select>
                    </Form.Item>
                  </Col>
                  <Col xs={24} md={6}>
                    <Form.Item
                      label={confidenceLabel(t("upload.field.payment_frequency", language), "payment_frequency")}
                      name="payment_frequency"
                    >
                      <Select placeholder={t("upload.placeholder.select", language)} allowClear>
                        <Select.Option value="monthly">{t("upload.option.frequency_monthly", language)}</Select.Option>
                        <Select.Option value="quarterly">{t("upload.option.frequency_quarterly", language)}</Select.Option>
                        <Select.Option value="yearly">{t("upload.option.frequency_yearly", language)}</Select.Option>
                      </Select>
                    </Form.Item>
                  </Col>
                </Row>

                {/* ── 折现率设置 ── */}
                <Divider orientation="left" plain>
                  {t("upload.section_discount", language)}
                </Divider>

                <Alert
                  message={t("upload.discount_rate_alert_title", language)}
                  description={t("upload.discount_rate_alert_desc", language)}
                  type="info"
                  showIcon
                  style={{ marginBottom: 16 }}
                />

                <Row gutter={24}>
                  <Col xs={24} md={8}>
                    <Form.Item
                      label={confidenceLabel(
                        t("upload.field.discount_rate_type", language),
                        "discount_rate_type"
                      )}
                      name="discount_rate_type"
                    >
                      <Select placeholder={t("upload.placeholder.discount_rate_type", language)} allowClear>
                        <Select.Option value="ibr">{t("upload.option.rate_type_ibr", language)}</Select.Option>
                        <Select.Option value="entity_specific">
                          {t("upload.option.rate_type_entity", language)}
                        </Select.Option>
                        <Select.Option value="contract_specific">
                          {t("upload.option.rate_type_contract", language)}
                        </Select.Option>
                        <Select.Option value="implicit_rate">
                          {t("upload.option.rate_type_implicit", language)}
                        </Select.Option>
                      </Select>
                    </Form.Item>
                  </Col>
                  <Col xs={24} md={8}>
                    <Form.Item
                      label={confidenceLabel(t("upload.field.discount_rate", language), "discount_rate")}
                      name="discount_rate"
                    >
                      <InputNumber
                        style={{ width: "100%" }}
                        placeholder={t("upload.placeholder.discount_rate", language)}
                        min={0}
                        max={100}
                        precision={2}
                        addonAfter="%"
                      />
                    </Form.Item>
                  </Col>
                </Row>

                {/* ── 其他费用 ── */}
                <Divider orientation="left" plain>
                  {t("upload.section_other", language)}
                </Divider>

                <Row gutter={24}>
                  <Col xs={24} md={8}>
                    <Form.Item
                      label={confidenceLabel(t("upload.field.cam_amount", language), "cam_amount")}
                      name="cam_amount"
                    >
                      <InputNumber
                        style={{ width: "100%" }}
                        placeholder={t("upload.placeholder.cam_amount", language)}
                        min={0}
                        precision={2}
                      />
                    </Form.Item>
                  </Col>
                  <Col xs={24} md={8}>
                    <Form.Item
                      label={confidenceLabel(t("upload.field.service_fee", language), "service_fee")}
                      name="service_fee"
                    >
                      <InputNumber
                        style={{ width: "100%" }}
                        placeholder={t("upload.placeholder.service_fee", language)}
                        min={0}
                        precision={2}
                      />
                    </Form.Item>
                  </Col>
                </Row>

                {/* ── 标签/备注 ── */}
                <Divider orientation="left" plain>
                  {t("upload.section_tags", language)}
                </Divider>

                <Row gutter={24}>
                  <Col xs={24} md={12}>
                    <Form.Item
                      label={confidenceLabel(t("upload.field.tags", language), "tags")}
                      name="tags"
                    >
                      <Input
                        placeholder={t("upload.placeholder.tags", language)}
                      />
                    </Form.Item>
                  </Col>
                </Row>
              </Form>
            </>
          )}
        </Modal>

        {/* Step 0 initial hint */}
        {step === 0 && batchItems.length === 0 && (
          <Card>
            <div
              style={{
                textAlign: "center",
                padding: "24px",
                color: "#999",
              }}
            >
              <InfoCircleOutlined style={{ fontSize: 20, marginRight: 8 }} />
              {t("upload.hint_text", language)}
            </div>
          </Card>
        )}
      </AppLayout>
    </ProtectedRoute>
  );
}
