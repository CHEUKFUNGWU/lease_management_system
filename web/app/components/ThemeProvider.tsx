"use client";

import React from "react";
import { ConfigProvider } from "antd";
import { antdTheme } from "../design-system/theme";

export default function ThemeProvider({ children }: { children: React.ReactNode }) {
  return (
    <ConfigProvider theme={antdTheme}>
      {children}
    </ConfigProvider>
  );
}
