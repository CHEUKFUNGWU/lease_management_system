"use client";

import React, { useState } from "react";
import { useLanguage } from "../context/LanguageContext";
import { LandingHeader } from "./components/LandingHeader";
import { HeroSection } from "./components/HeroSection";
import { AsymmetricBentoSection } from "./components/AsymmetricBentoSection";
import { InteractiveDemoSection } from "./components/InteractiveDemoSection";
import { DarkServiceMatrixSection } from "./components/DarkServiceMatrixSection";
import { BeforeAfterLensSection } from "./components/BeforeAfterLensSection";
import { FeatureDeepDiveSection } from "./components/FeatureDeepDiveSection";
import { DataArchitectureSection } from "./components/DataArchitectureSection";
import { ConcentricRadarSection } from "./components/ConcentricRadarSection";
import { RoiCalculatorSection } from "./components/RoiCalculatorSection";
import { PersonaSection } from "./components/PersonaSection";
import { PricingSection } from "./components/PricingSection";
import { FaqSection } from "./components/FaqSection";
import { LeadCaptureModal } from "./components/LeadCaptureModal";
import { LandingFooter } from "./components/LandingFooter";
import { ScrollReveal } from "./components/ScrollReveal";
import styles from "./landing.module.css";

export default function LandingPage() {
  const { language, setLanguage } = useLanguage();
  const [demoModalOpen, setDemoModalOpen] = useState(false);

  return (
    <div className={styles.landingRoot}>
      <LandingHeader
        language={language}
        onLanguageChange={setLanguage}
        onOpenDemoModal={() => setDemoModalOpen(true)}
      />

      <main>
        {/* 1. Hero: Editorial Display + Asymmetric Mosaic Bento */}
        <ScrollReveal direction="fade">
          <HeroSection
            language={language}
            onOpenDemoModal={() => setDemoModalOpen(true)}
          />
        </ScrollReveal>

        {/* 2. Asymmetric Bento: Vertical Growth Bars + Radar NPV */}
        <ScrollReveal direction="up" delayMs={100}>
          <AsymmetricBentoSection language={language} />
        </ScrollReveal>

        {/* 3. Interactive Cockpit: Live macOS Simulation */}
        <ScrollReveal direction="scale" delayMs={100}>
          <InteractiveDemoSection language={language} />
        </ScrollReveal>

        {/* 4. Deep Emerald Service Matrix: 6-Grid with Corner Triggers & Bold Metrics */}
        <ScrollReveal direction="up" delayMs={100}>
          <DarkServiceMatrixSection language={language} />
        </ScrollReveal>

        {/* 5. Before vs After Reality Lens: Interactive Dual-Stage Contrast */}
        <ScrollReveal direction="up" delayMs={100}>
          <BeforeAfterLensSection language={language} />
        </ScrollReveal>

        {/* 6. Feature Deep Dive: 3-Chapter Asymmetrical Interactive Stage */}
        <ScrollReveal direction="up" delayMs={100}>
          <FeatureDeepDiveSection language={language} />
        </ScrollReveal>

        {/* 7. Technical Data Architecture: Full-Width SVG Pipeline Flowchart */}
        <ScrollReveal direction="scale" delayMs={100}>
          <DataArchitectureSection language={language} />
        </ScrollReveal>

        {/* 8. Ecosystem Concentric Radar: Connected System Nodes */}
        <ScrollReveal direction="up" delayMs={100}>
          <ConcentricRadarSection language={language} />
        </ScrollReveal>

        {/* 9. Interactive ROI Simulator */}
        <ScrollReveal direction="up" delayMs={100}>
          <RoiCalculatorSection
            language={language}
            onOpenDemoModal={() => setDemoModalOpen(true)}
          />
        </ScrollReveal>

        {/* 10. Executive Roles & Persona Value */}
        <ScrollReveal direction="up" delayMs={100}>
          <PersonaSection language={language} />
        </ScrollReveal>

        {/* 11. Transparent 4-Tier Pricing Grid */}
        <ScrollReveal direction="up" delayMs={100}>
          <PricingSection
            language={language}
            onOpenDemoModal={() => setDemoModalOpen(true)}
          />
        </ScrollReveal>

        {/* 12. Minimalist FAQ Accordion */}
        <ScrollReveal direction="fade" delayMs={100}>
          <FaqSection language={language} />
        </ScrollReveal>
      </main>

      {/* 13. High-Contrast Dark Bottom CTA & Clean Footer */}
      <ScrollReveal direction="fade">
        <LandingFooter
          language={language}
          onOpenDemoModal={() => setDemoModalOpen(true)}
        />
      </ScrollReveal>

      <LeadCaptureModal
        isOpen={demoModalOpen}
        onClose={() => setDemoModalOpen(false)}
        language={language}
      />
    </div>
  );
}
