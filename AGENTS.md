<!-- taskflow:project-context:start -->
## TaskFlow workflow

TaskFlow operates the development workflow for this repository. Use `.taskflow/config.json` as the source of truth for the project and remote tracker context.

- Tracker: `github`
- GitHub repository: `sebastienferry/taskflow`
- Git remote: `git@github.com:sebastienferry/taskflow.git`

Development work follows TaskFlow's stages: clarify, specify, implement, review and pull request, then human merge and handoff. Keep the assigned branch/worktree, use TaskFlow's local stage handler for standalone runs, and let managed TaskFlow runs own stage transitions and tracker synchronization.
<!-- taskflow:project-context:end -->
