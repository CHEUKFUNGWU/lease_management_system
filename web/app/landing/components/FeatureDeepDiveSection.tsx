"use client";

import React, { useState } from "react";
import { CheckOutlined } from "@ant-design/icons";
import { Language, t } from "../../lib/i18n";
import styles from "../landing.module.css";

interface FeatureDeepDiveSectionProps {
  language: Language;
}

export const FeatureDeepDiveSection: React.FC<FeatureDeepDiveSectionProps> = ({
  language,
}) => {
  const [activeChapter, setActiveChapter] = useState<number>(0);

  const chapters = [
    {
      num: "01",
      titleKey: "landing.pillar1_title",
      descKey: "landing.pillar1_desc",
      f1Key: "landing.pillar1_f1",
      f2Key: "landing.pillar1_f2",
      f3Key: "landing.pillar1_f3",
      tag: "DAILY TRIAGE",
    },
    {
      num: "02",
      titleKey: "landing.pillar2_title",
      descKey: "landing.pillar2_desc",
      f1Key: "landing.pillar2_f1",
      f2Key: "landing.pillar2_f2",
      f3Key: "landing.pillar2_f3",
      tag: "4-WALL EBITDA",
    },
    {
      num: "03",
      titleKey: "landing.pillar3_title",
      descKey: "landing.pillar3_desc",
      f1Key: "landing.pillar3_f1",
      f2Key: "landing.pillar3_f2",
      f3Key: "landing.pillar3_f3",
      tag: "NPV SIMULATION",
    },
  ];

  return (
    <section className={styles.section}>
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

      <div className={styles.deepDiveLayout}>
        {/* Left Side: Chapter Selectors */}
        <div className={styles.deepDiveNavCol}>
          {chapters.map((ch, idx) => (
            <div
              key={idx}
              className={`${styles.deepDiveNavItem} ${activeChapter === idx ? styles.deepDiveNavActive : ""}`}
              onClick={() => setActiveChapter(idx)}
            >
              <div className={styles.deepDiveNavHeader}>
                <span className={styles.deepDiveNavNumber}>{ch.num}</span>
                <span className={styles.deepDiveNavTag}>{ch.tag}</span>
              </div>
              <h3 className={styles.deepDiveNavTitle}>
                {t(ch.titleKey, language)}
              </h3>
              <p className={styles.deepDiveNavDesc}>
                {t(ch.descKey, language)}
              </p>
              <ul className={styles.deepDiveFeatureBullets}>
                <li className={styles.deepDiveBulletItem}>
                  <span className={styles.deepDiveBulletIcon}>
                    <CheckOutlined />
                  </span>
                  <span>{t(ch.f1Key, language)}</span>
                </li>
                <li className={styles.deepDiveBulletItem}>
                  <span className={styles.deepDiveBulletIcon}>
                    <CheckOutlined />
                  </span>
                  <span>{t(ch.f2Key, language)}</span>
                </li>
                <li className={styles.deepDiveBulletItem}>
                  <span className={styles.deepDiveBulletIcon}>
                    <CheckOutlined />
                  </span>
                  <span>{t(ch.f3Key, language)}</span>
                </li>
              </ul>
            </div>
          ))}
        </div>

        {/* Right Side: Morphing Interactive Visual Stage */}
        <div className={styles.deepDiveVisualCol}>
          {activeChapter === 0 && (
            <div className={styles.stageWindow}>
              <div className={styles.stageWindowHeader}>
                <span className={styles.stageWindowDot} />
                <span className={styles.stageWindowTitle}>
                  STORE-DAY PULSE RADAR · T+1 ANOMALY STREAM
                </span>
              </div>
              <div className={styles.stageWindowContent}>
                <div className={styles.stageMetricBoxRow}>
                  <div className={styles.stageMetricMiniBox}>
                    <span className={styles.stageMiniLabel}>MONITORED STORES</span>
                    <span className={styles.stageMiniVal}>128 STORES</span>
                  </div>
                  <div className={styles.stageMetricMiniBox}>
                    <span className={styles.stageMiniLabel}>ATTENTION SCORE</span>
                    <span className={styles.stageMiniValHighlight}>88 / HIGH RISK</span>
                  </div>
                  <div className={styles.stageMetricMiniBox}>
                    <span className={styles.stageMiniLabel}>DATA INTEGRITY</span>
                    <span className={styles.stageMiniVal}>99.4% VERIFIED</span>
                  </div>
                </div>

                <div className={styles.stageAnomalyList}>
                  <div className={styles.stageAnomalyItem}>
                    <div className={styles.stageAnomalyBadgeRed}>CRITICAL</div>
                    <div className={styles.stageAnomalyText}>
                      {t("landing.deepdive_anomaly_critical", language)}
                    </div>
                  </div>
                  <div className={styles.stageAnomalyItem}>
                    <div className={styles.stageAnomalyBadgeAmber}>WARNING</div>
                    <div className={styles.stageAnomalyText}>
                      {t("landing.deepdive_anomaly_warning", language)}
                    </div>
                  </div>
                  <div className={styles.stageAnomalyItem}>
                    <div className={styles.stageAnomalyBadgeGreen}>RESOLVED</div>
                    <div className={styles.stageAnomalyText}>
                      {t("landing.deepdive_anomaly_resolved", language)}
                    </div>
                  </div>
                </div>
              </div>
            </div>
          )}

          {activeChapter === 1 && (
            <div className={styles.stageWindow}>
              <div className={styles.stageWindowHeader}>
                <span className={styles.stageWindowDot} />
                <span className={styles.stageWindowTitle}>
                  4-WALL EBITDA DECOMPOSITION · COHORT BENCHMARK
                </span>
              </div>
              <div className={styles.stageWindowContent}>
                <div className={styles.stageCohortBarRow}>
                  <div className={styles.stageCohortHeader}>
                    <span>COHORT POSITIONING (28 REGIONAL STORES)</span>
                    <span className={styles.stageCohortP35}>P35 (BELOW MEDIAN)</span>
                  </div>
                  <div className={styles.stageCohortBarTrack}>
                    <div className={`${styles.stageCohortFill} ${styles.cohortFillP35}`} />
                  </div>
                </div>

                <div className={styles.stageWaterfallGraphic}>
                  <div className={styles.stageWfItem}>
                    <span className={styles.stageWfLabel}>REVENUE (¥420K)</span>
                    <div className={styles.stageWfBarTrack}>
                      <div className={`${styles.stageWfBarFill} ${styles.barFill100}`} />
                    </div>
                  </div>
                  <div className={styles.stageWfItem}>
                    <span className={styles.stageWfLabel}>GROSS MARGIN (55.0%)</span>
                    <div className={styles.stageWfBarTrack}>
                      <div className={`${styles.stageWfBarFill} ${styles.barFill55}`} />
                    </div>
                  </div>
                  <div className={styles.stageWfItem}>
                    <span className={styles.stageWfLabel}>OCCUPANCY RENT (22.0% ⚠️)</span>
                    <div className={styles.stageWfBarTrack}>
                      <div className={`${styles.stageWfBarFill} ${styles.barFill22}`} />
                    </div>
                  </div>
                  <div className={styles.stageWfItem}>
                    <span className={styles.stageWfLabel}>4-WALL EBITDA (10.0%)</span>
                    <div className={styles.stageWfBarTrack}>
                      <div className={`${styles.stageWfBarFill} ${styles.barFill10}`} />
                    </div>
                  </div>
                </div>
              </div>
            </div>
          )}

          {activeChapter === 2 && (
            <div className={styles.stageWindow}>
              <div className={styles.stageWindowHeader}>
                <span className={styles.stageWindowDot} />
                <span className={styles.stageWindowTitle}>
                  DYNAMIC LEASE SIMULATION · NPV & PAYBACK MODEL
                </span>
              </div>
              <div className={styles.stageWindowContent}>
                <div className={styles.stageScenarioCardActive}>
                  <div className={styles.stageScenarioTitleRow}>
                    <span className={styles.stageScenarioTitle}>
                      {t("landing.deepdive_scenario_opt1_title", language)}
                    </span>
                    <span className={styles.stageNpvPill}>+¥382,000 NPV</span>
                  </div>
                  <div className={styles.stageScenarioGrid}>
                    <div className={styles.stageScenItem}>
                      <span className={styles.stageScenLabel}>RENT-TO-SALES</span>
                      <span className={styles.stageScenVal}>22.0% ➔ 18.7%</span>
                    </div>
                    <div className={styles.stageScenItem}>
                      <span className={styles.stageScenLabel}>ANNUAL SAVINGS</span>
                      <span className={styles.stageScenVal}>+¥166,320 / YR</span>
                    </div>
                    <div className={styles.stageScenItem}>
                      <span className={styles.stageScenLabel}>DECISION MEMO</span>
                      <span className={styles.stageScenVal}>READY FOR BOARD</span>
                    </div>
                  </div>
                </div>

                <div className={styles.stageMemoBox}>
                  <span className={styles.stageMemoHeader}>ACTION PROPOSAL #MAX-009</span>
                  <p className={styles.stageMemoText}>
                    {t("landing.deepdive_memo_desc", language)}
                  </p>
                </div>
              </div>
            </div>
          )}
        </div>
      </div>
    </section>
  );
};
