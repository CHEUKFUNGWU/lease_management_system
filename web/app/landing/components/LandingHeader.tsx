"use client";

import React from "react";
import Link from "next/link";
import { GlobalOutlined } from "@ant-design/icons";
import { Language, t } from "../../lib/i18n";
import { LANGUAGE_LABELS } from "../../context/LanguageContext";
import { BrandIcon } from "../../components/BrandIcon";
import styles from "../landing.module.css";

interface LandingHeaderProps {
  language: Language;
  onLanguageChange: (lang: Language) => void;
  onOpenDemoModal: () => void;
}

export const LandingHeader: React.FC<LandingHeaderProps> = ({
  language,
  onLanguageChange,
  onOpenDemoModal,
}) => {
  const nextLanguage: Language =
    language === "zh-CN" ? "zh-HK" : language === "zh-HK" ? "en" : "zh-CN";

  const langLabel = LANGUAGE_LABELS[language];

  const scrollToSection = (id: string) => {
    const el = document.getElementById(id);
    if (el) {
      el.scrollIntoView({ behavior: "smooth" });
    }
  };

  return (
    <header className={styles.header}>
      <div className={styles.headerContainer}>
        <div className={styles.brandLockup} onClick={() => window.scrollTo({ top: 0, behavior: "smooth" })}>
          <div className={styles.brandLogo}>
            <BrandIcon size={22} variant="inverse" />
          </div>
          <span className={styles.brandTitle}>{t("landing.brand_name", language)}</span>
        </div>

        <nav>
          <ul className={styles.navLinks}>
            <li>
              <span className={styles.navLink} onClick={() => scrollToSection("demo")}>
                {t("landing.nav_demo", language)}
              </span>
            </li>
            <li>
              <span className={styles.navLink} onClick={() => scrollToSection("pillars")}>
                {t("landing.nav_features", language)}
              </span>
            </li>
            <li>
              <span className={styles.navLink} onClick={() => scrollToSection("comparison")}>
                {t("landing.nav_comparison", language)}
              </span>
            </li>
            <li>
              <span className={styles.navLink} onClick={() => scrollToSection("calculator")}>
                {t("landing.nav_calculator", language)}
              </span>
            </li>
            <li>
              <span className={styles.navLink} onClick={() => scrollToSection("pricing")}>
                {t("landing.nav_pricing", language)}
              </span>
            </li>
            <li>
              <span className={styles.navLink} onClick={() => scrollToSection("faq")}>
                {t("landing.nav_faq", language)}
              </span>
            </li>
          </ul>
        </nav>

        <div className={styles.headerActions}>
          <button
            type="button"
            className={styles.langToggle}
            onClick={() => onLanguageChange(nextLanguage)}
            title="Switch Language"
          >
            <GlobalOutlined />
            <span>{langLabel}</span>
          </button>

          <Link href="/login" className={styles.btnSecondary}>
            {t("landing.nav_login", language)}
          </Link>

          <button
            type="button"
            className={styles.btnPrimary}
            onClick={onOpenDemoModal}
          >
            {t("landing.nav_book_demo", language)}
          </button>
        </div>
      </div>
    </header>
  );
};
