import { createElement, Fragment, memo, useEffect, useMemo, useState, type ReactNode } from "react";
import { decodeHTMLStrict } from "entities";
import { marked, type MarkedToken, type Token } from "marked";
import { invoke, isTauri } from "@tauri-apps/api/core";

export function safeWebLink(value: string): string | undefined {
  try {
    const url = new URL(value);
    if (["https:", "http:"].includes(url.protocol) && !url.username && !url.password) return url.href;
  } catch { /* Relative paths and non-web schemes are rendered as text. */ }
}
function CodeBlock({ text, language }: { text: string; language?: string }) {
  const [status, setStatus] = useState("");
  useEffect(() => setStatus(""), [text]);
  return <div className="markdown-code"><div className="code-toolbar"><span>{language?.split(/\s/)[0] || "Code"}</span>
    <button type="button" aria-label="Copier le code" onClick={async () => {
      try { await navigator.clipboard.writeText(text); setStatus("Copié"); }
      catch { setStatus("Copie indisponible"); }
    }}>Copier</button><span role="status">{status}</span></div><pre><code>{text}</code></pre></div>;
}
function WebLink({ href, children }: { href: string; children: ReactNode }) {
  const [error, setError] = useState(false);
  return <><a href={href} target="_blank" rel="noreferrer noopener" onClick={event => {
    if (isTauri()) { event.preventDefault(); void invoke("open_conversation_link", { url: href }).catch(() => setError(true)); }
  }}>{children}</a>{error && <span role="alert"> Impossible d’ouvrir le lien.</span>}</>;
}
function renderTokens(tokens: Token[] = []): ReactNode {
  return tokens.map((value, index) => {
    const token = value as MarkedToken;
    const nested = "tokens" in token ? renderTokens(token.tokens) : null;
    let content: ReactNode;
    switch (token.type) {
      case "space": case "def": return null;
      case "heading": content = createElement(`h${Math.min(6, token.depth + 1)}`, null, nested); break;
      case "paragraph": content = <p>{nested}</p>; break;
      case "text": content = token.tokens ? nested : decodeHTMLStrict(token.text); break;
      case "escape": content = token.text; break;
      case "strong": content = <strong>{nested}</strong>; break;
      case "em": content = <em>{nested}</em>; break;
      case "del": content = <del>{nested}</del>; break;
      case "br": content = <br />; break;
      case "hr": content = <hr />; break;
      case "blockquote": content = <blockquote>{nested}</blockquote>; break;
      case "codespan": content = <code>{token.text}</code>; break;
      case "code": content = <CodeBlock text={token.text} language={token.lang} />; break;
      case "list": {
        const items = token.items.map((item, i) => <li key={i}>{item.task && <input type="checkbox" checked={!!item.checked} disabled aria-label="État de la tâche" />}{renderTokens(item.tokens)}</li>);
        content = token.ordered ? <ol start={Number(token.start) || 1}>{items}</ol> : <ul>{items}</ul>; break;
      }
      case "link": {
        const href = safeWebLink(token.autolink ? token.href : decodeHTMLStrict(token.href));
        content = href ? <WebLink href={href}>{token.autolink ? token.text : nested}</WebLink> : nested; break;
      }
      // Never execute generated HTML or fetch remote image resources automatically.
      case "html": content = token.raw; break;
      case "image": content = <span className="muted">[Image : {token.text}]</span>; break;
      case "table": content = <div className="markdown-table"><table><thead><tr>{token.header.map((cell, i) => <th key={i}>{renderTokens(cell.tokens)}</th>)}</tr></thead><tbody>{token.rows.map((row, i) => <tr key={i}>{row.map((cell, j) => <td key={j}>{renderTokens(cell.tokens)}</td>)}</tr>)}</tbody></table></div>; break;
      case "checkbox": content = <input type="checkbox" checked={token.checked} disabled aria-label="État de la tâche" />; break;
      case "list_item": content = <li>{nested}</li>; break;
      default: content = value.raw;
    }
    return <Fragment key={index}>{content}</Fragment>;
  });
}
export const Markdown = memo(function Markdown({ text }: { text: string }) {
  const tokens = useMemo(() => marked.lexer(text, { gfm: true }), [text]);
  return <div className="markdown-body">{renderTokens(tokens)}</div>;
});
