#!/usr/bin/env python3
"""Check agent and MCP compatibility between current and released v0.7.6 binaries."""

import json
import os
import pathlib
import shutil
import signal
import socket
import subprocess
import tempfile
import threading
import time
import urllib.error
import urllib.request
import zipfile
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


def run(args, cwd, env=None):
    creationflags = getattr(subprocess, "CREATE_NO_WINDOW", 0) if os.name == "nt" else 0
    result = subprocess.run(
        args, cwd=cwd, env=env, text=True, capture_output=True,
        creationflags=creationflags,
    )
    if result.returncode:
        raise RuntimeError(
            f"Command failed ({result.returncode}): {' '.join(map(str, args))}\n"
            f"{result.stdout}\n{result.stderr}"
        )
    return result.stdout


def isolated_environment(home):
    excluded = {
        "HOME", "USERPROFILE", "APPDATA", "LOCALAPPDATA", "DB_PATH", "PORT",
        "TOKEN", "REMOTE_URL", "DATABASE_URL", "GOOS", "GOARCH", "CGO_ENABLED",
        "GOFLAGS",
    }
    env = {
        key: value
        for key, value in os.environ.items()
        if key not in excluded
        and not key.startswith(("SECTILE_", "AUTH_", "OIDC_", "PG"))
    }
    env.update(
        {
            "HOME": str(home),
            "USERPROFILE": str(home),
            "APPDATA": str(home / "AppData" / "Roaming"),
            "LOCALAPPDATA": str(home / "AppData" / "Local"),
            "XDG_CONFIG_HOME": str(home / ".config"),
            "XDG_CACHE_HOME": str(home / ".cache"),
        }
    )
    return env


def stop(process):
    if process is None or process.poll() is not None:
        return
    try:
        process.send_signal(signal.SIGINT)
        process.wait(timeout=5)
    except Exception:
        process.kill()
        process.wait(timeout=3)


