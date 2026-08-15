"use client";

import React from "react";
import Link from "next/link";
import { Language, t } from "../../lib/i18n";
import { BrandIcon } from "../../components/BrandIcon";
import styles from "../landing.module.css";

interface LandingFooterProps {
  language: Language;
  onOpenDemoModal: () => void;
}

export const LandingFooter: React.FC<LandingFooterProps> = ({
  language,
  onOpenDemoModal,
}) => {
  const scrollToSection = (id: string) => {
    const el = document.getElementById(id);
    if (el) {
      el.scrollIntoView({ behavior: "smooth" });
    }
  };

  return (
    <>
      {/* Bottom CTA Banner */}
      <section className={styles.bottomCtaSection}>
        <div className={styles.bottomCtaContainer}>
          <h2 className={styles.bottomCtaTitle}>
            {t("landing.cta_title", language)}
          </h2>
          <p className={styles.bottomCtaDesc}>
            {t("landing.cta_desc", language)}
          </p>
          <button
            type="button"
            className={styles.btnPrimaryInverse}
            onClick={onOpenDemoModal}
          >
            {t("landing.cta_btn", language)}
          </button>
        </div>
      </section>

      {/* Main Footer */}
      <footer className={styles.footer}>
        <div className={styles.footerContainer}>
          <div className={styles.footerTop}>
            <div>
              <div className={styles.brandLockup} onClick={() => window.scrollTo({ top: 0, behavior: "smooth" })}>
                <div className={styles.brandLogo}>
                  <BrandIcon size={20} variant="inverse" />
                </div>
                <span className={styles.brandTitle}>
                  {t("landing.brand_name", language)}
                </span>
              </div>
              <p className={styles.footerBrandDesc}>
                {t("landing.footer_desc", language)}
              </p>
            </div>

            <div className={styles.footerLinksCol}>
              <div className={styles.footerLinkGroup}>
                <div className={styles.footerLinkGroupTitle}>
                  {t("landing.nav_features", language)}
                </div>
                <span className={styles.footerLink} onClick={() => scrollToSection("demo")}>
                  {t("landing.demo_tab_pulse", language)}
                </span>
                <span className={styles.footerLink} onClick={() => scrollToSection("demo")}>
                  {t("landing.demo_tab_store", language)}
                </span>
                <span className={styles.footerLink} onClick={() => scrollToSection("demo")}>
                  {t("landing.demo_tab_scenario", language)}
                </span>
                <span className={styles.footerLink} onClick={() => scrollToSection("demo")}>
                  {t("landing.demo_tab_ifrs", language)}
                </span>
              </div>

              <div className={styles.footerLinkGroup}>
                <div className={styles.footerLinkGroupTitle}>
                  {t("landing.nav_comparison", language)}
                </div>
                <span className={styles.footerLink} onClick={() => scrollToSection("comparison")}>
                  {t("landing.nav_comparison", language)}
                </span>
                <span className={styles.footerLink} onClick={() => scrollToSection("calculator")}>
                  {t("landing.nav_calculator", language)}
                </span>
                <span className={styles.footerLink} onClick={() => scrollToSection("security")}>
                  {t("landing.nav_security", language)}
                </span>
                <span className={styles.footerLink} onClick={() => scrollToSection("faq")}>
                  {t("landing.nav_faq", language)}
                </span>
              </div>

              <div className={styles.footerLinkGroup}>
                <div className={styles.footerLinkGroupTitle}>
                  {t("landing.nav_login", language)}
                </div>
                <Link href="/login" className={styles.footerLink}>
                  {t("landing.nav_login", language)}
                </Link>
                <span className={styles.footerLink} onClick={onOpenDemoModal}>
                  {t("landing.nav_book_demo", language)}
                </span>
              </div>
            </div>
          </div>

          <div className={styles.footerBottom}>
            <div>
              <span>© {new Date().getFullYear()} {t("landing.brand_name", language)}. {t("landing.footer_rights", language)}</span>
            </div>
            <div>
              <span>{t("landing.footer_compliance_tag", language)}</span>
            </div>
          </div>
        </div>
      </footer>
    </>
  );
};
