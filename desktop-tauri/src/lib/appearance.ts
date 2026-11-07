import { useEffect, useState } from "react";
export type Appearance = "system" | "light" | "dark";
export type FontSize = "compact" | "default" | "large" | "xlarge";

const APPEARANCE_EVENT = "sectile:appearance-change";

export function readPreference(key: string, fallback: string) {
  try {
    return localStorage.getItem(`sectile-tauri.${key}`) || fallback;
  } catch {
    return fallback;
  }
}
export function savePreference(key: string, value: string) {
  try {
    localStorage.setItem(`sectile-tauri.${key}`, value);
  } catch {
    /* Preferences are optional when storage is unavailable. */
  }
}
function parseAppearance(value: string): Appearance {
  return value === "light" || value === "dark" ? value : "system";
}
function parseFontSize(value: string): FontSize {
  return value === "compact" || value === "default" || value === "large" || value === "xlarge"
    ? value
    : "default";
}
export function useAppearance() {
  const [appearance, setAppearanceState] = useState<Appearance>(() =>
    parseAppearance(readPreference("appearance", "system")),
  );
  useEffect(() => {
    const media = window.matchMedia?.("(prefers-color-scheme: dark)");
    const update = () => {
      document.documentElement.dataset.theme =
        appearance === "system"
          ? media?.matches
            ? "dark"
            : "light"
          : appearance;
    };
    update();
    savePreference("appearance", appearance);
    media?.addEventListener("change", update);
    return () => media?.removeEventListener("change", update);
  }, [appearance]);
  useEffect(() => {
    const onSync = () => setAppearanceState(parseAppearance(readPreference("appearance", "system")));
    window.addEventListener(APPEARANCE_EVENT, onSync);
    return () => window.removeEventListener(APPEARANCE_EVENT, onSync);
  }, []);
  const setAppearance = (next: Appearance | ((prev: Appearance) => Appearance)) => {
    setAppearanceState(prev => {
      const resolved = typeof next === "function" ? next(prev) : next;
      savePreference("appearance", resolved);
      window.dispatchEvent(new Event(APPEARANCE_EVENT));
      return resolved;
    });
  };
  return [appearance, setAppearance] as const;
}

export function useFontSize() {
  const [fontSize, setFontSizeState] = useState<FontSize>(() =>
    parseFontSize(readPreference("fontSize", "default")),
  );
  useEffect(() => {
    document.documentElement.dataset.fontSize = fontSize;
    savePreference("fontSize", fontSize);
  }, [fontSize]);
  useEffect(() => {
    const onSync = () => setFontSizeState(parseFontSize(readPreference("fontSize", "default")));
    window.addEventListener(APPEARANCE_EVENT, onSync);
    return () => window.removeEventListener(APPEARANCE_EVENT, onSync);
  }, []);
  const setFontSize = (next: FontSize | ((prev: FontSize) => FontSize)) => {
    setFontSizeState(prev => {
      const resolved = typeof next === "function" ? next(prev) : next;
      savePreference("fontSize", resolved);
      window.dispatchEvent(new Event(APPEARANCE_EVENT));
      return resolved;
    });
  };
  return [fontSize, setFontSize] as const;
}
