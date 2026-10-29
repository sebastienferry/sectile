#!/usr/bin/env python3
"""Check a current Go agent against the released v0.7.6 server binary."""

import json
import os
import pathlib
import shutil
import signal
import socket
import subprocess
import tempfile
import time
import urllib.error
import urllib.request
import zipfile


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


def main():
    repo = pathlib.Path(__file__).resolve().parents[2]
    work = pathlib.Path(tempfile.mkdtemp(prefix="sectile-current-agent-v076-server-"))
    old_source = work / "old-source"
    home_server = work / "server-home"
    home_agent = work / "agent-home"
    for path in (old_source, home_server, home_agent):
        path.mkdir(parents=True, exist_ok=True)

    server_log_path = work / "server.log"
    agent_log_path = work / "agent.log"
    server_log = None
    agent_log = None
    server_process = None
    agent_process = None
    try:
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
import ("encoding/json"; "os"; "tasks/internal/db")
func main() {
 d, err := db.Open(db.SQLiteConfig(os.Getenv("DB_PATH"))); if err != nil { panic(err) }; defer d.Close()
 if err = d.EnsureUser(db.ImplicitUserID); err != nil { panic(err) }
 key, _, err := d.CreateAPIKey(db.ImplicitUserID, "binary-compatibility-test", db.DefaultAPIKeyTTL); if err != nil { panic(err) }
 session, _, err := d.CreateWebSession(db.ImplicitUserID); if err != nil { panic(err) }
 if err = json.NewEncoder(os.Stdout).Encode(map[string]string{"apiKey":key,"session":session}); err != nil { panic(err) }
}
''',
            encoding="utf-8",
        )

        old_server = work / "sectile-server-v0.7.6"
        old_agent = work / "sectile-agent-v0.7.6"
        seed_binary = work / "compat-seed"
        current_agent = work / "sectile-agent-current"
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

        database_path = work / "tasks.db"
        seed_env = isolated_environment(home_server)
        seed_env["DB_PATH"] = str(database_path)
        credentials = json.loads(run([str(seed_binary)], old_source, seed_env))

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
                "--repo", str(work),
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

        stop(agent_process)
        agent_process = None
        stop(server_process)
        server_process = None
        agent_log.close()
        agent_log = None
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
                }
            )
        )
    except Exception:
        for log in (agent_log, server_log):
            if log:
                log.flush()
        for path in (agent_log_path, server_log_path):
            if path.exists():
                print(f"--- {path.name} ---")
                print(path.read_text(encoding="utf-8", errors="replace"))
        raise
    finally:
        stop(agent_process)
        stop(server_process)
        if agent_log:
            agent_log.close()
        if server_log:
            server_log.close()
        shutil.rmtree(work, ignore_errors=True)


if __name__ == "__main__":
    main()
