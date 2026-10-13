import { useEffect, useRef } from "react";

// Inert every sibling along the path to body, including the global header.
// Restore their prior state and the invoking control when a sheet closes.
export default function useWorldDialog(
  container,
  active,
  onClose,
  returnFocus,
) {
  const close = useRef(onClose);
  close.current = onClose;
  useEffect(() => {
    if (!active || !container.current) return;
    const root = container.current;
    const invoking = returnFocus?.current || document.activeElement;
    const previous = [];
    let branch = root;
    while (branch.parentElement && branch !== document.body) {
      for (const sibling of branch.parentElement.children) {
        if (sibling === branch) continue;
        previous.push([sibling, sibling.inert]);
        sibling.inert = true;
      }
      branch = branch.parentElement;
    }
    const controls = () =>
      [
        ...root.querySelectorAll(
          'button:not(:disabled),a[href],input:not(:disabled),select:not(:disabled),textarea:not(:disabled),summary,[tabindex="0"]',
        ),
      ].filter((node) => node.getClientRects().length);
    controls()[0]?.focus({ preventScroll: true });
    const keydown = (event) => {
      if (event.key === "Escape") {
        event.preventDefault();
        close.current();
      }
      if (event.key !== "Tab") return;
      const items = controls();
      const index = items.indexOf(document.activeElement);
      if (!items.length) {
        event.preventDefault();
        root.focus();
      } else if (event.shiftKey && index <= 0) {
        event.preventDefault();
        items.at(-1).focus();
      } else if (!event.shiftKey && (index === items.length - 1 || index < 0)) {
        event.preventDefault();
        items[0].focus();
      }
    };
    root.addEventListener("keydown", keydown);
    return () => {
      root.removeEventListener("keydown", keydown);
      previous.forEach(([node, inert]) => {
        node.inert = inert;
      });
      if (invoking?.isConnected) invoking.focus({ preventScroll: true });
    };
  }, [container, active]);
}
