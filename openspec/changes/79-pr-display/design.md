# Design

Reuse the existing PR map, label formatter and external-opening API. Append the PR button inside `.local-task` after the `.run` button and before archive/menu. Render a decorative inline SVG instead of visible PR text; retain the URL title and `Open PR ... for ...` accessible name.

Replace the old block/margin styling with a fixed, nonshrinking inline icon control and visible keyboard focus. Keep the title flexible and truncated so adding a PR does not change row height or force wrapping at the minimum supported sidebar width.

A separate text badge is rejected because it adds vertical space and contradicts #79. Changing the web views or selected-task toolbar is outside the clarified scope. No new package or architectural decision is needed.

Extend the isolated Electron mock-agent pattern to verify icon presence/absence, geometry with a long title at minimum width, mouse and keyboard activation without selecting the PR's task, and dynamic link removal. Existing console tests cover the other row actions. Validate with OpenSpec, desktop build, JavaScript syntax checks and the complete desktop UI suite. No desktop lint script is configured.
