"use client";

import React, { useState } from "react";
import { Language, t } from "../../lib/i18n";
import styles from "../landing.module.css";

interface RoiCalculatorSectionProps {
  language: Language;
  onOpenDemoModal: () => void;
}

export const RoiCalculatorSection: React.FC<RoiCalculatorSectionProps> = ({
  language,
  onOpenDemoModal,
}) => {
  const [stores, setStores] = useState<number>(30);
  const [revenueWan, setRevenueWan] = useState<number>(40); // 400k RMB/mo
  const [rentRatio, setRentRatio] = useState<number>(20); // 20%
  const [laborRatio, setLaborRatio] = useState<number>(18); // 18%

  // Calculations
  const annualNetworkRevenue = stores * (revenueWan * 10000) * 12;
  const estimatedProfitLift = Math.round(annualNetworkRevenue * 0.03); // 3.0% margin lift
  const recoveredLeakage = Math.round(annualNetworkRevenue * (rentRatio / 100) * 0.06); // 6% of rent cost
  const savedHours = Math.round(stores * 8.5 + 160);

  const formatCurrency = (val: number) => {
    return "¥" + val.toLocaleString("en-US");
  };

  return (
    <section id="calculator" className={styles.section}>
      <div className={styles.sectionHeader}>
        <span className={styles.sectionBadge}>
          {t("landing.calc_badge", language)}
        </span>
        <h2 className={styles.sectionTitle}>
          {t("landing.calc_title", language)}
        </h2>
        <p className={styles.sectionSubtitle}>
          {t("landing.calc_subtitle", language)}
        </p>
      </div>

      <div className={styles.calcWrapper}>
        {/* Sliders Input Column */}
        <div className={styles.calcInputs}>
          {/* Store count */}
          <div className={styles.calcInputGroup}>
            <div className={styles.calcLabelRow}>
              <span>{t("landing.calc_stores", language)}</span>
              <span className={styles.calcValDisplay}>{stores}</span>
            </div>
            <input
              type="range"
              min={10}
              max={300}
              step={5}
              value={stores}
              onChange={(e) => setStores(Number(e.target.value))}
              className={styles.calcSlider}
            />
          </div>

          {/* Monthly Revenue per store */}
          <div className={styles.calcInputGroup}>
            <div className={styles.calcLabelRow}>
              <span>{t("landing.calc_revenue", language)}</span>
              <span className={styles.calcValDisplay}>{revenueWan}</span>
            </div>
            <input
              type="range"
              min={10}
              max={150}
              step={5}
              value={revenueWan}
              onChange={(e) => setRevenueWan(Number(e.target.value))}
              className={styles.calcSlider}
            />
          </div>

          {/* Rent to sales ratio */}
          <div className={styles.calcInputGroup}>
            <div className={styles.calcLabelRow}>
              <span>{t("landing.calc_rent_ratio", language)}</span>
              <span className={styles.calcValDisplay}>{rentRatio}%</span>
            </div>
            <input
              type="range"
              min={10}
              max={35}
              step={1}
              value={rentRatio}
              onChange={(e) => setRentRatio(Number(e.target.value))}
              className={styles.calcSlider}
            />
          </div>

          {/* Labor ratio */}
          <div className={styles.calcInputGroup}>
            <div className={styles.calcLabelRow}>
              <span>{t("landing.calc_labor_ratio", language)}</span>
              <span className={styles.calcValDisplay}>{laborRatio}%</span>
            </div>
            <input
              type="range"
              min={10}
              max={30}
              step={1}
              value={laborRatio}
              onChange={(e) => setLaborRatio(Number(e.target.value))}
              className={styles.calcSlider}
            />
          </div>
        </div>

        {/* Dynamic ROI Outputs */}
        <div className={styles.calcOutputs}>
          <div className={styles.calcOutputItem}>
            <div className={styles.calcOutputValue}>
              {formatCurrency(estimatedProfitLift)}
            </div>
            <div className={styles.calcOutputLabel}>
              {t("landing.calc_res_profit", language)}
            </div>
            <div className={styles.calcOutputTip}>
              {t("landing.calc_res_profit_tip", language)}
            </div>
          </div>

          <div className={styles.calcOutputItem}>
            <div className={styles.calcOutputValue}>
              {formatCurrency(recoveredLeakage)}
            </div>
            <div className={styles.calcOutputLabel}>
              {t("landing.calc_res_leakage", language)}
            </div>
            <div className={styles.calcOutputTip}>
              {t("landing.calc_res_leakage_tip", language)}
            </div>
          </div>

          <div className={styles.calcOutputItem}>
            <div className={styles.calcOutputValue}>
              {savedHours} {t("landing.calc_res_hours_unit", language)}
            </div>
            <div className={styles.calcOutputLabel}>
              {t("landing.calc_res_hours", language)}
            </div>
            <div className={styles.calcOutputTip}>
              {t("landing.calc_res_hours_tip", language)}
            </div>
          </div>

          <button
            type="button"
            className={`${styles.btnPrimary} ${styles.btnPrimaryLarge}`}
            onClick={onOpenDemoModal}
          >
            {t("landing.calc_cta", language)}
          </button>
        </div>
      </div>
    </section>
  );
};
