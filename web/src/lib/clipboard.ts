/**
 * Clipboard writes that say whether they worked.
 *
 * `navigator.clipboard` is missing outside a secure context, and its
 * `writeText` rejects when the browser denies the permission. A caller that
 * confirms a copy must know which happened, so failure is a result here, not
 * an exception.
 */

export interface ClipboardWriter {
  writeText(text: string): Promise<void>
}

/**
 * Writes text to the clipboard; resolves false instead of throwing when the
 * clipboard is missing or refuses the write.
 */
export async function copyText(
  text: string,
  clipboard: ClipboardWriter | undefined = globalThis.navigator?.clipboard,
): Promise<boolean> {
  if (!clipboard) return false
  try {
    await clipboard.writeText(text)
    return true
  } catch {
    return false
  }
}
