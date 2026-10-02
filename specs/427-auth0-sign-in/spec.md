# Feature specification: Auth0 browser sign-in

Ticket: [#427](https://github.com/sebastienferry/sectile/issues/427)
Branch: `feat/427-auth0-sign-in`
Clarification: [427.md](../../docs/clarifications/427.md), Round 4.

## Scope

Make the existing Auth0 browser sign-in integration documented and verifiable for a deployment operator. Preserve existing authorization and workstation behavior. No new identity-provider service, machine bearer-token flow, federated logout, automatic account linking or organization-specific access policy is introduced.

## User stories and acceptance criteria

### US1 - Sign in through Auth0 (P1)

- Given a deployment with a valid Auth0 application configuration, when an allowed user completes browser sign-in, then Sectile opens a session for the provider identity and returns the user to a safe local destination.
- Given the same provider identity returns, when sign-in completes again, then Sectile resolves the same account under existing identity rules.
- Given the provider rejects sign-in or the callback state is invalid, expired or reused, when the callback is handled, then no new authenticated session is created.
- Given Auth0 is configured, when someone attempts local e-mail sign-in, then that path cannot grant access.

### US2 - Retain existing access and logout behavior (P1)

- Given a signed-in user, when they sign out, then their Sectile session is revoked. Ending the provider's SSO session is not required.
- Given existing role settings, when Auth0 sign-in completes, then the existing role rules apply; this feature adds no new grants or group policy.
- Given an existing paired workstation, when browser sign-in or logout occurs, then its credential mechanism remains unchanged.
- Given no provider is configured, when a user signs in locally, then existing local sign-in behavior is preserved.

### US3 - Configure without publishing infrastructure details (P1)

- Given the public setup guide, when an operator follows it, then it identifies the required application type, callback registration, configuration variables, secret handling and verification steps using synthetic examples only.
- Given a configured provider cannot be discovered or its metadata is invalid, when Sectile starts, then startup fails instead of silently enabling local sign-in.
- Given any public report, commit, issue or pull request for this work, then it contains no organization-specific infrastructure references, tenant identifiers, deployment addresses or credentials.

## Completion criteria

Automated integration checks cover successful sign-in and meaningful rejection cases. Public setup instructions describe the existing supported behavior accurately. Actual deployment acceptance additionally requires a private successful-login/logout check on the intended environment; automated fixtures are not evidence of a live Auth0 rollout.

No product decisions remain open for this baseline. Deployment-specific inputs and live acceptance are tracked privately.
