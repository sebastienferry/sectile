import { useEffect, useState } from "react";
export type Appearance = "system" | "light" | "dark";
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
export function useAppearance() {
  const [appearance, setAppearance] = useState<Appearance>(() => {
    const value = readPreference("appearance", "system");
    return value === "light" || value === "dark" ? value : "system";
  });
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
  return [appearance, setAppearance] as const;
}
