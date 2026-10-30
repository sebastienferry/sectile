// Keep this palette and hash in sync with Sectile's shared epicColor module.
const palette = [
  "#6366f1", "#8b5cf6", "#10b981", "#f59e0b", "#f43f5e", "#06b6d4",
  "#3b82f6", "#f97316", "#00f0ff", "#d946ef", "#10f070", "#ffd000",
];

export function epicColorHex(parentKey?: string): string | undefined {
  const key = parentKey?.trim();
  if (!key) return undefined;
  let hash = 0x811c9dc5;
  for (let index = 0; index < key.length; index++) {
    hash ^= key.charCodeAt(index);
    hash = Math.imul(hash, 0x01000193);
  }
  return palette[(hash >>> 0) % palette.length];
}
