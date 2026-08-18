"use client";

import React from "react";

export interface FlowStep {
  key: string;
  label: string;
  desc?: string;
}

export default function HelpFlowDiagram({ steps }: { steps: FlowStep[] }) {
  if (!steps || steps.length === 0) return null;

  return (
    <div className="help-flow-vertical" style={{ display: "flex", flexDirection: "column", gap: 0, padding: "4px 0 12px" }}>
      {steps.map((step, index) => {
        const isLast = index === steps.length - 1;
        return (
          <div key={step.key} style={{ display: "flex", gap: 12 }}>
            <div style={{ display: "flex", flexDirection: "column", alignItems: "center", width: 22 }}>
              <div
                style={{
                  width: 22,
                  height: 22,
                  borderRadius: "50%",
                  background: "var(--bg-inset, #F1F5F9)",
                  border: "1.5px solid var(--fg-primary, #0F172A)",
                  color: "var(--fg-primary, #0F172A)",
                  fontSize: 11,
                  fontWeight: 600,
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  flexShrink: 0,
                }}
              >
                {index + 1}
              </div>
              {!isLast && (
                <div
                  style={{
                    width: 1.5,
                    flex: 1,
                    minHeight: 20,
                    background: "var(--border-strong, #CBD5E1)",
                    margin: "4px 0",
                  }}
                />
              )}
            </div>
            <div style={{ paddingBottom: isLast ? 0 : 12, flex: 1 }}>
              <div
                style={{
                  background: "var(--bg-surface, #FFFFFF)",
                  border: "1px solid var(--border-default, #E2E8F0)",
                  borderRadius: 6,
                  padding: "6px 12px",
                  boxShadow: "0 1px 2px rgba(0, 0, 0, 0.04)",
                }}
              >
                <div style={{ fontSize: 13, fontWeight: 600, color: "var(--fg-primary, #0F172A)" }}>
                  {step.label}
                </div>
                {step.desc && (
                  <div style={{ fontSize: 12, color: "var(--fg-secondary, #64748B)", marginTop: 2 }}>
                    {step.desc}
                  </div>
                )}
              </div>
            </div>
          </div>
        );
      })}
    </div>
  );
}

