// The text a user message shows once the trailers Sectile injects into a
// prompt (run contracts, sandbox notes, folder blocks, task UUID prefixes) are
// stripped. The transcript and the composer's prompt history (#913) both use it.

const INJECTED_USER_TRAILERS = [
  /^Remote execution runId:\s*[0-9a-f-]+\./i,
  /^Sandbox:\s*a Bash call runs outside/i,
  /^Preserve accepted artifacts\./i,
  /^Specification artefacts are dropped/i,
  /^Folders of this task \(never search for them/i,
  /^Work in the primary worktree:/i,
];

export function cleanUserEventText(text: string): string {
  const paragraphs = text
    .split(/\n{2,}/)
    .map(part => part.trim())
    .filter(part => part.length > 0 && !INJECTED_USER_TRAILERS.some(pattern => pattern.test(part)));
  const cleaned = (paragraphs.length > 0 ? paragraphs.join("\n\n") : text.trim()).replace(
    /^(\/[a-z-]+)\s+(?:jira|gh|gl|azdo)-[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}-([A-Za-z0-9_-]+)/i,
    "$1 $2",
  );
  return cleaned;
}