MCP_CLIENT_SOURCE = '''package main
import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "os"
    "time"
    "github.com/modelcontextprotocol/go-sdk/mcp"
)
type tokenTransport struct { token string }
func (t tokenTransport) RoundTrip(r *http.Request) (*http.Response, error) {
    r = r.Clone(r.Context())
    r.Header.Set("Authorization", "Bearer "+t.token)
    return http.DefaultTransport.RoundTrip(r)
}
func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
    defer cancel()
    endpoint := os.Args[1] + "/mcp"
    client := mcp.NewClient(&mcp.Implementation{Name:"sectile-compatibility-check", Version:"1"}, nil)
    session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
        Endpoint:endpoint,
        HTTPClient:&http.Client{Transport:tokenTransport{os.Args[2]}},
    }, nil)
    if err != nil { panic(err) }
    defer session.Close()
    tools, err := session.ListTools(ctx, nil)
    if err != nil { panic(err) }
    found := false
    supportsTaskSpecWorktree := false
    for _, tool := range tools.Tools { if tool.Name == "list_projects" { found = true }; if tool.Name == "prepare_task_spec_worktree" { supportsTaskSpecWorktree = true } }
    if !found { panic("MCP catalog does not contain list_projects") }
    result, err := session.CallTool(ctx, &mcp.CallToolParams{Name:"list_projects", Arguments:map[string]any{}})
    if err != nil { panic(err) }
    if result.IsError { panic("list_projects returned an MCP error") }
    createResult, err := session.CallTool(ctx, &mcp.CallToolParams{Name:"create_task", Arguments:map[string]any{"projectId":os.Args[4], "title":"Cross-version tracker issue", "description":"Created by isolated compatibility test"}})
    if err != nil { panic(err) }
    if createResult.IsError { panic("create_task returned an MCP error") }
    createdRaw, err := json.Marshal(createResult.StructuredContent)
    if err != nil { panic(err) }
    var created struct { Task struct { ID string `json:"id"`; Key string `json:"key"` } `json:"task"` }
    if err := json.Unmarshal(createdRaw, &created); err != nil || created.Task.ID == "" || created.Task.Key != "#42" { panic("create_task did not return the confirmed remote issue") }
    trackerTransition, err := session.CallTool(ctx, &mcp.CallToolParams{Name:"transition_stage", Arguments:map[string]any{"taskKey":created.Task.ID, "stage":"clarified", "note":"Cross-version tracker sync check"}})
    if err != nil { panic(err) }
    if trackerTransition.IsError { panic("tracker-backed transition_stage returned an MCP error") }
    trackerTransitionRaw, err := json.Marshal(trackerTransition.StructuredContent)
    if err != nil { panic(err) }
    var trackerTransitionResult struct { TrackerSync string `json:"trackerSync"`; Task struct { Status string `json:"status"` } `json:"task"` }
    if err := json.Unmarshal(trackerTransitionRaw, &trackerTransitionResult); err != nil { panic(err) }
    syncQueued := trackerTransitionResult.TrackerSync == "queued" || trackerTransitionResult.TrackerSync == "done"
    if !syncQueued || trackerTransitionResult.Task.Status != "clarified" { panic(fmt.Sprintf("tracker-backed transition was not saved and queued: %s", trackerTransitionRaw)) }
    taskID := os.Args[3]
    started, err := session.CallTool(ctx, &mcp.CallToolParams{Name:"start_run", Arguments:map[string]any{"taskKey":taskID, "skill":"compatibility-check"}})
    if err != nil { panic(err) }
    if started.IsError { panic("start_run returned an MCP error") }
    raw, err := json.Marshal(started.StructuredContent)
    if err != nil { panic(err) }
    var activity struct { ID string `json:"id"` }
    if err := json.Unmarshal(raw, &activity); err != nil || activity.ID == "" { panic("start_run returned no activity ID") }
    transition, err := session.CallTool(ctx, &mcp.CallToolParams{Name:"transition_stage", Arguments:map[string]any{"taskKey":taskID, "stage":"clarified", "note":"Cross-version MCP compatibility check"}})
    if err != nil { panic(err) }
    if transition.IsError { panic("transition_stage returned an MCP error") }
    finished, err := session.CallTool(ctx, &mcp.CallToolParams{Name:"finish_run", Arguments:map[string]any{"taskKey":taskID, "runId":activity.ID, "status":"completed", "note":"Compatibility check completed"}})
    if err != nil { panic(err) }
    if finished.IsError { panic("finish_run returned an MCP error") }
    finishedRaw, err := json.Marshal(finished.StructuredContent)
    if err != nil { panic(err) }
    var finishedActivity struct { Status string `json:"status"` }
    if err := json.Unmarshal(finishedRaw, &finishedActivity); err != nil || finishedActivity.Status != "completed" { panic("finish_run did not persist completed status") }
    taskResult, err := session.CallTool(ctx, &mcp.CallToolParams{Name:"get_task", Arguments:map[string]any{"taskKey":taskID}})
    if err != nil { panic(err) }
    if taskResult.IsError { panic("get_task returned an MCP error") }
    taskRaw, err := json.Marshal(taskResult.StructuredContent)
    if err != nil { panic(err) }
    var taskState struct { Task struct { Status string `json:"status"` } `json:"task"` }
    if err := json.Unmarshal(taskRaw, &taskState); err != nil || taskState.Task.Status != "clarified" { panic("transition_stage did not persist clarified status") }
    output := map[string]any{"toolCount":len(tools.Tools), "listProjectsCall":"passed", "trackerCreate":"passed", "trackerTransition":trackerTransitionResult.TrackerSync, "runLifecycle":"passed", "taskStatus":taskState.Task.Status, "runStatus":finishedActivity.Status, "taskSpecWorktreeSupported":supportsTaskSpecWorktree}
    if supportsTaskSpecWorktree {
        specResult, err := session.CallTool(ctx, &mcp.CallToolParams{Name:"prepare_task_spec_worktree", Arguments:map[string]any{"taskKey":taskID}})
        if err != nil { panic(err) }
        if specResult.IsError { panic(fmt.Sprintf("prepare_task_spec_worktree returned an MCP error: %v", specResult.Content[0].(*mcp.TextContent).Text)) }
        specRaw, err := json.Marshal(specResult.StructuredContent)
        if err != nil { panic(err) }
        var specWorkspace struct { Repository string `json:"repository"`; Path string `json:"path"`; Branch string `json:"branch"`; Worktree bool `json:"worktree"`; Distinct bool `json:"distinct"` }
        if err := json.Unmarshal(specRaw, &specWorkspace); err != nil || specWorkspace.Repository != os.Args[5] || specWorkspace.Path == "" || specWorkspace.Branch == "" || !specWorkspace.Worktree || !specWorkspace.Distinct { panic(fmt.Sprintf("prepare_task_spec_worktree returned unexpected result: %s (%v)", specRaw, err)) }
        output["taskSpecWorktree"] = specWorkspace
    }
    if err := json.NewEncoder(os.Stdout).Encode(output); err != nil { panic(fmt.Sprint(err)) }
}
'''


