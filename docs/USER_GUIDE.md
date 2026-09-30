# Sectile user guide

Sectile brings tracker tickets, coding agents, local worktrees, and pull requests into one workflow. This guide takes you from your first browser sign-in to a pull request ready for human review. It assumes that your team already runs a Sectile server and that you can reach it from a workstation with your code repository and coding CLI.

## Contents

1. [Connect to the web interface](#connect-to-the-web-interface)
2. [Set up a personal Jira token](#set-up-a-personal-jira-token)
3. [Use an existing project](#use-an-existing-project)
4. [Add a new project](#add-a-new-project)
5. [Pair and set up your workstation](#pair-and-set-up-your-workstation)
6. [Configure the project in Desktop](#configure-the-project-in-desktop)
7. [Use a prompt in Claude Code](#use-a-prompt-in-claude-code)
8. [Run the full workflow autonomously](#run-the-full-workflow-autonomously)
9. [Follow work in Desktop](#follow-work-in-desktop)

## Connect to the web interface

1. Open the URL your team uses for Sectile. For local development, the [quick start](../README.md#quick-start) uses `http://localhost:8090`.
2. On **Sign in to Sectile** (*Se connecter à Sectile*), choose the identity provider if the deployment offers one. A deployment using local sign-in asks for your email address instead. Its optional sealing passphrase unlocks your own sealed tracker tokens; it is not a login password.
3. Select your project in the sidebar. The board and backlog show its tickets; the project picker can also show all projects together. Open a ticket to read its description, workflow stage, activity, and pull request link.

If your account or a project is missing, ask the Sectile administrator or project owner for access. The [root README](../README.md) describes how to start a local server for development.

## Set up a personal Jira token

A Jira project needs your own Jira access for actions attributed to you. Set it in the **Profile → Tracker credentials** (*Profil → Identifiants Trackers*) area of the web interface:

1. Select **Jira** and enter your Jira site URL, Atlassian account email, and API token. Create the token in your Atlassian account's API tokens section. The site and email belong to the same account as the token.
2. Select **Verify** (*Vérifier*), then **Save** (*Enregistrer*). The form enables saving after the site accepts the credentials.
3. Optionally select **Seal my tokens** (*Sceller mes jetons*) and set one master sealing passphrase for your personal tracker tokens. Keep it somewhere you can retrieve it. At a later sign-in, enter it on the sign-in screen or select **Unlock all tokens** (*Déverrouiller tous les jetons*) in the profile.

A sealed and locked personal token cannot authorize your task writes. Sectile reports the refusal instead of silently using another account. The same holds when you have no personal token at all: the error notification then offers to add it, and opens this area on the tracker concerned. A server credential, when configured by an administrator, is for unattended synchronization; it does not replace your personal credential for actions you cause. Never paste an API token or sealing passphrase into a ticket, coding prompt, or Desktop project setting. See [credential ownership](adrs/0029-server-tracker-credential-signs-unattended-work-only.md) for the full rule.

## Use an existing project

1. Select the project from the sidebar's project picker. Use its board, backlog, search, or saved views to find a ticket.
2. Open the ticket to check its tracker link, description, stage, and any current pull request. A task's stage tells you which workflow step comes next.
3. For local execution, connect a workstation and map the project to a local repository as described below. A project visible in the browser does not by itself give the agent a folder to work in.

If the project uses Jira, its **Tracker** settings identify the Jira project key. Your personal Jira site, email, and token remain in your profile. The project can also list several Git repositories; each repository you change may need its own pull request.

## Add a new project

1. In the web project's picker, choose **New project…** (*Nouveau projet...*).
2. On **General** (*Général*), enter a project title and optional description. Set a default project only if that is how you want the picker to open.
3. On **Tracker**, choose the issue tracker. For Jira, enter its project key; for GitHub or GitLab, enter the repository or project identity. Choose local storage for a project without a remote tracker. Complete the Git remote URL and any other repository identities the project uses.
4. Review **Agentic workflow** and **Skills & SDD**, then select **Create project** (*Créer le projet*). Configure any required tracker credential in your profile and synchronize the project to import remote tickets.
5. In Desktop, add the server project and select its local repository folder. Server project metadata and local folder paths are separate settings.

Keep Git remote identities consistent with the repositories on your workstation.

## Pair and set up your workstation

Pairing lets your local agent act as you without putting a long-lived key in a prompt.

1. In the web profile, open **Workstations & Agent** and select **Pair a workstation** (*Appairer une machine*). Copy the temporary, single-use code.
2. Install [Sectile Desktop](../desktop/README.md#install-a-release) on the workstation, open it, enter the server URL, and paste the code on its connection screen. Desktop starts its bundled agent. **Settings → Agent connection** shows whether the agent and server are connected and offers Start, Stop, and Restart controls.
3. To run the agent without Desktop, use the binary on that workstation:

   ```sh
   sectile-agent pair --url https://sectile.example.com --code '<pairing-code>'
   sectile-agent --url https://sectile.example.com --project '<project-id>' --repo /path/to/clone
   ```

   Replace the example URL, project ID, and path with your own values. Pairing stores the workstation credential for later starts. The [root README](../README.md#connect-a-workstation) shows the local-server form of these commands.

4. Confirm the workstation appears in the web profile and that Desktop reports **Connected**. If the agent cannot start a task, check **Settings → Agent logs** and the project's local folder mapping.

## Configure the project in Desktop

1. Select **Add project** from Desktop's project sidebar, or use the project configuration view for one already shown. Choose the local Git checkout with **Choose folder…**.
2. Open the project's **General** category to inspect its Git remote, SDD framework, and default coding engine. In **Folders**, map local repositories and, when needed, a specifications folder or attached folders. These paths stay on your workstation.
3. In **Execution**, choose whether tasks use worktrees, how many executions can run, and the terminal behavior. Use workstation **Execution defaults** for settings shared by projects; project overrides can inherit those defaults.
4. Select **Save local configuration**. In **Settings → Deployment**, install the project's skills and initialize its chosen SDD framework when those tools are not yet present. In **AI engines**, choose or configure the CLI you intend to run. The CLI must also be installed and signed in on the workstation.

The [Desktop guide](../desktop/README.md#user-configuration-and-commands) covers the full set of controls. The browser owns shared project and tracker settings; Desktop owns this workstation's paths, engine commands, and execution preferences.

## Use a prompt in Claude Code

You can run a workflow skill in an existing Claude Code session rather than launching it from a ticket:

1. Configure Claude Code's Sectile MCP connection in Desktop under **Settings → Execution defaults → MCP configuration**. Choose the transport appropriate to your setup, select **Update provider configuration**, and restart Claude Code. Desktop can also deploy the server's skills under **Settings → Deployment**. See [MCP connections](../desktop/README.md#mcp-connections).
2. Open Claude Code in the ticket's repository on the paired workstation. Open the ticket in the web interface and use **Copy** for the next workflow skill or **Copy** for `/pickup-issue`; on the board, the copy button of a full card copies the next workflow skill's prompt in one click. Paste the complete copied prompt into Claude Code. It includes the task's full ID and instructions to read Sectile MCP context and report the run.
3. Follow the conversation and task activity. Answer a clarification question if the skill asks one. For a single step, launch the next stage after Sectile records the prior stage. For the pickup prompt, the skill continues through the stages it can complete and stops before merge.

A free-form Claude Code prompt is useful for discussion or exploration, but it does not by itself record a Sectile stage transition. Use the copied workflow prompt or the task's configured skill action for tracked workflow work.

## Run the full workflow autonomously

For a configured project with an available workstation agent and coding engine:

1. Open the ticket on the board. Check that its description is sufficient, the repository mapping is correct, and your personal tracker credential is available for attributed writes.
2. On the task card choose **Full chain** (*Chaîne complète*). Sectile starts the pickup workflow from the ticket's current stage in autonomous mode.
3. Follow the activity in the browser or the execution in Desktop. The chain clarifies, specifies, implements, tests, and adjusts the pull request as its stage contracts allow. It may stop for an essential owner decision or a failed check; resolve that cause and resume from the recorded stage.
4. Open the linked pull request when the task reaches review. A person reviews and merges it; handoff follows the merge.

**Advance autonomously** (*Avancer en autonome*) on a card runs only the next step. Choose **Full chain** when you want the entire pickup sequence. The available chain starts at the ticket's current stage, so a previously specified ticket does not repeat clarification. Once the ticket has reached the stage where the project's full chain stops, the card no longer offers **Full chain**: a full card shows a robot button in its place, which runs the next step autonomously, and a condensed card's menu keeps **Advance autonomously**.

## Follow work in Desktop

Desktop lists your configured projects and local executions. Use **Open tasks** on a project or **Tasks list** in the command palette to find an existing ticket; selecting it does not launch anything. Launch a task skill or full pickup from its available actions. **Quick add task** creates a tracker task first and offers a separate launch action.

Select an execution to see its console, status, and recorded skill result. **Changes** shows the current local worktree diff; **Console** returns to output. A project queue shows running and waiting executions. The toolbar can stop or relaunch an execution, and **Next: Clarify**, **Next: Specify**, **Next: Implement**, or **Next: Adjust** advances one verified step. **Awaiting human merge** means the pull request is ready for its owner to review. Closing Desktop leaves the local agent and its running work active; reopen Desktop to reconnect.

See the [Desktop guide](../desktop/README.md#use) for installation, settings, console behavior, and recovery details.
