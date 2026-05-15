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
      message.success(`${file.name} 上传成功`);
    } catch (err: any) {
      setBatchItems((prev) =>
        prev.map((it) =>
          it.file_id === tempId
            ? {
                ...it,
                status: "error",
                error: err.message || "上传失败",
              }
            : it
        )
      );
      message.error(`${file.name} 上传失败: ${err.message}`);
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
        message.error("不支持的文件类型，请上传 PDF、Excel 或图片文件");
        return Upload.LIST_IGNORE;
      }
      const isLt50M = file.size / 1024 / 1024 < 50;
      if (!isLt50M) {
        message.error("文件大小不能超过 50MB");
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
      message.error("请先登录");
      return;
    }

    const toParse = batchItems.filter((it) => it.status === "uploaded");
    if (toParse.length === 0) {
      message.warning("没有待解析的文件");
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
                  error: err.message || "解析失败",
                  parseResult: null,
                }
              : it
          )
        );
        message.error(`${item.original_name} 解析失败: ${err.message}`);
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
        message.warning("只能编辑已解析完成的合同");
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
        message.success("修改已保存");
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
      message.error("请先登录");
      return;
    }
    if (selectedKeys.length === 0) {
      message.warning("请至少选择一个合同");
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
          `${item.original_name} 创建失败: ${err.message || "未知错误"}`
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
        <Tooltip title={`AI 置信度: ${(score * 100).toFixed(0)}%`}>
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
      title: "文件",
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
      title: "合同编号",
      key: "contract_number",
      width: 160,
      render: (_: any, record: BatchItem) => {
        const fd = record.formData;
        const val = fd?.contract_number || record.parseResult?.extracted_data?.contract_number;
        return renderConfidenceCell(record, "contract_number", val);
      },
    },
    {
      title: "承租方",
      key: "lessee",
      width: 140,
      render: (_: any, record: BatchItem) => {
        const fd = record.formData;
        const val = fd?.lessee || record.parseResult?.extracted_data?.lessee;
        return renderConfidenceCell(record, "lessee", val);
      },
    },
    {
      title: "出租方",
      key: "lessor",
      width: 140,
      render: (_: any, record: BatchItem) => {
        const fd = record.formData;
        const val = fd?.lessor || record.parseResult?.extracted_data?.lessor;
        return renderConfidenceCell(record, "lessor", val);
      },
    },
    {
      title: "门店",
      key: "store_name",
      width: 120,
      render: (_: any, record: BatchItem) => {
        const fd = record.formData;
        const val = fd?.store_name || record.parseResult?.extracted_data?.store_name;
        return renderConfidenceCell(record, "store_name", val);
      },
    },
    {
      title: "起始日",
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
      title: "结束日",
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
      title: "租金",
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
      title: "状态",
      key: "status",
      width: 90,
      fixed: "right",
      render: (_: any, record: BatchItem) => {
        const statusMap: Record<string, { color: string; label: string }> = {
          uploading: { color: "processing", label: "上传中" },
          uploaded: { color: "blue", label: "已上传" },
          parsing: { color: "processing", label: "解析中" },
          parsed: { color: "green", label: "已解析" },
          error: { color: "red", label: "失败" },
        };
        const s = statusMap[record.status] || { color: "default", label: record.status };
        return <Tag color={s.color}>{s.label}</Tag>;
      },
    },
    {
      title: "操作",
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
          编辑
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
          智能合同录入 — AI 辅助批量解析
        </Title>

        {/* ── Steps indicator ── */}
        <Card style={{ marginBottom: 24 }}>
          <Steps
            current={step}
            size="small"
            items={[
              {
                title: "上传文件",
                icon:
                  step === 0 ? <CloudUploadOutlined /> : <CheckCircleOutlined />,
              },
              {
                title: "AI 解析",
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
                title: "批量确认",
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
                title: "完成",
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
                  <Text>上传合同文件</Text>
                  {batchItems.length > 0 && (
                    <Tag color="blue">{batchItems.length} 个文件</Tag>
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
                    重新上传
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
                  点击或拖拽合同文件到此区域（支持批量）
                </p>
                <p className="ant-upload-hint">
                  支持 PDF、Excel、JPG、PNG、TIFF 格式，单个文件不超过 50MB
                </p>
              </Dragger>
            </Card>

            {/* ── Uploaded files list ── */}
            {batchItems.length > 0 && (
              <Card
                title="已上传文件"
                extra={
                  step === 0 && (
                    <Button
                      type="primary"
                      icon={<ThunderboltOutlined />}
                      onClick={handleStartParse}
                      disabled={!canStartParse}
                    >
                      开始解析 ({uploadedCount} 个)
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
                            ? "上传中"
                            : item.status === "uploaded"
                            ? "已上传"
                            : item.status === "parsing"
                            ? "解析中"
                            : item.status === "parsed"
                            ? "已解析"
                            : item.status === "error"
                            ? "失败"
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
                            移除
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
                解析第 {parsingIndex}/{parsingTotal} 份合同...
              </Paragraph>
              <Progress
                percent={Math.round((parsingIndex / parsingTotal) * 100)}
                status="active"
                style={{ maxWidth: 400, margin: "16px auto" }}
              />
              <Paragraph type="secondary">
                正在使用 PaddleOCR + DeepSeek 大模型提取合同关键字段
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
                <span>批量确认合同信息</span>
                <Tag color="green">{parsedCount} 份已解析</Tag>
                {errorCount > 0 && (
                  <Tag color="red">{errorCount} 份失败</Tag>
                )}
              </Space>
            }
            extra={
              <Space>
                <Button icon={<ReloadOutlined />} onClick={handleReset}>
                  重新上传
                </Button>
                <Button
                  type="primary"
                  size="large"
                  icon={<ArrowRightOutlined />}
                  disabled={!canBatchCreate}
                  onClick={handleBatchCreate}
                >
                  批量创建合同 ({selectedKeys.length > 0 ? selectedKeys.length : 0})
                </Button>
              </Space>
            }
          >
            {errorCount > 0 && (
              <Alert
                message={`${errorCount} 份文件解析失败，请检查后重新上传`}
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
                  创建第 {parsingIndex}/{parsingTotal} 份合同...
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
                    ? `部分创建成功`
                    : `全部创建成功！`
                }
                subTitle={
                  <Space direction="vertical">
                    <Text>
                      成功创建 {createResult.success} 份合同
                      {createResult.failed > 0 &&
                        `，${createResult.failed} 份失败`}
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
                    查看合同列表
                  </Button>,
                  <Button
                    key="reset"
                    icon={<ReloadOutlined />}
                    onClick={handleReset}
                  >
                    重新上传
                  </Button>,
                ]}
              />
            ) : (
              <div style={{ textAlign: "center", padding: "24px", color: "#999" }}>
                处理中...
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
              <span>编辑合同信息</span>
              {editingItem?.original_name && (
                <Tag color="blue">{editingItem.original_name}</Tag>
              )}
            </Space>
          }
          open={editingIndex !== null}
          onCancel={() => setEditingIndex(null)}
          onOk={handleSaveEdit}
          width={900}
          okText="保存"
          cancelText="取消"
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
                          AI 解析提示 (
                          {editingItem.parseResult.warnings.length} 条)
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
                          缺失关键字段 (
                          {editingItem.parseResult.missing_fields.length} 个):
                        </span>
                      </Space>
                    }
                    description={editingItem.parseResult.missing_fields
                      .map((f: string) => {
                        const labels: Record<string, string> = {
                          contract_number: "合同编号",
                          contract_name: "合同名称",
                          lessee: "承租方",
                          lessor: "出租方",
                          commencement_date: "租赁起始日",
                          lease_start_date: "租赁开始日",
                          lease_end_date: "租期结束日",
                          currency: "币种",
                          fixed_rent_amount: "固定租金金额",
                          payment_timing: "付款时点",
                          discount_rate: "折现率",
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
                  合同基本信息
                </Divider>

                <Row gutter={24}>
                  <Col xs={24} md={12}>
                    <Form.Item
                      label={confidenceLabel("合同编号", "contract_number")}
                      name="contract_number"
                      rules={[{ required: true, message: "请输入合同编号" }]}
                    >
                      <Input placeholder="例如: LEASE-2024-001" />
                    </Form.Item>
                  </Col>
                  <Col xs={24} md={12}>
                    <Form.Item
                      label={confidenceLabel("合同名称", "contract_name")}
                      name="contract_name"
                      rules={[{ required: true, message: "请输入合同名称" }]}
                    >
                      <Input placeholder="例如: 南京东路旗舰店租赁合同" />
                    </Form.Item>
                  </Col>
                </Row>

                <Row gutter={24}>
                  <Col xs={24} md={12}>
                    <Form.Item
                      label={confidenceLabel("承租方 (Lessee)", "lessee")}
                      name="lessee"
                      rules={[{ required: true, message: "请输入承租方" }]}
                    >
                      <Input placeholder="例如: 零售集团上海公司" />
                    </Form.Item>
                  </Col>
                  <Col xs={24} md={12}>
                    <Form.Item
                      label={confidenceLabel("出租方 (Lessor)", "lessor")}
                      name="lessor"
                      rules={[{ required: true, message: "请输入出租方" }]}
                    >
                      <Input placeholder="例如: 上海商业地产集团" />
                    </Form.Item>
                  </Col>
                </Row>

                <Row gutter={24}>
                  <Col xs={24} md={12}>
                    <Form.Item
                      label={confidenceLabel("门店/物业", "store_name")}
                      name="store_name"
                    >
                      <Input placeholder="例如: 南京东路旗舰店（可选）" />
                    </Form.Item>
                  </Col>
                  <Col xs={24} md={12}>
                    <Form.Item
                      label={confidenceLabel("物业地址", "store_address")}
                      name="store_address"
                    >
                      <Input placeholder="例如: 上海市黄浦区南京东路100号（可选）" />
                    </Form.Item>
                  </Col>
                </Row>

                {/* ── 日期与金额 ── */}
                <Divider orientation="left" plain>
                  日期与金额
                </Divider>

                <Row gutter={24}>
                  <Col xs={24} md={8}>
                    <Form.Item
                      label={confidenceLabel(
                        "租赁起始日 (Commencement)",
                        "commencement_date"
                      )}
                      name="commencement_date"
                      rules={[
                        { required: true, message: "请选择租赁起始日" },
                      ]}
                    >
                      <DatePicker style={{ width: "100%" }} />
                    </Form.Item>
                  </Col>
                  <Col xs={24} md={8}>
                    <Form.Item
                      label={confidenceLabel(
                        "租赁开始日 (Start)",
                        "lease_start_date"
                      )}
                      name="lease_start_date"
                      rules={[
                        { required: true, message: "请选择租赁开始日" },
                      ]}
                    >
                      <DatePicker style={{ width: "100%" }} />
                    </Form.Item>
                  </Col>
                  <Col xs={24} md={8}>
                    <Form.Item
                      label={confidenceLabel(
                        "租赁结束日 (End)",
                        "lease_end_date"
                      )}
                      name="lease_end_date"
                      rules={[
                        { required: true, message: "请选择租赁结束日" },
                      ]}
                    >
                      <DatePicker style={{ width: "100%" }} />
                    </Form.Item>
                  </Col>
                </Row>

                <Row gutter={24}>
                  <Col xs={24} md={6}>
                    <Form.Item
                      label={confidenceLabel("币种", "currency")}
                      name="currency"
                      rules={[{ required: true, message: "请选择币种" }]}
                    >
                      <Select>
                        <Select.Option value="CNY">人民币 (CNY)</Select.Option>
                        <Select.Option value="USD">美元 (USD)</Select.Option>
                        <Select.Option value="EUR">欧元 (EUR)</Select.Option>
                      </Select>
                    </Form.Item>
                  </Col>
                  <Col xs={24} md={6}>
                    <Form.Item
                      label={confidenceLabel(
                        "固定租金金额",
                        "fixed_rent_amount"
                      )}
                      name="fixed_rent_amount"
                    >
                      <InputNumber
                        style={{ width: "100%" }}
                        placeholder="月租金金额"
                        min={0}
                        precision={2}
                        addonAfter="元/月"
                      />
                    </Form.Item>
                  </Col>
                  <Col xs={24} md={6}>
                    <Form.Item
                      label={confidenceLabel("付款时点", "payment_timing")}
                      name="payment_timing"
                    >
                      <Select placeholder="选择" allowClear>
                        <Select.Option value="prepaid">
                          先付 (Prepaid)
                        </Select.Option>
                        <Select.Option value="postpaid">
                          后付 (Postpaid)
                        </Select.Option>
                      </Select>
                    </Form.Item>
                  </Col>
                  <Col xs={24} md={6}>
                    <Form.Item
                      label={confidenceLabel("付款频率", "payment_frequency")}
                      name="payment_frequency"
                    >
                      <Select placeholder="选择" allowClear>
                        <Select.Option value="monthly">月度</Select.Option>
                        <Select.Option value="quarterly">季度</Select.Option>
                        <Select.Option value="yearly">年度</Select.Option>
                      </Select>
                    </Form.Item>
                  </Col>
                </Row>

                {/* ── 折现率设置 ── */}
                <Divider orientation="left" plain>
                  折现率设置
                </Divider>

                <Alert
                  message="提示"
                  description="AI 不会猜测折现率。如果合同未明确提到折现率，请留空，系统将标记为缺失状态，需后续人工确认。"
                  type="info"
                  showIcon
                  style={{ marginBottom: 16 }}
                />

                <Row gutter={24}>
                  <Col xs={24} md={8}>
                    <Form.Item
                      label={confidenceLabel(
                        "折现率类型",
                        "discount_rate_type"
                      )}
                      name="discount_rate_type"
                    >
                      <Select placeholder="选择折现率类型" allowClear>
                        <Select.Option value="ibr">集团 IBR</Select.Option>
                        <Select.Option value="entity_specific">
                          法人特定利率
                        </Select.Option>
                        <Select.Option value="contract_specific">
                          合同特定利率
                        </Select.Option>
                        <Select.Option value="implicit_rate">
                          隐含利率
                        </Select.Option>
                      </Select>
                    </Form.Item>
                  </Col>
                  <Col xs={24} md={8}>
                    <Form.Item
                      label={confidenceLabel("折现率 (%)", "discount_rate")}
                      name="discount_rate"
                    >
                      <InputNumber
                        style={{ width: "100%" }}
                        placeholder="例如: 5.0"
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
                  其他费用
                </Divider>

                <Row gutter={24}>
                  <Col xs={24} md={8}>
                    <Form.Item
                      label={confidenceLabel("物业管理费 (CAM)", "cam_amount")}
                      name="cam_amount"
                    >
                      <InputNumber
                        style={{ width: "100%" }}
                        placeholder="金额（可选）"
                        min={0}
                        precision={2}
                      />
                    </Form.Item>
                  </Col>
                  <Col xs={24} md={8}>
                    <Form.Item
                      label={confidenceLabel("服务费", "service_fee")}
                      name="service_fee"
                    >
                      <InputNumber
                        style={{ width: "100%" }}
                        placeholder="金额（可选）"
                        min={0}
                        precision={2}
                      />
                    </Form.Item>
                  </Col>
                </Row>

                {/* ── 标签/备注 ── */}
                <Divider orientation="left" plain>
                  标签 / 备注
                </Divider>

                <Row gutter={24}>
                  <Col xs={24} md={12}>
                    <Form.Item
                      label={confidenceLabel("标签/备注", "tags")}
                      name="tags"
                    >
                      <Input
                        placeholder="例如: #重要 #续租需关注 或输入备注信息（可选）"
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
              支持批量上传合同文件，上传后 AI 将依次解析并提取关键信息。
              您可以在批量确认表格中逐个编辑或选中合同一键创建。
            </div>
          </Card>
        )}
      </AppLayout>
    </ProtectedRoute>
  );
}