class FakeGitHubHandler(BaseHTTPRequestHandler):
    requests = []

    def respond(self, status, body):
        encoded = json.dumps(body).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(encoded)))
        self.end_headers()
        self.wfile.write(encoded)

    def do_POST(self):
        length = int(self.headers.get("Content-Length", "0"))
        body = self.rfile.read(length).decode() if length else ""
        self.requests.append((self.command, self.path, self.headers.get("Authorization"), body))
        if self.path == "/repos/acme/app/issues":
            payload = json.loads(body)
            return self.respond(201, {
                "number": 42,
                "title": payload["title"],
                "body": payload.get("body", ""),
                "state": "open",
                "html_url": "https://github.com/acme/app/issues/42",
                "labels": [],
            })
        return self.respond(201, {})

    def do_GET(self):
        self.requests.append((self.command, self.path, self.headers.get("Authorization"), ""))
        if self.path.startswith("/repos/acme/app/issues?"):
            return self.respond(200, [{"number": 42, "title": "Cross-version tracker issue", "state": "open", "labels": []}])
        if self.path.endswith("/comments"):
            return self.respond(200, [])
        return self.respond(200, {"number": 42, "title": "Cross-version tracker issue", "state": "open", "labels": []})

    def do_PATCH(self):
        length = int(self.headers.get("Content-Length", "0"))
        body = self.rfile.read(length).decode() if length else ""
        self.requests.append((self.command, self.path, self.headers.get("Authorization"), body))
        return self.respond(200, {})

    def log_message(self, _format, *_args):
        pass


def tracker_evidence(requests, server_version):
    creates = [request for request in requests if request[0] == "POST" and request[1] == "/repos/acme/app/issues"]
    updates = [request for request in requests if request[0] == "PATCH" and request[1] == "/repos/acme/app/issues/42"]
    if len(creates) != 1 or not updates:
        raise RuntimeError(f"{server_version} tracker requests missed issue creation or stage sync: {requests}")
    if any(request[2] != "Bearer compatibility-token" for request in creates + updates):
        raise RuntimeError(f"{server_version} tracker writes did not use the personal token: {creates + updates}")
    return {"server": server_version, "createdIssues": len(creates), "stageUpdates": len(updates), "personalTokenUsed": True}


