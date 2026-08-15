"use client";

import React from "react";
import { Language, t } from "../../lib/i18n";
import styles from "../landing.module.css";

interface WorkflowSectionProps {
  language: Language;
}

export const WorkflowSection: React.FC<WorkflowSectionProps> = ({ language }) => {
  const steps = [
    {
      num: "landing.step1_num",
      title: "landing.step1_title",
      desc: "landing.step1_desc",
      tag: "POS · ERP · HR",
    },
    {
      num: "landing.step2_num",
      title: "landing.step2_title",
      desc: "landing.step2_desc",
      tag: "EBITDA · 4-Wall",
    },
    {
      num: "landing.step3_num",
      title: "landing.step3_title",
      desc: "landing.step3_desc",
      tag: "NPV · Simulation",
    },
    {
      num: "landing.step4_num",
      title: "landing.step4_title",
      desc: "landing.step4_desc",
      tag: "IFRS 16 · Audit",
    },
  ];

  return (
    <section className={styles.section}>
      <div className={styles.sectionHeader}>
        <span className={styles.sectionBadge}>
          {t("landing.workflow_badge", language)}
        </span>
        <h2 className={styles.sectionTitle}>
          {t("landing.workflow_title", language)}
        </h2>
        <p className={styles.sectionSubtitle}>
          {t("landing.workflow_subtitle", language)}
        </p>
      </div>

      <div className={styles.workflowGrid}>
        {steps.map((step, idx) => (
          <div key={idx} className={styles.workflowCard}>
            <div className={styles.workflowCardTop}>
              <span className={styles.workflowStepNumber}>
                {t(step.num, language)}
              </span>
              <span className={styles.workflowTag}>{step.tag}</span>
            </div>
            <h3 className={styles.workflowStepTitle}>
              {t(step.title, language)}
            </h3>
            <p className={styles.workflowStepDesc}>
              {t(step.desc, language)}
            </p>
          </div>
        ))}
      </div>
    </section>
  );
};
