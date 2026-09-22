# ADR 0016: A merge into main promotes itself to dev, and only to dev

Status: Accepted

## Context

The pipeline on the GitLab mirror publishes `server:<pipeline iid>-<ref slug>`
for every mirrored branch, plus `latest` on `main`. Which of those images dev
actually runs is decided somewhere else entirely: in argocd-sp, one line of
`apps/sectile/dev/values.yml`, `features.main.image.tag`. ArgoCD reads that
repository and nothing else — the pipeline has never deployed anything, and
that separation is worth keeping.

What was missing was the half-step between the two: somebody opening a merge
request in argocd-sp to copy the pipeline number across. That shape does not
fail loudly, it drifts. When this record was written, the pinned tag was
`49-main` while `main` had reached pipeline 61 — twelve pipelines of dev
quietly running something older than the branch it is supposed to show, with
nothing anywhere saying so.

A number that exists in two repositories and is moved between them by hand is
a number that will eventually be wrong, late, or forgotten.

## Decision

**A merge into `main` promotes itself to dev. There is no production
promotion.**

- **The trigger is the merge, not a tag.** Sectile had no release-tag
  convention when this was written; `main` is what the team's board runs, and
  every merge into it
  already builds an image. The `promote:dev` job runs on the default branch
  only, after `publish:image` has pushed the image it will pin.
- **What it promotes is `${SECTILE_VERSION}`**, the exact string the image was
  built and tagged with. Nothing is derived, recomputed or retyped.
- **The mechanism is the shared automerge template**,
  `smartadserver/templates/jobs/argocd` at `6.0.1` — the same one
  platform-portal and kuard use. It rewrites the value, pushes a branch to
  argocd-sp and merges it. The change reaches the cluster as a commit in the
  manifest repository, which is exactly how it reached it before.
- **The `ref` is pinned exactly.** A floating `ref` would let another team's
  change to a shared template alter what this pipeline does to our cluster.
- **`YQ_QUERY` is spelled out**, `.features.main.image.tag`. The meta-chart
  keeps the tag under the feature rather than at the root, so the template's
  default `.image.tag` would silently find nothing.
- **Production is not promoted, by omission and on purpose.** There is no
  production deployment of sectile to promote. Adding one is a decision, not a
  follow-up.

The test environments are untouched. They pin `preview-<commit sha>` through
argocd-sp's `testenv` feature and its pullRequest generator, which reads
GitLab merge requests and never this job.

## Consequences

- **The mirror needs a credential it did not have.** The template requires an
  `AUTOMERGE_TOKEN` variable holding a project access token on argocd-sp, with
  `api`, `read_repository` and `write_repository`, at Maintainer level —
  `main` there is protected for push and merge, so Developer is not enough.
  That is a new secret and a new blast radius: this pipeline can now write to
  the repository that drives a cluster. It is accepted for one narrow reason,
  that the alternative costs a human a correct copy of a number, forever.
  Until the variable exists, `promote:dev` fails on every merge into `main`.
- **Dev stops being a place to pin an older tag by hand.** An edit in
  argocd-sp now lasts until the next merge, which overwrites it. Holding dev
  back is a revert here, not an edit there.
- **A failed promotion is visible where the merge happened**, red on the main
  pipeline, with the image already published — the recovery is retrying the
  job, never rebuilding.
- **The comments in the promoted file survive.** `yq` edits one value in
  place instead of re-encoding the document, and that file carries the reasons
  for the `testenv` naming override and for the extra PersistentVolumeClaim.
- **A deployment failure still does not read as a build failure.** The job
  writes a line; if ArgoCD then fails to sync, this pipeline is green and
  ArgoCD's state is what says what is wrong.

## Alternatives rejected

- **Renovate watching the registry from argocd-sp.** No new secret here, and
  that repository already runs Renovate. Rejected because it inverts the
  source of truth: the cluster would follow whatever shows up in a registry
  rather than what was merged, and it puts the delay at Renovate's schedule
  instead of at the merge.
- **Opening a merge request in argocd-sp instead of merging it.** Rejected
  because a merge request nobody is assigned to is a slower way of forgetting.
  The review that matters already happened, on the pull request being merged.
- **Promoting on a git tag, as platform-portal does.** Rejected because that
  repository releases by tagging and this one does not; a tag convention would
  have to be invented first, and inventing one to justify a deployment trigger
  is the wrong way round. (ADR 0018 has since invented one. The answer here is
  unchanged: dev shows what `main` holds, which is a statement about the branch
  and not about releases.)
- **Keeping the copy manual and adding it to a checklist.** Costs no secret.
  Rejected because the checklist is the part that drifts, and the failure it
  guards against is silent.

## Still open

- **Whether a promotion should be announced.** The template can post a
  Timeline event; it is off by default and left off. Several dev promotions a
  week is noise. The question returns the day production is promoted too.
- **Whether `promote:dev` should wait for anything.** It fires as soon as the
  image exists. There is no smoke test between the push and the promotion,
  because there is nothing yet to run one against.
