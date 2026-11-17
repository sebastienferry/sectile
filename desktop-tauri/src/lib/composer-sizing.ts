// Composer height cap (#913).
//
// The composer editor grows with its content from a ~2-line resting height up
// to `max(150px, 40% of its view)`, then scrolls internally. The cap is the
// CSS custom property `--composer-max-height`, kept in sync with the view
// height by a ResizeObserver so a window or pane resize moves it.

import { useLayoutEffect, type RefObject } from "react";

const MIN_MAX_HEIGHT = 150;
const MAX_HEIGHT_SHARE = 0.4;

export function composerMaxHeight(viewHeight: number): number {
  return Math.max(MIN_MAX_HEIGHT, viewHeight * MAX_HEIGHT_SHARE);
}

/**
 * Keeps `--composer-max-height` on `ref`'s element at `max(150px, 40%)` of its closest
 * `viewSelector` ancestor, following that view's resizes.
 */
export function useComposerMaxHeight(ref: RefObject<HTMLElement | null>, viewSelector: string) {
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
  }, [ref, viewSelector]);
}
