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
    <div className="help-flow-vertical">
      {steps.map((step, index) => {
        const isLast = index === steps.length - 1;
        return (
          <div key={step.key} className="help-flow-step">
            <div className="help-flow-rail">
              <div className="help-flow-number">{index + 1}</div>
              {!isLast && <div className="help-flow-connector" />}
            </div>
            <div className={`help-flow-copy${isLast ? " is-last" : ""}`}>
              <div className="help-flow-card">
                <div className="help-flow-label">{step.label}</div>
                {step.desc && <div className="help-flow-description">{step.desc}</div>}
              </div>
            </div>
          </div>
        );
      })}
    </div>
  );
}

