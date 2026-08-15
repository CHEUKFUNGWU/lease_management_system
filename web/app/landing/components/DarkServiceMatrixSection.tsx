"use client";

import React from "react";
import { Language, t } from "../../lib/i18n";
import { StaggerGroup, StaggerItem } from "./ScrollReveal";
import styles from "../landing.module.css";

interface DarkServiceMatrixSectionProps {
  language: Language;
}

export const DarkServiceMatrixSection: React.FC<DarkServiceMatrixSectionProps> = ({
  language,
}) => {
  const cards = [
    {
      title: "landing.matrix_card1_title",
      desc: "landing.matrix_card1_desc",
    },
    {
      title: "landing.matrix_card2_title",
      desc: "landing.matrix_card2_desc",
    },
    {
      title: "landing.matrix_card3_title",
      desc: "landing.matrix_card3_desc",
    },
    {
      title: "landing.matrix_card4_title",
      desc: "landing.matrix_card4_desc",
    },
    {
      title: "landing.matrix_card5_title",
      desc: "landing.matrix_card5_desc",
    },
    {
      title: "landing.matrix_card6_title",
      desc: "landing.matrix_card6_desc",
    },
  ];

  return (
    <section className={styles.darkMatrixWrapper}>
      <div className={styles.darkMatrixInner}>
        <StaggerGroup>
          <StaggerItem index={0}>
            <div className={styles.darkMatrixHeader}>
              <span className={styles.darkMatrixBadge}>
                {t("landing.matrix_dark_badge", language)}
              </span>
              <h2 className={styles.darkMatrixTitle}>
                {t("landing.matrix_dark_title", language)}
              </h2>
              <p className={styles.darkMatrixSubtitle}>
                {t("landing.matrix_dark_subtitle", language)}
              </p>
            </div>
          </StaggerItem>

          {/* 6 Geometric Dark Cards with Cascading Delay */}
          <div className={styles.darkMatrixGrid}>
            {cards.map((card, idx) => (
              <StaggerItem key={idx} index={idx + 1} className={styles.darkMatrixCard}>
                <div className={styles.darkMatrixCardTop}>
                  <h3 className={styles.darkMatrixCardTitle}>
                    {t(card.title, language)}
                  </h3>
                  <span className={styles.darkMatrixArrow}>↗</span>
                </div>
                <p className={styles.darkMatrixCardDesc}>
                  {t(card.desc, language)}
                </p>
              </StaggerItem>
            ))}
          </div>

          {/* Bold Numeric Metric Highlights Bar */}
          <StaggerItem index={4}>
            <div className={styles.darkMatrixStatsBar}>
              <div className={styles.darkStatItem}>
                <span className={styles.darkStatValue}>
                  {t("landing.hero_metric_stores", language)}
                </span>
                <span className={styles.darkStatLabel}>
                  {t("landing.hero_metric_stores_label", language)}
                </span>
              </div>

              <div className={styles.darkStatItem}>
                <span className={styles.darkStatValue}>
                  {t("landing.hero_metric_compliance", language)}
                </span>
                <span className={styles.darkStatLabel}>
                  {t("landing.hero_metric_compliance_label", language)}
                </span>
              </div>

              <div className={styles.darkStatItem}>
                <span className={styles.darkStatValue}>
                  {t("landing.hero_metric_margin", language)}
                </span>
                <span className={styles.darkStatLabel}>
                  {t("landing.hero_metric_margin_label", language)}
                </span>
              </div>

              <div className={styles.darkStatItem}>
                <span className={styles.darkStatValue}>
                  {t("landing.hero_metric_closing", language)}
                </span>
                <span className={styles.darkStatLabel}>
                  {t("landing.hero_metric_closing_label", language)}
                </span>
              </div>
            </div>
          </StaggerItem>
        </StaggerGroup>
      </div>
    </section>
  );
};
