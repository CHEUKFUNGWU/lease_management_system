"use client";

import React from "react";
import { Language, t } from "../../lib/i18n";
import styles from "../landing.module.css";

interface PersonaSectionProps {
  language: Language;
}

export const PersonaSection: React.FC<PersonaSectionProps> = ({ language }) => {
  return (
    <section id="personas" className={styles.section}>
      <div className={styles.sectionHeader}>
        <span className={styles.sectionBadge}>
          {t("landing.persona_badge", language)}
        </span>
        <h2 className={styles.sectionTitle}>
          {t("landing.persona_title", language)}
        </h2>
        <p className={styles.sectionSubtitle}>
          {t("landing.persona_subtitle", language)}
        </p>
      </div>

      <div className={styles.personaGrid}>
        {/* COO */}
        <div className={styles.personaCard}>
          <div>
            <h3 className={styles.personaRoleTitle}>
              {t("landing.persona_coo_title", language)}
            </h3>
            <p className={styles.personaQuote}>
              {t("landing.persona_coo_quote", language)}
            </p>
          </div>
          <span className={styles.personaTag}>
            {t("landing.persona_coo_tag", language)}
          </span>
        </div>

        {/* CFO */}
        <div className={styles.personaCard}>
          <div>
            <h3 className={styles.personaRoleTitle}>
              {t("landing.persona_cfo_title", language)}
            </h3>
            <p className={styles.personaQuote}>
              {t("landing.persona_cfo_quote", language)}
            </p>
          </div>
          <span className={styles.personaTag}>
            {t("landing.persona_cfo_tag", language)}
          </span>
        </div>

        {/* Expansion / Lease Director */}
        <div className={styles.personaCard}>
          <div>
            <h3 className={styles.personaRoleTitle}>
              {t("landing.persona_dev_title", language)}
            </h3>
            <p className={styles.personaQuote}>
              {t("landing.persona_dev_quote", language)}
            </p>
          </div>
          <span className={styles.personaTag}>
            {t("landing.persona_dev_tag", language)}
          </span>
        </div>

        {/* Store Manager */}
        <div className={styles.personaCard}>
          <div>
            <h3 className={styles.personaRoleTitle}>
              {t("landing.persona_mgr_title", language)}
            </h3>
            <p className={styles.personaQuote}>
              {t("landing.persona_mgr_quote", language)}
            </p>
          </div>
          <span className={styles.personaTag}>
            {t("landing.persona_mgr_tag", language)}
          </span>
        </div>
      </div>
    </section>
  );
};
