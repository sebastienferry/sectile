# Spec #533 - Web i18n: sign-in and account credential/workstation flows

Clarification: `docs/clarifications/533.md` (batch decisions D1 to D9 in
`docs/clarifications/526.md`).

## User stories

### US1 (P1) - A signed-out visitor chooses French or English

- **Given** no remembered language and a French browser, **when** I open the
  sign-in screen, **then** it is in French; with any other browser language
  it is in English.
- **Given** a language used before in this browser, **when** I open the
  sign-in screen, **then** it uses that language, whatever the browser says.
- **Given** the sign-in screen, **when** I pick the other language in its
  French/English switch, **then** the screen switches at once, the choice is
  remembered, and the document language and title follow.
- **Given** I sign in, **when** the application loads, **then** my personal
  language preference applies and becomes the remembered language.

### US2 (P1) - Sign-in texts

- **Given** either language, **when** I read the local or identity-provider
  sign-in, its deployment-mode notices, the sealing passphrase field, the
  buttons and the errors, **then** they are in that language; an error the
  server returns is quoted verbatim, with a translated fallback when it
  returns none.

### US3 (P2) - Account, credentials and workstations

- **Given** French, **when** I open API keys, workstations, the local agent
  setup, the MCP connection and tracker credentials, **then** status, expiry
  ("Valid until", "expiry unknown", expiry warnings), copy feedback ("Copy
  unavailable", "Copy Server URL") and confirmations are French; in English,
  English.
- **Given** any language, **when** I create, check or revoke a credential,
  **then** the behaviour is exactly as before.

## Functional requirements

- FR1 `SignInScreen.tsx` resolves its locale with `resolveInitialLocale()`,
  reads `translations[locale].signIn`, offers a French/English switch,
  calls `rememberLocale` and `applyDocumentLocale` on change.
- FR2 After sign-in, `settings.language` wins and is remembered (#534 T3.2).
- FR3 Every Sectile-owned string of `SignInScreen.tsx`, `SignInStatus.tsx`,
  `ApiKeys.tsx`, `LocalAgentSetup.tsx`, `MCPEngineConfig.tsx`,
  `TrackerCredentialsTab.tsx`, `TrackerCredentialForm.tsx`,
  `ServerTrackerCredentialsPanel.tsx`, `ProfileModal.tsx` and
  `UsersPanel.tsx` comes from the catalog; strings already translated are
  left as they are.
- FR4 `lib/apiKeys.ts` returns structured values or takes its strings as a
  parameter; no English prose inside.
- FR5 Dates use the shared formatters.
- FR6 No authentication, sealing or credential behaviour changes.
