"use client";

import React from "react";
import { Language, t } from "../../lib/i18n";
import styles from "../landing.module.css";

interface DataArchitectureSectionProps {
  language: Language;
}

export const DataArchitectureSection: React.FC<DataArchitectureSectionProps> = ({
  language,
}) => {
  return (
    <section className={styles.section}>
      <div className={styles.sectionHeader}>
        <span className={styles.sectionBadge}>
          {t("landing.arch_badge", language)}
        </span>
        <h2 className={styles.sectionTitle}>
          {t("landing.arch_title", language)}
        </h2>
        <p className={styles.sectionSubtitle}>
          {t("landing.arch_subtitle", language)}
        </p>
      </div>

      <div className={styles.archPipelineContainer}>
        {/* Stage 1 */}
        <div className={styles.archStageCol}>
          <div className={styles.archStageHeader}>
            <span className={styles.archStageNum}>LAYER 01</span>
            <span className={styles.archStageName}>{t("landing.arch_layer1", language)}</span>
          </div>
          <div className={styles.archNodeStack}>
            <div className={styles.archNode}>{t("landing.arch_node_pos", language)}</div>
            <div className={styles.archNode}>{t("landing.arch_node_hr", language)}</div>
            <div className={styles.archNode}>{t("landing.arch_node_lease", language)}</div>
            <div className={styles.archNode}>{t("landing.arch_node_erp_flow", language)}</div>
          </div>
        </div>

        {/* Connector 1 */}
        <div className={styles.archConnector}>
          <svg className={styles.archConnectorSvg} viewBox="0 0 40 24" fill="none">
            <path d="M0 12 H36 M30 6 L36 12 L30 18" stroke="#a3a3a3" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
        </div>

        {/* Stage 2 */}
        <div className={styles.archStageCol}>
          <div className={styles.archStageHeader}>
            <span className={styles.archStageNum}>LAYER 02</span>
            <span className={styles.archStageName}>{t("landing.arch_layer2", language)}</span>
          </div>
          <div className={styles.archNodeStack}>
            <div className={`${styles.archNode} ${styles.archNodeHighlight}`}>
              {t("landing.arch_node_reconcile", language)}
            </div>
            <div className={styles.archNode}>{t("landing.arch_node_semantics", language)}</div>
            <div className={styles.archNode}>{t("landing.arch_node_currency", language)}</div>
            <div className={styles.archNode}>{t("landing.arch_node_provenance", language)}</div>
          </div>
        </div>

        {/* Connector 2 */}
        <div className={styles.archConnector}>
          <svg className={styles.archConnectorSvg} viewBox="0 0 40 24" fill="none">
            <path d="M0 12 H36 M30 6 L36 12 L30 18" stroke="#a3a3a3" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
        </div>

        {/* Stage 3 */}
        <div className={styles.archStageCol}>
          <div className={styles.archStageHeader}>
            <span className={styles.archStageNum}>LAYER 03</span>
            <span className={styles.archStageName}>{t("landing.arch_layer3", language)}</span>
          </div>
          <div className={styles.archNodeStack}>
            <div className={`${styles.archNode} ${styles.archNodeHighlight}`}>
              {t("landing.arch_node_ebitda", language)}
            </div>
            <div className={styles.archNode}>{t("landing.arch_node_cohort", language)}</div>
            <div className={`${styles.archNode} ${styles.archNodeHighlight}`}>
              {t("landing.arch_node_ifrs", language)}
            </div>
            <div className={styles.archNode}>{t("landing.arch_node_scenario", language)}</div>
          </div>
        </div>

        {/* Connector 3 */}
        <div className={styles.archConnector}>
          <svg className={styles.archConnectorSvg} viewBox="0 0 40 24" fill="none">
            <path d="M0 12 H36 M30 6 L36 12 L30 18" stroke="#a3a3a3" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
        </div>

        {/* Stage 4 */}
        <div className={styles.archStageCol}>
          <div className={styles.archStageHeader}>
            <span className={styles.archStageNum}>LAYER 04</span>
            <span className={styles.archStageName}>{t("landing.arch_layer4", language)}</span>
          </div>
          <div className={styles.archNodeStack}>
            <div className={styles.archNode}>{t("landing.arch_node_pulse", language)}</div>
            <div className={styles.archNode}>{t("landing.arch_node_action", language)}</div>
            <div className={`${styles.archNode} ${styles.archNodeSuccess}`}>
              {t("landing.arch_node_posting", language)}
            </div>
            <div className={styles.archNode}>{t("landing.arch_node_audit", language)}</div>
          </div>
        </div>
      </div>
    </section>
  );
};
