"use client";

import React, { useState } from "react";
import { DownOutlined, UpOutlined } from "@ant-design/icons";
import { Language, t } from "../../lib/i18n";
import styles from "../landing.module.css";

interface FaqSectionProps {
  language: Language;
}

export const FaqSection: React.FC<FaqSectionProps> = ({ language }) => {
  const [openIndices, setOpenIndices] = useState<number[]>([0]);

  const toggleIndex = (idx: number) => {
    setOpenIndices((prev) =>
      prev.includes(idx) ? prev.filter((i) => i !== idx) : [...prev, idx]
    );
  };

  const faqs = [
    { q: "landing.faq_q0", a: "landing.faq_a0" },
    { q: "landing.faq_q1", a: "landing.faq_a1" },
    { q: "landing.faq_q2", a: "landing.faq_a2" },
    { q: "landing.faq_q3", a: "landing.faq_a3" },
    { q: "landing.faq_q4", a: "landing.faq_a4" },
  ];

  return (
    <section id="faq" className={styles.section}>
      <div className={styles.sectionHeader}>
        <span className={styles.sectionBadge}>
          {t("landing.faq_badge", language)}
        </span>
        <h2 className={styles.sectionTitle}>
          {t("landing.faq_title", language)}
        </h2>
        <p className={styles.sectionSubtitle}>
          {t("landing.faq_subtitle", language)}
        </p>
      </div>

      <div className={styles.faqList}>
        {faqs.map((faq, idx) => {
          const isOpen = openIndices.includes(idx);
          return (
            <div key={idx} className={styles.faqItem}>
              <button
                type="button"
                className={styles.faqQuestion}
                onClick={() => toggleIndex(idx)}
                aria-expanded={isOpen}
              >
                <span>{t(faq.q, language)}</span>
                {isOpen ? <UpOutlined /> : <DownOutlined />}
              </button>
              {isOpen && (
                <div className={styles.faqAnswer}>
                  {t(faq.a, language)}
                </div>
              )}
            </div>
          );
        })}
      </div>
    </section>
  );
};
