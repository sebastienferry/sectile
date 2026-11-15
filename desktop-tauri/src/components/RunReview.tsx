import { useEffect, useRef, useState } from "react";
import type { Run } from "../domain/execution";
import {
  editorPreference,
  openRunEditor,
  runDiff,
  type RunDiff,
  type RunDiffFile,
} from "../lib/workspace";
import { Icon } from "./Icon";
import { Markdown } from "./Markdown";

function isMarkdownFile(file: RunDiffFile): boolean {
  return file.kind === "text" && /\.(md|markdown)$/i.test(file.path);
}

function statusBadgeLabel(status: string): string {
  switch (status.toLowerCase()) {
    case "added":
    case "a":
      return "A";
    case "deleted":
    case "d":
      return "D";
    case "renamed":
    case "r":
      return "R";
    default:
      return "M";
  }
}

function statusFullLabel(status: string): string {
  switch (status.toLowerCase()) {
    case "added":
    case "a":
      return "Added";
    case "deleted":
    case "d":
      return "Deleted";
    case "renamed":
    case "r":
      return "Renamed";
    default:
      return "Modified";
  }
}

function splitFilePath(path: string): { name: string; dir: string } {
  const idx = path.lastIndexOf("/");
  if (idx === -1) return { name: path, dir: "" };
  return { name: path.slice(idx + 1), dir: path.slice(0, idx + 1) };
}

function diffLineClass(line: string): string {
  if (line.startsWith("+++") || line.startsWith("---") || line.startsWith("diff --git") || line.startsWith("index ")) {
    return "diff-meta";
  }
  if (line.startsWith("+")) return "diff-add diff-added";
  if (line.startsWith("-")) return "diff-delete diff-removed";
  if (line.startsWith("@@")) return "diff-hunk";
  return "diff-line";
}

