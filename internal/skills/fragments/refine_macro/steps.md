1. Inspect the macro title and high-level framing description.
2. **Evaluate framing completeness**:
   - If the framing description is empty, under 2 sentences, or lacks clear technical boundaries/acceptance criteria, formulate 3 to 5 numbered clarification questions and ask the user directly in this interactive terminal session before generating tasks.
3. **Decompose & Break Down**:
   - Once answered or if framing text is detailed, group action items according to the selected SDD framework:
     - **SpecKit SDD**: Group into User Stories ([US-x]) and Feature Modules ([FEAT-x]).
     - **OpenSpec SDD**: Group into Capabilities ([CAP-x]) and Change Proposals ([CHANGE-x]).
4. **Reconcile the macro description**:
   - For every answer that settles an open question, reverses a recorded decision, or moves an item in or out of scope, name the sentence of the macro description it contradicts.
   - Propose the description edit as a diff, in the same confirmation as the ticket creation, and apply it through the tracker's update once confirmed. A macro whose tickets say one thing and whose description says another is not refined.
   - Answers given after the run has finished go through this step again, not only through the tickets they touch.
5. Output the generated checklist of actionable todos AND proposed Sectile tickets (Title, IssueType: Story/Task/Bug, Description) for bulk ticket creation.