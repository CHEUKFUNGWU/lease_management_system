"use client";

import React, { useState } from "react";
import { CloseOutlined } from "@ant-design/icons";
import { Language, t } from "../../lib/i18n";
import styles from "../landing.module.css";

interface LeadCaptureModalProps {
  isOpen: boolean;
  onClose: () => void;
  language: Language;
}

export const LeadCaptureModal: React.FC<LeadCaptureModalProps> = ({
  isOpen,
  onClose,
  language,
}) => {
  const [name, setName] = useState("");
  const [company, setCompany] = useState("");
  const [phone, setPhone] = useState("");
  const [email, setEmail] = useState("");
  const [storeScale, setStoreScale] = useState("10-50");
  const [interest, setInterest] = useState("pulse");
  const [submitting, setSubmitting] = useState(false);
  const [submitted, setSubmitted] = useState(false);

  if (!isOpen) return null;

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitting(true);
    setTimeout(() => {
      setSubmitting(false);
      setSubmitted(true);
    }, 600);
  };

  const handleModalClose = () => {
    setSubmitted(false);
    onClose();
  };

  return (
    <div className={styles.modalOverlay} onClick={handleModalClose}>
      <div className={styles.modalCard} onClick={(e) => e.stopPropagation()}>
        <button
          type="button"
          className={styles.modalCloseBtn}
          onClick={handleModalClose}
          aria-label="Close modal"
        >
          <CloseOutlined />
        </button>

        {submitted ? (
          <div className={styles.modalSuccessBox}>
            <p>{t("landing.modal_success", language)}</p>
            <button
              type="button"
              className={styles.btnSecondary}
              onClick={handleModalClose}
            >
              {t("landing.nav_login", language)}
            </button>
          </div>
        ) : (
          <div>
            <h3 className={styles.modalTitle}>
              {t("landing.modal_title", language)}
            </h3>
            <p className={styles.modalSubtitle}>
              {t("landing.modal_subtitle", language)}
            </p>

            <form onSubmit={handleSubmit} className={styles.formGrid}>
              <div className={styles.formGroup}>
                <label className={styles.formLabel}>
                  {t("landing.modal_name", language)} *
                </label>
                <input
                  type="text"
                  required
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  className={styles.formInput}
                  placeholder="e.g. Alex Chen"
                />
              </div>

              <div className={styles.formGroup}>
                <label className={styles.formLabel}>
                  {t("landing.modal_company", language)} *
                </label>
                <input
                  type="text"
                  required
                  value={company}
                  onChange={(e) => setCompany(e.target.value)}
                  className={styles.formInput}
                  placeholder="e.g. Acme Retail Group"
                />
              </div>

              <div className={styles.formGroup}>
                <label className={styles.formLabel}>
                  {t("landing.modal_phone", language)} *
                </label>
                <input
                  type="tel"
                  required
                  value={phone}
                  onChange={(e) => setPhone(e.target.value)}
                  className={styles.formInput}
                  placeholder="e.g. +86 13800000000"
                />
              </div>

              <div className={styles.formGroup}>
                <label className={styles.formLabel}>
                  {t("landing.modal_email", language)}
                </label>
                <input
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  className={styles.formInput}
                  placeholder="e.g. name@company.com"
                />
              </div>

              <div className={styles.formGroup}>
                <label className={styles.formLabel}>
                  {t("landing.modal_stores", language)}
                </label>
                <select
                  value={storeScale}
                  onChange={(e) => setStoreScale(e.target.value)}
                  className={styles.formSelect}
                >
                  <option value="10-50">
                    {t("landing.modal_stores_opt1", language)}
                  </option>
                  <option value="50-200">
                    {t("landing.modal_stores_opt2", language)}
                  </option>
                  <option value="200+">
                    {t("landing.modal_stores_opt3", language)}
                  </option>
                </select>
              </div>

              <div className={styles.formGroup}>
                <label className={styles.formLabel}>
                  {t("landing.modal_interest", language)}
                </label>
                <select
                  value={interest}
                  onChange={(e) => setInterest(e.target.value)}
                  className={styles.formSelect}
                >
                  <option value="pulse">
                    {t("landing.modal_interest_pulse", language)}
                  </option>
                  <option value="scenario">
                    {t("landing.modal_interest_scenario", language)}
                  </option>
                  <option value="ifrs">
                    {t("landing.modal_interest_ifrs", language)}
                  </option>
                </select>
              </div>

              <button
                type="submit"
                disabled={submitting}
                className={`${styles.btnPrimary} ${styles.btnPrimaryLarge}`}
              >
                {submitting
                  ? t("landing.modal_submitting", language)
                  : t("landing.modal_submit", language)}
              </button>
            </form>
          </div>
        )}
      </div>
    </div>
  );
};
