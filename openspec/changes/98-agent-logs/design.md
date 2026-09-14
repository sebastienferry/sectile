# Design

Use a dedicated workspace section alongside the existing execution article. Toggle article/pane visibility without destroying the terminal or changing the selected run. Keep the workspace visible for logs while offline; restore setup when closed offline. Explicit sidebar selection closes logs, but background execution refresh does not. Close and Escape restore focus to Agent logs; dialogs retain their own Escape handling.

Sanitize snapshot text in a pure renderer helper before assigning textContent. Consume CSI (including private parameters and intermediate bytes), OSC title/hyperlink sequences terminated by BEL or ST, other terminal strings, and ordinary escape commands. Drop remaining nonprinting controls except newline/tab, normalize carriage-return line endings, and omit unfinished escape sequences at the end of a snapshot. Preserve printable Unicode and HTML-like text. Raw bounded reads and files remain unchanged. A tail starting inside an escape sequence cannot reconstruct the missing prefix; do not guess or discard ordinary text.

Rejected: browser fullscreen (hides navigation), enlarging a modal (blocks the sidebar), and executing terminal commands to render diagnostics (unnecessary terminal semantics). No architectural migration is required.
