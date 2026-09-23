1. Take the inputs from the calling skill, or from the user when invoked directly: the full task key, the completed stage (if any), the report note, the actual branch, the pull request URL (if any) and the comment to post (if any).
2. Resolve the task and its project as described in "Sectile task access" below, and check the task's current stage against the stage to record.
3. In a managed run, write the result through the supplied contract and stop there.
4. Standalone, record the stage with `transition_stage`, post the comment with `add_comment`, and check each tool result before going on.
5. When invoked directly to record a stage by hand, do not start a run: a transition or a comment is not a run. The run rules below apply to the skill that does the work.