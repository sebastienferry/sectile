# Design

The Tickets menu already sends all skill launches through `submitTicketLaunch`, which accepts a mode override. Supply `autonomous` only for its dedicated `pickup` item. Keep the shared launch path and empty override for the other menu skills.

Extend the existing Tickets UI test, which captures launch requests from the fake agent. Give its project an interactive configured default and assert the pickup request still carries `mode: autonomous`, while a regular skill omits `mode`. The existing toolbar and execution-mode tests cover neighboring controls.

Rejected alternatives: changing server precedence would affect all launch sources; changing generic dialogs would exceed the dedicated-action scope. No architecture decision record is needed.
