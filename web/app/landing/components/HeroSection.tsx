"use client";

import React from "react";
import Link from "next/link";
import { Language, t } from "../../lib/i18n";
import { StaggerGroup, StaggerItem } from "./ScrollReveal";
import styles from "../landing.module.css";

interface HeroSectionProps {
  language: Language;
  onOpenDemoModal: () => void;
}

export const HeroSection: React.FC<HeroSectionProps> = ({
  language,
  onOpenDemoModal,
}) => {
  const scrollToDemo = () => {
    const el = document.getElementById("demo");
    if (el) {
      el.scrollIntoView({ behavior: "smooth" });
    }
  };

  return (
    <section className={styles.heroSection}>
      {/* Ambient Blueprint Lighting */}
      <div className={styles.heroAura} aria-hidden="true" />
      <div className={styles.heroGridLines} aria-hidden="true" />

      <StaggerGroup>
        <div className={styles.heroContainerSplit}>
          {/* Left Column: Editorial Display Typography & CTAs */}
          <div className={styles.heroTextCol}>
            <StaggerItem index={0}>
              <div className={styles.heroBadgeWrap}>
                <span className={styles.heroTagDot} />
                <span>{t("landing.hero_free_badge", language)}</span>
              </div>
            </StaggerItem>

            <StaggerItem index={1}>
              <h1 className={styles.heroTitleEditorial}>
                <span className={styles.heroTitlePrefix}>
                  {t("landing.hero_title_prefix", language)}
                </span>
                {t("landing.hero_title_suffix", language)}
              </h1>
            </StaggerItem>

            <StaggerItem index={2}>
              <p className={styles.heroSubtitleEditorial}>
                {t("landing.hero_subtitle", language)}
              </p>
            </StaggerItem>

            <StaggerItem index={3}>
              <div className={styles.heroCtasRow}>
                <Link
                  href="/login"
                  className={`${styles.btnPrimary} ${styles.btnPrimaryLarge}`}
                >
                  {t("landing.hero_cta_free", language)}
                </Link>

                <button
                  type="button"
                  className={`${styles.btnSecondary} ${styles.btnSecondaryLarge}`}
                  onClick={onOpenDemoModal}
                >
                  {t("landing.hero_cta_primary", language)}
                </button>

                <button
                  type="button"
                  className={`${styles.btnSecondary} ${styles.btnSecondaryLarge}`}
                  onClick={scrollToDemo}
                >
                  {t("landing.hero_cta_secondary", language)}
                </button>
              </div>
            </StaggerItem>

            {/* Social Proof Sectors Marquee */}
            <StaggerItem index={4}>
              <div className={styles.sectorMarqueeInline}>
                <div className={styles.sectorPillsWrap}>
                  <span className={styles.sectorPill}>{t("landing.sector_1", language)}</span>
                  <span className={styles.sectorPill}>{t("landing.sector_2", language)}</span>
                  <span className={styles.sectorPill}>{t("landing.sector_3", language)}</span>
                  <span className={styles.sectorPill}>{t("landing.sector_4", language)}</span>
                  <span className={styles.sectorPill}>{t("landing.sector_5", language)}</span>
                </div>
              </div>
            </StaggerItem>
          </div>

          {/* Right Column: Mosaic Asymmetrical 4-Quadrant Visual Stage */}
          <div className={styles.heroMosaicCol}>
            {/* Quadrant 1: Top-Left Monitored Stores */}
            <StaggerItem index={1}>
              <div className={styles.mosaicCardQ1}>
                <div className={styles.mosaicHeaderSmall}>
                  <span className={styles.mosaicDotActive} />
                  <span className={styles.mosaicLabelSmall}>ACTIVE STORES</span>
                </div>
                <div className={styles.mosaicValLarge}>
                  {t("landing.mosaic_live_stores", language)}
                </div>
                <div className={styles.mosaicBadgePill}>
                  {t("landing.mosaic_reconciled", language)}
                </div>
              </div>
            </StaggerItem>

            {/* Quadrant 2: Top-Right Arched Emerald Shape */}
            <StaggerItem index={2}>
              <div className={styles.mosaicCardQ2Arch}>
                <div className={styles.mosaicArchGraphic}>
                  <span className={styles.mosaicArchNumber}>500+</span>
                  <span className={styles.mosaicArchLabel}>
                    {t("landing.mosaic_arch_badge", language)}
                  </span>
                </div>
              </div>
            </StaggerItem>

            {/* Quadrant 3: Bottom-Left Triage & Avatars */}
            <StaggerItem index={3}>
              <div className={styles.mosaicCardQ3}>
                <div className={styles.mosaicHeaderSmall}>
                  <span className={styles.mosaicLabelSmall}>STORE TRIAGE</span>
                </div>
                <div className={styles.mosaicTriageStatus}>
                  {t("landing.mosaic_triage_team", language)}
                </div>
                <div className={styles.mosaicAvatarRow}>
                  <span className={styles.mosaicAvatarA}>OP</span>
                  <span className={styles.mosaicAvatarB}>FP</span>
                  <span className={styles.mosaicAvatarC}>RE</span>
                </div>
              </div>
            </StaggerItem>

            {/* Quadrant 4: Bottom-Right Deep Emerald Card with Growth Chart */}
            <StaggerItem index={4}>
              <div className={styles.mosaicCardQ4Dark}>
                <div className={styles.mosaicGrowthHeader}>
                  <span className={styles.mosaicGrowthVal}>
                    {t("landing.mosaic_growth_val", language)}
                  </span>
                  <span className={styles.mosaicGrowthPill}>
                    {t("landing.mosaic_growth_label", language)}
                  </span>
                </div>

                {/* Glowing SVG Area Chart */}
                <div className={styles.mosaicChartWrapper}>
                  <svg className={styles.mosaicChartSvg} viewBox="0 0 200 70" fill="none">
                    <defs>
                      <linearGradient id="growthGrad" x1="0" y1="0" x2="0" y2="1">
                        <stop offset="0%" stopColor="#10b981" stopOpacity="0.4" />
                        <stop offset="100%" stopColor="#10b981" stopOpacity="0.0" />
                      </linearGradient>
                    </defs>
                    <path
                      d="M0 65 Q 40 55, 80 40 T 160 18 T 200 6 L 200 70 L 0 70 Z"
                      fill="url(#growthGrad)"
                    />
                    <path
                      d="M0 65 Q 40 55, 80 40 T 160 18 T 200 6"
                      stroke="#10b981"
                      strokeWidth="3"
                      strokeLinecap="round"
                    />
                    <circle cx="200" cy="6" r="4" fill="#ffffff" stroke="#10b981" strokeWidth="2" />
                  </svg>
                </div>
              </div>
            </StaggerItem>
          </div>
        </div>
      </StaggerGroup>
    </section>
  );
};
