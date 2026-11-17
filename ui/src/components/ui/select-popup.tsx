import { useLayoutEffect, useState } from "react";
import type { CSSProperties, RefObject } from "react";

// Mount the list outside scroll/overflow containers.
export function useSelectPopup(
  visible: boolean,
  control: RefObject<HTMLElement | null>,
  list: RefObject<HTMLElement | null>,
  count: number,
) {
  const [position, setPosition] = useState<CSSProperties | null>(null);
  useLayoutEffect(() => {
    if (!visible) return;
    const place = () => {
      if (!control.current || !list.current) return;
      const rect = control.current.getBoundingClientRect();
      const style = getComputedStyle(control.current);
      const gap = 4,
        margin = 8;
      const below = window.innerHeight - rect.bottom - gap - margin;
      const above = rect.top - gap - margin;
      const upward =
        below < Math.min(list.current.scrollHeight + 8, 224) && above > below;
      const maxHeight = Math.max(40, Math.min(280, upward ? above : below));
      const width = Math.min(rect.width, window.innerWidth - margin * 2);
      const height = Math.min(list.current.scrollHeight + 8, maxHeight);
      const next: CSSProperties = {
        position: "fixed",
        zIndex: 10000,
        boxSizing: "border-box",
        margin: 0,
        left: Math.max(
          margin,
          Math.min(rect.left, window.innerWidth - width - margin),
        ),
        top: upward
          ? Math.max(margin, rect.top - gap - height)
          : rect.bottom + gap,
        width,
        maxHeight,
        fontFamily: style.fontFamily,
        fontSize: style.fontSize,
        lineHeight: style.lineHeight,
      };
      for (const key of [
        "--ink",
        "--paper",
        "--line",
        "--selected",
        "--focus",
        "--muted",
        "--control-cursor",
      ] as const)
        next[key] = style.getPropertyValue(key);
      setPosition(next);
    };
    const scroll = (event: Event) => {
      if (event.target instanceof Node && !list.current?.contains(event.target))
        place();
    };
    place();
    const observer = new ResizeObserver(place);
    if (control.current) observer.observe(control.current);
    window.addEventListener("resize", place);
    document.addEventListener("scroll", scroll, true);
    return () => {
      observer.disconnect();
      window.removeEventListener("resize", place);
      document.removeEventListener("scroll", scroll, true);
    };
  }, [visible, control, list, count]);
  return position || { visibility: "hidden" };
}
