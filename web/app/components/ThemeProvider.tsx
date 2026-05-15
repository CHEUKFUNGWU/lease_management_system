"use client";

import React from "react";
import { ConfigProvider } from "antd";

export default function ThemeProvider({ children }: { children: React.ReactNode }) {
  return (
    <ConfigProvider
      theme={{
        token: {
          // Core colors: monochrome
          colorPrimary: "#000000",
          colorInfo: "#000000",
          colorSuccess: "#10B981",
          colorWarning: "#F59E0B",
          colorError: "#EF4444",

          // Base
          colorBgBase: "#FFFFFF",
          colorTextBase: "#000000",
          borderRadius: 12,
          wireframe: false,
          fontFamily:
            '"Inter", -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',

          // Softer borders
          colorBorder: "#E5E5E5",
          colorBorderSecondary: "#F0F0F0",

          // Typography
          fontSize: 14,
          lineHeight: 1.6,
        },
        components: {
          Button: {
            borderRadius: 9999,
            controlHeight: 40,
            fontWeight: 600,
            defaultBg: "#FFFFFF",
            defaultBorderColor: "#E5E5E5",
            primaryShadow: "none",
          },
          Card: {
            borderRadiusLG: 20,
            boxShadowTertiary: "none",
          },
          Menu: {
            itemBorderRadius: 8,
            activeBarBorderWidth: 0,
            itemSelectedBg: "#F5F5F5",
            itemSelectedColor: "#000000",
            itemHoverBg: "#FAFAFA",
            itemColor: "#666666",
          },
          Layout: {
            bodyBg: "#FFFFFF",
            headerBg: "#FFFFFF",
            siderBg: "#FFFFFF",
          },
          Table: {
            headerBg: "#FAFAFA",
            headerBorderRadius: 12,
            borderColor: "#F0F0F0",
            rowHoverBg: "#FAFAFA",
          },
          Input: {
            borderRadius: 10,
            activeBorderColor: "#000000",
            hoverBorderColor: "#000000",
          },
          Select: {
            borderRadius: 10,
          },
          DatePicker: {
            borderRadius: 10,
          },
          Modal: {
            borderRadiusLG: 20,
          },
          Tag: {
            borderRadiusSM: 6,
          },
          Descriptions: {
            borderRadiusLG: 12,
          },
          Timeline: {
            dotBorderWidth: 2,
          },
          Statistic: {
            titleFontSize: 13,
            contentFontSize: 24,
          },
        },
      }}
    >
      {children}
    </ConfigProvider>
  );
}
