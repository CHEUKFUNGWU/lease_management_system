"use client";

import React, { useState } from "react";
import { Language, t } from "../../lib/i18n";
import { StaggerGroup, StaggerItem } from "./ScrollReveal";
import styles from "../landing.module.css";

interface BeforeAfterLensSectionProps {
  language: Language;
}

export const BeforeAfterLensSection: React.FC<BeforeAfterLensSectionProps> = ({
  language,
}) => {
  const [activeDim, setActiveDim] = useState<number>(0);

  const dimensions = [
    {
      title: "landing.pain_dim1_title",
      trad: "landing.pain_dim1_trad",
      our: "landing.pain_dim1_our",
      tradTag: "landing.lens_dim1_trad_tag",
      ourTag: "landing.lens_dim1_our_tag",
      tradMetrics: [
        { label: "landing.lens_d0_t_m1_l", val: "landing.lens_d0_t_m1_v" },
        { label: "landing.lens_d0_t_m2_l", val: "landing.lens_d0_t_m2_v" },
        { label: "landing.lens_d0_t_m3_l", val: "landing.lens_d0_t_m3_v" },
      ],
      ourMetrics: [
        { label: "landing.lens_d0_o_m1_l", val: "landing.lens_d0_o_m1_v" },
        { label: "landing.lens_d0_o_m2_l", val: "landing.lens_d0_o_m2_v" },
        { label: "landing.lens_d0_o_m3_l", val: "landing.lens_d0_o_m3_v" },
      ],
    },
    {
      title: "landing.pain_dim2_title",
      trad: "landing.pain_dim2_trad",
      our: "landing.pain_dim2_our",
      tradTag: "landing.lens_dim2_trad_tag",
      ourTag: "landing.lens_dim2_our_tag",
      tradMetrics: [
        { label: "landing.lens_d1_t_m1_l", val: "landing.lens_d1_t_m1_v" },
        { label: "landing.lens_d1_t_m2_l", val: "landing.lens_d1_t_m2_v" },
        { label: "landing.lens_d1_t_m3_l", val: "landing.lens_d1_t_m3_v" },
      ],
      ourMetrics: [
        { label: "landing.lens_d1_o_m1_l", val: "landing.lens_d1_o_m1_v" },
        { label: "landing.lens_d1_o_m2_l", val: "landing.lens_d1_o_m2_v" },
        { label: "landing.lens_d1_o_m3_l", val: "landing.lens_d1_o_m3_v" },
      ],
    },
    {
      title: "landing.pain_dim3_title",
      trad: "landing.pain_dim3_trad",
      our: "landing.pain_dim3_our",
      tradTag: "landing.lens_dim3_trad_tag",
      ourTag: "landing.lens_dim3_our_tag",
      tradMetrics: [
        { label: "landing.lens_d2_t_m1_l", val: "landing.lens_d2_t_m1_v" },
        { label: "landing.lens_d2_t_m2_l", val: "landing.lens_d2_t_m2_v" },
        { label: "landing.lens_d2_t_m3_l", val: "landing.lens_d2_t_m3_v" },
      ],
      ourMetrics: [
        { label: "landing.lens_d2_o_m1_l", val: "landing.lens_d2_o_m1_v" },
        { label: "landing.lens_d2_o_m2_l", val: "landing.lens_d2_o_m2_v" },
        { label: "landing.lens_d2_o_m3_l", val: "landing.lens_d2_o_m3_v" },
      ],
    },
    {
      title: "landing.pain_dim4_title",
      trad: "landing.pain_dim4_trad",
      our: "landing.pain_dim4_our",
      tradTag: "landing.lens_dim4_trad_tag",
      ourTag: "landing.lens_dim4_our_tag",
      tradMetrics: [
        { label: "landing.lens_d3_t_m1_l", val: "landing.lens_d3_t_m1_v" },
        { label: "landing.lens_d3_t_m2_l", val: "landing.lens_d3_t_m2_v" },
        { label: "landing.lens_d3_t_m3_l", val: "landing.lens_d3_t_m3_v" },
      ],
      ourMetrics: [
        { label: "landing.lens_d3_o_m1_l", val: "landing.lens_d3_o_m1_v" },
        { label: "landing.lens_d3_o_m2_l", val: "landing.lens_d3_o_m2_v" },
        { label: "landing.lens_d3_o_m3_l", val: "landing.lens_d3_o_m3_v" },
      ],
    },
  ];

  const curr = dimensions[activeDim];

  return (
    <section className={styles.section}>
      <StaggerGroup>
        <StaggerItem index={0}>
          <div className={styles.sectionHeader}>
            <span className={styles.sectionBadge}>
              {t("landing.pain_badge", language)}
            </span>
            <h2 className={styles.sectionTitle}>
              {t("landing.pain_title", language)}
            </h2>
            <p className={styles.sectionSubtitle}>
              {t("landing.pain_subtitle", language)}
            </p>
          </div>
        </StaggerItem>

        <div className={styles.lensContainer}>
          {/* Left Side: Interactive Dimension Selector Tabs */}
          <StaggerItem index={1} className={styles.lensNavList}>
            {dimensions.map((dim, idx) => (
              <button
                key={idx}
                type="button"
                className={`${styles.lensNavBtn} ${activeDim === idx ? styles.lensNavBtnActive : ""}`}
                onClick={() => setActiveDim(idx)}
              >
                <span className={styles.lensNavIndex}>0{idx + 1}</span>
                <span className={styles.lensNavTitle}>{t(dim.title, language)}</span>
              </button>
            ))}
          </StaggerItem>

          {/* Right Side: Dual-Stage Reality Contrast Lens */}
          <div className={styles.lensDisplayStage}>
            {/* Traditional Reality Box (Before) */}
            <StaggerItem index={2} className={styles.lensBoxTrad}>
              <div>
                <div className={styles.lensBoxHeader}>
                  <span className={styles.lensBadgeTrad}>
                    {t("landing.pain_col_trad", language)}
                  </span>
                  <span className={styles.lensTagTrad}>
                    {t(curr.tradTag, language)}
                  </span>
                </div>

                {/* Structured Metric Diagnostics Widget */}
                <div className={styles.lensMetricsWrapTrad}>
                  {curr.tradMetrics.map((m, mIdx) => (
                    <div key={mIdx} className={styles.lensMetricRowTrad}>
                      <span className={styles.lensMetricLabelTrad}>{t(m.label, language)}</span>
                      <span className={styles.lensMetricValTrad}>{t(m.val, language)}</span>
                    </div>
                  ))}
                </div>
              </div>

              <p className={styles.lensBoxDesc}>
                {t(curr.trad, language)}
              </p>
            </StaggerItem>

            {/* Workstation Solution Box (After) */}
            <StaggerItem index={3} className={styles.lensBoxOur}>
              <div>
                <div className={styles.lensBoxHeader}>
                  <span className={styles.lensBadgeOur}>
                    {t("landing.pain_col_our", language)}
                  </span>
                  <span className={styles.lensTagOur}>
                    {t(curr.ourTag, language)}
                  </span>
                </div>

                {/* Structured Metric Solutions Widget */}
                <div className={styles.lensMetricsWrapOur}>
                  {curr.ourMetrics.map((m, mIdx) => (
                    <div key={mIdx} className={styles.lensMetricRowOur}>
                      <span className={styles.lensMetricLabelOur}>{t(m.label, language)}</span>
                      <span className={styles.lensMetricValOur}>{t(m.val, language)}</span>
                    </div>
                  ))}
                </div>
              </div>

              <p className={styles.lensBoxDesc}>
                {t(curr.our, language)}
              </p>
            </StaggerItem>
          </div>
        </div>
      </StaggerGroup>
    </section>
  );
};
