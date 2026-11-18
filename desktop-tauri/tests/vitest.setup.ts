// jsdom lays nothing out. CodeMirror measures text ranges; give them empty
// rectangles so it can run, and production code needs no jsdom workaround.
const emptyRect = () => ({ x: 0, y: 0, top: 0, left: 0, right: 0, bottom: 0, width: 0, height: 0, toJSON() { return this; } }) as DOMRect;
const emptyRects = () => Object.assign([], { item: () => null }) as unknown as DOMRectList;
if (typeof Range !== "undefined") {
  Range.prototype.getBoundingClientRect ??= emptyRect;
  Range.prototype.getClientRects ??= emptyRects;
}
if (typeof document !== "undefined" && !document.elementFromPoint) document.elementFromPoint = () => null;
// Native dialog behavior is supplied by the WebView; jsdom only models the DOM.
if (typeof HTMLDialogElement !== "undefined") {
  HTMLDialogElement.prototype.showModal ??= function () { this.setAttribute("open", ""); };
  HTMLDialogElement.prototype.close ??= function () { this.removeAttribute("open"); };
}
