# Design

Relocate the existing `run-state` and `skill-result` elements into a grouped status area inside `task-status`. Keep IDs and rendering paths so polling, result matching, selection, animation, and accessibility remain unchanged. Rename the selected-state styling to reflect its footer location. Let the footer wrap and long result text break at narrow widths.

Keep the next-step button's accessible current-skill name but hide its decorative badge whenever work is active or submitted. Keep the badge for available Next actions.

Rejected alternatives: duplicating indicators would add redundant information; changing skill-result semantics would exceed this presentation-only request. No architecture decision record is needed.
