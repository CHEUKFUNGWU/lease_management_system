"use client";

import React from "react";
import Link from "next/link";
import { CheckOutlined } from "@ant-design/icons";
import { Language, t } from "../../lib/i18n";
import styles from "../landing.module.css";

interface PricingSectionProps {
  language: Language;
  onOpenDemoModal: () => void;
}

export const PricingSection: React.FC<PricingSectionProps> = ({
  language,
  onOpenDemoModal,
}) => {
  return (
    <section id="pricing" className={styles.section}>
      <div className={styles.sectionHeader}>
        <span className={styles.sectionBadge}>
          {t("landing.pricing_badge", language)}
        </span>
        <h2 className={styles.sectionTitle}>
          {t("landing.pricing_title", language)}
        </h2>
        <p className={styles.sectionSubtitle}>
          {t("landing.pricing_subtitle", language)}
        </p>
      </div>

      <div className={styles.pricingGrid}>
        {/* Tier 0: Individual Free (New) */}
        <div className={styles.pricingCard}>
          <span className={styles.pricingBadge}>
            {t("landing.plan_free_badge", language)}
          </span>
          <h3 className={styles.pricingPlanTitle}>
            {t("landing.plan_free_title", language)}
          </h3>
          <p className={styles.pricingPlanDesc}>
            {t("landing.plan_free_desc", language)}
          </p>
          <div className={styles.pricingPrice}>
            {t("landing.plan_free_price", language)}
          </div>
          <ul className={styles.pricingFeatures}>
            <li className={styles.pricingFeatureItem}>
              <span className={styles.pillarCheckIcon} aria-hidden="true">
                <CheckOutlined />
              </span>
              <span>{t("landing.plan_free_f1", language)}</span>
            </li>
            <li className={styles.pricingFeatureItem}>
              <span className={styles.pillarCheckIcon} aria-hidden="true">
                <CheckOutlined />
              </span>
              <span>{t("landing.plan_free_f2", language)}</span>
            </li>
            <li className={styles.pricingFeatureItem}>
              <span className={styles.pillarCheckIcon} aria-hidden="true">
                <CheckOutlined />
              </span>
              <span>{t("landing.plan_free_f3", language)}</span>
            </li>
            <li className={styles.pricingFeatureItem}>
              <span className={styles.pillarCheckIcon} aria-hidden="true">
                <CheckOutlined />
              </span>
              <span>{t("landing.plan_free_f4", language)}</span>
            </li>
          </ul>
          <Link href="/login" className={styles.btnPrimary}>
            {t("landing.plan_free_btn", language)}
          </Link>
        </div>

        {/* Tier 1: Professional */}
        <div className={styles.pricingCard}>
          <h3 className={styles.pricingPlanTitle}>
            {t("landing.plan_starter_title", language)}
          </h3>
          <p className={styles.pricingPlanDesc}>
            {t("landing.plan_starter_desc", language)}
          </p>
          <div className={styles.pricingPrice}>
            {t("landing.plan_starter_price", language)}
          </div>
          <ul className={styles.pricingFeatures}>
            <li className={styles.pricingFeatureItem}>
              <span className={styles.pillarCheckIcon} aria-hidden="true">
                <CheckOutlined />
              </span>
              <span>{t("landing.plan_starter_f1", language)}</span>
            </li>
            <li className={styles.pricingFeatureItem}>
              <span className={styles.pillarCheckIcon} aria-hidden="true">
                <CheckOutlined />
              </span>
              <span>{t("landing.plan_starter_f2", language)}</span>
            </li>
            <li className={styles.pricingFeatureItem}>
              <span className={styles.pillarCheckIcon} aria-hidden="true">
                <CheckOutlined />
              </span>
              <span>{t("landing.plan_starter_f3", language)}</span>
            </li>
            <li className={styles.pricingFeatureItem}>
              <span className={styles.pillarCheckIcon} aria-hidden="true">
                <CheckOutlined />
              </span>
              <span>{t("landing.plan_starter_f4", language)}</span>
            </li>
          </ul>
          <button
            type="button"
            className={styles.btnSecondary}
            onClick={onOpenDemoModal}
          >
            {t("landing.plan_starter_btn", language)}
          </button>
        </div>

        {/* Tier 2: Enterprise (Featured) */}
        <div className={`${styles.pricingCard} ${styles.pricingCardFeatured}`}>
          <span className={styles.pricingBadge}>
            {t("landing.plan_pro_badge", language)}
          </span>
          <h3 className={styles.pricingPlanTitle}>
            {t("landing.plan_pro_title", language)}
          </h3>
          <p className={styles.pricingPlanDesc}>
            {t("landing.plan_pro_desc", language)}
          </p>
          <div className={styles.pricingPrice}>
            {t("landing.plan_pro_price", language)}
          </div>
          <ul className={styles.pricingFeatures}>
            <li className={styles.pricingFeatureItem}>
              <span className={styles.pillarCheckIcon} aria-hidden="true">
                <CheckOutlined />
              </span>
              <span>{t("landing.plan_pro_f1", language)}</span>
            </li>
            <li className={styles.pricingFeatureItem}>
              <span className={styles.pillarCheckIcon} aria-hidden="true">
                <CheckOutlined />
              </span>
              <span>{t("landing.plan_pro_f2", language)}</span>
            </li>
            <li className={styles.pricingFeatureItem}>
              <span className={styles.pillarCheckIcon} aria-hidden="true">
                <CheckOutlined />
              </span>
              <span>{t("landing.plan_pro_f3", language)}</span>
            </li>
            <li className={styles.pricingFeatureItem}>
              <span className={styles.pillarCheckIcon} aria-hidden="true">
                <CheckOutlined />
              </span>
              <span>{t("landing.plan_pro_f4", language)}</span>
            </li>
            <li className={styles.pricingFeatureItem}>
              <span className={styles.pillarCheckIcon} aria-hidden="true">
                <CheckOutlined />
              </span>
              <span>{t("landing.plan_pro_f5", language)}</span>
            </li>
          </ul>
          <button
            type="button"
            className={styles.btnPrimary}
            onClick={onOpenDemoModal}
          >
            {t("landing.plan_pro_btn", language)}
          </button>
        </div>

        {/* Tier 3: Ultimate Group */}
        <div className={styles.pricingCard}>
          <h3 className={styles.pricingPlanTitle}>
            {t("landing.plan_ent_title", language)}
          </h3>
          <p className={styles.pricingPlanDesc}>
            {t("landing.plan_ent_desc", language)}
          </p>
          <div className={styles.pricingPrice}>
            {t("landing.plan_ent_price", language)}
          </div>
          <ul className={styles.pricingFeatures}>
            <li className={styles.pricingFeatureItem}>
              <span className={styles.pillarCheckIcon} aria-hidden="true">
                <CheckOutlined />
              </span>
              <span>{t("landing.plan_ent_f1", language)}</span>
            </li>
            <li className={styles.pricingFeatureItem}>
              <span className={styles.pillarCheckIcon} aria-hidden="true">
                <CheckOutlined />
              </span>
              <span>{t("landing.plan_ent_f2", language)}</span>
            </li>
            <li className={styles.pricingFeatureItem}>
              <span className={styles.pillarCheckIcon} aria-hidden="true">
                <CheckOutlined />
              </span>
              <span>{t("landing.plan_ent_f3", language)}</span>
            </li>
            <li className={styles.pricingFeatureItem}>
              <span className={styles.pillarCheckIcon} aria-hidden="true">
                <CheckOutlined />
              </span>
              <span>{t("landing.plan_ent_f4", language)}</span>
            </li>
            <li className={styles.pricingFeatureItem}>
              <span className={styles.pillarCheckIcon} aria-hidden="true">
                <CheckOutlined />
              </span>
              <span>{t("landing.plan_ent_f5", language)}</span>
            </li>
          </ul>
          <button
            type="button"
            className={styles.btnSecondary}
            onClick={onOpenDemoModal}
          >
            {t("landing.plan_ent_btn", language)}
          </button>
        </div>
      </div>
    </section>
  );
};
