"use client";

// R0-3：页面定位说明（scope note）。三个页面（/performance、/operating-pulse、
// /roi）各有一条 type="info" 的定位说明，放在 PageHeader 之下、内容之上。
// 抽成组件是为了让守卫测试能直接渲染它——整页 SSR 被 ProtectedRoute /
// AuthContext 挡住，够不着这条 Alert。
//
// 信息色不是警告色：这不是错误，是告诉使用者这一页回答什么问题、
// 另一个相近页面在哪里。

import React from "react";
import { Alert } from "antd";
import { t, type Language } from "./i18n";

export default function ScopeNote({ noteKey, className, language }: { noteKey: string; className?: string; language: Language }) {
  return <Alert type="info" showIcon className={className} message={t(noteKey, language)} />;
}
