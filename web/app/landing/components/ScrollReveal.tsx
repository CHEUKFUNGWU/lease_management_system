"use client";

import React, { useEffect, useRef, useState, createContext, useContext } from "react";
import styles from "../landing.module.css";

interface ScrollRevealProps {
  children: React.ReactNode;
  delayMs?: number;
  direction?: "up" | "down" | "left" | "right" | "fade" | "scale" | "fluid";
  className?: string;
  threshold?: number;
}

export const ScrollReveal: React.FC<ScrollRevealProps> = ({
  children,
  delayMs = 0,
  direction = "fluid",
  className = "",
  threshold = 0.08,
}) => {
  const ref = useRef<HTMLDivElement>(null);
  const [isVisible, setIsVisible] = useState(false);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;

    if (typeof IntersectionObserver === "undefined") {
      setIsVisible(true);
      return;
    }

    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            setIsVisible(true);
            observer.unobserve(entry.target);
          }
        });
      },
      {
        threshold,
        rootMargin: "0px 0px -60px 0px",
      }
    );

    observer.observe(el);

    return () => {
      observer.disconnect();
    };
  }, [threshold]);

  const getDirectionClass = () => {
    switch (direction) {
      case "down":
        return styles.revealFromDown;
      case "left":
        return styles.revealFromLeft;
      case "right":
        return styles.revealFromRight;
      case "scale":
        return styles.revealFromScale;
      case "fade":
        return styles.revealFromFade;
      case "fluid":
      case "up":
      default:
        return styles.revealFromFluid;
    }
  };

  const getDelayClass = () => {
    if (delayMs >= 500) return styles.revealDelay5;
    if (delayMs >= 400) return styles.revealDelay4;
    if (delayMs >= 300) return styles.revealDelay3;
    if (delayMs >= 200) return styles.revealDelay2;
    if (delayMs >= 100) return styles.revealDelay1;
    if (delayMs >= 50) return styles.revealDelay05;
    return "";
  };

  return (
    <div
      ref={ref}
      className={`${styles.revealWrapper} ${getDirectionClass()} ${getDelayClass()} ${
        isVisible ? styles.revealVisible : styles.revealHidden
      } ${className}`}
    >
      {children}
    </div>
  );
};

// Context for Cascading Stagger Groups
const StaggerContext = createContext<boolean>(false);

export const StaggerGroup: React.FC<{
  children: React.ReactNode;
  className?: string;
  threshold?: number;
}> = ({ children, className = "", threshold = 0.08 }) => {
  const ref = useRef<HTMLDivElement>(null);
  const [isVisible, setIsVisible] = useState(false);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;

    if (typeof IntersectionObserver === "undefined") {
      setIsVisible(true);
      return;
    }

    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            setIsVisible(true);
            observer.unobserve(entry.target);
          }
        });
      },
      {
        threshold,
        rootMargin: "0px 0px -50px 0px",
      }
    );

    observer.observe(el);

    return () => {
      observer.disconnect();
    };
  }, [threshold]);

  return (
    <StaggerContext.Provider value={isVisible}>
      <div ref={ref} className={`${styles.staggerContainer} ${className}`}>
        {children}
      </div>
    </StaggerContext.Provider>
  );
};

export const StaggerItem: React.FC<{
  children: React.ReactNode;
  index?: number;
  className?: string;
}> = ({ children, index = 0, className = "" }) => {
  const isVisible = useContext(StaggerContext);

  const delayClass =
    index === 0
      ? ""
      : index === 1
      ? styles.revealDelay1
      : index === 2
      ? styles.revealDelay2
      : index === 3
      ? styles.revealDelay3
      : index === 4
      ? styles.revealDelay4
      : styles.revealDelay5;

  return (
    <div
      className={`${styles.staggerItem} ${delayClass} ${
        isVisible ? styles.staggerItemVisible : styles.staggerItemHidden
      } ${className}`}
    >
      {children}
    </div>
  );
};