def agent_operation_evidence(base_url, session, repo_path, version):
    request = urllib.request.Request(
        base_url + "/api/projects/default/skills-status",
        headers={"Cookie": "sectile_session=" + session},
    )
    with urllib.request.urlopen(request, timeout=15) as response:
        if response.status != 200:
            raise RuntimeError(f"{version} agent operation returned HTTP {response.status}")
        status = json.loads(response.read())
    if status.get("repoPath") != str(repo_path) or status.get("isGitRepo") is not True:
        raise RuntimeError(f"{version} agent operation returned unexpected workspace status: {status}")

    git_directory = repo_path / ".git"
    backup_directory = repo_path / ".git.compatibility-backup"
    git_directory.rename(backup_directory)
    try:
        init_request = urllib.request.Request(
            base_url + "/api/projects/default/init-git",
            data=json.dumps({"projectId": "default"}).encode(),
            headers={"Cookie": "sectile_session=" + session, "Content-Type": "application/json"},
            method="POST",
        )
        with urllib.request.urlopen(init_request, timeout=15) as response:
            if response.status != 200:
                raise RuntimeError(f"{version} init-git operation returned HTTP {response.status}")
            initialized = json.loads(response.read())
        if initialized.get("repoPath") != str(repo_path) or initialized.get("isGitRepo") is not True or initialized.get("initialized") is not True:
            raise RuntimeError(f"{version} init-git operation returned unexpected result: {initialized}")
        top_level = run(["git", "rev-parse", "--show-toplevel"], repo_path).strip()
        if pathlib.Path(top_level).resolve() != repo_path.resolve():
            raise RuntimeError(f"{version} init-git initialized an unexpected repository: {top_level}")
    finally:
        if git_directory.exists():
            shutil.rmtree(git_directory)
        backup_directory.rename(git_directory)
    return {
        "agentVersion": version,
        "readOnlyOperation": "skills_status",
        "workspaceVerified": True,
        "mutatingOperation": "init_git",
        "initializedEmptyWorkspace": True,
    }


def configure_issue_spec_folder(home, project_root, spec_root, project_id):
    settings_path = home / ".config" / "sectile" / "settings.json"
    settings_path.parent.mkdir(parents=True, exist_ok=True)
    settings_path.write_text(
        json.dumps({
            "layout": 7,
            "projectSettings": {project_id: {"path": str(project_root), "issueSpecPath": str(spec_root)}},
            "seeded": {"projects": {project_id: "compatibility-test"}},
            "engines": {},
        }),
        encoding="utf-8",
    )


def verify_task_spec_worktree(result, repo_path, server_version):
    workspace = result["taskSpecWorktree"]
    if pathlib.Path(workspace["repository"]).resolve() != repo_path.resolve():
        raise RuntimeError(f"{server_version} task spec worktree used an unexpected repository: {workspace}")
    path = pathlib.Path(workspace["path"])
    if not path.is_dir() or path.resolve() == repo_path.resolve():
        raise RuntimeError(f"{server_version} task spec worktree was not created separately: {workspace}")
    top_level = run(["git", "-C", str(path), "rev-parse", "--show-toplevel"], repo_path).strip()
    branch = run(["git", "-C", str(path), "branch", "--show-current"], repo_path).strip()
    if pathlib.Path(top_level).resolve() != path.resolve() or branch != workspace["branch"]:
        raise RuntimeError(f"{server_version} task spec worktree did not check out its reported branch: {workspace}")
    listed = run(["git", "-C", str(repo_path), "worktree", "list", "--porcelain"], repo_path)
    if f"worktree {path}" not in listed:
        raise RuntimeError(f"{server_version} task spec worktree is not registered with its Git repository: {workspace}")
    return {"server": server_version, "created": True, "branchCheckedOut": branch, "distinct": True}


