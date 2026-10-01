1. Inspect the macro title and high-level framing description.
2. **Evaluate framing completeness**:
   - If the framing description is empty, under 2 sentences, or lacks clear technical boundaries/acceptance criteria, formulate 3 to 5 numbered clarification questions and ask the user directly in this interactive terminal session before generating tasks.
3. **Decompose & Break Down**:
   - Once answered or if framing text is detailed, group action items according to the selected SDD framework:
     - **SpecKit SDD**: Group into User Stories ([US-x]) and Feature Modules ([FEAT-x]).
     - **OpenSpec SDD**: Group into Capabilities ([CAP-x]) and Change Proposals ([CHANGE-x]).
4. Output the generated checklist of actionable todos AND proposed Sectile tickets (Title, IssueType: Story/Task/Bug, Description) for bulk ticket creation.
5. **Save the confirmed todos**:
   - Show the merged list in the order of execution, top first: every existing todo kept with its `id`, the new ones appended or placed where the owner says. Drop an existing todo only when the owner asked for it.
   - Ask the owner to confirm that list in this session. Without an explicit confirmation, save nothing.
   - Once confirmed, call the `update_macro_todos` MCP tool with the project ID, the macro key and the full ordered list: `id` for an existing todo, none for a new one, then `text`, and `done` when it is ticked. The tool replaces the whole list, so a todo left out is removed.
   - If the call is refused, report the refusal and the proposed list as they are, and do not retry through another route.