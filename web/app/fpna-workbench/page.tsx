"use client";

import React, { useState } from "react";
import { Tabs, Card, Alert, Space, Typography, Button } from "antd";
import { InfoCircleOutlined, ArrowRightOutlined, CalculatorOutlined } from "@ant-design/icons";
import Link from "next/link";
import AppLayout from "../components/AppLayout";
import ProtectedRoute from "../components/ProtectedRoute";
import { useLanguage } from "../context/LanguageContext";
import { t } from "../lib/i18n";
import { pageTransition } from "../design-system/animations";
import { useFPnAWorkbench } from "./useFPnAWorkbench";
import { VersionManagementTab } from "./components/VersionManagementTab";
import { VersionCompareTab } from "./components/VersionCompareTab";
import { RollingForecastTab } from "./components/RollingForecastTab";
import { DataQualityTab } from "./components/DataQualityTab";
import { GovernanceRegistryTab } from "./components/GovernanceRegistryTab";
import { HelpTrigger } from "../components/HelpDrawer";
import { fpnaWorkbenchHelpContent } from "../components/help-content";

const { Text, Title, Paragraph } = Typography;

export default function FPnAWorkbenchPage() {
  const { language } = useLanguage();
  const [activeTab, setActiveTab] = useState<string>("versions");

  const { snapshot, commands } = useFPnAWorkbench();

  return (
    <ProtectedRoute>
      <AppLayout>
        <div className="fpna-page-container">
          {/* Header Area */}
          <div className="fpna-header-row">
            <div>
              <Space align="center">
                <Title level={3} className="fpna-header-title">
                  <CalculatorOutlined className="fpna-tree-icon" />
                  {t("fpna.workbench_title", language)}
                </Title>
                <HelpTrigger content={fpnaWorkbenchHelpContent(language)} language={language} />
              </Space>
              <Paragraph type="secondary" className="fpna-header-subtitle">
                {t("fpna.workbench_subtitle", language)}
              </Paragraph>
            </div>

            <Link href="/reports?tab=budget">
              <Button icon={<ArrowRightOutlined />}>
                {t("fpna.link_lease_budget", language)}
              </Button>
            </Link>
          </div>

          {/* Operating vs Lease Budget Notice */}
          <Alert
            type="info"
            showIcon
            icon={<InfoCircleOutlined />}
            className="fpna-margin-bottom-16"
            message={
              <Space>
                <strong>{t("fpna.operating_scope_badge", language)}</strong>
                <span>
                  {t("fpna.operating_scope_desc", language)}
                </span>
                <Link href="/reports?tab=budget" className="fpna-bold-link">
                  {t("fpna.lease_budget_name", language)}
                </Link>
              </Space>
            }
          />

          {/* Main Tabs */}
          <Card styles={{ body: { padding: "16px 20px" } }}>
            <Tabs
              activeKey={activeTab}
              onChange={setActiveTab}
              items={[
                {
                  key: "versions",
                  label: `${t("fpna.tab_versions", language)} (${snapshot.versions.length})`,
                  children: (
                    <VersionManagementTab
                      snapshot={snapshot}
                      commands={commands}
                      language={language}
                    />
                  ),
                },
                {
                  key: "compare",
                  label: t("fpna.tab_compare", language),
                  children: (
                    <VersionCompareTab
                      snapshot={snapshot}
                      commands={commands}
                      language={language}
                    />
                  ),
                },
                {
                  key: "forecast",
                  label: t("fpna.tab_rolling_forecast", language) || "滚动预测编制",
                  children: (
                    <RollingForecastTab
                      snapshot={snapshot}
                      commands={commands}
                      language={language}
                    />
                  ),
                },
                {
                  key: "data-quality",
                  label: `${t("fpna.tab_data_quality", language)} (${snapshot.dataQualityItems.filter((i) => i.status === "open").length})`,
                  children: (
                    <DataQualityTab
                      snapshot={snapshot}
                      commands={commands}
                      language={language}
                    />
                  ),
                },
                {
                  key: "governance",
                  label: t("fpna.tab_governance", language),
                  children: (
                    <GovernanceRegistryTab
                      snapshot={snapshot}
                      commands={commands}
                      language={language}
                    />
                  ),
                },
              ]}
            />
          </Card>
        </div>
      </AppLayout>
    </ProtectedRoute>
  );
}
