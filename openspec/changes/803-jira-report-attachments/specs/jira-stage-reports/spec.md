## ADDED Requirements

### Requirement: Explicit report publication
Sectile SHALL recognize reports through explicit metadata for discussion publication and through the stage context for transition and managed result notes. Updated callers SHALL supply a short plain-text summary, stage and clarification round when applicable. Unmarked discussion comments SHALL retain existing behaviour without content guessing. Existing stage callers without summaries SHALL receive a generic stage summary without semantic summarization.

#### Scenario: Intermediate clarification
- **GIVEN** a Jira task and an explicitly marked clarification round with three settled decisions and open questions
- **WHEN** add_comment publishes the full round and supplied summary
- **THEN** the round is published as a report with one short sentence per settled decision followed by the open questions

#### Scenario: Legacy stage caller
- **GIVEN** a transition or managed result with a full note but no summary
- **WHEN** its report is published
- **THEN** a generic plain-text stage summary is used with the attachment reference

#### Scenario: Ordinary discussion
- **GIVEN** an unmarked comment containing Markdown or a long body
- **WHEN** it is posted
- **THEN** ordinary comment publication is used and no report attachment is inferred

### Requirement: Immutable Markdown attachments
Each Jira report publication SHALL attach the exact supplied full report text as a UTF-8 .md file, preserving whitespace, Markdown and line endings. Names SHALL identify task and stage and clarification round when applicable, such as GODE-158-clarification-round-3.md and GODE-158-specification.md. Distinct publications SHALL retain distinct attachments without replacing or deleting older ones.

#### Scenario: Formatting that previously failed
- **GIVEN** a full report containing inline code inside bold text, tables, nested lists and continued list lines
- **WHEN** it is published to Jira
- **THEN** the attachment contains the unchanged report and report formatting does not affect summary publication

#### Scenario: New report at the same stage
- **GIVEN** a previously published specification report
- **WHEN** a genuinely new specification report is published
- **THEN** a separate attachment is created and the earlier attachment remains accessible

### Requirement: Plain-text summaries
Jira report comments SHALL contain plain sentences without interpreting Markdown, name or link the attachment, and include available PR/MR URLs for implementation, review and handoff outcomes. Specification summaries SHALL describe the goal in one or two sentences; other stage outcomes SHALL use one or two outcome sentences. Summary publication SHALL bypass the general Markdown conversion path.

#### Scenario: Specification summary
- **GIVEN** a specification note and caller-supplied summary
- **WHEN** specified is published
- **THEN** Jira receives the full report attachment and a short goal summary identifying that attachment

#### Scenario: Outcome with PR
- **GIVEN** an implementation report with an associated PR URL
- **WHEN** it is published
- **THEN** the summary contains the outcome, attachment reference and PR URL as readable text or links

### Requirement: Durable full report history
Sectile SHALL retain the full supplied report before any upload, including intermediate rounds, alongside publication identity, stage, round, author and remote publication status. Authorized task readers SHALL be able to read the full report in the task view even when upload or tracker reads fail. Report access SHALL obey existing task authorization and retain chronological history.

#### Scenario: Successful attachment
- **GIVEN** a Jira report successfully published
- **WHEN** the task view is opened
- **THEN** the full report is accessible and its attachment link is available

#### Scenario: Tracker unavailable
- **GIVEN** a retained intermediate report and unavailable Jira reads
- **WHEN** an authorized reader opens its task
- **THEN** the complete report remains accessible from Sectile history

### Requirement: Non-blocking upload failures
An attachment failure SHALL produce a structured caller-visible warning with publication identity, retain the full report and still attempt the summary comment stating that the report could not be attached. Upload failure alone SHALL NOT fail or roll back an otherwise successful stage transition. Summary-comment failure SHALL remain a visible publication failure. No fallback SHALL split the full report into Jira comments.

#### Scenario: Upload refused
- **GIVEN** disabled attachments, missing permission, oversized content or a failed attachment request
- **WHEN** a report is published
- **THEN** the summary is attempted with an attachment-failure notice, the full report is retained, and a warning is returned without blocking the stage solely for that upload failure

#### Scenario: Comment fails after upload
- **GIVEN** an uploaded attachment and a failed summary comment
- **WHEN** publication finishes
- **THEN** the caller sees a publication failure and Sectile retains the attachment reference for retry

#### Scenario: Retention fails
- **GIVEN** full report persistence fails
- **WHEN** publication is requested
- **THEN** no remote report upload begins and the failure is visible

### Requirement: Retry and acting-user integrity
Retry of the same publication SHALL reuse its retained report and known successful attachment and SHALL NOT replace historic reports. Upload and comment SHALL use the same tracker and acting-user credential rules, including Jira OAuth gateway routing. Missing personal credentials SHALL NOT silently switch to another account.

#### Scenario: Retry after comment failure
- **GIVEN** an upload succeeded and its identity was retained but commenting failed
- **WHEN** that publication is retried after restart
- **THEN** the existing attachment is reused and the summary is retried without a second upload

#### Scenario: Publication identity collision
- **GIVEN** a retained publication identity
- **WHEN** a caller reuses it with different report content or metadata
- **THEN** the conflicting request is rejected without modifying the retained report

#### Scenario: OAuth write
- **GIVEN** an acting user with Jira OAuth credentials
- **WHEN** a report is published
- **THEN** upload and comment use that user's Jira site gateway and required authorization

### Requirement: Provider and workflow compatibility
Standalone tools and managed results SHALL share report behaviour while preserving existing execution guards and result validation. GitHub and GitLab SHALL keep full Markdown comments and existing splitting rules. Updated generated clarify, specify, implement, adjust, handoff and pickup skills SHALL supply report metadata and meaningful summaries, with no duplicate final clarification comment. Implementation SHALL add a user-visible English changelog entry under Unreleased.

#### Scenario: Managed stage
- **GIVEN** a valid managed stage result with full note and report summary
- **WHEN** Sectile validates and publishes the result
- **THEN** it publishes the same Jira attachment and summary behaviour without an additional agent comment or transition call

#### Scenario: Other providers
- **GIVEN** a GitHub or GitLab task
- **WHEN** a stage report is published
- **THEN** its full Markdown comment publication remains unchanged

#### Scenario: Generated guidance
- **WHEN** the updated issue skills are generated
- **THEN** Jira instructions send full report text and summary through the authorized report contract rather than splitting Markdown into Jira comments
