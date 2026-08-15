"use client";

import React from "react";
import { Language, t } from "../../lib/i18n";
import { StaggerGroup, StaggerItem } from "./ScrollReveal";
import styles from "../landing.module.css";

interface AsymmetricBentoSectionProps {
  language: Language;
}

export const AsymmetricBentoSection: React.FC<AsymmetricBentoSectionProps> = ({
  language,
}) => {
  return (
    <section className={styles.section}>
      <StaggerGroup>
        <StaggerItem index={0}>
          <div className={styles.sectionHeader}>
            <span className={styles.sectionBadge}>
              {t("landing.asym_badge", language)}
            </span>
            <h2 className={styles.sectionTitle}>
              {t("landing.asym_title", language)}
            </h2>
            <p className={styles.sectionSubtitle}>
              {t("landing.asym_subtitle", language)}
            </p>
          </div>
        </StaggerItem>

        <div className={styles.asymBentoContainer}>
          {/* Left Bento: Tall Deep Forest Emerald Card with Vertical Bar Chart */}
          <StaggerItem index={1} className={styles.asymCardLeftDark}>
            <div className={styles.asymCardHeader}>
              <span className={styles.asymTagDark}>4-WALL EBITDA</span>
              <h3 className={styles.asymTitleDark}>
                {t("landing.asym_card1_title", language)}
              </h3>
              <p className={styles.asymDescDark}>
                {t("landing.asym_card1_desc", language)}
              </p>
            </div>

            {/* Vertical Bar Chart Graphic with Arrow */}
            <div className={styles.asymChartContainer}>
              <div className={styles.asymGrowthIndicator}>
                <span className={styles.asymGrowthVal}>
                  {t("landing.asym_card1_growth", language)}
                </span>
                <span className={styles.asymArrowUp}>↑</span>
              </div>

              <div className={styles.asymBarChartGrid}>
                <div className={styles.asymBarCol}>
                  <div className={`${styles.asymBarFill} ${styles.barHeight40}`} />
                  <span className={styles.asymBarLabel}>Q1</span>
                </div>
                <div className={styles.asymBarCol}>
                  <div className={`${styles.asymBarFill} ${styles.barHeight55}`} />
                  <span className={styles.asymBarLabel}>Q2</span>
                </div>
                <div className={styles.asymBarCol}>
                  <div className={`${styles.asymBarFill} ${styles.barHeight70}`} />
                  <span className={styles.asymBarLabel}>Q3</span>
                </div>
                <div className={styles.asymBarCol}>
                  <div className={`${styles.asymBarFill} ${styles.barHeight95} ${styles.barHighlightEmerald}`} />
                  <span className={styles.asymBarLabel}>Q4</span>
                </div>
              </div>
            </div>
          </StaggerItem>

          {/* Right Bento: Wide Soft Sage Card with Concentric NPV Radar */}
          <StaggerItem index={2} className={styles.asymCardRightSage}>
            <div className={styles.asymCardHeader}>
              <span className={styles.asymTagLight}>SCENARIO WORKBENCH</span>
              <h3 className={styles.asymTitleLight}>
                {t("landing.asym_card2_title", language)}
              </h3>
              <p className={styles.asymDescLight}>
                {t("landing.asym_card2_desc", language)}
              </p>
            </div>

            {/* Concentric Radar Graph & Floating NPV Badge */}
            <div className={styles.asymRadarContainer}>
              <div className={styles.asymRadarPillBadge}>
                <span className={styles.asymRadarPillValue}>
                  {t("landing.asym_card2_npv", language)}
                </span>
                <span className={styles.asymRadarPillSub}>OPTION A SIMULATED</span>
              </div>

              <div className={styles.asymConcentricCircles}>
                <div className={styles.asymCircleOuter} />
                <div className={styles.asymCircleMiddle} />
                <div className={styles.asymCircleInner}>
                  <span className={styles.asymCenterPulse} />
                </div>
              </div>
            </div>
          </StaggerItem>
        </div>
      </StaggerGroup>
    </section>
  );
};
