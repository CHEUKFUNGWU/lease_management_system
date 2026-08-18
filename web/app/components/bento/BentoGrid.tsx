"use client";

import React, { type CSSProperties, type ReactNode } from "react";

export type BentoTileVariant = "hero" | "feature" | "metric" | "accent";

export interface BentoGridProps {
  children: ReactNode;
  columns?: number;
  gap?: number;
  className?: string;
  style?: CSSProperties;
}

export interface BentoTileProps {
  children?: ReactNode;
  span?: number; // grid-column span (1..12)
  rows?: number; // grid-row span (1..3)
  variant?: BentoTileVariant;
  title?: ReactNode;
  subtitle?: ReactNode;
  action?: ReactNode;
  className?: string;
  style?: CSSProperties;
  bodyStyle?: CSSProperties;
  noPadding?: boolean;
}

export function BentoGrid({
  children,
  columns = 12,
  gap = 16,
  className = "",
  style,
}: BentoGridProps) {
  return (
    <div
      className={`bento-grid ${className}`}
      style={{
        display: "grid",
        gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))`,
        gap,
        width: "100%",
        ...style,
      }}
    >
      {children}
    </div>
  );
}

export function BentoTile({
  children,
  span = 12,
  rows = 1,
  variant = "feature",
  title,
  subtitle,
  action,
  className = "",
  style,
  bodyStyle,
  noPadding = false,
}: BentoTileProps) {
  const isHero = variant === "hero";
  const isAccent = variant === "accent";

  return (
    <div
      className={`bento-tile bento-tile--${variant} ${className}`}
      style={{
        gridColumn: `span ${span} / span ${span}`,
        gridRow: `span ${rows} / span ${rows}`,
        position: "relative",
        display: "flex",
        flexDirection: "column",
        background: isAccent ? "var(--accent-bg, #FAF8F5)" : "var(--bg-surface, #FFFFFF)",
        border: "1px solid var(--border-default, #E5E5E5)",
        borderRadius: isHero ? 12 : 10,
        boxShadow: isHero ? "0 4px 12px rgba(0,0,0,0.04), 0 1px 2px rgba(0,0,0,0.02)" : "0 1px 3px rgba(0,0,0,0.02)",
        overflow: "hidden",
        ...style,
      }}
    >
      {/* Corner Wash Accent for Hero */}
      {isHero && (
        <div
          style={{
            position: "absolute",
            top: 0,
            right: 0,
            width: 140,
            height: 140,
            background: "radial-gradient(circle at top right, var(--morandi-sand, #D8BB8F) 0%, transparent 70%)",
            opacity: 0.12,
            pointerEvents: "none",
            zIndex: 0,
          }}
        />
      )}

      {/* Header if title or action is provided */}
      {(title || action) && (
        <div
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            padding: "14px 18px",
            borderBottom: "1px solid var(--border-subtle, #F0F0F0)",
            position: "relative",
            zIndex: 1,
          }}
        >
          <div>
            {title && (
              <div
                style={{
                  fontSize: isHero ? 16 : 14,
                  fontWeight: 600,
                  color: "var(--fg-primary, #1A1A1A)",
                }}
              >
                {title}
              </div>
            )}
            {subtitle && (
              <div style={{ fontSize: 12, color: "var(--fg-muted, #8C8C8C)", marginTop: 2 }}>
                {subtitle}
              </div>
            )}
          </div>
          {action && <div>{action}</div>}
        </div>
      )}

      {/* Body Content */}
      <div
        style={{
          flex: 1,
          padding: noPadding ? 0 : "16px 18px",
          position: "relative",
          zIndex: 1,
          display: "flex",
          flexDirection: "column",
          ...bodyStyle,
        }}
      >
        {children}
      </div>
    </div>
  );
}
