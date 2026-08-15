"use client";

import React from "react";
import { Language, t } from "../../lib/i18n";
import styles from "../landing.module.css";

interface TrustSecuritySectionProps {
  language: Language;
}

export const TrustSecuritySection: React.FC<TrustSecuritySectionProps> = ({
  language,
}) => {
  return (
    <section id="security" className={styles.section}>
      <div className={styles.sectionHeader}>
        <span className={styles.sectionBadge}>
          {t("landing.trust_badge", language)}
        </span>
        <h2 className={styles.sectionTitle}>
          {t("landing.trust_title", language)}
        </h2>
        <p className={styles.sectionSubtitle}>
          {t("landing.trust_subtitle", language)}
        </p>
      </div>

      <div className={styles.trustGrid}>
        {/* Guardrail 1 */}
        <div className={styles.trustCard}>
          <h3 className={styles.trustTitle}>
            {t("landing.trust_g1_title", language)}
          </h3>
          <p className={styles.trustDesc}>
            {t("landing.trust_g1_desc", language)}
          </p>
        </div>

        {/* Guardrail 2 */}
        <div className={styles.trustCard}>
          <h3 className={styles.trustTitle}>
            {t("landing.trust_g2_title", language)}
          </h3>
          <p className={styles.trustDesc}>
            {t("landing.trust_g2_desc", language)}
          </p>
        </div>

        {/* Guardrail 3 */}
        <div className={styles.trustCard}>
          <h3 className={styles.trustTitle}>
            {t("landing.trust_g3_title", language)}
          </h3>
          <p className={styles.trustDesc}>
            {t("landing.trust_g3_desc", language)}
          </p>
        </div>

        {/* Guardrail 4 */}
        <div className={styles.trustCard}>
          <h3 className={styles.trustTitle}>
            {t("landing.trust_g4_title", language)}
          </h3>
          <p className={styles.trustDesc}>
            {t("landing.trust_g4_desc", language)}
          </p>
        </div>
      </div>
    </section>
  );
};
