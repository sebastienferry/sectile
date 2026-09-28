# Design

Place existing controls in two explicit toolbar rows, retaining their IDs and handlers. Allow action controls to wrap inside the second row at narrow widths, and truncate the title and path without hiding controls.

Wrap the terminal and Changes elements in a shared flex content area. Keep both mounted so console attachment and output continue while Changes is visible. A separator appears only when both views are selected. Pointer drag and arrow keys change the Console share, bounded so both panes remain usable. The share resets on restart.

Keep diff selection and request invalidation in `gitDiff.js`. Showing Changes starts a refresh; hiding it invalidates pending responses. The console fit and input gates depend on console visibility, including split mode.

Using a CSS-only fixed split was rejected because the owner requested pointer and keyboard adjustment. Replacing either pane when toggled was rejected because independent toggles must support both views.
