"use client";

import React from "react";
import { CheckOutlined } from "@ant-design/icons";
import { Language, t } from "../../lib/i18n";
import styles from "../landing.module.css";

interface PillarsSectionProps {
  language: Language;
}

export const PillarsSection: React.FC<PillarsSectionProps> = ({ language }) => {
  return (
    <section id="pillars" className={styles.section}>
      <div className={styles.sectionHeader}>
        <span className={styles.sectionBadge}>
          {t("landing.pillars_badge", language)}
        </span>
        <h2 className={styles.sectionTitle}>
          {t("landing.pillars_title", language)}
        </h2>
        <p className={styles.sectionSubtitle}>
          {t("landing.pillars_subtitle", language)}
        </p>
      </div>

      <div className={styles.pillarsGrid}>
        {/* Pillar 1 */}
        <div className={styles.pillarCard}>
          <div className={styles.pillarGraphicBox}>
            <div className={styles.pillarMiniBadgeRow}>
              <span className={styles.pillarMiniTag}>STORE-DAY FACTS</span>
              <span className={styles.pillarMiniStatus}>● 99.4% RECONCILED</span>
            </div>
            <div className={styles.pillarGraphicContent}>
              <div className={styles.pillarMiniDataRow}>
                <span>{t("landing.pillar_widget_sync", language)}</span>
                <span className={styles.pillarMiniVal}>{t("landing.pillar_widget_sync_status", language)}</span>
              </div>
            </div>
          </div>

          <h3 className={styles.pillarTitle}>
            {t("landing.pillar1_title", language)}
          </h3>
          <p className={styles.pillarDesc}>
            {t("landing.pillar1_desc", language)}
          </p>
          <ul className={styles.pillarFeatureList}>
            <li className={styles.pillarFeatureItem}>
              <span className={styles.pillarCheckIcon} aria-hidden="true">
                <CheckOutlined />
              </span>
              <span>{t("landing.pillar1_f1", language)}</span>
            </li>
            <li className={styles.pillarFeatureItem}>
              <span className={styles.pillarCheckIcon} aria-hidden="true">
                <CheckOutlined />
              </span>
              <span>{t("landing.pillar1_f2", language)}</span>
            </li>
            <li className={styles.pillarFeatureItem}>
              <span className={styles.pillarCheckIcon} aria-hidden="true">
                <CheckOutlined />
              </span>
              <span>{t("landing.pillar1_f3", language)}</span>
            </li>
          </ul>
        </div>

        {/* Pillar 2 */}
        <div className={styles.pillarCard}>
          <div className={styles.pillarGraphicBox}>
            <div className={styles.pillarMiniBadgeRow}>
              <span className={styles.pillarMiniTag}>4-WALL EBITDA</span>
              <span className={styles.pillarMiniStatus}>● COHORT P35</span>
            </div>
            <div className={styles.pillarGraphicContent}>
              <div className={styles.pillarMiniWaterfallBar}>
                <div className={`${styles.pillarMiniBarFill} ${styles.miniFillGreen}`} />
                <div className={`${styles.pillarMiniBarFill} ${styles.miniFillRed}`} />
                <div className={`${styles.pillarMiniBarFill} ${styles.miniFillBlue}`} />
              </div>
            </div>
          </div>

          <h3 className={styles.pillarTitle}>
            {t("landing.pillar2_title", language)}
          </h3>
          <p className={styles.pillarDesc}>
            {t("landing.pillar2_desc", language)}
          </p>
          <ul className={styles.pillarFeatureList}>
            <li className={styles.pillarFeatureItem}>
              <span className={styles.pillarCheckIcon} aria-hidden="true">
                <CheckOutlined />
              </span>
              <span>{t("landing.pillar2_f1", language)}</span>
            </li>
            <li className={styles.pillarFeatureItem}>
              <span className={styles.pillarCheckIcon} aria-hidden="true">
                <CheckOutlined />
              </span>
              <span>{t("landing.pillar2_f2", language)}</span>
            </li>
            <li className={styles.pillarFeatureItem}>
              <span className={styles.pillarCheckIcon} aria-hidden="true">
                <CheckOutlined />
              </span>
              <span>{t("landing.pillar2_f3", language)}</span>
            </li>
          </ul>
        </div>

        {/* Pillar 3 */}
        <div className={styles.pillarCard}>
          <div className={styles.pillarGraphicBox}>
            <div className={styles.pillarMiniBadgeRow}>
              <span className={styles.pillarMiniTag}>NPV SIMULATION</span>
              <span className={styles.pillarMiniStatus}>● 3 OPTIONS READY</span>
            </div>
            <div className={styles.pillarGraphicContent}>
              <div className={styles.pillarMiniDataRow}>
                <span>{t("landing.pillar_widget_npv_opt", language)}</span>
                <span className={styles.pillarMiniVal}>{t("landing.pillar_widget_npv_val", language)}</span>
              </div>
            </div>
          </div>

          <h3 className={styles.pillarTitle}>
            {t("landing.pillar3_title", language)}
          </h3>
          <p className={styles.pillarDesc}>
            {t("landing.pillar3_desc", language)}
          </p>
          <ul className={styles.pillarFeatureList}>
            <li className={styles.pillarFeatureItem}>
              <span className={styles.pillarCheckIcon} aria-hidden="true">
                <CheckOutlined />
              </span>
              <span>{t("landing.pillar3_f1", language)}</span>
            </li>
            <li className={styles.pillarFeatureItem}>
              <span className={styles.pillarCheckIcon} aria-hidden="true">
                <CheckOutlined />
              </span>
              <span>{t("landing.pillar3_f2", language)}</span>
            </li>
            <li className={styles.pillarFeatureItem}>
              <span className={styles.pillarCheckIcon} aria-hidden="true">
                <CheckOutlined />
              </span>
              <span>{t("landing.pillar3_f3", language)}</span>
            </li>
          </ul>
        </div>

        {/* Pillar 4 */}
        <div className={styles.pillarCard}>
          <div className={styles.pillarGraphicBox}>
            <div className={styles.pillarMiniBadgeRow}>
              <span className={styles.pillarMiniTag}>IFRS 16 / ASC 842</span>
              <span className={styles.pillarMiniStatus}>● SAP / ORACLE POSTED</span>
            </div>
            <div className={styles.pillarGraphicContent}>
              <div className={styles.pillarMiniDataRow}>
                <span>{t("landing.pillar_widget_ifrs_ledger", language)}</span>
                <span className={styles.pillarMiniVal}>{t("landing.pillar_widget_ifrs_assert", language)}</span>
              </div>
            </div>
          </div>

          <h3 className={styles.pillarTitle}>
            {t("landing.pillar4_title", language)}
          </h3>
          <p className={styles.pillarDesc}>
            {t("landing.pillar4_desc", language)}
          </p>
          <ul className={styles.pillarFeatureList}>
            <li className={styles.pillarFeatureItem}>
              <span className={styles.pillarCheckIcon} aria-hidden="true">
                <CheckOutlined />
              </span>
              <span>{t("landing.pillar4_f1", language)}</span>
            </li>
            <li className={styles.pillarFeatureItem}>
              <span className={styles.pillarCheckIcon} aria-hidden="true">
                <CheckOutlined />
              </span>
              <span>{t("landing.pillar4_f2", language)}</span>
            </li>
            <li className={styles.pillarFeatureItem}>
              <span className={styles.pillarCheckIcon} aria-hidden="true">
                <CheckOutlined />
              </span>
              <span>{t("landing.pillar4_f3", language)}</span>
            </li>
          </ul>
        </div>
      </div>
    </section>
  );
};