export function RunReview({
  run,
  defaultOpen = false,
  fullPage = false,
  expanded = false,
  onToggleExpand,
  onClose,
}: {
  run: Run;
  defaultOpen?: boolean;
  fullPage?: boolean;
  expanded?: boolean;
  onToggleExpand?: () => void;
  onClose?: () => void;
}) {
  const [folder, setFolder] = useState("");
  const [diff, setDiff] = useState<RunDiff>();
  const [selectedPath, setSelectedPath] = useState<string>("");
  const [renderedMarkdown, setRenderedMarkdown] = useState(true);
  const [error, setError] = useState("");
  const [pending, setPending] = useState(false);
  const [editor, setEditor] = useState("");
  const [editorReady, setEditorReady] = useState(false);
  const [opened, setOpened] = useState(false);
  const request = useRef(0);

  const folders = [
    ...(run.directory ? [{ path: run.directory, name: "Working folder" }] : []),
    ...(run.folders ?? []),
  ].filter((f, i, all) => all.findIndex(other => other.path === f.path) === i);

  async function refresh(path = folder) {
    const seq = ++request.current;
    setPending(true);
    setError("");
    try {
      const data = await runDiff(run.id, path || undefined);
      if (seq === request.current) {
        setDiff(data);
        setSelectedPath(prev =>
          data.files?.some(f => f.path === prev) ? prev : (data.files?.[0]?.path ?? ""),
        );
      }
    } catch {
      if (seq === request.current) {
        setDiff(undefined);
        setError(
          "Review unavailable: check this run's Git checkout. A plain folder or deleted worktree does not provide a diff.",
        );
      }
    } finally {
      if (seq === request.current) setPending(false);
    }
  }

  useEffect(() => {
    if ((!defaultOpen && !fullPage) || opened) return;
    setOpened(true);
    void refresh();
    void editorPreference()
      .then(view => {
        setEditor(view.selection);
        setEditorReady(view.configured);
      })
      .catch(() => {});
  }, [defaultOpen, fullPage]);

  const selectedFile =
    diff?.files?.find(f => f.path === selectedPath) ?? diff?.files?.[0];
  const canRenderMarkdown =
    !!selectedFile && isMarkdownFile(selectedFile) && !!selectedFile.document?.content;

  if (fullPage) {
    return (
      <section className={`changes-page ${expanded ? "is-expanded" : "is-split"}`} aria-label="Git changes review">
        <div className="changes-page-toolbar">
          <div className="changes-page-toolbar-left">
            <span className="eyebrow">CHANGES</span>
            {diff?.branch && (
              <span className="changes-branch-badge" title="Run Git branch">
                <Icon name="branch" />
                <span>{diff.branch}</span>
              </span>
            )}
            {diff && (
              <span className="changes-summary-stats">
                {diff.isClean ? (
                  "No changes"
                ) : (
                  <>
                    <strong>{diff.filesChanged} {diff.filesChanged > 1 ? "files" : "file"}</strong>
                    {" · "}
                    <span className="diff-summary-add">+{diff.additions}</span>
                    {" "}
                    <span className="diff-summary-delete">−{diff.deletions}</span>
                    {(!diff.complete || diff.countsPartial) && " (partial)"}
                  </>
                )}
              </span>
            )}
            {pending && (
              <span className="changes-status-pill" role="status">
                Reading…
              </span>
            )}
          </div>
          <div className="changes-page-toolbar-right">
            {folders.length > 1 && (
              <label className="changes-folder-select-label">
                <span className="sr-only">Review folder</span>
                <select
                  aria-label="Review folder"
                  value={folder}
                  onChange={e => {
                    setFolder(e.target.value);
                    void refresh(e.target.value);
                  }}
                >
                  <option value="">Primary folder</option>
                  {folders
                    .filter(f => f.path !== run.directory)
                    .map(f => (
                      <option value={f.path} key={f.path}>
                        {f.name} · {f.path}
                      </option>
                    ))}
                </select>
              </label>
            )}
            {selectedFile && isMarkdownFile(selectedFile) && (
              <button
                type="button"
                role="switch"
                aria-checked={canRenderMarkdown && renderedMarkdown}
                disabled={!canRenderMarkdown}
                className="diff-render-toggle"
                onClick={() => setRenderedMarkdown(prev => !prev)}
              >
                <span className="diff-switch-track" aria-hidden="true" />
                <span>Rendered</span>
              </button>
            )}
            <button
              type="button"
              className="changes-toolbar-btn"
              disabled={pending}
              onClick={() => void refresh()}
              title="Refresh Git changes"
            >
              Refresh
            </button>
            <button
              type="button"
              className="changes-toolbar-btn"
              disabled={!editorReady || pending}
              onClick={() =>
                void openRunEditor(run.id, folder || undefined).catch(() =>
                  setError("Unable to open editor. Check its installation and the run folder."),
                )
              }
            >
              Editor
            </button>
            {onToggleExpand && (
              <button
                type="button"
                className="icon-button"
                aria-label={expanded ? "Collapse to split view" : "Expand to full screen"}
                title={expanded ? "Collapse to split view" : "Expand to full screen"}
                onClick={onToggleExpand}
              >
                <Icon name={expanded ? "collapse" : "expand"} />
              </button>
            )}
            {onClose && (
              <button
                type="button"
                className="icon-button"
                aria-label="Close changes review"
                title="Close changes review"
                onClick={onClose}
              >
                <Icon name="close" />
              </button>
            )}
          </div>
        </div>

        {error && (
          <p role="alert" className="error changes-error-banner">
            {error}
          </p>
        )}
        {diff?.warnings?.map((w, i) => (
          <p key={i} className="changes-warning-banner">
            {w.message}
          </p>
        ))}

        {diff && (diff.isClean || !diff.files?.length) ? (
          <div className="changes-empty-state">
            <p>No changes detected in this working folder compared to the base branch.</p>
          </div>
        ) : diff && diff.files?.length > 0 ? (
          <div className="changes-workspace">
            <aside className="changes-file-list" aria-label="Modified files">
              <div className="changes-file-list-header">
                <span>Files ({diff.files.length})</span>
              </div>
              <div className="changes-file-list-items">
                {diff.files.map(file => {
                  const { name, dir } = splitFilePath(file.path);
                  const active = file.path === selectedFile?.path;
                  const badge = statusBadgeLabel(file.status);
                  return (
                    <button
                      key={file.path}
                      type="button"
                      className={`changes-file-item ${active ? "active" : ""}`}
                      onClick={() => setSelectedPath(file.path)}
                      title={file.path}
                    >
                      <span className={`changes-status-badge status-${badge.toLowerCase()}`}>
                        {badge}
                      </span>
                      <span className="changes-file-item-path">
                        <strong>{name}</strong>
                        {dir && <small>{dir}</small>}
                      </span>
                      {(file.additions != null || file.deletions != null) && (
                        <span className="changes-file-item-counts">
                          {file.additions ? <span className="diff-summary-add">+{file.additions}</span> : null}
                          {file.deletions ? <span className="diff-summary-delete">−{file.deletions}</span> : null}
                        </span>
                      )}
                    </button>
                  );
                })}
              </div>
            </aside>

            <div className="changes-detail">
              {selectedFile && (
                <>
                  <header className="changes-detail-header">
                    <div className="changes-detail-path">
                      <span className={`changes-status-badge status-${statusBadgeLabel(selectedFile.status).toLowerCase()}`}>
                        {statusFullLabel(selectedFile.status)}
                      </span>
                      <code>
                        {selectedFile.oldPath
                          ? `${selectedFile.oldPath} → ${selectedFile.path}`
                          : selectedFile.path}
                      </code>
                    </div>
                    <div className="changes-detail-meta">
                      {selectedFile.additions != null && (
                        <span className="diff-summary-add">+{selectedFile.additions}</span>
                      )}
                      {selectedFile.deletions != null && (
                        <span className="diff-summary-delete">−{selectedFile.deletions}</span>
                      )}
                    </div>
                  </header>

                  <div className="changes-detail-viewport">
                    {canRenderMarkdown && renderedMarkdown ? (
                      <div className="diff-rendered" tabIndex={0} aria-label="Markdown preview">
                        {selectedFile.document?.side === "old" && (
                          <p className="diff-render-note">Old version: this file was deleted.</p>
                        )}
                        <Markdown text={selectedFile.document?.content || ""} />
                      </div>
                    ) : selectedFile.omittedReason ? (
                      <p className="muted changes-omitted-note">{selectedFile.omittedReason}</p>
                    ) : (
                      <pre className="diff-patch" tabIndex={0} aria-label="Selected file diff">
                        <code>
                          {(selectedFile.patch || "").split("\n").map((line, i) => (
                            <span key={i} className={diffLineClass(line)}>
                              {line}
                              {"\n"}
                            </span>
                          ))}
                        </code>
                      </pre>
                    )}
                  </div>
                </>
              )}
            </div>
          </div>
        ) : null}
      </section>
    );
  }

  return (
    <details
      className="run-review"
      open={defaultOpen || undefined}
      onToggle={e => {
        if (e.currentTarget.open && !opened) {
          setOpened(true);
          void refresh();
          void editorPreference()
            .then(view => {
              setEditor(view.selection);
              setEditorReady(view.configured);
            })
            .catch(() => {});
        }
      }}
    >
      <summary>Changes review</summary>
      <p className="muted">Reads changes in this run's folder without modifying files.</p>
      <label>
        Folder to inspect
        <select
          aria-label="Review folder"
          value={folder}
          onChange={e => {
            setFolder(e.target.value);
            void refresh(e.target.value);
          }}
        >
          <option value="">Run working folder</option>
          {folders
            .filter(f => f.path !== run.directory)
            .map(f => (
              <option value={f.path} key={f.path}>
                {f.name} · {f.path}
              </option>
            ))}
        </select>
      </label>
      <button disabled={pending} onClick={() => void refresh()}>
        Refresh changes
      </button>
      <details>
        <summary>Workstation editor</summary>
        <label>
          Editor
          <select aria-label="Workstation editor" value={editor} onChange={e => setEditor(e.target.value)}>
            <option value="">{editorReady ? "Existing configuration" : "Choose an editor"}</option>
            <option value="code">VS Code</option>
            <option value="cursor">Cursor</option>
            <option value="zed">Zed</option>
          </select>
        </label>
        <button
          disabled={!editor || pending}
          onClick={() => {
            setPending(true);
            void editorPreference(editor)
              .then(view => setEditorReady(view.configured))
              .catch(() => setError("Unable to save editor."))
              .finally(() => setPending(false));
          }}
        >
          Save this editor
        </button>
      </details>
      <button
        disabled={!editorReady || pending}
        onClick={() =>
          void openRunEditor(run.id, folder || undefined).catch(() =>
            setError("Unable to open editor. Check its installation and the run folder."),
          )
        }
      >
        Open in editor
      </button>
      {pending && <p role="status">Reading changes…</p>}
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      {diff && (
        <>
          <p>
            {diff.isClean ? "No changes" : `${diff.filesChanged} files · +${diff.additions} −${diff.deletions}`}
            {!diff.complete && " · Partial preview"}
          </p>
          {diff.warnings?.map((w, i) => (
            <p key={i}>{w.message}</p>
          ))}
          {diff.files?.map(file => (
            <details key={file.path} className="diff-file">
              <summary>
                {file.path} · {file.status}
              </summary>
              {file.omittedReason ? (
                <p>{file.omittedReason}</p>
              ) : (
                <pre>
                  <code>
                    {file.patch?.split("\n").map((line, i) => (
                      <span
                        key={i}
                        className={
                          line.startsWith("+") ? "diff-added" : line.startsWith("-") ? "diff-removed" : ""
                        }
                      >
                        {line}
                        {"\n"}
                      </span>
                    ))}
                  </code>
                </pre>
              )}
            </details>
          ))}
        </>
      )}
    </details>
  );
}
