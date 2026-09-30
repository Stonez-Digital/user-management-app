"use client";

import { useEffect, useRef, useState } from "react";

type Theme = "light" | "dark";

export default function ThemeToggle() {
  const [theme, setTheme] = useState<Theme>("dark");
  const buttonRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    const stored = localStorage.getItem("stonez-theme") as Theme | null;
    const initial: Theme = stored === "light" || stored === "dark"
      ? stored
      : window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
    document.documentElement.dataset.theme = initial;
    setTheme(initial);
  }, []);

  useEffect(() => {
    const placeSafely = () => {
      const button = buttonRef.current;
      if (!button) return;
      const size = 46;
      const gap = 12;
      const candidates = [
        [window.innerWidth - size - gap, gap],
        [window.innerWidth - size - gap, window.innerHeight - size - gap],
        [gap, gap],
        [gap, window.innerHeight - size - gap],
      ];
      const controls = Array.from(document.querySelectorAll<HTMLElement>(
        "button, [role='button'], a.button, a[role='button']"
      )).filter((el) => el !== button && el.offsetParent !== null);

      const safe = candidates.find(([left, top]) => {
        const rect = { left, top, right: left + size, bottom: top + size };
        return !controls.some((el) => {
          const r = el.getBoundingClientRect();
          return rect.left < r.right + 6 && rect.right > r.left - 6 &&
                 rect.top < r.bottom + 6 && rect.bottom > r.top - 6;
        });
      }) || candidates[0];

      button.style.left = `${safe[0]}px`;
      button.style.top = `${safe[1]}px`;
      button.style.right = "auto";
      button.style.bottom = "auto";
    };

    placeSafely();
    window.addEventListener("resize", placeSafely);
    window.addEventListener("scroll", placeSafely, { passive: true });
    return () => {
      window.removeEventListener("resize", placeSafely);
      window.removeEventListener("scroll", placeSafely);
    };
  });

  const toggle = () => {
    const next: Theme = theme === "dark" ? "light" : "dark";
    document.documentElement.dataset.theme = next;
    localStorage.setItem("stonez-theme", next);
    setTheme(next);
  };

  return (
    <button
      ref={buttonRef}
      type="button"
      className="theme-toggle"
      onClick={toggle}
      aria-label={`Switch to ${theme === "dark" ? "light" : "dark"} mode`}
      title={`Switch to ${theme === "dark" ? "light" : "dark"} mode`}
    >
      {theme === "dark" ? "☀" : "☾"}
    </button>
  );
}
