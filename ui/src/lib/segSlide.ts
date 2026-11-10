/**
 * Callback ref for a `.ss-seg`: tracks the selected (`.on`) segment's box in CSS variables so the
 * Clean style can draw one pill that slides between segments instead of jumping. Only a change of
 * segment slides; until then, placement and resizes (fonts loading on mount) jump into place.
 */
export function segSlide(el: HTMLElement | null) {
  if (!el) return;
  let last: HTMLElement | null = null;
  const measure = () => {
    const on = el.querySelector<HTMLElement>(':scope > .on');
    if (!on) { delete el.dataset.slide; last = null; return; }
    if (!el.dataset.slide) el.dataset.slide = 'still';
    // The first segment change turns sliding on, and it stays on so the resize that follows (bolder
    // label) slides too. The style is flushed first: switched in the same change as the new position,
    // the transition would not start.
    if (last && last !== on && el.dataset.slide !== 'move') {
      el.dataset.slide = 'move';
      void getComputedStyle(el, '::before').transitionProperty;
    }
    // Relative to this control's padding box even when it is not the offset parent (Playful leaves it
    // static), so the pill starts in the right place after a switch to Clean.
    const own = on.offsetParent === el;
    el.style.setProperty('--seg-x', `${own ? on.offsetLeft : on.offsetLeft - el.offsetLeft - el.clientLeft}px`);
    el.style.setProperty('--seg-y', `${own ? on.offsetTop : on.offsetTop - el.offsetTop - el.clientTop}px`);
    el.style.setProperty('--seg-w', `${on.offsetWidth}px`);
    el.style.setProperty('--seg-h', `${on.offsetHeight}px`);
    last = on;
  };
  measure();
  const mo = new MutationObserver(measure);
  // A label that changes width resizes the control, which the ResizeObserver catches.
  mo.observe(el, { subtree: true, childList: true, attributes: true, attributeFilter: ['class'] });
  const ro = typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(measure);
  ro?.observe(el);
  return () => { mo.disconnect(); ro?.disconnect(); };
}
