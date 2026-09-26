/**
 * Strings of the operational feedback (#532): notifications, connection and
 * read failures, the activities and sync views, and the English rendering of
 * the server's activity templates (see `lib/activityText.ts`).
 *
 * French is the reference and keeps the wording the interface already had;
 * English is typed after it, so a key missing in English fails the build.
 */
const fr = {
}

export type OperationsStrings = typeof fr

const en: OperationsStrings = {
}

export const operations = { fr, en }
