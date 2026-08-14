"use client";

import React from "react";
import type { CSSProperties, ReactNode } from "react";
import {
  CheckCircleFilled,
  ClockCircleFilled,
  CloseCircleFilled,
  ExclamationCircleFilled,
  MinusCircleFilled,
} from "@ant-design/icons";
import { colors } from "../design-system/tokens";

export type StatusKind = "success" | "processing" | "warning" | "error" | "neutral";

const STATUS_ICONS = {
  success: <CheckCircleFilled />,
  processing: <ClockCircleFilled />,
  warning: <ExclamationCircleFilled />,
  error: <CloseCircleFilled />,
  neutral: <MinusCircleFilled />,
} as const;

export function statusKindFromAntColor(color?: string | null): StatusKind {
  switch (color) {
    case "success":
    case "green":
      return "success";
    case "processing":
    case "blue":
      return "processing";
    case "warning":
    case "orange":
    case "gold":
      return "warning";
    case "error":
    case "red":
      return "error";
    default:
      return "neutral";
  }
}

interface StatusTagProps {
  kind?: StatusKind;
  children: ReactNode;
  style?: CSSProperties;
  className?: string;
  icon?: ReactNode;
}

export function StatusTag({ kind = "neutral", children, style, className, icon }: StatusTagProps) {
  const palette = colors.status[kind];
  return (
    <span
      className={className}
      style={{
        ...style,
        display: "inline-flex",
        alignItems: "center",
        gap: 5,
        padding: "2px 9px",
        borderRadius: 4,
        fontSize: 12,
        lineHeight: "18px",
        fontWeight: 500,
        whiteSpace: "nowrap",
        background: palette.bg,
        color: palette.text,
        border: `1px solid ${palette.border}`,
      }}
    >
      <span aria-hidden="true" style={{ display: "inline-flex", fontSize: 11 }}>
        {icon || STATUS_ICONS[kind]}
      </span>
      {children}
    </span>
  );
}
