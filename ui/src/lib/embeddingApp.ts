// Messages between this page and an app that embeds it in an iframe (the desktop shell).

/** Tell the embedding app something; does nothing when the page is not embedded.
 *  The target stays '*': the app's origin differs by platform (tauri://localhost,
 *  http://tauri.localhost, its dev server) and the messages carry only the theme, language and
 *  route. The app checks that each message comes from its own iframe. */
export function postToApp(message: object) {
  if (window.parent !== window) window.parent.postMessage(message, '*');
}

/** Whether a message came from the window that embeds this page. */
export function isFromApp(event: MessageEvent) {
  return event.source === window.parent;
}
