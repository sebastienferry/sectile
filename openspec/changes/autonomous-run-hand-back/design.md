# Design

## Why the worker does not post the transition itself

`123-autonomous-or-interactive-skill-runs` specifies that in autonomous mode "the workflow stage
is posted by the worker without any user confirmation". That is not what is built here, and the
reason is in the evidence this change starts from.

The four autonomous runs on #132 all exited zero. Three of them had done no work at all: they
had been denied every tool and printed a refusal. A worker posting the stage on exit would have
advanced #132 from `implemented` to `reviewed` on a run that explicitly reported it could not
open the PR, could not fetch the review feedback, and had changed nothing. Exit status is not
evidence of work, and the server has nothing else to go on.

So the transition stays where the skill contract already puts it: the run calls
`transition_stage` itself, and `finish_run` when it ends. What was missing is not a server-side
transition but the *check* that the hand-back happened — and, when it did not, saying so
somewhere the user looks. That is what `handBackRun` does.

The check is deliberately narrow:

- Only an autonomous run is judged. An interactive run is handed back by the user closing the
  session, and its stage moves on the user's confirmation.
- Only a skill that owns a stage is judged. A story rewrite or a macro refinement legitimately
  moves nothing, and reporting them as having failed to would be false.
- A run recorded before these columns existed reads as unknown and is judged on nothing.

## Where the chain state lives

`SkillJob.AutoChain` could not work where it was: the job ends as soon as the launch is
dispatched, seconds in, while the run it launched lives for minutes. Whatever decides there is a
next step has to survive to the end of the *run*, so the chain's stop stage is recorded on the
run activity and read back when it finishes. It also survives a server restart, which the job
queue does not.

The continuation is guarded on the row count of the `running → finished` update, so the second,
idempotent `finish_run` a client may send cannot enqueue a second step. And it only continues
when the stage actually changed, which is what stops a run that does nothing from relaunching
the same skill forever.

## The approval flag

`headlessCommandLine` already refuses a provider whose headless invocation is not attested here,
on the grounds that a guessed flag either fails opaquely or is swallowed as prompt text. The
approval flag follows the same rule: `--permission-mode bypassPermissions` is verified against
the `claude` CLI on the machine this was built on, `--auto-approve` is the form this repository's
own README documents for `vibe`, and `codex` gets nothing until someone can check it.

Bypassing approvals is the semantics of an autonomous run, not a loosening of it: the mode
exists to run with nobody watching, on a project that opted into it, in the task's own worktree.
The interactive form is untouched — that is where a human answers.
