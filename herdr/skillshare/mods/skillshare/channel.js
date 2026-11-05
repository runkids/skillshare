// How the panel and its dialogs talk: requests a dialog hands to the panel, one lock per machine,
// and the notice line. It all lives in Herdr's shared values, because a dialog is a separate view
// whose tasks end when it closes. No Herdr imports: main.js passes shared/setShared in, the tests
// a Map.
export const REQUEST = "skillshare.request.v1"; // { id, op, ... }
export const NOTICE = "skillshare.notice.v1"; // { kind, text, at }
export const BUSY = "skillshare.busy.v1"; // { [machine]: { text, at } }; "" is this computer

// createChannel({ get, set, now }): get(key) and set(key, value) read and write shared values.
//   request(op): hands op to the panel.
//   listen(): for the panel; returns next(), which gives each request made after listen() once,
//     then null.
//   claim(on, text): takes machine on's lock; false when something already holds it.
//   step(on, text): says what the holder is doing now. release(on): gives the lock back.
//   touch(on): the holder is still at work; call it before each command (it writes at most every
//     10 s). A lock untouched for staleMs is dropped: a dialog's task ends without a word when the
//     dialog closes, and a lock outlives a Herdr restart in shared.
//   busyAt(on), busyText(): read the locks.
//   say(kind, text), notice(), clearNotice(): the notice line ("done", "info", "warn", "error").
// Writes must not happen while a view renders; reads may.
export function createChannel({ get, set, now = Date.now, staleMs = 60 * 1000 }) {
  const key = (on) => on ?? "";
  const locks = () => {
    const b = get(BUSY);
    if (!b || typeof b !== "object" || Array.isArray(b)) return {}; // older panels kept a string here
    return Object.fromEntries(Object.entries(b).filter(([, l]) => l && now() - l.at < staleMs));
  };
  const busyAt = (on) => key(on) in locks();
  return {
    request(op) { set(REQUEST, { ...op, id: `${now()}-${Math.random().toString(36).slice(2, 7)}` }); },
    listen() {
      let last = get(REQUEST)?.id ?? null;
      return () => {
        const r = get(REQUEST);
        if (!r?.id || r.id === last) return null;
        last = r.id;
        return r;
      };
    },
    busyAt,
    busyText: () => Object.values(locks()).map((l) => l.text).join(" · ") || null,
    claim(on, text) {
      const l = locks();
      if (key(on) in l) return false;
      set(BUSY, { ...l, [key(on)]: { text, at: now() } });
      return true;
    },
    step(on, text) { set(BUSY, { ...locks(), [key(on)]: { text, at: now() } }); },
    release(on) {
      const l = locks();
      delete l[key(on)];
      set(BUSY, l);
    },
    touch(on) {
      const l = locks(), held = l[key(on)];
      if (held && now() - held.at >= 10 * 1000) set(BUSY, { ...l, [key(on)]: { ...held, at: now() } });
    },
    say(kind, text) { set(NOTICE, { kind, text, at: now() }); },
    notice: () => get(NOTICE) ?? null,
    clearNotice() { set(NOTICE, null); },
  };
}
