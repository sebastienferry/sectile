// Auto-growing composer textarea (#913).
//
// The textarea follows its content from a ~2-line resting height up to
// `max(150px, 40% of its view)`, then scrolls internally. The cap is the CSS
// custom property `--composer-max-height`, kept in sync with the view height
// by a ResizeObserver so a window or pane resize moves it.

import { useLayoutEffect, type RefObject } from "react";

const MIN_MAX_HEIGHT = 150;
const MAX_HEIGHT_SHARE = 0.4;

export function composerMaxHeight(viewHeight: number): number {
  return Math.max(MIN_MAX_HEIGHT, viewHeight * MAX_HEIGHT_SHARE);
}

function fitToContent(textarea: HTMLTextAreaElement) {
  textarea.style.height = "auto";
  const height = textarea.scrollHeight;
  const cap = parseFloat(getComputedStyle(textarea).maxHeight);
  // A zero scrollHeight means no layout (hidden or jsdom): leave the CSS resting height alone.
  if (height > 0) textarea.style.height = `${height}px`;
  textarea.style.overflowY = Number.isFinite(cap) && height > cap ? "auto" : "hidden";
}

/**
 * Keeps `--composer-max-height` on `ref`'s element at `max(150px, 40%)` of its closest
 * `viewSelector` ancestor, following that view's resizes. `onChange` runs after each update.
 */
export function useComposerMaxHeight(ref: RefObject<HTMLElement | null>, viewSelector: string, onChange?: (element: HTMLElement) => void) {
  useLayoutEffect(() => {
    const element = ref.current;
    if (!element) return;
    const view = element.closest<HTMLElement>(viewSelector);
    const ownerWindow = element.ownerDocument.defaultView;
    let current = "";
    const update = (viewHeight: number) => {
      const next = `${Math.round(composerMaxHeight(viewHeight))}px`;
      if (next === current) return;
      current = next;
      element.style.setProperty("--composer-max-height", next);
      onChange?.(element);
    };
    update(view?.clientHeight || ownerWindow?.innerHeight || MIN_MAX_HEIGHT);
    const Observer = ownerWindow?.ResizeObserver;
    if (!view || !Observer) return;
    const observer = new Observer(entries => {
      const entry = entries[entries.length - 1];
      if (entry) update(entry.contentRect.height);
    });
    observer.observe(view);
    return () => observer.disconnect();
    // onChange is a stable module function for its callers.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ref, viewSelector]);
}

/**
 * Sizes `ref`'s textarea to its content on every `value` change, capped by
 * `--composer-max-height` computed from the closest `viewSelector` ancestor.
 */
export function useAutoGrowTextarea(ref: RefObject<HTMLTextAreaElement | null>, value: string, viewSelector: string) {
  useComposerMaxHeight(ref, viewSelector, element => fitToContent(element as HTMLTextAreaElement));
  useLayoutEffect(() => {
    if (ref.current) fitToContent(ref.current);
  }, [ref, value]);
}

const MIRRORED_STYLES = [
  "boxSizing", "width", "paddingTop", "paddingRight", "paddingBottom", "paddingLeft",
  "borderTopWidth", "borderRightWidth", "borderBottomWidth", "borderLeftWidth",
  "fontFamily", "fontSize", "fontWeight", "fontStyle", "letterSpacing", "lineHeight",
  "textTransform", "wordSpacing", "tabSize", "textIndent",
] as const;

/**
 * Whether the caret is on the first ("up") or last ("down") rendered row of
 * the textarea, soft wraps included, measured with a hidden mirror element.
 * Returns undefined without layout, so the caller falls back to logical lines.
 */
export function caretOnEdgeRow(textarea: HTMLTextAreaElement, direction: "up" | "down"): boolean | undefined {
  const style = getComputedStyle(textarea);
  const lineHeight = parseFloat(style.lineHeight);
  if (!Number.isFinite(lineHeight) || lineHeight <= 0 || textarea.clientWidth === 0) return undefined;
  const caret = textarea.selectionStart ?? 0;
  const mirror = textarea.ownerDocument.createElement("div");
  for (const property of MIRRORED_STYLES) mirror.style[property] = style[property];
  Object.assign(mirror.style, { position: "absolute", visibility: "hidden", top: "0", left: "-9999px", whiteSpace: "pre-wrap", overflowWrap: "break-word", height: "auto", overflow: "hidden" });
  mirror.style.width = `${textarea.getBoundingClientRect().width}px`;
  const marker = textarea.ownerDocument.createElement("span");
  marker.textContent = "​";
  mirror.append(textarea.value.slice(0, caret), marker, textarea.value.slice(caret) || "​");
  textarea.ownerDocument.body.append(mirror);
  try {
    if (mirror.scrollHeight === 0) return undefined;
    // offsetTop and scrollHeight both count from the padding edge, borders excluded.
    const paddingTop = parseFloat(style.paddingTop) || 0;
    const paddingBottom = parseFloat(style.paddingBottom) || 0;
    const rows = Math.max(1, Math.round((mirror.scrollHeight - paddingTop - paddingBottom) / lineHeight));
    const row = Math.round((marker.offsetTop - paddingTop) / lineHeight);
    return direction === "up" ? row <= 0 : row >= rows - 1;
  } finally {
    mirror.remove();
  }
}
