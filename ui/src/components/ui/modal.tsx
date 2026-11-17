import React, { useEffect, useLayoutEffect, useRef, useState } from "react";
import { Dialog } from "radix-ui";
import { Button } from "./controls.tsx";

export function Modal({
  title,
  children,
  onClose,
  wide = false,
  footer,
  anchor,
}: React.PropsWithChildren<{
  title: string;
  onClose?: () => void;
  wide?: boolean;
  footer?: React.ReactNode;
  anchor?: HTMLElement | null;
}>) {
  const [panel, setPanel] = useState<HTMLElement | null>(null);
  const closeRef = useRef(onClose);
  const returnFocus = useRef(anchor || document.activeElement);
  closeRef.current = onClose;
  useLayoutEffect(() => {
    if (!anchor) return;
    const node = panel;
    if (!node) return;
    anchor.setAttribute("aria-expanded", "true");
    const position = () => {
      if (!anchor.isConnected) {
        closeRef.current?.();
        return;
      }
      const rect = anchor.getBoundingClientRect();
      const box = node.getBoundingClientRect();
      const left = Math.max(
        8,
        Math.min(rect.right - box.width, window.innerWidth - box.width - 8),
      );
      const below = rect.bottom + 8;
      const top =
        below + box.height <= window.innerHeight - 8
          ? below
          : Math.max(8, rect.top - box.height - 8);
      node.style.left = `${left}px`;
      node.style.top = `${top}px`;
    };
    position();
    const observer = new ResizeObserver(position);
    observer.observe(node);
    window.addEventListener("resize", position);
    document.addEventListener("scroll", position, true);
    return () => {
      observer.disconnect();
      window.removeEventListener("resize", position);
      document.removeEventListener("scroll", position, true);
    };
  }, [anchor, panel]);
  useEffect(() => {
    const closeEvent = () => {
      if ([...document.querySelectorAll('[role="dialog"]')].at(-1) === panel)
        closeRef.current?.();
    };
    document.addEventListener("eden-close-modal", closeEvent);
    return () => {
      document.removeEventListener("eden-close-modal", closeEvent);
    };
  }, [panel]);
  return (
    <Dialog.Root
      open
      onOpenChange={(open) => {
        if (!open) onClose?.();
      }}
    >
      <Dialog.Portal>
        <Dialog.Overlay asChild>
          <div
            className={`modal-scrim${anchor ? " anchored-modal-scrim" : ""}`}
            onPointerDown={(event) => {
              if (
                event.target !== event.currentTarget ||
                event.button !== 0 ||
                event.ctrlKey
              )
                return;
              closeRef.current?.();
            }}
          >
            <Dialog.Content
              asChild
              aria-describedby={undefined}
              onEscapeKeyDown={(event) => {
                if (!panel?.isConnected) return;
                // Radix sees Escape during capture; an expanded combobox must
                // receive it before the containing dialog can dismiss.
                if (
                  event.target instanceof Element &&
                  event.target.closest(
                    '[role="combobox"][aria-expanded="true"]',
                  )
                ) {
                  event.preventDefault();
                  return;
                }
                // The dialog owns this key even if closing removes it before
                // underlying page listeners inspect the DOM.
                event.stopPropagation();
                if (!closeRef.current) event.preventDefault();
              }}
              onPointerDownOutside={(event) => {
                // The rendered backdrop owns pointer dismissal. It works from
                // first paint rather than waiting for a document listener.
                event.preventDefault();
              }}
              onCloseAutoFocus={(event) => {
                event.preventDefault();
                const target = returnFocus.current;
                const active = document.activeElement;
                // Radix dispatches this after unmount. A newer panel or user
                // interaction may already own focus by then.
                const restore =
                  !active ||
                  active === document.body ||
                  !active.isConnected ||
                  (event.currentTarget instanceof Node &&
                    event.currentTarget.contains(active));
                if (
                  restore &&
                  target instanceof HTMLElement &&
                  target.isConnected
                )
                  target.focus();
                anchor?.removeAttribute("aria-expanded");
              }}
            >
              <section
                ref={setPanel}
                className={`modal ${wide ? "wide" : ""}`}
                role="dialog"
                aria-modal="true"
                aria-label={title}
              >
                <header className="modal-head">
                  <Dialog.Title asChild>
                    <h2>{title}</h2>
                  </Dialog.Title>
                  {onClose && (
                    <Button
                      className="icon-button close-modal"
                      aria-label="关闭面板"
                      onClick={onClose}
                    >
                      ×
                    </Button>
                  )}
                </header>
                {children}
                {footer && <footer className="modal-footer">{footer}</footer>}
              </section>
            </Dialog.Content>
          </div>
        </Dialog.Overlay>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
