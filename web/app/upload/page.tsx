"use client";

import { useState } from "react";
import {
  Upload,
  Button,
  Card,
  message,
  Tabs,
  List,
  Progress,
  Tag,
} from "antd";
import {
  UploadOutlined,
  FilePdfOutlined,
  FileExcelOutlined,
  FileImageOutlined,
} from "@ant-design/icons";
import AppLayout from "../components/AppLayout";
import ProtectedRoute from "../components/ProtectedRoute";

const { Dragger } = Upload;

export default function UploadPage() {
  const [fileList, setFileList] = useState<any[]>([]);

  const uploadProps = {
    name: "file",
    multiple: true,
    action: "/api/ai/files/upload",
    onChange(info: any) {
      const { status } = info.file;
      if (status === "done") {
        message.success(`${info.file.name} 上传成功`);
      } else if (status === "error") {
        message.error(`${info.file.name} 上传失败`);
      }
      setFileList(info.fileList);
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

  const getFileIcon = (type: string) => {
    if (type.includes("pdf")) return <FilePdfOutlined style={{ color: "#ff4d4f" }} />;
    if (type.includes("excel") || type.includes("sheet"))
      return <FileExcelOutlined style={{ color: "#52c41a" }} />;
    return <FileImageOutlined style={{ color: "#1890ff" }} />;
  };

  return (
    <ProtectedRoute>
      <AppLayout>
        <h1 style={{ marginBottom: 24 }}>文件上传</h1>

        <Tabs
          items={[
            {
              key: "contract",
              label: "合同文件",
              children: (
                <Card>
                  <Dragger {...uploadProps}>
                    <p className="ant-upload-drag-icon">
                      <UploadOutlined />
                    </p>
                    <p className="ant-upload-text">
                      点击或拖拽文件到此区域上传
                    </p>
                    <p className="ant-upload-hint">
                      支持 PDF、Excel、JPG、PNG、TIFF 格式，单个文件不超过 50MB
                    </p>
                  </Dragger>
                </Card>
              ),
            },
            {
              key: "payment",
              label: "租金表",
              children: (
                <Card>
                  <Dragger {...uploadProps}>
                    <p className="ant-upload-drag-icon">
                      <UploadOutlined />
                    </p>
                    <p className="ant-upload-text">
                      点击或拖拽租金表到此区域上传
                    </p>
                    <p className="ant-upload-hint">
                      支持 Excel、PDF 格式
                    </p>
                  </Dragger>
                </Card>
              ),
            },
          ]}
        />

        {fileList.length > 0 && (
          <Card title="上传队列" style={{ marginTop: 16 }}>
            <List
              dataSource={fileList}
              renderItem={(file: any) => (
                <List.Item
                  actions={[
                    file.status === "uploading" ? (
                      <Progress percent={file.percent} size="small" />
                    ) : file.status === "done" ? (
                      <Tag color="success">完成</Tag>
                    ) : file.status === "error" ? (
                      <Tag color="error">失败</Tag>
                    ) : (
                      <Tag>等待中</Tag>
                    ),
                  ]}
                >
                  <List.Item.Meta
                    avatar={getFileIcon(file.type)}
                    title={file.name}
                    description={`${(file.size / 1024 / 1024).toFixed(2)} MB`}
                  />
                </List.Item>
              )}
            />
          </Card>
        )}
      </AppLayout>
    </ProtectedRoute>
  );
}
