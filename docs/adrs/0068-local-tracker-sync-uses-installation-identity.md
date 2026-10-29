# ADR 0068: Local tracker synchronization uses the installation identity

- Date: 2026-10-29
- Status: Accepted and implemented

## Context

Fleet synchronizes remote trackers with a provider-level server credential,
including when a person starts the pass manually (ADR 0028). A Local companion
has one persistent installation identity and no separate server service
account. Its native setup lets that identity connect Jira through OAuth or
store personal API tokens. With the Fleet rule applied unchanged, Local sync
fails for a connected user unless they configure a second server credential.

## Decision

After the authenticated Local execution agent connects, the Local backend
selects the installation user as its synchronization identity. Both manual and
scheduled tracker reads resolve that user's credential. The identity cannot
change while the database is open. The activity still records the person who
requested a manual pass, or no requester for a scheduled pass.

Fleet never selects a Local synchronization identity. It continues to use its
provider-level server credential and marks synchronization as unattended.
Tracker writes initiated by a user retain their existing per-user authorization.

## Consequences

A Local installation can synchronize with its Jira OAuth grant or personal API
token without duplicating a server credential. An expired, revoked, locked or
missing personal credential causes a visible synchronization failure. The
desktop follows the resulting activity and shows its actual outcome.

Fleet credential behavior is unchanged. Local and Fleet do not share databases,
and this rule does not make personal credentials available to a Fleet service.
