"use client";

import React from "react";
import { Language, t } from "../../lib/i18n";
import styles from "../landing.module.css";

interface ConcentricRadarSectionProps {
  language: Language;
}

export const ConcentricRadarSection: React.FC<ConcentricRadarSectionProps> = ({
  language,
}) => {
  return (
    <section className={styles.section}>
      <div className={styles.radarSectionGrid}>
        {/* Left Column: Context & Narrative */}
        <div className={styles.radarTextCol}>
          <span className={styles.sectionBadge}>
            {t("landing.radar_badge", language)}
          </span>
          <h2 className={styles.sectionTitle}>
            {t("landing.radar_title", language)}
          </h2>
          <p className={styles.sectionSubtitle}>
            {t("landing.radar_subtitle", language)}
          </p>

          <div className={styles.radarBrandPillsRow}>
            <span className={styles.radarBrandPill}>SAP ERP</span>
            <span className={styles.radarBrandPill}>Oracle Fusion</span>
            <span className={styles.radarBrandPill}>
              {t("landing.radar_brand_yonyou", language)}
            </span>
            <span className={styles.radarBrandPill}>
              {t("landing.radar_brand_kingdee", language)}
            </span>
            <span className={styles.radarBrandPill}>
              {t("landing.radar_brand_meituan", language)}
            </span>
            <span className={styles.radarBrandPill}>
              {t("landing.radar_brand_feishu", language)}
            </span>
          </div>
        </div>

        {/* Right Column: Concentric Integration Radar Graphic */}
        <div className={styles.radarVisualCol}>
          <div className={styles.radarCanvas}>
            {/* Concentric Rings */}
            <div className={styles.radarRingOuter} />
            <div className={styles.radarRingMiddle} />
            <div className={styles.radarRingInner} />

            {/* Central Node */}
            <div className={styles.radarCenterCore}>
              <span className={styles.radarCoreDot} />
              <span className={styles.radarCoreLabel}>
                {t("landing.radar_center_node", language)}
              </span>
            </div>

            {/* Orbiting Satellite Nodes */}
            <div className={`${styles.radarSatelliteNode} ${styles.satTopLeft}`}>
              {t("landing.radar_pos_node", language)}
            </div>
            <div className={`${styles.radarSatelliteNode} ${styles.satTopRight}`}>
              {t("landing.radar_erp_node", language)}
            </div>
            <div className={`${styles.radarSatelliteNode} ${styles.satBottomLeft}`}>
              {t("landing.radar_hr_node", language)}
            </div>
            <div className={`${styles.radarSatelliteNode} ${styles.satBottomRight}`}>
              {t("landing.radar_lease_node", language)}
            </div>
          </div>
        </div>
      </div>
    </section>
  );
};
