"use client";

import React, { useState } from "react";
import Link from "next/link";
import { Language, t } from "../../lib/i18n";
import styles from "../landing.module.css";

interface InteractiveDemoSectionProps {
  language: Language;
}

type DemoTab = "pulse" | "store" | "scenario" | "ifrs" | "ai";

export const InteractiveDemoSection: React.FC<InteractiveDemoSectionProps> = ({
  language,
}) => {
  const [activeTab, setActiveTab] = useState<DemoTab>("pulse");
  const [activeScenario, setActiveScenario] = useState<number>(0);
  const [selectedStore, setSelectedStore] = useState<number>(0);

  return (
    <section id="demo" className={styles.section}>
      <div className={styles.sectionHeader}>
        <span className={styles.sectionBadge}>
          {t("landing.demo_badge", language)}
        </span>
        <h2 className={styles.sectionTitle}>
          {t("landing.demo_title", language)}
        </h2>
        <p className={styles.sectionSubtitle}>
          {t("landing.demo_subtitle", language)}
        </p>
      </div>

      <div className={styles.demoContainer}>
        {/* macOS Window Header */}
        <div className={styles.windowBar}>
          <div className={styles.windowDots}>
            <span className={styles.dotRed} />
            <span className={styles.dotYellow} />
            <span className={styles.dotGreen} />
          </div>
          <div className={styles.windowUrl}>
            app.retail-workstation.internal / cockpit / {activeTab}
          </div>
          <div className={styles.windowStatus}>
            <span className={styles.windowStatusDot} />
            <span>LIVE COCKPIT</span>
          </div>
        </div>

        {/* Tab Navigation Segmented Bar */}
        <div className={styles.demoTabBar}>
          <button
            type="button"
            className={`${styles.demoTabBtn} ${activeTab === "pulse" ? styles.demoTabActive : ""}`}
            onClick={() => setActiveTab("pulse")}
          >
            {t("landing.demo_tab_pulse", language)}
          </button>
          <button
            type="button"
            className={`${styles.demoTabBtn} ${activeTab === "store" ? styles.demoTabActive : ""}`}
            onClick={() => setActiveTab("store")}
          >
            {t("landing.demo_tab_store", language)}
          </button>
          <button
            type="button"
            className={`${styles.demoTabBtn} ${activeTab === "scenario" ? styles.demoTabActive : ""}`}
            onClick={() => setActiveTab("scenario")}
          >
            {t("landing.demo_tab_scenario", language)}
          </button>
          <button
            type="button"
            className={`${styles.demoTabBtn} ${activeTab === "ifrs" ? styles.demoTabActive : ""}`}
            onClick={() => setActiveTab("ifrs")}
          >
            {t("landing.demo_tab_ifrs", language)}
          </button>
          <button
            type="button"
            className={`${styles.demoTabBtn} ${activeTab === "ai" ? styles.demoTabActive : ""}`}
            onClick={() => setActiveTab("ai")}
          >
            {t("landing.demo_tab_ai", language)}
          </button>
        </div>

        {/* Tab Panel Body */}
        <div className={styles.demoPanelBody}>
          {activeTab === "pulse" && (
            <div>
              <div className={styles.demoPanelHeader}>
                <h3 className={styles.demoHeadline}>
                  {t("landing.demo_pulse_headline", language)}
                </h3>
                <p className={styles.demoDesc}>
                  {t("landing.demo_pulse_desc", language)}
                </p>
              </div>

              <div className={styles.pulseGrid}>
                {/* Store 1 */}
                <div
                  className={`${styles.pulseCard} ${selectedStore === 0 ? styles.pulseCardSelected : ""}`}
                  onClick={() => setSelectedStore(0)}
                >
                  <div className={styles.pulseCardHeader}>
                    <span className={styles.pulseStoreName}>
                      {t("landing.demo_pulse_store1", language)}
                    </span>
                    <span className={`${styles.statusTag} ${styles.statusHigh}`}>
                      {t("landing.demo_pulse_attention_high", language)}
                    </span>
                  </div>
                  <ul className={styles.pulseSignalList}>
                    <li className={styles.pulseSignalItem}>
                      <span className={styles.pulseSignalDot} />
                      <span>{t("landing.demo_pulse_signal1", language)}</span>
                    </li>
                    <li className={styles.pulseSignalItem}>
                      <span className={styles.pulseSignalDot} />
                      <span>{t("landing.demo_pulse_signal2", language)}</span>
                    </li>
                    <li className={styles.pulseSignalItem}>
                      <span className={styles.pulseSignalDot} />
                      <span>{t("landing.demo_pulse_signal3", language)}</span>
                    </li>
                  </ul>
                </div>

                {/* Store 2 */}
                <div
                  className={`${styles.pulseCard} ${selectedStore === 1 ? styles.pulseCardSelected : ""}`}
                  onClick={() => setSelectedStore(1)}
                >
                  <div className={styles.pulseCardHeader}>
                    <span className={styles.pulseStoreName}>
                      {t("landing.demo_pulse_store2", language)}
                    </span>
                    <span className={`${styles.statusTag} ${styles.statusMed}`}>
                      {t("landing.demo_pulse_attention_med", language)}
                    </span>
                  </div>
                  <ul className={styles.pulseSignalList}>
                    <li className={styles.pulseSignalItem}>
                      <span className={styles.pulseSignalDot} />
                      <span>{t("landing.demo_pulse_signal1", language)}</span>
                    </li>
                    <li className={styles.pulseSignalItem}>
                      <span className={styles.pulseSignalDot} />
                      <span>{t("landing.demo_pulse_signal3", language)}</span>
                    </li>
                  </ul>
                </div>

                {/* Store 3 */}
                <div
                  className={`${styles.pulseCard} ${selectedStore === 2 ? styles.pulseCardSelected : ""}`}
                  onClick={() => setSelectedStore(2)}
                >
                  <div className={styles.pulseCardHeader}>
                    <span className={styles.pulseStoreName}>
                      {t("landing.demo_pulse_store3", language)}
                    </span>
                    <span className={`${styles.statusTag} ${styles.statusLow}`}>
                      {t("landing.demo_pulse_attention_low", language)}
                    </span>
                  </div>
                  <ul className={styles.pulseSignalList}>
                    <li className={styles.pulseSignalItem}>
                      <span className={styles.pulseSignalDot} />
                      <span>{t("landing.demo_pulse_signal2", language)}</span>
                    </li>
                  </ul>
                </div>

                {/* Store 4 */}
                <div
                  className={`${styles.pulseCard} ${selectedStore === 3 ? styles.pulseCardSelected : ""}`}
                  onClick={() => setSelectedStore(3)}
                >
                  <div className={styles.pulseCardHeader}>
                    <span className={styles.pulseStoreName}>
                      {t("landing.demo_pulse_store4", language)}
                    </span>
                    <span className={`${styles.statusTag} ${styles.statusLow}`}>
                      {t("landing.demo_pulse_attention_low", language)}
                    </span>
                  </div>
                  <ul className={styles.pulseSignalList}>
                    <li className={styles.pulseSignalItem}>
                      <span className={styles.pulseSignalDot} />
                      <span>{t("landing.demo_pulse_signal1", language)}</span>
                    </li>
                  </ul>
                </div>
              </div>

              <div className={styles.demoStatusBar}>
                <span>{t("landing.demo_pulse_ready_tag", language)}</span>
                <span>{t("landing.demo_pulse_meta", language)}</span>
              </div>
            </div>
          )}

          {activeTab === "store" && (
            <div>
              <div className={styles.demoPanelHeader}>
                <h3 className={styles.demoHeadline}>
                  {t("landing.demo_store_headline", language)}
                </h3>
                <p className={styles.demoDesc}>
                  {t("landing.demo_store_desc", language)}
                </p>
              </div>

              <div className={styles.waterfallContainer}>
                {/* Revenue Bar */}
                <div className={styles.waterfallBarItem}>
                  <div className={`${styles.waterfallRow} ${styles.wfPositive}`}>
                    <span>{t("landing.demo_store_rev", language)}</span>
                    <span>100.0% Base</span>
                  </div>
                  <div className={styles.barProgressTrack}>
                    <div className={`${styles.barProgressFill} ${styles.barFill100}`} />
                  </div>
                </div>

                {/* Gross Profit Bar */}
                <div className={styles.waterfallBarItem}>
                  <div className={`${styles.waterfallRow} ${styles.wfPositive}`}>
                    <span>{t("landing.demo_store_gp", language)}</span>
                    <span>Margin 55.0%</span>
                  </div>
                  <div className={styles.barProgressTrack}>
                    <div className={`${styles.barProgressFill} ${styles.barFill55}`} />
                  </div>
                </div>

                {/* Labor Cost Bar */}
                <div className={styles.waterfallBarItem}>
                  <div className={`${styles.waterfallRow} ${styles.wfDeduction}`}>
                    <span>{t("landing.demo_store_labor", language)}</span>
                    <span>Labor 18.0%</span>
                  </div>
                  <div className={styles.barProgressTrack}>
                    <div className={`${styles.barProgressFill} ${styles.barFill18}`} />
                  </div>
                </div>

                {/* Occupancy Cost Bar */}
                <div className={styles.waterfallBarItem}>
                  <div className={`${styles.waterfallRow} ${styles.wfDeduction}`}>
                    <span>{t("landing.demo_store_rent", language)}</span>
                    <span>Rent 22.0% (Warning)</span>
                  </div>
                  <div className={styles.barProgressTrack}>
                    <div className={`${styles.barProgressFill} ${styles.barFill22}`} />
                  </div>
                </div>

                {/* Other OPEX Bar */}
                <div className={styles.waterfallBarItem}>
                  <div className={`${styles.waterfallRow} ${styles.wfDeduction}`}>
                    <span>{t("landing.demo_store_other", language)}</span>
                    <span>OPEX 5.0%</span>
                  </div>
                  <div className={styles.barProgressTrack}>
                    <div className={`${styles.barProgressFill} ${styles.barFill5}`} />
                  </div>
                </div>

                {/* Four-Wall EBITDA Final Bar */}
                <div className={styles.waterfallBarItem}>
                  <div className={`${styles.waterfallRow} ${styles.wfFinal}`}>
                    <span>{t("landing.demo_store_ebit", language)}</span>
                    <span>Target: &gt; 15.0%</span>
                  </div>
                  <div className={styles.barProgressTrack}>
                    <div className={`${styles.barProgressFill} ${styles.barFill10}`} />
                  </div>
                </div>
              </div>

              <div className={styles.demoStatusBar}>
                <span>{t("landing.demo_store_cohort_tag", language)}</span>
                <span>{t("landing.demo_store_meta", language)}</span>
              </div>
            </div>
          )}

          {activeTab === "scenario" && (
            <div>
              <div className={styles.demoPanelHeader}>
                <h3 className={styles.demoHeadline}>
                  {t("landing.demo_scenario_headline", language)}
                </h3>
                <p className={styles.demoDesc}>
                  {t("landing.demo_scenario_desc", language)}
                </p>
              </div>

              <div className={styles.scenarioGrid}>
                <div
                  className={`${styles.scenarioCard} ${activeScenario === 0 ? styles.scenarioCardActive : ""}`}
                  onClick={() => setActiveScenario(0)}
                >
                  <div className={styles.scenarioCardTitle}>
                    {t("landing.demo_scenario_opt1", language)}
                  </div>
                  <div className={styles.metricLabel}>
                    NPV: +¥382,000 · Margin: +3.3%
                  </div>
                </div>

                <div
                  className={`${styles.scenarioCard} ${activeScenario === 1 ? styles.scenarioCardActive : ""}`}
                  onClick={() => setActiveScenario(1)}
                >
                  <div className={styles.scenarioCardTitle}>
                    {t("landing.demo_scenario_opt2", language)}
                  </div>
                  <div className={styles.metricLabel}>
                    NPV: +¥215,000 · Margin: +1.8%
                  </div>
                </div>

                <div
                  className={`${styles.scenarioCard} ${activeScenario === 2 ? styles.scenarioCardActive : ""}`}
                  onClick={() => setActiveScenario(2)}
                >
                  <div className={styles.scenarioCardTitle}>
                    {t("landing.demo_scenario_opt3", language)}
                  </div>
                  <div className={styles.metricLabel}>
                    Penalty: -¥120,000 · Payback: 4 Mo
                  </div>
                </div>
              </div>

              <div className={styles.scenarioOutcomeBox}>
                {t("landing.demo_scenario_result", language)}
              </div>
            </div>
          )}

          {activeTab === "ifrs" && (
            <div>
              <div className={styles.demoPanelHeader}>
                <h3 className={styles.demoHeadline}>
                  {t("landing.demo_ifrs_headline", language)}
                </h3>
                <p className={styles.demoDesc}>
                  {t("landing.demo_ifrs_desc", language)}
                </p>
              </div>

              <div className={styles.ifrsGrid}>
                <div className={styles.ifrsCard}>
                  <div className={styles.ifrsCardValue}>
                    {t("landing.demo_ifrs_liability", language)}
                  </div>
                  <div className={styles.ifrsCardSub}>
                    Discount Rate: 4.75% · Amortization: 36 Mo
                  </div>
                </div>

                <div className={styles.ifrsCard}>
                  <div className={styles.ifrsCardValue}>
                    {t("landing.demo_ifrs_rou", language)}
                  </div>
                  <div className={styles.ifrsCardSub}>
                    Accumulated Depreciation: ¥645,200.00
                  </div>
                </div>
              </div>

              <div className={styles.demoStatusBar}>
                <span>{t("landing.demo_ifrs_entries", language)}</span>
                <span>{t("landing.demo_ifrs_meta", language)}</span>
              </div>
            </div>
          )}

          {activeTab === "ai" && (
            <div>
              <div className={styles.demoPanelHeader}>
                <h3 className={styles.demoHeadline}>
                  {t("landing.demo_ai_headline", language)}
                </h3>
                <p className={styles.demoDesc}>
                  {t("landing.demo_ai_desc", language)}
                </p>
              </div>

              <div className={styles.aiChatContainer}>
                <div className={styles.aiMessageUser}>
                  {t("landing.demo_ai_query", language)}
                </div>
                <div className={styles.aiMessageAssistant}>
                  {t("landing.demo_ai_response", language)}
                </div>
              </div>

              <div className={styles.demoStatusBar}>
                <span>{t("landing.demo_ai_free_badge", language)}</span>
                <Link href="/login" className={styles.btnPrimary}>
                  {t("landing.demo_ai_free_cta", language)}
                </Link>
              </div>
            </div>
          )}
        </div>
      </div>
    </section>
  );
};