def main():
    repo = pathlib.Path(__file__).resolve().parents[2]
    work = pathlib.Path(tempfile.mkdtemp(prefix="sectile-current-agent-v076-server-"))
    old_source = work / "old-source"
    home_server = work / "server-home"
    home_agent = work / "agent-home"
    agent_root = work / "agent-repository"
    issue_spec_root = work / "issue-specifications"
    v076_agent_home = work / "v076-agent-home"
    for path in (old_source, home_server, home_agent, issue_spec_root, v076_agent_home):
        path.mkdir(parents=True, exist_ok=True)
    agent_root.mkdir()
    FakeGitHubHandler.requests = []
    fake_tracker = ThreadingHTTPServer(("127.0.0.1", 0), FakeGitHubHandler)
    fake_tracker_thread = threading.Thread(target=fake_tracker.serve_forever, daemon=True)
    fake_tracker_thread.start()
    fake_tracker_url = f"http://127.0.0.1:{fake_tracker.server_port}"

    server_log_path = work / "server.log"
    agent_log_path = work / "agent.log"
    server_log = None
    agent_log = None
    v076_agent_log_handle = None
    server_process = None
    agent_process = None
    try:
        for git_root, description in (
            (agent_root, "Temporary code repository"),
            (issue_spec_root, "Temporary issue specifications"),
        ):
            run(["git", "init", "-q", "-b", "main", str(git_root)], repo)
            run(["git", "-C", str(git_root), "config", "user.name", "Compatibility Test"], repo)
            run(["git", "-C", str(git_root), "config", "user.email", "compatibility@example.invalid"], repo)
            (git_root / "README.md").write_text(f"# {description}\n", encoding="utf-8")
            run(["git", "-C", str(git_root), "add", "README.md"], repo)
            run(["git", "-C", str(git_root), "commit", "-q", "-m", "Initialize temporary test repository"], repo)
        archive = work / "v076.zip"
        run(
            ["git", "archive", "--format=zip", f"--output={archive}", "v0.7.6"],
            repo,
        )
        with zipfile.ZipFile(archive) as source_archive:
            source_archive.extractall(old_source)

        seed_dir = old_source / "cmd" / "compat-seed"
        seed_dir.mkdir(parents=True)
        (seed_dir / "main.go").write_text(
            '''package main
import ("encoding/json"; "os"; "tasks/internal/db"; "tasks/internal/models")
func main() {
 d, err := db.Open(db.SQLiteConfig(os.Getenv("DB_PATH"))); if err != nil { panic(err) }; defer d.Close()
 if err = d.EnsureUser(db.ImplicitUserID); err != nil { panic(err) }
 project, err := d.CreateProject(models.CreateProjectRequest{Name:"Compatibility", IssueTracker:"local"}); if err != nil { panic(err) }
 task, err := d.CreateTask(models.CreateTaskRequest{ProjectID:project.ID, Title:"Cross-version MCP run", Status:models.StatusToClarify}); if err != nil { panic(err) }
 githubProject, err := d.CreateProject(models.CreateProjectRequest{Name:"GitHub compatibility", IssueTracker:"github", GithubRepo:"acme/app", GithubApiUrl:os.Getenv("COMPAT_GITHUB_API_URL")}); if err != nil { panic(err) }
 if err = d.SetUserTrackerCredential(db.ImplicitUserID, "github", "", "", "compatibility-token", ""); err != nil { panic(err) }
 key, _, err := d.CreateAPIKey(db.ImplicitUserID, "binary-compatibility-test", db.DefaultAPIKeyTTL); if err != nil { panic(err) }
 session, _, err := d.CreateWebSession(db.ImplicitUserID); if err != nil { panic(err) }
 if err = json.NewEncoder(os.Stdout).Encode(map[string]string{"apiKey":key,"session":session,"projectID":project.ID,"taskID":task.ID,"githubProjectID":githubProject.ID}); err != nil { panic(err) }
}
''',
            encoding="utf-8",
        )

        old_server = work / "sectile-server-v0.7.6"
        old_agent = work / "sectile-agent-v0.7.6"
        current_server = work / "sectile-server-current"
        seed_binary = work / "compat-seed"
        current_agent = work / "sectile-agent-current"
        old_mcp_client = work / "mcp-client-v0.7.6"
        current_mcp_client = work / "mcp-client-current"
        run(
            [
                "go", "build", "-trimpath", "-ldflags",
                "-X tasks/internal/version.Version=v0.7.6", "-o", str(old_server),
                "./cmd/server",
            ],
            old_source,
        )
        run(
            [
                "go", "build", "-trimpath", "-ldflags",
                "-X tasks/internal/version.Version=v0.7.6", "-o", str(old_agent),
                "./cmd/agent",
            ],
            old_source,
        )
        run(["go", "build", "-trimpath", "-o", str(seed_binary), "./cmd/compat-seed"], old_source)
        run(["go", "build", "-trimpath", "-o", str(current_agent), "./cmd/agent"], repo)
        run(
            ["go", "build", "-trimpath", "-ldflags", "-X tasks/internal/version.Version=dev", "-o", str(current_server), "./cmd/server"],
            repo,
        )
        old_client_source = old_source / "cmd" / "compat-mcp-client"
        old_client_source.mkdir(parents=True)
        (old_client_source / "main.go").write_text(MCP_CLIENT_SOURCE, encoding="utf-8")
        current_client_source = repo / "cmd" / "compat-mcp-client-temp"
        current_client_source.mkdir()
        (current_client_source / "main.go").write_text(MCP_CLIENT_SOURCE, encoding="utf-8")
        try:
            run(["go", "build", "-trimpath", "-o", str(old_mcp_client), "./cmd/compat-mcp-client"], old_source)
            run(["go", "build", "-trimpath", "-o", str(current_mcp_client), "./cmd/compat-mcp-client-temp"], repo)
        finally:
            shutil.rmtree(current_client_source, ignore_errors=True)

        database_path = work / "tasks.db"
        seed_env = isolated_environment(home_server)
        seed_env["DB_PATH"] = str(database_path)
        seed_env["COMPAT_GITHUB_API_URL"] = fake_tracker_url
        credentials = json.loads(run([str(seed_binary)], old_source, seed_env))
        configure_issue_spec_folder(home_agent, agent_root, issue_spec_root, credentials["projectID"])
        configure_issue_spec_folder(v076_agent_home, agent_root, issue_spec_root, credentials["projectID"])

        port_socket = socket.socket()
        port_socket.bind(("127.0.0.1", 0))
        port = port_socket.getsockname()[1]
        port_socket.close()

        server_env = isolated_environment(home_server)
        server_env["DB_PATH"] = str(database_path)
        server_env["PORT"] = str(port)
        server_log = server_log_path.open("w", encoding="utf-8")
        creationflags = getattr(subprocess, "CREATE_NO_WINDOW", 0) if os.name == "nt" else 0
        server_process = subprocess.Popen(
            [str(old_server)], cwd=work, env=server_env, stdin=subprocess.DEVNULL,
            stdout=server_log, stderr=subprocess.STDOUT, creationflags=creationflags,
        )

        base_url = f"http://127.0.0.1:{port}"
        for _ in range(150):
            if server_process.poll() is not None:
                raise RuntimeError("The v0.7.6 server exited during startup.")
            try:
                with urllib.request.urlopen(base_url + "/api/health", timeout=0.5) as response:
                    if response.status == 200:
                        break
            except Exception:
                pass
            time.sleep(0.1)
        else:
            raise RuntimeError("The v0.7.6 server did not become healthy.")

        agent_env = isolated_environment(home_agent)
        agent_log = agent_log_path.open("w", encoding="utf-8")
        agent_process = subprocess.Popen(
            [
                str(current_agent), "--url", base_url, "--token", credentials["apiKey"],
                "--project", "default", "--device", "current-agent-integration",
                "--desktop-info", str(home_agent / "agent-connection.json"),
                "--repo", str(agent_root),
            ],
            cwd=work, env=agent_env, stdin=subprocess.DEVNULL, stdout=agent_log,
            stderr=subprocess.STDOUT, creationflags=creationflags,
        )

        request = urllib.request.Request(
            base_url + "/api/agent/status",
            headers={"Cookie": "sectile_session=" + credentials["session"]},
        )
        for _ in range(200):
            if agent_process.poll() is not None:
                raise RuntimeError("The current agent exited before connecting.")
            try:
                with urllib.request.urlopen(request, timeout=1) as response:
                    status = json.loads(response.read())
            except Exception:
                status = {}
            connected = next(
                (
                    agent for agent in status.get("agents", [])
                    if agent.get("deviceId") == "current-agent-integration"
                ),
                None,
            )
            if connected:
                operations = connected.get("operations", [])
                if not connected.get("agentVersion") or not operations or connected.get("outdated") is not False:
                    raise RuntimeError(f"Unexpected agent status: {connected}")
                reverse_result = {
                    "serverVersion": "v0.7.6",
                    "agentVersion": connected["agentVersion"],
                    "operationCount": len(operations),
                    "outdated": connected["outdated"],
                    "connected": True,
                }
                break
            time.sleep(0.1)
        else:
            raise RuntimeError("The v0.7.6 server did not register the current agent.")

        old_mcp = json.loads(run([
            str(current_mcp_client), base_url, credentials["apiKey"],
            credentials["taskID"], credentials["githubProjectID"], str(issue_spec_root),
        ], work))
        old_mcp["client"] = "current"
        old_mcp["server"] = "v0.7.6"
        old_agent_operation = agent_operation_evidence(base_url, credentials["session"], agent_root, "current")

        stop(agent_process)
        agent_process = None
        stop(server_process)
        server_process = None
        old_tracker_evidence = tracker_evidence(list(FakeGitHubHandler.requests), "v0.7.6")
        FakeGitHubHandler.requests.clear()
        agent_log.close()
        agent_log = None
        server_log.close()
        server_log = None

        current_home = work / "current-server-home"
        current_home.mkdir()
        current_db = work / "current-tasks.db"
        current_seed_dir = repo / "cmd" / "compat-seed-temp"
        current_seed_dir.mkdir()
        (current_seed_dir / "main.go").write_text(
            '''package main
import ("encoding/json"; "os"; "tasks/internal/db"; "tasks/internal/models")
func main() { d, err := db.Open(db.SQLiteConfig(os.Getenv("DB_PATH"))); if err != nil { panic(err) }; defer d.Close(); if err = d.EnsureUser(db.ImplicitUserID); err != nil { panic(err) }; project, err := d.CreateProject(models.CreateProjectRequest{Name:"Compatibility", IssueTracker:"local"}); if err != nil { panic(err) }; task, err := d.CreateTask(models.CreateTaskRequest{ProjectID:project.ID, Title:"Cross-version MCP run", Status:models.StatusToClarify}); if err != nil { panic(err) }; githubProject, err := d.CreateProject(models.CreateProjectRequest{Name:"GitHub compatibility", IssueTracker:"github", GithubRepo:"acme/app", GithubApiUrl:os.Getenv("COMPAT_GITHUB_API_URL")}); if err != nil { panic(err) }; if err = d.SetUserTrackerCredential(db.ImplicitUserID, "github", "", "", "compatibility-token", ""); err != nil { panic(err) }; key, _, err := d.CreateAPIKey(db.ImplicitUserID, "binary-compatibility-test", db.DefaultAPIKeyTTL); if err != nil { panic(err) }; session, _, err := d.CreateWebSession(db.ImplicitUserID); if err != nil { panic(err) }; if err = json.NewEncoder(os.Stdout).Encode(map[string]string{"apiKey":key,"session":session,"projectID":project.ID,"taskID":task.ID,"githubProjectID":githubProject.ID}); err != nil { panic(err) } }
''', encoding="utf-8")
        try:
            current_seed_binary = work / "compat-seed-current"
            run(["go", "build", "-trimpath", "-o", str(current_seed_binary), "./cmd/compat-seed-temp"], repo)
        finally:
            shutil.rmtree(current_seed_dir, ignore_errors=True)
        current_env = isolated_environment(current_home)
        current_env["DB_PATH"] = str(current_db)
        current_env["COMPAT_GITHUB_API_URL"] = fake_tracker_url
        current_credentials = json.loads(run([str(current_seed_binary)], repo, current_env))
        configure_issue_spec_folder(v076_agent_home, agent_root, issue_spec_root, current_credentials["projectID"])
        current_port_socket = socket.socket()
        current_port_socket.bind(("127.0.0.1", 0))
        current_port = current_port_socket.getsockname()[1]
        current_port_socket.close()
        current_env["PORT"] = str(current_port)
        server_log = server_log_path.open("w", encoding="utf-8")
        server_process = subprocess.Popen(
            [str(current_server)], cwd=work, env=current_env, stdin=subprocess.DEVNULL,
            stdout=server_log, stderr=subprocess.STDOUT, creationflags=creationflags,
        )
        current_url = f"http://127.0.0.1:{current_port}"
        for _ in range(150):
            if server_process.poll() is not None:
                raise RuntimeError("The current Go server exited during startup.")
            try:
                with urllib.request.urlopen(current_url + "/api/health", timeout=0.5) as response:
                    if response.status == 200:
                        break
            except Exception:
                pass
            time.sleep(0.1)
        else:
            raise RuntimeError("The current Go server did not become healthy.")
        v076_agent_log = work / "v076-agent.log"
        v076_agent_log_handle = v076_agent_log.open("w", encoding="utf-8")
        agent_process = subprocess.Popen(
            [
                str(old_agent), "--url", current_url, "--token", current_credentials["apiKey"],
                "--project", "default", "--device", "v076-agent-integration",
                "--desktop-info", str(v076_agent_home / "agent-connection.json"),
                "--repo", str(agent_root),
            ],
            cwd=work, env=isolated_environment(v076_agent_home), stdin=subprocess.DEVNULL,
            stdout=v076_agent_log_handle, stderr=subprocess.STDOUT, creationflags=creationflags,
        )
        for _ in range(200):
            if agent_process.poll() is not None:
                raise RuntimeError("The v0.7.6 agent exited before connecting to the current server.")
            status_request = urllib.request.Request(
                current_url + "/api/agent/status",
                headers={"Cookie": "sectile_session=" + current_credentials["session"]},
            )
            try:
                with urllib.request.urlopen(status_request, timeout=1) as response:
                    status = json.loads(response.read())
            except Exception:
                status = {}
            if any(item.get("deviceId") == "v076-agent-integration" for item in status.get("agents", [])):
                break
            time.sleep(0.1)
        else:
            raise RuntimeError("The current server did not register the v0.7.6 agent.")
        current_mcp_result = json.loads(
            run([
                str(old_mcp_client), current_url, current_credentials["apiKey"],
                current_credentials["taskID"], current_credentials["githubProjectID"], str(issue_spec_root),
            ], work)
        )
        current_mcp_result["client"] = "v0.7.6"
        current_mcp_result["server"] = "current"
        task_spec_worktrees = [{"server": "v0.7.6", "supported": old_mcp["taskSpecWorktreeSupported"]}]
        if current_mcp_result["taskSpecWorktreeSupported"]:
            task_spec_worktrees.append(verify_task_spec_worktree(current_mcp_result, issue_spec_root, "current"))
        v076_agent_operation = agent_operation_evidence(current_url, current_credentials["session"], agent_root, "v0.7.6")
        stop(agent_process)
        agent_process = None
        v076_agent_log_handle.close()
        v076_agent_log_handle = None
        stop(server_process)
        server_process = None
        current_tracker_evidence = tracker_evidence(list(FakeGitHubHandler.requests), "current")
        server_log.close()
        server_log = None

        legacy_test_env = isolated_environment(home_agent)
        legacy_test_env["SECTILE_V076_AGENT_BINARY"] = str(old_agent)
        run(
            [
                "go", "test", "./internal/handlers", "-run",
                "^TestV076AgentBinaryConnectsToCurrentServer$", "-count=1",
            ],
            repo,
            legacy_test_env,
        )
        print(
            json.dumps(
                {
                    "currentAgentToV076Server": reverse_result,
                    "v076AgentToCurrentServer": {"connected": True},
                    "mcpCompatibility": [old_mcp, current_mcp_result],
                    "trackerWrites": [old_tracker_evidence, current_tracker_evidence],
                    "agentOperations": [old_agent_operation, v076_agent_operation],
                    "taskSpecWorktrees": task_spec_worktrees,
                }
            )
        )
    except Exception:
        for log in (agent_log, server_log):
            if log:
                log.flush()
        for path in (agent_log_path, server_log_path, work / "v076-agent.log"):
            if path.exists():
                print(f"--- {path.name} ---")
                print(path.read_text(encoding="utf-8", errors="replace"))
        raise
    finally:
        stop(agent_process)
        stop(server_process)
        if agent_log:
            agent_log.close()
        if v076_agent_log_handle:
            v076_agent_log_handle.close()
        if server_log:
            server_log.close()
        fake_tracker.shutdown()
        fake_tracker.server_close()
        fake_tracker_thread.join(timeout=3)
        shutil.rmtree(work, ignore_errors=True)


if __name__ == "__main__":
    main()
