type Box = { values: string[]; index: number; at: number };

/** Where a keyed control's selection was when it unmounted. Each settings page renders its own tab
 *  row, so a route change replaces the row; the new one starts from here and slides. */
const handoff = new Map<string, Box>();
/** Long enough for a lazily loaded page to mount, short enough that coming back later does not slide. */
const HANDOFF_MS = 1000;

const VARS = ['--seg-x', '--seg-y', '--seg-w', '--seg-h'] as const;

/**
 * Callback ref for a `.ss-seg` or `.ss-tabs`: tracks the selected (`.on`) item's box in CSS variables
 * so the Clean style can draw one pill (segments) or underline (tabs) that slides between items
 * instead of jumping. Only a change of item slides; until then, placement and resizes (fonts loading
 * on mount) jump into place. Controls with the same `data-slide-key` hand the position over, so a
 * row that a route change replaces slides from where the old one was.
 */
export function segSlide(el: HTMLElement | null) {
  if (!el) return;
  const key = el.dataset.slideKey;
  let last: HTMLElement | null = null;
  let ro: ResizeObserver | null = null;
  const watched = new WeakSet<Element>();
  const measure = () => {
    // The items too: a `.ss-tabs` row is full width, so a label that changes width (a count, another
    // language, a late font) does not resize it. Once each, as observing again reports again.
    for (const item of el.children) {
      if (watched.has(item)) continue;
      watched.add(item);
      ro?.observe(item);
    }
    const on = el.querySelector<HTMLElement>(':scope > .on');
    if (!on) {
      delete el.dataset.slide;
      last = null;
      return;
    }
    if (!last && key) {
      const prev = handoff.get(key);
      handoff.delete(key);
      const index = Array.prototype.indexOf.call(el.children, on);
      if (prev && prev.index !== index && performance.now() - prev.at < HANDOFF_MS) {
        el.dataset.slide = 'still';
        VARS.forEach((v, i) => el.style.setProperty(v, prev.values[i]));
        void getComputedStyle(el, '::before').transform;
        el.dataset.slide = 'move';
        void getComputedStyle(el, '::before').transitionProperty;
      }
    }
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
    el.style.setProperty(
      '--seg-x',
      `${own ? on.offsetLeft : on.offsetLeft - el.offsetLeft - el.clientLeft}px`,
    );
    el.style.setProperty(
      '--seg-y',
      `${own ? on.offsetTop : on.offsetTop - el.offsetTop - el.clientTop}px`,
    );
    el.style.setProperty('--seg-w', `${on.offsetWidth}px`);
    el.style.setProperty('--seg-h', `${on.offsetHeight}px`);
    last = on;
  };
  ro = typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(measure);
  ro?.observe(el);
  measure();
  const mo = new MutationObserver(measure);
  mo.observe(el, { subtree: true, childList: true, attributes: true, attributeFilter: ['class'] });
  return () => {
    mo.disconnect();
    ro?.disconnect();
    if (key && last) {
      handoff.set(key, {
        values: VARS.map((v) => el.style.getPropertyValue(v)),
        index: Array.prototype.indexOf.call(el.children, last),
        at: performance.now(),
      });
    }
  };
}
