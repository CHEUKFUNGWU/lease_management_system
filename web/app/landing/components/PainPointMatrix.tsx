"use client";

import React from "react";
import { Language, t } from "../../lib/i18n";
import styles from "../landing.module.css";

interface PainPointMatrixProps {
  language: Language;
}

export const PainPointMatrix: React.FC<PainPointMatrixProps> = ({ language }) => {
  return (
    <section id="comparison" className={styles.section}>
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

      <table className={styles.matrixTable}>
        <thead>
          <tr>
            <th className={styles.matrixTh}>
              {t("landing.matrix_dimension", language)}
            </th>
            <th className={styles.matrixTh}>
              {t("landing.pain_col_trad", language)}
            </th>
            <th className={styles.matrixTh}>
              {t("landing.pain_col_our", language)}
            </th>
          </tr>
        </thead>
        <tbody>
          <tr className={styles.matrixRow}>
            <td className={`${styles.matrixTd} ${styles.matrixDimTitle}`}>
              {t("landing.pain_dim1_title", language)}
            </td>
            <td className={`${styles.matrixTd} ${styles.matrixTradCell}`}>
              {t("landing.pain_dim1_trad", language)}
            </td>
            <td className={`${styles.matrixTd} ${styles.matrixOurCell}`}>
              {t("landing.pain_dim1_our", language)}
            </td>
          </tr>

          <tr className={styles.matrixRow}>
            <td className={`${styles.matrixTd} ${styles.matrixDimTitle}`}>
              {t("landing.pain_dim2_title", language)}
            </td>
            <td className={`${styles.matrixTd} ${styles.matrixTradCell}`}>
              {t("landing.pain_dim2_trad", language)}
            </td>
            <td className={`${styles.matrixTd} ${styles.matrixOurCell}`}>
              {t("landing.pain_dim2_our", language)}
            </td>
          </tr>

          <tr className={styles.matrixRow}>
            <td className={`${styles.matrixTd} ${styles.matrixDimTitle}`}>
              {t("landing.pain_dim3_title", language)}
            </td>
            <td className={`${styles.matrixTd} ${styles.matrixTradCell}`}>
              {t("landing.pain_dim3_trad", language)}
            </td>
            <td className={`${styles.matrixTd} ${styles.matrixOurCell}`}>
              {t("landing.pain_dim3_our", language)}
            </td>
          </tr>

          <tr className={styles.matrixRow}>
            <td className={`${styles.matrixTd} ${styles.matrixDimTitle}`}>
              {t("landing.pain_dim4_title", language)}
            </td>
            <td className={`${styles.matrixTd} ${styles.matrixTradCell}`}>
              {t("landing.pain_dim4_trad", language)}
            </td>
            <td className={`${styles.matrixTd} ${styles.matrixOurCell}`}>
              {t("landing.pain_dim4_our", language)}
            </td>
          </tr>
        </tbody>
      </table>
    </section>
  );
};
