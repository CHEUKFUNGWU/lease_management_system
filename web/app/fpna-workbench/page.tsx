"use client";

import React, { useState } from "react";
import { Tabs, Card, Button } from "antd";
import { ArrowRightOutlined } from "@ant-design/icons";
import Link from "next/link";
import AppLayout from "../components/AppLayout";
import PageHeader from "../components/PageHeader";
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


export default function FPnAWorkbenchPage() {
  const { language } = useLanguage();
  const [activeTab, setActiveTab] = useState<string>("versions");

  const { snapshot, commands } = useFPnAWorkbench();

  return (
    <ProtectedRoute>
      <AppLayout>
        <div className="fpna-page-container">
          <PageHeader
            title={t("fpna.workbench_title", language)}
            help={<HelpTrigger content={fpnaWorkbenchHelpContent(language)} language={language} />}
            secondaryAction={
              <Link href="/reports?tab=budget">
                <Button icon={<ArrowRightOutlined />}>
                  {t("fpna.link_lease_budget", language)}
                </Button>
              </Link>
            }
          />

          <div className="fpna-scope-notice" role="note">
            <div className="fpna-scope-notice-head">
              <strong>{t("fpna.operating_scope_badge", language)}</strong>
              <Link href="/reports?tab=budget">
                {t("fpna.link_lease_budget", language)} <ArrowRightOutlined />
              </Link>
            </div>
            <div className="fpna-scope-notice-copy">{t("fpna.operating_scope_desc", language)}</div>
          </div>

          {/* Main Tabs */}
          <Card className="fpna-workbench-tabs-card">
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
