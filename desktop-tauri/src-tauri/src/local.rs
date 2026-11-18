use reqwest::{Client, Method};
use serde::{Deserialize, Serialize};
use serde_json::{Value, json};
use std::{
    fs,
    io::{Read, Seek, SeekFrom, Write},
    net::TcpListener,
    path::{Path, PathBuf},
    process::{Command, Stdio},
    time::Duration,
};
use tauri::Manager;

#[derive(Clone, Copy, Debug, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum RuntimeMode {
    Local,
    Fleet,
}

#[derive(Default)]
pub struct LocalRuntime(
    pub tokio::sync::Mutex<()>,
    pub std::sync::atomic::AtomicBool,
    pub tokio::sync::Mutex<()>,
    pub std::sync::atomic::AtomicBool,
);

pub fn invalidate(app: &tauri::AppHandle) {
    app.state::<LocalRuntime>()
        .1
        .store(false, std::sync::atomic::Ordering::Release);
}

#[derive(Serialize, Deserialize)]
struct Installation {
    port: u16,
    key: String,
    #[serde(default)]
    token: String,
    #[serde(default)]
    device_id: String,
    #[serde(default)]
    agent_device_id: String,
    #[serde(default)]
    user_id: String,
    #[serde(default)]
    email: String,
}

#[derive(Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct FleetCredentials {
    server: String,
    #[serde(rename = "apiKey")]
    api_key: String,
    #[serde(rename = "deviceId")]
    device_id: String,
}

#[derive(Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct FleetRuntime {
    server: String,
    desktop_token: String,
    user_id: String,
}

struct FleetPairing {
    api_token: String,
    device_id: String,
    user_id: String,
}
impl Installation {
    fn url(&self) -> String {
        format!("http://127.0.0.1:{}", self.port)
    }
}

fn root(app: &tauri::AppHandle) -> Result<PathBuf, String> {
    app.path()
        .app_data_dir()
        .map(|p| p.join("local-runtime"))
        .map_err(|_| "Data directory unavailable".into())
}

const MAX_SERVICE_LOG_BYTES: u64 = 256 * 1024;

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ServiceLog {
    path: String,
    text: String,
    missing: bool,
    truncated: bool,
}

#[derive(Clone, Copy, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum ServiceLogKind {
    Companion,
    Agent,
}

fn read_service_log_file(path: &Path) -> Result<ServiceLog, String> {
    let display_path = path.to_string_lossy().into_owned();
    let metadata = match fs::symlink_metadata(path) {
        Ok(metadata) => metadata,
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => {
            return Ok(ServiceLog {
                path: display_path,
                text: String::new(),
                missing: true,
                truncated: false,
            });
        }
        Err(_) => return Err("Service log unreadable".into()),
    };
    if !metadata.file_type().is_file() {
        return Err("Service log is not a regular file".into());
    }
    let mut file = fs::File::open(path).map_err(|_| "Service log unreadable")?;
    let length = file
        .metadata()
        .map_err(|_| "Service log unreadable")?
        .len();
    let start = length.saturating_sub(MAX_SERVICE_LOG_BYTES);
    file.seek(SeekFrom::Start(start))
        .map_err(|_| "Service log unreadable")?;
    let mut bytes = Vec::new();
    file.take(MAX_SERVICE_LOG_BYTES)
        .read_to_end(&mut bytes)
        .map_err(|_| "Service log unreadable")?;
    if start > 0 {
        while bytes.first().is_some_and(|byte| byte & 0xc0 == 0x80) {
            bytes.remove(0);
        }
    }
    Ok(ServiceLog {
        path: display_path,
        text: String::from_utf8_lossy(&bytes).into_owned(),
        missing: false,
        truncated: start > 0,
    })
}

fn service_log_path(root: &Path, service: ServiceLogKind) -> Result<PathBuf, String> {
    if matches!(service, ServiceLogKind::Agent) {
        Ok(fleet_root(root)?.join("agent.log"))
    } else {
        Ok(root.join("companion.log"))
    }
}

#[tauri::command]
pub fn local_service_log(
    app: tauri::AppHandle,
    service: ServiceLogKind,
) -> Result<ServiceLog, String> {
    let root = root(&app)?;
    read_service_log_file(&service_log_path(&root, service)?)
}

#[cfg(test)]
mod service_log_tests {
    use super::*;

    #[test]
    fn reads_only_the_tail_of_a_regular_service_log() {
        let dir =
            std::env::temp_dir().join(format!("sectile-service-log-{}", uuid::Uuid::new_v4()));
        fs::create_dir(&dir).unwrap();
        let path = dir.join("companion.log");
        assert_eq!(
            service_log_path(&dir, ServiceLogKind::Companion).unwrap(),
            path
        );
        assert_eq!(
            service_log_path(&dir, ServiceLogKind::Agent).unwrap(),
            dir.join("fleet/agent.log")
        );
        assert!(read_service_log_file(&path).unwrap().missing);
        let mut content = b"OLD_MARKER\n".to_vec();
        content.extend(vec![b'x'; MAX_SERVICE_LOG_BYTES as usize]);
        content.extend(b"\nLATEST_MARKER\n");
        fs::write(&path, content).unwrap();
        let result = read_service_log_file(&path).unwrap();
        assert!(result.truncated);
        assert!(!result.missing);
        assert!(result.text.ends_with("LATEST_MARKER\n"));
        assert!(!result.text.contains("OLD_MARKER"));
        #[cfg(unix)]
        {
            use std::os::unix::fs::symlink;
            let link = dir.join("agent.log");
            symlink(&path, &link).unwrap();
            assert!(read_service_log_file(&link).is_err());
        }
        fs::remove_dir_all(dir).unwrap();
    }
}

fn mode_path(root: &Path) -> PathBuf {
    root.join("runtime-mode.json")
}

pub fn runtime_mode(app: &tauri::AppHandle) -> Result<Option<RuntimeMode>, String> {
    let root = root(app)?;
    runtime_mode_from_root(&root)
}

fn save_mode(root: &Path, mode: RuntimeMode, server: Option<&str>) -> Result<(), String> {
    private_dir(root)?;
    let path = mode_path(root);
    let current = fs::read(&path)
        .ok()
        .and_then(|bytes| serde_json::from_slice::<Value>(&bytes).ok());
    let same_selection = current.as_ref().is_some_and(|current| {
        current["mode"] == json!(mode)
            && current["server"].as_str().unwrap_or("") == server.unwrap_or("")
    });
    let connection_id = current
        .as_ref()
        .filter(|_| same_selection)
        .and_then(|value| value["connectionId"].as_str())
        .and_then(|value| uuid::Uuid::parse_str(value).ok())
        .map(|value| value.to_string())
        .unwrap_or_else(|| uuid::Uuid::new_v4().to_string());
    let record =
        json!({"protocol": 1, "mode": mode, "server": server, "connectionId": connection_id});
    if current.is_some_and(|current| current == record) {
        return Ok(());
    }
    let next = root.join(format!("runtime-mode.{}.next.json", uuid::Uuid::new_v4()));
    let mut options = fs::OpenOptions::new();
    options.write(true).create(true).truncate(true);
    #[cfg(unix)]
    {
        use std::os::unix::fs::OpenOptionsExt;
        options.mode(0o600);
    }
    let mut file = options
        .open(&next)
        .map_err(|_| "Mode write denied")?;
    file.write_all(&serde_json::to_vec(&record).map_err(|_| "Invalid mode")?)
        .map_err(|_| "Mode write interrupted")?;
    file.sync_all()
        .map_err(|_| "Mode save interrupted")?;
    fs::rename(next, path).map_err(|_| "Mode save denied".into())
}

fn ensure_runtime_connection_id(root: &Path) -> Result<String, String> {
    let mode = runtime_mode_from_root(root)?
        .or_else(|| {
            active_root(root)
                .ok()
                .filter(|path| path.join("installation.json").exists())
                .map(|_| RuntimeMode::Local)
        })
        .ok_or_else(|| "Select a runtime mode in settings.".to_string())?;
    let path = mode_path(root);
    let value = fs::read(&path)
        .ok()
        .and_then(|bytes| serde_json::from_slice::<Value>(&bytes).ok());
    if let Some(id) = value
        .as_ref()
        .and_then(|value| value["connectionId"].as_str())
    {
        if let Ok(id) = uuid::Uuid::parse_str(id) {
            return Ok(id.to_string());
        }
    }
    let server = value
        .as_ref()
        .and_then(|value| value["server"].as_str())
        .filter(|value| !value.is_empty());
    save_mode(root, mode, server)?;
    let value: Value =
        serde_json::from_slice(&fs::read(path).map_err(|_| "Connection identity unavailable")?)
            .map_err(|_| "Invalid connection identity")?;
    value["connectionId"]
        .as_str()
        .map(str::to_owned)
        .ok_or_else(|| "Connection identity unavailable".into())
}

pub fn runtime_connection_id(app: &tauri::AppHandle) -> Result<String, String> {
    ensure_runtime_connection_id(&root(app)?)
}

fn selected_fleet_server(root: &Path) -> Result<String, String> {
    let value: Value =
        serde_json::from_slice(&fs::read(mode_path(root)).map_err(|_| "Fleet mode not found")?)
            .map_err(|_| "Fleet mode unreadable")?;
    value["server"]
        .as_str()
        .filter(|server| !server.is_empty())
        .map(str::to_owned)
        .ok_or_else(|| "Fleet server missing".into())
}

fn normalize_fleet_server_url(input: &str) -> Result<String, String> {
    let parsed =
        reqwest::Url::parse(input.trim()).map_err(|_| "Invalid Fleet server address")?;
    let is_loopback = parsed.host_str().is_some_and(|host| {
        host == "localhost"
            || host
                .parse::<std::net::IpAddr>()
                .is_ok_and(|ip| ip.is_loopback())
    });
    if (parsed.scheme() != "https" && !(parsed.scheme() == "http" && is_loopback))
        || parsed.host_str().is_none()
        || !parsed.username().is_empty()
        || parsed.password().is_some()
        || (parsed.path() != "/" && !parsed.path().is_empty())
        || parsed.query().is_some()
        || parsed.fragment().is_some()
    {
        return Err(
            "Use the HTTPS address of the Fleet backend, without credentials or path.".into(),
        );
    }
    Ok(parsed.as_str().trim_end_matches('/').to_owned())
}

pub fn connection_path(app: &tauri::AppHandle) -> Option<PathBuf> {
    let root = active_root(&root(app).ok()?).ok()?;
    connection_file(&root, runtime_mode(app).ok().flatten()?)
}

fn connection_file(root: &Path, mode: RuntimeMode) -> Option<PathBuf> {
    let path = match mode {
        RuntimeMode::Local if root.join("installation.json").exists() => {
            root.join("agent-connection.json")
        }
        RuntimeMode::Fleet => root.join("fleet/agent-connection.json"),
        RuntimeMode::Local => return None,
    };
    path.exists().then_some(path)
}
// The committed manifest wins over an interrupted progress journal.
fn active_root(root: &Path) -> Result<PathBuf, String> {
    let path = root.join("runtime-selection.json");
    if !path.exists() {
        return Ok(root.to_path_buf());
    }
    let value: Value =
        serde_json::from_slice(&fs::read(path).map_err(|_| "Service selection unreadable")?)
            .map_err(|_| "Invalid service selection")?;
    if value["protocol"] != 1
        || value["kind"] != "companion"
        || value["generation"] != "generations/companion-v1"
    {
        return Err("Incompatible local service version".into());
    }
    Ok(root.join("generations/companion-v1"))
}

fn fleet_root(root: &Path) -> Result<PathBuf, String> {
    Ok(active_root(root)?.join("fleet"))
}

// Serialize startup and pairing across multiple desktop processes as well as
// within one webview. The OS releases this lock if the desktop crashes.
fn installation_lock(root: &Path) -> Result<fs::File, String> {
    let mut options = fs::OpenOptions::new();
    options.create(true).read(true).write(true).truncate(false);
    #[cfg(unix)]
    {
        use std::os::unix::fs::OpenOptionsExt;
        options.mode(0o600);
    }
    let file = options
        .open(root.join("runtime.lock"))
        .map_err(|_| "Local lock unavailable")?;
    file.try_lock()
        .map_err(|_| "Another window is preparing local services. Try again in a moment.")?;
    Ok(file)
}
fn read_installation(root: &Path) -> Result<Installation, String> {
    let root = active_root(root)?;
    let bytes = fs::read(root.join("installation.json"))
        .map_err(|_| "Enable the local installation first")?;
    let installation: Installation =
        serde_json::from_slice(&bytes).map_err(|_| "Local configuration unreadable")?;
    if installation.port == 0 || installation.key.len() < 32 {
        return Err("Invalid local configuration".into());
    }
    Ok(installation)
}
fn private_dir(path: &Path) -> Result<(), String> {
    fs::create_dir_all(path).map_err(|_| "Unable to create local folder")?;
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        fs::set_permissions(path, fs::Permissions::from_mode(0o700))
            .map_err(|_| "Folder permissions denied")?;
    }
    Ok(())
}
fn save(root: &Path, value: &Installation) -> Result<(), String> {
    let root = active_root(root)?;
    let path = root.join("installation.next.json");
    let mut options = fs::OpenOptions::new();
    options.write(true).create(true).truncate(true);
    #[cfg(unix)]
    {
        use std::os::unix::fs::OpenOptionsExt;
        options.mode(0o600);
    }
    let mut file = options
        .open(&path)
        .map_err(|_| "Configuration write denied")?;
    file.write_all(&serde_json::to_vec(value).map_err(|_| "Invalid configuration")?)
        .map_err(|_| "Write interrupted")?;
    file.sync_all().map_err(|_| "Save interrupted")?;
    fs::rename(path, root.join("installation.json"))
        .map_err(|_| "Configuration save denied".into())
}
fn client() -> Result<Client, String> {
    Client::builder()
        .no_proxy()
        .redirect(reqwest::redirect::Policy::none())
        .timeout(Duration::from_secs(45))
        .build()
        .map_err(|_| "Local connection unavailable".into())
}

async fn exchange_fleet_pairing(
    client: &Client,
    server: &str,
    pairing_code: &str,
) -> Result<FleetPairing, String> {
    let endpoint = reqwest::Url::parse(server)
        .map_err(|_| "Invalid Fleet server address")?
        .join("/api/v1/agent/pair")
        .map_err(|_| "Invalid Fleet server address")?;
    let response = client
        .post(endpoint)
        .json(&json!({"code":pairing_code,"label":"Sectile Desktop"}))
        .timeout(Duration::from_secs(12))
        .send()
        .await
        .map_err(|_| "Unable to reach Fleet server")?;
    if response.status() != reqwest::StatusCode::CREATED {
        return Err(if response.status() == reqwest::StatusCode::UNAUTHORIZED {
            "Invalid or expired pairing code. Generate a new one from your Fleet profile."
                .into()
        } else {
            format!(
                "The Fleet server refused pairing (HTTP {}).",
                response.status().as_u16()
            )
        });
    }
    let payload: Value = response
        .json()
        .await
        .map_err(|_| "Fleet pairing response unreadable")?;
    let (Some(api_token), Some(device_id), Some(user_id)) = (
        payload["token"].as_str(),
        payload["deviceId"].as_str(),
        payload["userId"].as_str(),
    ) else {
        return Err("The Fleet server did not return a valid pairing.".into());
    };
    if api_token.is_empty() || device_id.is_empty() || user_id.is_empty() {
        return Err("The Fleet server did not return a complete pairing.".into());
    }
    Ok(FleetPairing {
        api_token: api_token.to_owned(),
        device_id: device_id.to_owned(),
        user_id: user_id.to_owned(),
    })
}
fn binary(app: &tauri::AppHandle, service: &str) -> Result<PathBuf, String> {
    let name = format!(
        "sectile-{service}{}",
        if cfg!(windows) { ".exe" } else { "" }
    );
    #[cfg(debug_assertions)]
    {
        let development = PathBuf::from(env!("CARGO_MANIFEST_DIR"))
            .join("binaries")
            .join(&name);
        if development.is_file() {
            return Ok(development);
        }
    }
    let packaged = app
        .path()
        .resource_dir()
        .map_err(|_| "Resources unavailable")?
        .join("binaries")
        .join(&name);
    if packaged.is_file() {
        return Ok(packaged);
    }
    if cfg!(debug_assertions) {
        Err(format!(
            "Service {service} is missing. Run npm run prepare:services and rebuild the application."
        ))
    } else {
        Err(format!(
            "Service {service} is missing from this installation. Reinstall Sectile Desktop."
        ))
    }
}
fn command(app: &tauri::AppHandle, root: &Path, service: &str) -> Result<Command, String> {
    let mut cmd = Command::new(binary(app, service)?);
    cmd.current_dir(root).env_clear();
    // Keep workstation tools and OS facilities, never inherited deployment secrets.
    for name in [
        "PATH",
        "HOME",
        "USER",
        "USERPROFILE",
        "APPDATA",
        "LOCALAPPDATA",
        "SYSTEMROOT",
        "WINDIR",
        "TMP",
        "TEMP",
        "TMPDIR",
        "LANG",
        "SSH_AUTH_SOCK",
    ] {
        if let Some(value) = std::env::var_os(name) {
            cmd.env(name, value);
        }
    }
    #[cfg(windows)]
    {
        use std::os::windows::process::CommandExt;
        cmd.creation_flags(0x08000000);
    }
    let mut options = fs::OpenOptions::new();
    options.create(true).append(true);
    #[cfg(unix)]
    {
        use std::os::unix::fs::OpenOptionsExt;
        options.mode(0o600);
    }
    let log = options
        .open(root.join(format!("{service}.log")))
        .map_err(|_| "Local log unavailable")?;
    cmd.stdin(Stdio::null())
        .stdout(log.try_clone().map_err(|_| "Log unavailable")?)
        .stderr(log);
    Ok(cmd)
}
// Discover the fixed transport contract from Go without starting a server or DB.
fn runtime_info(app: &tauri::AppHandle, root: &Path) -> Result<Value, String> {
    let output = command(app, root, "companion")?
        .arg("--local-runtime-info")
        .stdout(Stdio::piped())
        .output()
        .map_err(|_| "Go server configuration unavailable")?;
    let info: Value = serde_json::from_slice(&output.stdout)
        .map_err(|_| "Invalid Go server configuration")?;
    if !output.status.success()
        || info["protocol"] != 1
        || info["port"]
            .as_u64()
            .filter(|p| *p > 0 && *p <= 65535)
            .is_none()
        || info["origin"] != format!("http://127.0.0.1:{}", info["port"])
    {
        return Err("Incompatible Go server configuration".into());
    }
    Ok(info)
}
async fn probe(installation: &Installation) -> bool {
    let Ok(client) = client() else { return false };
    match client
        .get(format!("{}/api/local-runtime", installation.url()))
        .bearer_auth(&installation.key)
        .timeout(Duration::from_secs(2))
        .send()
        .await
    {
        Ok(response) if response.status().is_success() => response
            .json::<Value>()
            .await
            .ok()
            .is_some_and(|v| v["protocol"] == 1 && v["ready"] == true && v["companion"] == true),
        _ => false,
    }
}
async fn ensure_server(
    app: &tauri::AppHandle,
    root: &Path,
    installation: &Installation,
) -> Result<(), String> {
    require_runtime_start(app)?;
    let info = runtime_info(app, root)?;
    if info["port"].as_u64() != Some(u64::from(installation.port)) {
        return Err("Local port is incompatible with the registered OAuth callback".into());
    }
    {
        let mut upgrade = command(app, root, "companion")?;
        upgrade
            .arg("--upgrade")
            .arg("--data-dir")
            .arg(root)
            .arg("--legacy-server")
            .arg(binary(app, "server")?)
            .arg("--legacy-agent")
            .arg(binary(app, "agent")?);
        let result = tauri::async_runtime::spawn_blocking(move || upgrade.status())
            .await
            .map_err(|_| "Local migration interrupted")?
            .map_err(|_| "Unable to prepare local migration")?;
        if !result.success() {
            return Err("Local migration paused. Finish active runs, then try again. Original data is preserved; check companion.log.".into());
        }
    }
    if probe(installation).await {
        return Ok(());
    }
    let data = active_root(root)?;
    let reservation = TcpListener::bind(("127.0.0.1", installation.port))
        .map_err(|_| "Local port is already in use by another service")?;
    let mut cmd = command(app, root, "companion")?;
    cmd.arg("--data-dir")
        .arg(&data)
        .env("SECTILE_LOCAL_RUNTIME_KEY", &installation.key)
        .env("SECTILE_COMPANION_MCP_URL", installation.url());
    drop(reservation);
    let mut child = cmd
        .spawn()
        .map_err(|_| "Unable to start Go companion")?;
    for _ in 0..120 {
        if probe(installation).await {
            std::thread::spawn(move || {
                let _ = child.wait();
            });
            return Ok(());
        }
        if child
            .try_wait()
            .map_err(|_| "Companion status unavailable")?
            .is_some()
        {
            return Err("Go companion stopped. Check companion.log.".into());
        }
        tokio::time::sleep(Duration::from_millis(250)).await;
    }
    let _ = child.kill();
    let _ = child.wait();
    Err("Local companion is not responding. Check companion.log.".into())
}
async fn agent_ready(root: &Path, installation: &Installation) -> bool {
    let Ok(root) = active_root(root) else {
        return false;
    };
    let Ok(bytes) = fs::read(root.join("agent-connection.json")) else {
        return false;
    };
    let Ok(value) = serde_json::from_slice::<Value>(&bytes) else {
        return false;
    };
    let (Some(url), Some(token)) = (value["url"].as_str(), value["token"].as_str()) else {
        return false;
    };
    let Ok(parsed) = reqwest::Url::parse(url) else {
        return false;
    };
    if parsed.scheme() != "http" || parsed.host_str() != Some("127.0.0.1") || parsed.path() != "/" {
        return false;
    }
    let Ok(client) = client() else { return false };
    match client
        .get(format!("{}/desktop/status", url.trim_end_matches('/')))
        .bearer_auth(token)
        .timeout(Duration::from_secs(2))
        .send()
        .await
    {
        Ok(response) if response.status().is_success() => response
            .json::<Value>()
            .await
            .ok()
            .is_some_and(|v| v["server"] == installation.url()),
        _ => false,
    }
}

async fn fleet_status(fleet_root: &Path) -> Option<Value> {
    let connection: Value =
        serde_json::from_slice(&fs::read(fleet_root.join("agent-connection.json")).ok()?).ok()?;
    let (url, token) = (connection["url"].as_str()?, connection["token"].as_str()?);
    let parsed = reqwest::Url::parse(url).ok()?;
    if parsed.scheme() != "http" || parsed.host_str() != Some("127.0.0.1") || parsed.path() != "/" {
        return None;
    }
    let response = client()
        .ok()?
        .get(format!("{}/desktop/status", url.trim_end_matches('/')))
        .bearer_auth(token)
        .timeout(Duration::from_secs(2))
        .send()
        .await
        .ok()?;
    if !response.status().is_success() {
        return None;
    }
    response.json().await.ok()
}

// A managed Fleet agent shares provider logins with Electron, not its MCP
// registration. The selected backend is supplied only to child processes.
fn scope_fleet_mcp(command: &mut Command, server: &str) {
    command
        .env("SECTILE_DESKTOP_MCP_URL", server)
        .env("SECTILE_DISABLE_MCP_REFRESH", "1");
}

async fn ensure_fleet_agent(
    app: &tauri::AppHandle,
    root: &Path,
    expected_server: &str,
) -> Result<(), String> {
    let fleet_root = fleet_root(root)?;
    if let Some(status) = fleet_status(&fleet_root).await {
        if status["server"] == expected_server {
            return Ok(());
        }
        return Err("Fleet agent responded with a different server identity".into());
    }
    let bytes = fs::read(fleet_root.join("settings.json")).map_err(
        |_| "The paired Fleet agent is stopped. Restart it from its usual installation.",
    )?;
    let credentials: FleetCredentials =
        serde_json::from_slice(&bytes).map_err(|_| "Fleet agent configuration unreadable")?;
    let runtime: FleetRuntime = serde_json::from_slice(
        &fs::read(fleet_root.join("fleet-runtime.json"))
            .map_err(|_| "Fleet runtime configuration missing")?,
    )
    .map_err(|_| "Fleet runtime configuration unreadable")?;
    if credentials.server != expected_server
        || runtime.server != expected_server
        || credentials.api_key.is_empty()
        || runtime.desktop_token.is_empty()
        || runtime.user_id.is_empty()
        || credentials.device_id.is_empty()
    {
        return Err("Incompatible Fleet agent configuration".into());
    }
    private_dir(&fleet_root)?;
    let _installation_lock = installation_lock(root)?;
    if let Some(status) = fleet_status(&fleet_root).await {
        if status["server"] == expected_server {
            return Ok(());
        }
        return Err("Fleet agent responded with a different server identity".into());
    }
    let discovery = fleet_root.join("agent-connection.json");
    let mut cmd = command(app, &fleet_root, "agent")?;
    cmd.arg("--url")
        .arg(&credentials.server)
        .arg("--device")
        .arg(&credentials.device_id)
        .arg("--desktop-info")
        .arg(&discovery)
        .env("SECTILE_DESKTOP_TOKEN", &runtime.desktop_token)
        .env("SECTILE_DESKTOP_DATA_DIR", &fleet_root);
    scope_fleet_mcp(&mut cmd, &credentials.server);
    let mut child = cmd
        .spawn()
        .map_err(|_| "Unable to start Fleet agent")?;
    for _ in 0..240 {
        if let Some(status) = fleet_status(&fleet_root).await {
            if status["server"] == expected_server {
                std::thread::spawn(move || {
                    let _ = child.wait();
                });
                return Ok(());
            }
        }
        if child
            .try_wait()
            .map_err(|_| "Fleet agent status unavailable")?
            .is_some()
        {
            return Err("Fleet agent stopped. Check agent.log.".into());
        }
        tokio::time::sleep(Duration::from_millis(250)).await;
    }
    let _ = child.kill();
    let _ = child.wait();
    Err("Fleet agent did not connect to the server. Check agent.log.".into())
}

fn write_private_json(path: &Path, value: &impl Serialize) -> Result<(), String> {
    let parent = path.parent().ok_or("Configuration folder missing")?;
    private_dir(parent)?;
    let name = path
        .file_name()
        .and_then(|name| name.to_str())
        .unwrap_or("config");
    let next = parent.join(format!("{name}.{}.next", uuid::Uuid::new_v4()));
    let mut options = fs::OpenOptions::new();
    options.write(true).create_new(true);
    #[cfg(unix)]
    {
        use std::os::unix::fs::OpenOptionsExt;
        options.mode(0o600);
    }
    let mut file = options
        .open(&next)
        .map_err(|_| "Configuration write denied")?;
    file.write_all(&serde_json::to_vec(value).map_err(|_| "Invalid configuration")?)
        .map_err(|_| "Configuration save interrupted")?;
    file.sync_all()
        .map_err(|_| "Configuration save interrupted")?;
    fs::rename(next, path).map_err(|_| "Configuration save denied".into())
}

fn write_fleet_credentials(
    root: &Path,
    credentials: &FleetCredentials,
    runtime: &FleetRuntime,
) -> Result<(), String> {
    let fleet_root = fleet_root(root)?;
    write_private_json(&fleet_root.join("settings.json"), credentials)?;
    write_private_json(&fleet_root.join("fleet-runtime.json"), runtime)
}

async fn ensure_agent(
    _app: &tauri::AppHandle,
    root: &Path,
    installation: &Installation,
) -> Result<(), String> {
    if installation.token.is_empty() {
        return Ok(());
    }
    for _ in 0..150 {
        if agent_ready(root, installation).await {
            let ready = client()?
                .get(format!("{}/api/ready", installation.url()))
                .timeout(Duration::from_secs(2))
                .send()
                .await;
            if ready.is_ok_and(|response| response.status().is_success()) {
                return Ok(());
            }
        }
        tokio::time::sleep(Duration::from_millis(200)).await;
    }
    Err("Built-in agent is not ready. Check companion.log.".into())
}
async fn request(
    installation: &Installation,
    method: Method,
    route: &str,
    body: Option<Value>,
) -> Result<Value, String> {
    let mut request = client()?
        .request(method, format!("{}{route}", installation.url()))
        .bearer_auth(&installation.token);
    if let Some(body) = body {
        request = request.json(&body);
    }
    let response = request
        .send()
        .await
        .map_err(|_| "Local response not received. Check status before repeating a creation.")?;
    let status = response.status();
    if status == reqwest::StatusCode::NO_CONTENT {
        return Ok(Value::Null);
    }
    let value: Value = response
        .json()
        .await
        .map_err(|_| "Local response unreadable")?;
    if !status.is_success() {
        // Do not return payloads that could echo a submitted tracker credential.
        return Err(format!(
            "Action refused by local server (HTTP {}). Check fields and connection.",
            status.as_u16()
        ));
    }
    Ok(value)
}

fn read_fleet_credentials(root: &Path) -> Result<FleetCredentials, String> {
    let fleet_root = fleet_root(root)?;
    let credentials: FleetCredentials = serde_json::from_slice(
        &fs::read(fleet_root.join("settings.json"))
            .map_err(|_| "Fleet API credentials are missing. Pair this workstation with a pairing code.".to_string())?,
    )
    .map_err(|_| "Fleet API configuration unreadable".to_string())?;
    let normalized = normalize_fleet_server_url(&credentials.server)?;
    if normalized != credentials.server
        || credentials.api_key.is_empty()
        || credentials.device_id.is_empty()
        || credentials.server != selected_fleet_server(root)?
    {
        return Err("Incompatible Fleet API configuration".into());
    }
    Ok(credentials)
}

fn read_fleet_runtime(root: &Path) -> Result<FleetRuntime, String> {
    let runtime: FleetRuntime = serde_json::from_slice(
        &fs::read(fleet_root(root)?.join("fleet-runtime.json"))
            .map_err(|_| "Fleet identity configuration missing".to_string())?,
    )
    .map_err(|_| "Fleet identity configuration unreadable".to_string())?;
    if runtime.server != selected_fleet_server(root)?
        || runtime.user_id.trim().is_empty()
        || runtime.desktop_token.trim().is_empty()
    {
        return Err("Incompatible Fleet identity configuration".into());
    }
    Ok(runtime)
}

fn validate_fleet_profile_identity(profile: &Value, expected_user: &str) -> Result<(), String> {
    if profile["signedIn"] != true
        || profile["userId"]
            .as_str()
            .is_none_or(|user| user != expected_user)
    {
        return Err(
            "Fleet backend identity no longer matches this workstation's pairing.".into(),
        );
    }
    Ok(())
}

async fn fleet_request(
    root: &Path,
    method: Method,
    route: &str,
    body: Option<Value>,
) -> Result<Value, String> {
    let credentials = read_fleet_credentials(root)?;
    let base = reqwest::Url::parse(&credentials.server).map_err(|_| "Invalid Fleet address")?;
    let endpoint = base
        .join(route.trim_start_matches('/'))
        .map_err(|_| "Invalid Fleet route")?;
    if endpoint.origin() != base.origin() || !endpoint.path().starts_with("/api/") {
        return Err("Fleet route refused".into());
    }
    let mut request = client()?
        .request(method, endpoint)
        .bearer_auth(&credentials.api_key);
    if let Some(body) = body {
        request = request.json(&body);
    }
    let response = request
        .send()
        .await
        .map_err(|_| "Fleet backend is unreachable. Check its connection.")?;
    let status = response.status();
    if status == reqwest::StatusCode::NO_CONTENT {
        return Ok(Value::Null);
    }
    if !status.is_success() {
        return Err(match status {
            reqwest::StatusCode::UNAUTHORIZED => {
                "Fleet token has expired or been revoked. Re-pair this workstation.".into()
            }
            reqwest::StatusCode::FORBIDDEN => {
                "Your Fleet profile is not authorized to perform this action.".into()
            }
            reqwest::StatusCode::NOT_FOUND => {
                "This operation is not available on this version of the Fleet backend.".into()
            }
            _ => format!(
                "The Fleet backend refused this action (HTTP {}).",
                status.as_u16()
            ),
        });
    }
    response
        .json()
        .await
        .map_err(|_| "Fleet backend response unreadable".into())
}

async fn selected_request(
    root: &Path,
    method: Method,
    route: &str,
    body: Option<Value>,
) -> Result<Value, String> {
    match runtime_mode_from_root(root)? {
        Some(RuntimeMode::Fleet) => fleet_request(root, method, route, body).await,
        Some(RuntimeMode::Local) => request(&read_installation(root)?, method, route, body).await,
        None => Err("Select a runtime mode in settings.".into()),
    }
}

fn runtime_mode_from_root(root: &Path) -> Result<Option<RuntimeMode>, String> {
    let path = mode_path(root);
    if !path.exists() {
        return Ok(active_root(root)?
            .join("installation.json")
            .exists()
            .then_some(RuntimeMode::Local));
    }
    let value: Value =
        serde_json::from_slice(&fs::read(path).map_err(|_| "Runtime mode unreadable")?)
            .map_err(|_| "Invalid runtime mode")?;
    if value["protocol"] != 1
        || (value["mode"] == "fleet" && value["server"].as_str().is_none_or(str::is_empty))
    {
        return Err("Incompatible mode configuration".into());
    }
    serde_json::from_value(value["mode"].clone())
        .map(Some)
        .map_err(|_| "Incompatible mode configuration".into())
}

fn require_unconfigured_runtime(mode: Option<RuntimeMode>) -> Result<(), String> {
    if mode.is_some() {
        return Err("A runtime mode is already selected. Its migration must be performed explicitly.".into());
    }
    Ok(())
}

fn require_runtime_start(app: &tauri::AppHandle) -> Result<(), String> {
    if app
        .state::<LocalRuntime>()
        .3
        .load(std::sync::atomic::Ordering::Acquire)
    {
        return Err("Update preparation in progress. Retry stopping or explicitly start services from settings.".into());
    }
    Ok(())
}

pub async fn resume(app: &tauri::AppHandle) -> Result<(), String> {
    let state = app.state::<LocalRuntime>();
    let _lock = state.2.lock().await;
    require_runtime_start(app)?;
    let root = root(app)?;
    if runtime_mode(app)?.is_some() || active_root(&root)?.join("installation.json").exists() {
        ensure_runtime_connection_id(&root)?;
    }
    if state.1.load(std::sync::atomic::Ordering::Acquire) {
        return Ok(());
    }
    if runtime_mode(app)? == Some(RuntimeMode::Fleet) {
        let server = selected_fleet_server(&root)?;
        ensure_fleet_agent(app, &root, &server).await?;
        state.1.store(true, std::sync::atomic::Ordering::Release);
        return Ok(());
    }
    if !active_root(&root)?.join("installation.json").exists() {
        return Ok(());
    }
    let _installation_lock = installation_lock(&root)?;
    let installation = read_installation(&root)?;
    ensure_server(app, &root, &installation).await?;
    ensure_agent(app, &root, &installation).await?;
    state.1.store(true, std::sync::atomic::Ordering::Release);
    Ok(())
}
#[tauri::command]
pub async fn prepare_local_update(app: tauri::AppHandle) -> Result<(), String> {
    let state = app.state::<LocalRuntime>();
    // Match explicit setup lock order and exclude an in-flight startup/resume.
    let _setup = state.0.lock().await;
    let _startup = state.2.lock().await;
    if runtime_mode(&app)? != Some(RuntimeMode::Local) {
        return Err(
            "Update preparation is reserved for the Local companion managed by this desktop."
                .into(),
        );
    }
    let root = root(&app)?;
    let _installation = installation_lock(&root)?;
    let data = active_root(&root)?;
    let mut stop = command(&app, &root, "companion")?;
    stop.arg("--prepare-update")
        .arg("--data-dir")
        .arg(data)
        .stdout(Stdio::piped());
    // Preserve this fence on failure too: a timeout can follow an acknowledged
    // shutdown. Only a deliberate restart or retry may change that intent.
    state.3.store(true, std::sync::atomic::Ordering::Release);
    state.1.store(false, std::sync::atomic::Ordering::Release);
    let output = tauri::async_runtime::spawn_blocking(move || stop.output())
        .await
        .map_err(|_| "Update preparation interrupted")?
        .map_err(|_| "Unable to prepare companion update")?;
    validate_update_result(output.status.success(), &output.stdout)?;
    // The helper proved that the agent, HTTP server and store users stopped.
    // Exit while the installation/startup locks still exclude another launch.
    app.exit(0);
    Ok(())
}

fn validate_update_result(success: bool, bytes: &[u8]) -> Result<(), String> {
    let response = serde_json::from_slice::<Value>(bytes).unwrap_or(Value::Null);
    if success && response["ready"] == true {
        return Ok(());
    }
    if response["reason"] == "busy" {
        return Err("Stop active runs, open conversations in the sidebar, and preparations before updating. To continue working, click Check / start local companion.".into());
    }
    Err("Shutdown not confirmed. Try again or explicitly start the companion from settings. Check companion.log if needed.".into())
}

#[tauri::command]
pub async fn enable_local_runtime(app: tauri::AppHandle) -> Result<(), String> {
    let state = app.state::<LocalRuntime>();
    let _lock = state.0.lock().await;
    let root = root(&app)?;
    if runtime_mode(&app)? == Some(RuntimeMode::Fleet) {
        return Err(
            "Fleet mode is selected. Detach it before enabling a local installation."
                .into(),
        );
    }
    private_dir(&root)?;
    let _installation_lock = installation_lock(&root)?;
    // Keep legacy resources available for verified migration and recovery.
    binary(&app, "companion")?;
    binary(&app, "server")?;
    binary(&app, "agent")?;
    if !root.join("installation.json").exists() {
        let info = runtime_info(&app, &root)?;
        let port = info["port"].as_u64().ok_or("Go port missing")? as u16;
        save(
            &root,
            &Installation {
                port,
                key: uuid::Uuid::new_v4().to_string(),
                token: String::new(),
                device_id: String::new(),
                agent_device_id: String::new(),
                user_id: String::new(),
                email: String::new(),
            },
        )?;
    }
    let installation = read_installation(&root)?;
    if probe(&installation).await {
        if let Ok(data) = active_root(&root) {
            if let Ok(mut stop) = command(&app, &root, "companion") {
                stop.arg("--prepare-update")
                    .arg("--data-dir")
                    .arg(data)
                    .stdout(Stdio::piped())
                    .stderr(Stdio::null());
                let output = tauri::async_runtime::spawn_blocking(move || stop.output())
                    .await
                    .map_err(|_| "Companion restart interrupted")?
                    .map_err(|_| "Unable to restart local companion")?;
                validate_update_result(output.status.success(), &output.stdout)?;
            }
        }
    }
    // Only this explicit user action cancels update preparation. Polling never does.
    state.3.store(false, std::sync::atomic::Ordering::Release);
    state.1.store(false, std::sync::atomic::Ordering::Release);
    ensure_server(&app, &root, &installation).await?;
    ensure_agent(&app, &root, &installation).await?;
    save_mode(&root, RuntimeMode::Local, None)
}
#[tauri::command]
pub async fn local_setup_status(app: tauri::AppHandle) -> Result<Value, String> {
    let resumed = resume(&app).await;
    let state = app.state::<LocalRuntime>();
    let _lock = state.0.lock().await;
    let root = root(&app)?;
    if state.3.load(std::sync::atomic::Ordering::Acquire) {
        // A settings remount must retain explicit recovery controls without
        // polling accidentally restarting a service whose shutdown is pending.
        return Ok(json!({"enabled":true,"ready":false,"signedIn":false,
            "agentReady":false,"mode":"local","updatePreparing":true,
            "dataDirectory":active_root(&root)?,
            "backendError":"Automatic restart paused for update. Retry stopping or explicitly start the companion."}));
    }
    if runtime_mode(&app)? == Some(RuntimeMode::Fleet) {
        let server = selected_fleet_server(&root)?;
        let unavailable = |error: String| {
            json!({"enabled":true,"ready":false,"signedIn":false,
                "agentReady":false,"mode":"fleet","fleetServer":server,
                "apiSetupAvailable":fleet_root(&root).is_ok_and(|path| path.join("settings.json").exists()),
                "backendError":error})
        };
        if let Err(error) = resumed {
            return Ok(unavailable(error));
        }
        let status = async {
            let path = connection_path(&app).ok_or("Fleet connection not found")?;
            let connection: Value =
                serde_json::from_slice(&fs::read(path).map_err(|_| "Fleet connection unreadable")?)
                    .map_err(|_| "Invalid Fleet connection")?;
            let (Some(url), Some(token)) =
                (connection["url"].as_str(), connection["token"].as_str())
            else {
                return Err("Incomplete Fleet connection".into());
            };
            let status = client()?
                .get(format!("{}/desktop/status", url.trim_end_matches('/')))
                .bearer_auth(token)
                .timeout(Duration::from_secs(3))
                .send()
                .await
                .map_err(|_| "The paired Fleet agent is unreachable.")?;
            if !status.status().is_success() {
                return Err("The paired Fleet agent refused the connection.".into());
            }
            let value: Value = status.json().await.map_err(|_| "Fleet status unreadable")?;
            if value["server"] != server {
                return Err("Fleet agent responded with a different server identity".into());
            }
            Ok::<_, String>(value)
        }
        .await;
        let value = match status {
            Ok(value) => value,
            Err(error) => return Ok(unavailable(error)),
        };
        if !fleet_root(&root)?.join("settings.json").exists() {
            return Ok(json!({"enabled":true,"ready":true,"signedIn":false,
                "agentReady":value["connected"],"mode":"fleet","fleetServer":value["server"],
                "apiSetupAvailable":false}));
        }
        let backend = async {
            let profile = fleet_request(&root, Method::GET, "/api/me", None).await?;
            let runtime = read_fleet_runtime(&root)?;
            validate_fleet_profile_identity(&profile, &runtime.user_id)?;
            let credentials =
                fleet_request(&root, Method::GET, "/api/me/tracker-credentials", None).await?;
            let trackers = fleet_request(&root, Method::GET, "/api/trackers", None).await?;
            let projects = fleet_request(&root, Method::GET, "/api/projects", None).await?;
            Ok::<_, String>((profile, credentials, trackers, projects))
        }
        .await;
        let (profile, credentials, trackers, projects) = match backend {
            Ok(backend) => backend,
            Err(error) => {
                return Ok(json!({"enabled":true,"ready":false,"signedIn":false,
                    "agentReady":value["connected"],"mode":"fleet","fleetServer":value["server"],
                    "apiSetupAvailable":true,"backendError":error}));
            }
        };
        return Ok(
            json!({"enabled":true,"ready":true,"signedIn":profile["signedIn"] == true,
            "agentReady":value["connected"],"mode":"fleet","fleetServer":value["server"],
            "apiSetupAvailable":true,
            "profile":profile,"credentials":credentials,"trackers":trackers,"projects":projects}),
        );
    }
    resumed?;
    if !root.join("installation.json").exists() {
        return Ok(json!({"enabled":false,"mode":"unconfigured"}));
    }
    let installation = read_installation(&root)?;
    if !probe(&installation).await {
        let _installation_lock = installation_lock(&root)?;
        ensure_server(&app, &root, &installation).await?;
        ensure_agent(&app, &root, &installation).await?;
    }
    if installation.token.is_empty() {
        return Ok(
            json!({"enabled":true,"ready":true,"signedIn":false,"mode":"local","dataDirectory":active_root(&root)?}),
        );
    }
    Ok(
        json!({"enabled":true,"ready":true,"signedIn":true,"agentReady":agent_ready(&root,&installation).await,"mode":"local",
        "dataDirectory":active_root(&root)?,
        "profile":request(&installation,Method::GET,"/api/me",None).await?,
        "credentials":request(&installation,Method::GET,"/api/me/tracker-credentials",None).await?,
        "trackers":request(&installation,Method::GET,"/api/trackers",None).await?,
        "projects":request(&installation,Method::GET,"/api/projects",None).await?}),
    )
}

#[tauri::command]
pub async fn pair_fleet_agent(
    app: tauri::AppHandle,
    server_url: String,
    pairing_code: String,
) -> Result<(), String> {
    let state = app.state::<LocalRuntime>();
    let settings_lock = state.0.lock().await;
    let root = root(&app)?;
    require_unconfigured_runtime(runtime_mode(&app)?)?;
    let pairing_code = pairing_code.trim();
    if pairing_code.is_empty() || pairing_code.len() > 256 {
        return Err("Pairing code required.".into());
    }
    let server = normalize_fleet_server_url(&server_url)?;
    let pairing = exchange_fleet_pairing(&client()?, &server, pairing_code).await?;
    let desktop_token = uuid::Uuid::new_v4().to_string();
    let credentials = FleetCredentials {
        server: server.clone(),
        api_key: pairing.api_token,
        device_id: pairing.device_id,
    };
    let runtime = FleetRuntime {
        server: server.clone(),
        desktop_token,
        user_id: pairing.user_id,
    };
    write_fleet_credentials(&root, &credentials, &runtime)?;
    save_mode(&root, RuntimeMode::Fleet, Some(&server))?;
    drop(settings_lock);
    resume(&app).await
}

#[tauri::command]
pub async fn attach_fleet_agent(app: tauri::AppHandle) -> Result<(), String> {
    let state = app.state::<LocalRuntime>();
    let _lock = state.0.lock().await;
    let root = root(&app)?;
    require_unconfigured_runtime(runtime_mode(&app)?)?;
    let legacy = app
        .path()
        .home_dir()
        .map_err(|_| "Home directory not found")?
        .join(".taskflow/agent-connection.json");
    let bytes = fs::read(&legacy).map_err(|_| "No previously paired Fleet agent was found. Pair this workstation with the server from the Sectile agent first.")?;
    let connection: Value =
        serde_json::from_slice(&bytes).map_err(|_| "Invalid Fleet connection file")?;
    let (Some(url), Some(token)) = (connection["url"].as_str(), connection["token"].as_str())
    else {
        return Err("Incomplete Fleet connection file".into());
    };
    let parsed = reqwest::Url::parse(url).map_err(|_| "Invalid agent address")?;
    if parsed.scheme() != "http"
        || parsed.host_str() != Some("127.0.0.1")
        || parsed.path() != "/"
        || token.trim().is_empty()
    {
        return Err("Fleet agent must be a local agent paired with this workstation".into());
    }
    let status = client()?
        .get(format!("{}/desktop/status", url.trim_end_matches('/')))
        .bearer_auth(token)
        .timeout(Duration::from_secs(3))
        .send()
        .await
        .map_err(|_| "Unable to reach the paired Fleet agent")?;
    if !status.status().is_success() {
        return Err("Fleet agent refused connection verification".into());
    }
    let value: Value = status
        .json()
        .await
        .map_err(|_| "Fleet agent status unreadable")?;
    if value["connected"] != true || value["server"].as_str().is_none_or(str::is_empty) {
        return Err("Fleet agent is not connected to its server".into());
    }
    let fleet_root = fleet_root(&root)?;
    private_dir(&fleet_root)?;
    let _installation_lock = installation_lock(&root)?;
    let path = fleet_root.join("agent-connection.json");
    let contents = serde_json::to_vec(&json!({"url":url,"token":token}))
        .map_err(|_| "Invalid Fleet connection")?;
    let mut options = fs::OpenOptions::new();
    options.write(true).create(true).truncate(true);
    #[cfg(unix)]
    {
        use std::os::unix::fs::OpenOptionsExt;
        options.mode(0o600);
    }
    let mut file = options
        .open(path)
        .map_err(|_| "Fleet connection write denied")?;
    file.write_all(&contents)
        .map_err(|_| "Fleet connection save interrupted")?;
    file.sync_all()
        .map_err(|_| "Fleet connection save interrupted")?;
    save_mode(&root, RuntimeMode::Fleet, value["server"].as_str())
}
#[tauri::command]
pub async fn save_local_profile(
    app: tauri::AppHandle,
    email: String,
    display_name: String,
) -> Result<(), String> {
    let state = app.state::<LocalRuntime>();
    let _lock = state.0.lock().await;
    let root = root(&app)?;
    if runtime_mode(&app)? == Some(RuntimeMode::Fleet) {
        let _installation_lock = installation_lock(&root)?;
        selected_request(
            &root,
            Method::PATCH,
            "/api/me",
            Some(json!({"displayName":display_name})),
        )
        .await?;
        return Ok(());
    }
    let _installation_lock = installation_lock(&root)?;
    let mut installation = read_installation(&root)?;
    ensure_server(&app, &root, &installation).await?;
    if installation.token.is_empty() {
        let response = client()?
            .post(format!("{}/auth/local", installation.url()))
            .json(&json!({"email":email}))
            .send()
            .await
            .map_err(|_| "Local sign-in interrupted")?;
        if !response.status().is_success() {
            return Err("Sign-in refused. Check your email address.".into());
        }
        let cookie = response
            .headers()
            .get(reqwest::header::SET_COOKIE)
            .and_then(|v| v.to_str().ok())
            .and_then(|v| v.split(';').next())
            .ok_or("Local session missing")?
            .to_owned();
        let profile: Value = response.json().await.map_err(|_| "Profile unreadable")?;
        let response = client()?
            .post(format!("{}/api/pairing-codes", installation.url()))
            .header(reqwest::header::COOKIE, cookie)
            .send()
            .await
            .map_err(|_| "Pairing interrupted")?;
        if !response.status().is_success() {
            return Err("Pairing refused".into());
        }
        let code: Value = response
            .json()
            .await
            .map_err(|_| "Pairing code missing")?;
        let pair = request(
            &installation,
            Method::POST,
            "/api/v1/agent/pair",
            Some(json!({"code":code["code"],"label":"Sectile Tauri local"})),
        )
        .await?;
        installation.token = pair["token"]
            .as_str()
            .filter(|s| !s.is_empty())
            .ok_or("Pairing token missing")?
            .to_owned();
        installation.device_id = pair["deviceId"]
            .as_str()
            .ok_or("Workstation identity missing")?
            .to_owned();
        installation.user_id = profile["userId"]
            .as_str()
            .ok_or("Profile missing")?
            .to_owned();
        installation.email = email;
        save(&root, &installation)?;
    } else if email != installation.email {
        return Err(
            "The email address identifies this local profile and cannot be replaced here.".into(),
        );
    }
    request(
        &installation,
        Method::PATCH,
        "/api/me",
        Some(json!({"displayName":display_name})),
    )
    .await?;
    ensure_agent(&app, &root, &installation).await
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct CredentialInput {
    tracker: String,
    site_url: String,
    email: String,
    token: String,
}
fn remote_provider(value: &str) -> Result<(), String> {
    if ["github", "gitlab", "jira"].contains(&value) {
        Ok(())
    } else {
        Err("Unknown remote tracker".into())
    }
}
#[tauri::command]
pub async fn save_local_credential(
    app: tauri::AppHandle,
    input: CredentialInput,
) -> Result<(), String> {
    remote_provider(&input.tracker)?;
    if input.token.trim().is_empty() {
        return Err("Token is required".into());
    }
    let state = app.state::<LocalRuntime>();
    let _lock = state.0.lock().await;
    let root = root(&app)?;
    selected_request(&root,Method::PUT,"/api/me/tracker-credentials",Some(json!({"tracker":input.tracker,"siteUrl":input.site_url,"email":input.email,"token":input.token}))).await?;
    Ok(())
}
#[derive(Deserialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct TrackerInput {
    name: String,
    provider: String,
    site: String,
    scope: String,
    #[serde(default)]
    issue_types: Option<Vec<String>>,
}
fn edited_tracker(mut current: Value, input: TrackerInput) -> Result<Value, String> {
    let fields = current
        .as_object_mut()
        .ok_or("Tracker configuration unreadable")?;
    fields.insert("name".into(), json!(input.name));
    fields.insert("provider".into(), json!(input.provider));
    fields.insert("site".into(), json!(input.site));
    fields.insert("scope".into(), json!(input.scope));
    if let Some(issue_types) = input.issue_types {
        fields.insert("issueTypes".into(), json!(issue_types));
    }
    Ok(current)
}
#[tauri::command]
pub async fn tracker_issue_types(
    app: tauri::AppHandle,
    tracker_id: String,
) -> Result<Value, String> {
    if uuid::Uuid::parse_str(&tracker_id).is_err() {
        return Err("Invalid tracker ID".into());
    }
    let state = app.state::<LocalRuntime>();
    let _lock = state.0.lock().await;
    let root = root(&app)?;
    let route = format!("/api/admin/trackers/{tracker_id}");
    let tracker = selected_request(&root, Method::GET, &route, None).await?;
    let available = selected_request(&root, Method::GET, &format!("{route}/issue-types"), None)
        .await
        .unwrap_or_else(|_| json!([]));
    Ok(json!({
        "selected": tracker.get("issueTypes").cloned().unwrap_or_else(|| json!([])),
        "available": available,
    }))
}
#[tauri::command]
pub async fn create_local_tracker(
    app: tauri::AppHandle,
    input: TrackerInput,
) -> Result<Value, String> {
    remote_provider(&input.provider)?;
    let state = app.state::<LocalRuntime>();
    let _lock = state.0.lock().await;
    let root = root(&app)?;
    selected_request(&root,Method::POST,"/api/admin/trackers",Some(json!({"name":input.name,"provider":input.provider,"site":input.site,"scope":input.scope,"autoSyncEnabled":true,"autoSyncIntervalMin":5}))).await
}
#[tauri::command]
pub async fn update_local_tracker(
    app: tauri::AppHandle,
    tracker_id: String,
    input: TrackerInput,
) -> Result<Value, String> {
    if uuid::Uuid::parse_str(&tracker_id).is_err() {
        return Err("Invalid tracker ID".into());
    }
    remote_provider(&input.provider)?;
    if input.name.trim().is_empty() || input.scope.trim().is_empty() {
        return Err("Name and repository or project key required".into());
    }
    let state = app.state::<LocalRuntime>();
    let _lock = state.0.lock().await;
    let root = root(&app)?;
    let route = format!("/api/admin/trackers/{tracker_id}");
    // The Go endpoint replaces the whole tracker. Retain board mappings and
    // synchronization settings that this focused desktop form does not edit.
    let current = selected_request(&root, Method::GET, &route, None).await?;
    selected_request(&root, Method::PUT, &route, Some(edited_tracker(current, input)?))
        .await
        .map_err(|error| {
            if error.contains("HTTP 409") {
                "This tracker already contains tickets: its source can no longer change. Its name can still be updated.".into()
            } else {
                error
            }
        })
}
#[tauri::command]
pub async fn delete_local_tracker(
    app: tauri::AppHandle,
    tracker_id: String,
) -> Result<Value, String> {
    if uuid::Uuid::parse_str(&tracker_id).is_err() {
        return Err("Invalid tracker ID".into());
    }
    let state = app.state::<LocalRuntime>();
    let _lock = state.0.lock().await;
    let root = root(&app)?;
    if let Ok(Value::Array(projects)) = selected_request(&root, Method::GET, "/api/projects", None).await {
        let linked: Vec<String> = projects
            .iter()
            .filter(|project| {
                project["defaultTrackerId"].as_str() == Some(tracker_id.as_str())
                    || project["trackers"].as_array().is_some_and(|items| {
                        items
                            .iter()
                            .any(|item| item["trackerId"].as_str() == Some(tracker_id.as_str()))
                    })
            })
            .filter_map(|project| project["name"].as_str().map(str::to_owned))
            .collect();
        if !linked.is_empty() {
            return Err(format!(
                "This tracker is still attached to project \"{}\". Detach it or delete that project in the Projects section before deleting the tracker.",
                linked.join("\", \"")
            ));
        }
    }
    let delete_route = format!("/api/admin/trackers/{tracker_id}?purge=1");
    match selected_request(&root, Method::DELETE, &delete_route, None).await {
        Ok(value) => Ok(value),
        Err(error) if error.contains("HTTP 409") => {
            if let Ok(Value::Array(tasks)) =
                selected_request(&root, Method::GET, &format!("/api/trackers/{tracker_id}/backlog"), None).await
            {
                for task in tasks {
                    if let Some(task_id) = task["id"].as_str() {
                        let _ = selected_request(
                            &root,
                            Method::DELETE,
                            &format!("/api/tasks/{task_id}"),
                            None,
                        )
                        .await;
                    }
                }
            }
            selected_request(&root, Method::DELETE, &delete_route, None)
                .await
                .map_err(|err| {
                    if err.contains("HTTP 409") {
                        "This tracker is still attached to a project or contains active tickets. Detach it in the Projects section before deleting it.".into()
                    } else {
                        err
                    }
                })
        }
        Err(error) => Err(error),
    }
}
#[tauri::command]
pub async fn create_local_project(
    app: tauri::AppHandle,
    name: String,
    tracker_id: String,
) -> Result<Value, String> {
    if name.trim().is_empty() || tracker_id.is_empty() {
        return Err("Name and tracker required".into());
    }
    let state = app.state::<LocalRuntime>();
    let _lock = state.0.lock().await;
    let root = root(&app)?;
    selected_request(&root,Method::POST,"/api/projects",Some(json!({"name":name,"trackers":[{"trackerId":tracker_id}],"defaultTrackerId":tracker_id}))).await
}

#[tauri::command]
pub async fn delete_local_project(
    app: tauri::AppHandle,
    project_id: String,
) -> Result<Value, String> {
    if project_id.trim().is_empty() || project_id.len() > 1024 {
        return Err("Invalid project ID".into());
    }
    let runs = crate::agent::request_agent(
        app.clone(),
        Method::GET,
        "/desktop/runs",
        &[],
        None,
    )
    .await
    .unwrap_or_else(|_| json!([]));
    if let Some(items) = runs.as_array() {
        let mut conversations_to_stop = Vec::new();
        for run in items {
            if run["projectId"].as_str() != Some(project_id.as_str()) {
                continue;
            }
            let active = matches!(
                run["status"].as_str(),
                Some("running" | "queued" | "preparing")
            );
            if !active {
                continue;
            }
            if run["conversation"].as_bool() == Some(true) {
                if let Some(id) = run["id"].as_str() {
                    conversations_to_stop.push(id.to_owned());
                }
            } else {
                return Err(
                    "Stop active runs in this project before deleting it.".into(),
                );
            }
        }
        for run_id in conversations_to_stop {
            let _ = crate::agent::request_agent(
                app.clone(),
                Method::POST,
                "/desktop/runs/stop",
                &[],
                Some(json!({"id": run_id})),
            )
            .await;
        }
    }
    let _ = crate::agent::request_agent(
        app.clone(),
        Method::DELETE,
        "/desktop/projects",
        &[("id", &project_id)],
        None,
    )
    .await;
    let state = app.state::<LocalRuntime>();
    let _lock = state.0.lock().await;
    if let Ok(root) = root(&app) {
        let route = format!("/api/projects/{project_id}");
        if let Err(error) = selected_request(&root, Method::DELETE, &route, None).await {
            if error.contains("default project") {
                if let Ok(projects) = selected_request(&root, Method::GET, "/api/projects", None).await {
                    if let Some(other_id) = projects
                        .as_array()
                        .into_iter()
                        .flatten()
                        .filter_map(|item| item["id"].as_str())
                        .find(|id| *id != project_id)
                    {
                        let _ = selected_request(
                            &root,
                            Method::PUT,
                            &format!("/api/projects/{other_id}"),
                            Some(json!({"isDefault": true})),
                        )
                        .await;
                    }
                }
                let _ = selected_request(
                    &root,
                    Method::PUT,
                    &route,
                    Some(json!({"isDefault": false, "trackers": [], "defaultTrackerId": ""})),
                )
                .await;
                let _ = selected_request(&root, Method::DELETE, &route, None).await;
            }
        }
    }
    Ok(json!({"status": "deleted", "id": project_id}))
}

fn project_tracker_attachment(project: &Value, tracker_id: &str) -> Result<Option<Value>, String> {
    let existing = project["trackers"]
        .as_array()
        .ok_or("Project trackers list unreadable")?;
    let mut trackers = Vec::with_capacity(existing.len() + 1);
    for entry in existing {
        let id = entry["trackerId"]
            .as_str()
            .filter(|id| !id.is_empty())
            .ok_or("Existing project tracker unreadable")?;
        if id == tracker_id {
            return Ok(None);
        }
        trackers.push(json!({"trackerId": id}));
    }
    trackers.push(json!({"trackerId": tracker_id}));
    let current_default = project["defaultTrackerId"].as_str().unwrap_or("");
    let local_default = existing
        .iter()
        .any(|entry| entry["trackerId"] == current_default && entry["provider"] == "local");
    let default_id = if current_default.is_empty()
        || local_default && existing.iter().all(|entry| entry["provider"] == "local")
    {
        tracker_id
    } else {
        current_default
    };
    Ok(Some(
        json!({"trackers":trackers,"defaultTrackerId":default_id}),
    ))
}

#[cfg(test)]
mod project_tracker_attachment_tests {
    use super::*;

    #[test]
    fn keeps_local_board_and_selects_first_remote_tracker_for_new_tickets() {
        let project = json!({"defaultTrackerId":"local-board","trackers":[{"trackerId":"local-board","provider":"local"}]});
        let body = project_tracker_attachment(&project, "remote")
            .unwrap()
            .unwrap();
        assert_eq!(
            body,
            json!({"trackers":[{"trackerId":"local-board"},{"trackerId":"remote"}],"defaultTrackerId":"remote"})
        );
    }

    #[test]
    fn keeps_existing_remote_default_when_adding_another_tracker() {
        let project = json!({"defaultTrackerId":"first","trackers":[{"trackerId":"local-board","provider":"local"},{"trackerId":"first","provider":"github"}]});
        let body = project_tracker_attachment(&project, "second")
            .unwrap()
            .unwrap();
        assert_eq!(body["defaultTrackerId"], "first");
        assert_eq!(body["trackers"].as_array().unwrap().len(), 3);
        assert!(
            project_tracker_attachment(&project, "first")
                .unwrap()
                .is_none()
        );
    }
}

#[tauri::command]
pub async fn attach_project_tracker(
    app: tauri::AppHandle,
    project_id: String,
    tracker_id: String,
) -> Result<Value, String> {
    if uuid::Uuid::parse_str(&project_id).is_err() || uuid::Uuid::parse_str(&tracker_id).is_err() {
        return Err("Invalid project or tracker ID".into());
    }
    let state = app.state::<LocalRuntime>();
    let _lock = state.0.lock().await;
    let root = root(&app)?;
    let route = format!("/api/projects/{project_id}");
    let project = selected_request(&root, Method::GET, &route, None).await?;
    let Some(body) = project_tracker_attachment(&project, &tracker_id)? else {
        return Ok(project);
    };
    selected_request(&root, Method::PUT, &route, Some(body)).await
}
#[tauri::command]
pub async fn sync_local_tracker(
    app: tauri::AppHandle,
    tracker_id: String,
) -> Result<Value, String> {
    if uuid::Uuid::parse_str(&tracker_id).is_err() {
        return Err("Invalid tracker ID".into());
    }
    let state = app.state::<LocalRuntime>();
    let _lock = state.0.lock().await;
    let root = root(&app)?;
    selected_request(
        &root,
        Method::POST,
        &format!("/api/trackers/{tracker_id}/sync"),
        Some(json!({})),
    )
    .await
}

#[tauri::command]
pub async fn tracker_sync_status(
    app: tauri::AppHandle,
    activity_id: String,
) -> Result<Value, String> {
    if uuid::Uuid::parse_str(&activity_id).is_err() {
        return Err("Invalid sync ID".into());
    }
    let state = app.state::<LocalRuntime>();
    let _lock = state.0.lock().await;
    let root = root(&app)?;
    selected_request(
        &root,
        Method::GET,
        &format!("/api/activities/{activity_id}"),
        None,
    )
    .await
}

#[tauri::command]
pub async fn active_tracker_syncs(app: tauri::AppHandle) -> Result<Vec<Value>, String> {
    let state = app.state::<LocalRuntime>();
    let _lock = state.0.lock().await;
    let root = root(&app)?;
    let mut active = Vec::new();
    let mut seen = std::collections::HashSet::new();
    for status in ["queued", "running"] {
        let activities = selected_request(
            &root,
            Method::GET,
            &format!("/api/activities?status={status}&limit=200"),
            None,
        )
        .await?;
        if let Some(items) = activities.as_array() {
            for item in items {
                let id = item["id"].as_str().unwrap_or_default();
                let tracker_id = item["trackerId"].as_str().unwrap_or_default();
                let skill = item["skillId"].as_str().unwrap_or_default();
                if !id.is_empty()
                    && !tracker_id.is_empty()
                    && skill.starts_with("sync_")
                    && seen.insert(id.to_owned())
                {
                    active.push(json!({
                        "id": id,
                        "trackerId": tracker_id,
                        "status": item["status"],
                    }));
                }
            }
        }
    }
    Ok(active)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn pairing_fixture(status: &str, payload: &str) -> (String, std::sync::mpsc::Receiver<String>) {
        use std::io::Read;
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let address = listener.local_addr().unwrap();
        let (sender, receiver) = std::sync::mpsc::channel();
        let status = status.to_owned();
        let payload = payload.to_owned();
        std::thread::spawn(move || {
            let (mut stream, _) = listener.accept().unwrap();
            let mut request = Vec::new();
            let mut chunk = [0; 1024];
            loop {
                let count = stream.read(&mut chunk).unwrap();
                if count == 0 {
                    break;
                }
                request.extend_from_slice(&chunk[..count]);
                let text = String::from_utf8_lossy(&request);
                if let Some((headers, body)) = text.split_once("\r\n\r\n") {
                    let content_length = headers
                        .lines()
                        .find_map(|line| {
                            let (name, value) = line.split_once(':')?;
                            name.eq_ignore_ascii_case("content-length")
                                .then(|| value.trim().parse::<usize>().ok())
                                .flatten()
                        })
                        .unwrap_or(0);
                    if body.len() >= content_length {
                        break;
                    }
                }
            }
            let _ = sender.send(String::from_utf8_lossy(&request).into_owned());
            let response = format!(
                "HTTP/1.1 {status}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{payload}",
                payload.len()
            );
            use std::io::Write;
            stream.write_all(response.as_bytes()).unwrap();
        });
        (format!("http://{address}"), receiver)
    }

    fn run_async<T>(future: impl std::future::Future<Output = T>) -> T {
        tokio::runtime::Builder::new_current_thread()
            .enable_all()
            .build()
            .unwrap()
            .block_on(future)
    }

    #[test]
    fn fleet_pairing_posts_the_one_time_code_and_decodes_the_backend_contract() {
        let (server, request) = pairing_fixture(
            "201 Created",
            r#"{"token":"api-token","deviceId":"device-1","userId":"user-1"}"#,
        );
        let result = run_async(exchange_fleet_pairing(
            &client().unwrap(),
            &server,
            "single-use-code",
        ))
        .unwrap();
        assert_eq!(result.api_token, "api-token");
        assert_eq!(result.device_id, "device-1");
        assert_eq!(result.user_id, "user-1");
        let request = request.recv_timeout(Duration::from_secs(2)).unwrap();
        assert!(request.starts_with("POST /api/v1/agent/pair HTTP/1.1\r\n"));
        assert!(request.contains(r#""code":"single-use-code""#));
        assert!(request.contains(r#""label":"Sectile Desktop""#));
    }

    #[test]
    fn fleet_pairing_reports_expired_codes_and_rejects_incomplete_contracts() {
        for (status, payload, expected) in [
            (
                "401 Unauthorized",
                r#"{"error":"expired"}"#,
                "Invalid or expired pairing code",
            ),
            (
                "201 Created",
                r#"{"token":"api-token","deviceId":"device-1"}"#,
                "valid pairing",
            ),
            ("503 Service Unavailable", r#"{}"#, "HTTP 503"),
        ] {
            let (server, _) = pairing_fixture(status, payload);
            let result = run_async(exchange_fleet_pairing(
                &client().unwrap(),
                &server,
                "single-use-code",
            ));
            let error = match result {
                Err(error) => error,
                Ok(_) => panic!("pairing unexpectedly accepted an invalid response"),
            };
            assert!(
                error.contains(expected),
                "{error:?} did not contain {expected:?}"
            );
        }
    }

    #[test]
    fn fleet_api_requests_use_the_paired_key_without_leaking_it_in_errors() {
        let (server, request) = pairing_fixture("200 OK", r#"{"signedIn":true}"#);
        let root = std::env::temp_dir().join(format!("sectile-fleet-api-{}", uuid::Uuid::new_v4()));
        let credentials = FleetCredentials {
            server: server.clone(),
            api_key: "private-api-key".into(),
            device_id: "device-1".into(),
        };
        let runtime = FleetRuntime {
            server: server.clone(),
            desktop_token: "private-desktop-token".into(),
            user_id: "user-1".into(),
        };
        write_fleet_credentials(&root, &credentials, &runtime).unwrap();
        save_mode(&root, RuntimeMode::Fleet, Some(&server)).unwrap();
        let response = run_async(fleet_request(&root, Method::GET, "/api/me", None)).unwrap();
        assert_eq!(response["signedIn"], true);
        let request = request.recv_timeout(Duration::from_secs(2)).unwrap();
        assert!(request.starts_with("GET /api/me HTTP/1.1\r\n"));
        assert!(
            request
                .lines()
                .any(|line| line.eq_ignore_ascii_case("authorization: Bearer private-api-key"))
        );
        fs::remove_dir_all(root).unwrap();
    }

    #[test]
    fn installation_credentials_are_private_and_round_trip() {
        let root = std::env::temp_dir().join(format!("sectile-local-{}", uuid::Uuid::new_v4()));
        private_dir(&root).unwrap();
        let installation = Installation {
            port: 12345,
            key: uuid::Uuid::new_v4().to_string(),
            token: "private-test-token".into(),
            device_id: "device".into(),
            agent_device_id: String::new(),
            user_id: "user".into(),
            email: "test@example.invalid".into(),
        };
        save(&root, &installation).unwrap();
        assert_eq!(read_installation(&root).unwrap().token, installation.token);
        save(&root, &installation).unwrap();
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            assert_eq!(
                fs::metadata(root.join("installation.json"))
                    .unwrap()
                    .permissions()
                    .mode()
                    & 0o777,
                0o600
            );
            assert_eq!(
                fs::metadata(&root).unwrap().permissions().mode() & 0o777,
                0o700
            );
        }
        fs::remove_dir_all(root).unwrap();
    }
    #[test]
    fn runtime_startup_is_serialized_across_process_handles() {
        let root = std::env::temp_dir().join(format!("sectile-lock-{}", uuid::Uuid::new_v4()));
        private_dir(&root).unwrap();
        let first = installation_lock(&root).unwrap();
        assert!(installation_lock(&root).is_err());
        drop(first);
        drop(installation_lock(&root).unwrap());
        fs::remove_dir_all(root).unwrap();
    }
    #[test]
    fn setup_rejects_local_boards_and_unknown_credential_fields() {
        assert!(remote_provider("local").is_err());
        assert!(remote_provider("github").is_ok());
        assert!(serde_json::from_value::<CredentialInput>(json!({"tracker":"github","siteUrl":"","email":"","token":"x","url":"https://unexpected.example"})).is_err());
    }
}

// Return presence flags only; saved credentials never populate renderer fields.
fn public_oauth_app(value: &Value) -> Value {
    json!({
        "configured":value["configured"] == true,
        "clientIdSet":value["clientId"].as_str().is_some_and(|id| !id.trim().is_empty()),
        "secretSet":value["secretSet"] == true,
        "redirectUrl":value["redirectUrl"],
        "expectedRedirectUrl":value["expectedRedirectUrl"],
    })
}
#[tauri::command]
pub async fn oauth_apps(app: tauri::AppHandle) -> Result<Value, String> {
    let root = root(&app)?;
    let mut apps = serde_json::Map::new();
    if runtime_mode(&app)? == Some(RuntimeMode::Fleet) {
        let configured =
            selected_request(&root, Method::GET, "/api/desktop-oauth/apps", None).await?;
        for provider in ["github", "gitlab", "jira"] {
            apps.insert(provider.into(), public_oauth_app(&configured[provider]));
        }
        return Ok(Value::Object(apps));
    }
    let installation = read_installation(&root)?;
    for provider in ["github", "gitlab", "jira"] {
        let value = request(
            &installation,
            Method::GET,
            &format!("/api/admin/{provider}-oauth"),
            None,
        )
        .await?;
        apps.insert(provider.into(), public_oauth_app(&value));
    }
    Ok(Value::Object(apps))
}
#[tauri::command]
pub async fn save_oauth_app(
    app: tauri::AppHandle,
    provider: String,
    client_id: String,
    client_secret: String,
) -> Result<(), String> {
    remote_provider(&provider)?;
    let state = app.state::<LocalRuntime>();
    let _lock = state.0.lock().await;
    let root = root(&app)?;
    if runtime_mode(&app)? == Some(RuntimeMode::Fleet) {
        return Err(
            "OAuth applications are managed by the Fleet backend administrator.".into(),
        );
    }
    let installation = read_installation(&root)?;
    let client_id = if client_id.trim().is_empty() {
        let current = request(
            &installation,
            Method::GET,
            &format!("/api/admin/{provider}-oauth"),
            None,
        )
        .await?;
        current["clientId"]
            .as_str()
            .filter(|id| !id.trim().is_empty())
            .ok_or("Application client ID required")?
            .to_owned()
    } else {
        client_id
    };
    request(
        &installation,
        Method::PUT,
        &format!("/api/admin/{provider}-oauth"),
        Some(json!({"clientId":client_id,"clientSecret":client_secret})),
    )
    .await?;
    Ok(())
}
// Only a server-issued one-use ticket can open a browser. No arbitrary URL or
// program reaches this native command from the renderer.
fn browser_ticket_url(server: &str, value: &Value) -> Result<String, String> {
    let path = value["startPath"]
        .as_str()
        .ok_or("Sign-in link missing")?;
    let base = reqwest::Url::parse(server).map_err(|_| "Invalid sign-in link")?;
    let parsed = base.join(path).map_err(|_| "Invalid sign-in link")?;
    let pairs: Vec<_> = parsed.query_pairs().collect();
    if parsed.origin() != base.origin()
        || parsed.path() != "/auth/desktop-oauth/start"
        || parsed.fragment().is_some()
        || pairs.len() != 1
        || pairs[0].0 != "ticket"
        || uuid::Uuid::parse_str(&pairs[0].1).is_err()
    {
        return Err("Sign-in link refused".into());
    }
    Ok(parsed.to_string())
}
#[tauri::command]
pub async fn begin_oauth(app: tauri::AppHandle, provider: String) -> Result<String, String> {
    use tauri_plugin_opener::OpenerExt;
    remote_provider(&provider)?;
    let root = root(&app)?;
    let mode = runtime_mode(&app)?;
    let base = match mode {
        Some(RuntimeMode::Fleet) => selected_fleet_server(&root)?,
        Some(RuntimeMode::Local) => read_installation(&root)?.url(),
        None => return Err("Select a runtime mode in settings.".into()),
    };
    let configured = if mode == Some(RuntimeMode::Fleet) {
        let apps = selected_request(&root, Method::GET, "/api/desktop-oauth/apps", None).await?;
        let app = &apps[provider.as_str()];
        app["configured"] == true
            && app["redirectUrl"]
                .as_str()
                .is_some_and(|url| !url.is_empty())
    } else {
        let app = selected_request(
            &root,
            Method::GET,
            &format!("/api/admin/{provider}-oauth"),
            None,
        )
        .await?;
        app["configured"] == true
            && app["expectedRedirectUrl"]
                .as_str()
                .is_some_and(|url| !url.is_empty())
            && app["redirectUrl"] == app["expectedRedirectUrl"]
    };
    if !configured {
        return Err(
            "Configure the OAuth application on the backend before signing in.".into(),
        );
    }
    let flow = selected_request(
        &root,
        Method::POST,
        "/api/desktop-oauth",
        Some(json!({"provider":provider})),
    )
    .await?;
    let id = flow["id"]
        .as_str()
        .ok_or("Incomplete sign-in")?
        .to_owned();
    if uuid::Uuid::parse_str(&id).is_err() {
        return Err("Invalid sign-in ID".into());
    }
    let url = browser_ticket_url(&base, &flow)?;
    if app.opener().open_url(url, None::<&str>).is_err() {
        let _ = selected_request(
            &root,
            Method::DELETE,
            &format!("/api/desktop-oauth?id={id}"),
            None,
        )
        .await;
        return Err("Unable to open browser. Try again from settings.".into());
    }
    Ok(id)
}
#[tauri::command]
pub async fn oauth_status(app: tauri::AppHandle, id: String) -> Result<Value, String> {
    if uuid::Uuid::parse_str(&id).is_err() {
        return Err("Invalid sign-in ID".into());
    }
    let root = root(&app)?;
    selected_request(
        &root,
        Method::GET,
        &format!("/api/desktop-oauth?id={id}"),
        None,
    )
    .await
}
#[tauri::command]
pub async fn cancel_oauth(app: tauri::AppHandle, id: String) -> Result<(), String> {
    if uuid::Uuid::parse_str(&id).is_err() {
        return Err("Invalid sign-in ID".into());
    }
    let root = root(&app)?;
    selected_request(
        &root,
        Method::DELETE,
        &format!("/api/desktop-oauth?id={id}"),
        None,
    )
    .await?;
    Ok(())
}

#[cfg(test)]
mod oauth_url_tests {
    use super::*;
    #[test]
    fn browser_opening_accepts_only_one_use_tickets_from_the_selected_server() {
        let local_server = "http://127.0.0.1:45678";
        let fleet_server = "https://fleet.example.invalid";
        let ticket = uuid::Uuid::new_v4();
        assert!(
            browser_ticket_url(
                local_server,
                &json!({"startPath":format!("/auth/desktop-oauth/start?ticket={ticket}")})
            )
            .is_ok()
        );
        assert!(
            browser_ticket_url(
                fleet_server,
                &json!({"startPath":format!("/auth/desktop-oauth/start?ticket={ticket}")})
            )
            .is_ok()
        );
        for path in [
            "https://evil.example/",
            "//evil.example/",
            "/auth/desktop-oauth/start?ticket=bad",
            "/auth/desktop-oauth/start?ticket=bad&url=https://evil.example",
            "/api/me",
        ] {
            assert!(browser_ticket_url(fleet_server, &json!({"startPath":path})).is_err());
        }
    }
}

pub(crate) async fn pick_project_folder(app: &tauri::AppHandle) -> Result<Option<String>, String> {
    use tauri_plugin_dialog::DialogExt;
    let handle = app.clone();
    let selected = tauri::async_runtime::spawn_blocking(move || {
        handle
            .dialog()
            .file()
            .set_title("Select project folder")
            .blocking_pick_folder()
    })
    .await
    .map_err(|_| "Unable to open folder picker")?;
    let Some(selected) = selected else {
        return Ok(None);
    };
    let path = selected.into_path().map_err(|_| "Local folder required")?;
    let path = std::fs::canonicalize(path).map_err(|_| "Folder not found")?;
    if !path.is_dir() {
        return Err("Select a folder".into());
    }
    let path = path.to_str().ok_or("Unsupported path")?.to_owned();
    Ok(Some(path))
}

#[tauri::command]
pub async fn choose_project_folder(
    app: tauri::AppHandle,
    project_id: String,
) -> Result<Option<String>, String> {
    if project_id.trim().is_empty() {
        return Err("Project required".into());
    }
    if connection_path(&app).is_none() {
        return Err("Connect this workstation's agent before attaching a folder".into());
    }
    let Some(path) = pick_project_folder(&app).await? else {
        return Ok(None);
    };
    // Associate the existing project; choosing a folder must never create another one.
    crate::agent::retry_folder_project(app, project_id, path.clone()).await?;
    Ok(Some(path))
}

#[tauri::command]
pub async fn open_folder_project(app: tauri::AppHandle) -> Result<Option<Value>, String> {
    resume(&app).await?;
    let Some(path) = pick_project_folder(&app).await? else {
        return Ok(None);
    };
    let name = std::path::Path::new(&path)
        .file_name()
        .and_then(|v| v.to_str())
        .unwrap_or("Local project")
        .to_owned();
    let state = app.state::<LocalRuntime>();
    let _lock = state.0.lock().await;
    let root = root(&app)?;
    // Reopening a mapped folder selects its existing project.
    let projects = crate::agent::list_projects(app.clone()).await?;
    if let Some(project) = projects.as_array().and_then(|items| {
        items
            .iter()
            .find(|p| p.get("path").and_then(Value::as_str) == Some(path.as_str()))
    }) {
        return Ok(Some(project.clone()));
    }
    let body = Some(json!({"name":name,"trackers":[],"folderOnly":true}));
    let project = match runtime_mode(&app)? {
        Some(RuntimeMode::Fleet) => {
            fleet_request(&root, Method::POST, "/api/projects", body).await?
        }
        Some(RuntimeMode::Local) => {
            let installation = read_installation(&root)?;
            request(&installation, Method::POST, "/api/projects", body).await?
        }
        None => return Err("Select a runtime mode before creating a project".into()),
    };
    let id = project
        .get("id")
        .and_then(Value::as_str)
        .ok_or("Project ID missing")?;
    if let Err(error) = crate::agent::map_folder_project(app.clone(), id, &path).await {
        return Ok(Some(
            json!({"id":id,"name":name,"path":path,"mappingError":error}),
        ));
    }
    Ok(Some(json!({"id":id,"name":name,"path":path})))
}

#[tauri::command]
pub async fn choose_attached_folder(
    app: tauri::AppHandle,
    project_id: String,
    run_id: Option<String>,
) -> Result<Option<Value>, String> {
    if project_id.trim().is_empty() {
        return Err("Project required".into());
    }
    let Some(path) = pick_project_folder(&app).await? else {
        return Ok(None);
    };
    let answer =
        crate::agent::attach_chosen_folder(app, &project_id, run_id.as_deref(), &path).await?;
    Ok(Some(
        json!({"path":path,"mappedAs":answer.get("mappedAs"),"appliesAt":answer.get("appliesAt").and_then(Value::as_str).unwrap_or("next-launch")}),
    ))
}

#[cfg(test)]
mod oauth_privacy_tests {
    use super::*;
    #[test]
    fn saved_credentials_are_replaced_by_presence_flags() {
        let state = public_oauth_app(
            &json!({"configured":true,"clientId":"private-client-id","secretSet":true,"clientSecret":"must-never-cross-ipc","expectedRedirectUrl":"http://127.0.0.1/callback"}),
        );
        assert_eq!(state["clientIdSet"], true);
        assert_eq!(state["secretSet"], true);
        assert!(state.get("clientId").is_none());
        assert!(!state.to_string().contains("private-client-id"));
        assert!(!state.to_string().contains("must-never-cross-ipc"));
    }
}

#[cfg(test)]
mod generation_tests {
    #[test]
    fn paired_fleet_profile_must_match_the_original_user() {
        let profile = serde_json::json!({"signedIn":true,"userId":"user-1"});
        assert!(super::validate_fleet_profile_identity(&profile, "user-1").is_ok());
        assert!(super::validate_fleet_profile_identity(&profile, "user-2").is_err());
        assert!(
            super::validate_fleet_profile_identity(
                &serde_json::json!({"signedIn":false,"userId":"user-1"}),
                "user-1"
            )
            .is_err()
        );
    }

    #[test]
    fn selected_local_or_fleet_runtime_cannot_be_replaced_by_pairing_or_attachment() {
        assert!(super::require_unconfigured_runtime(None).is_ok());
        assert!(super::require_unconfigured_runtime(Some(super::RuntimeMode::Local)).is_err());
        assert!(super::require_unconfigured_runtime(Some(super::RuntimeMode::Fleet)).is_err());
    }

    #[test]
    fn selected_runtime_keeps_a_private_connection_identity_across_restart_and_legacy_upgrade() {
        let root = std::env::temp_dir().join(format!("sectile-identity-{}", uuid::Uuid::new_v4()));
        private_dir(&root).unwrap();
        save_mode(&root, RuntimeMode::Fleet, Some("https://fleet.example.com")).unwrap();
        let first = ensure_runtime_connection_id(&root).unwrap();
        assert_eq!(ensure_runtime_connection_id(&root).unwrap(), first);

        let mut legacy: Value =
            serde_json::from_slice(&fs::read(mode_path(&root)).unwrap()).unwrap();
        legacy.as_object_mut().unwrap().remove("connectionId");
        fs::write(mode_path(&root), serde_json::to_vec(&legacy).unwrap()).unwrap();
        let adopted = ensure_runtime_connection_id(&root).unwrap();
        assert_ne!(adopted, "");
        assert_eq!(ensure_runtime_connection_id(&root).unwrap(), adopted);
        let saved: Value = serde_json::from_slice(&fs::read(mode_path(&root)).unwrap()).unwrap();
        assert_eq!(saved["server"], "https://fleet.example.com");
        assert_eq!(saved["connectionId"], adopted);
        fs::remove_dir_all(root).unwrap();
    }

    use super::*;

    #[test]
    fn runtime_modes_resolve_only_their_private_connection() {
        let root = std::env::temp_dir().join(format!("sectile-mode-{}", uuid::Uuid::new_v4()));
        private_dir(&root).unwrap();
        fs::write(
            root.join("agent-connection.json"),
            r#"{"url":"http://127.0.0.1:1","token":"local"}"#,
        )
        .unwrap();
        assert_eq!(connection_file(&root, RuntimeMode::Local), None);
        assert_eq!(connection_file(&root, RuntimeMode::Fleet), None);

        fs::write(root.join("installation.json"), "{}").unwrap();
        assert_eq!(
            connection_file(&root, RuntimeMode::Local),
            Some(root.join("agent-connection.json"))
        );
        fs::create_dir_all(root.join("fleet")).unwrap();
        fs::write(
            root.join("fleet/agent-connection.json"),
            r#"{"url":"http://127.0.0.1:2","token":"fleet"}"#,
        )
        .unwrap();
        assert_eq!(
            connection_file(&root, RuntimeMode::Fleet),
            Some(root.join("fleet/agent-connection.json"))
        );
        fs::remove_dir_all(root).unwrap();
    }

    #[test]
    fn runtime_mode_record_is_private_and_idempotent() {
        let root = std::env::temp_dir().join(format!("sectile-mode-{}", uuid::Uuid::new_v4()));
        save_mode(
            &root,
            RuntimeMode::Fleet,
            Some("https://fleet.example.invalid"),
        )
        .unwrap();
        let path = mode_path(&root);
        let original = fs::read(&path).unwrap();
        save_mode(
            &root,
            RuntimeMode::Fleet,
            Some("https://fleet.example.invalid"),
        )
        .unwrap();
        assert_eq!(fs::read(&path).unwrap(), original);
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            assert_eq!(
                fs::metadata(&path).unwrap().permissions().mode() & 0o777,
                0o600
            );
        }
        fs::remove_dir_all(root).unwrap();
    }

    #[test]
    fn fleet_pairing_requires_tls_except_for_loopback() {
        assert_eq!(
            normalize_fleet_server_url(" https://fleet.example.invalid/ ").unwrap(),
            "https://fleet.example.invalid"
        );
        assert_eq!(
            normalize_fleet_server_url("http://127.0.0.1:8080").unwrap(),
            "http://127.0.0.1:8080"
        );
        for invalid in [
            "http://fleet.example.invalid",
            "https://user:password@fleet.example.invalid",
            "https://fleet.example.invalid/prefix",
            "https://fleet.example.invalid/?next=https://attacker.invalid",
        ] {
            assert!(
                normalize_fleet_server_url(invalid).is_err(),
                "accepted {invalid}"
            );
        }
    }

    #[test]
    fn fleet_credentials_use_the_agent_settings_contract_and_private_files() {
        let root = std::env::temp_dir().join(format!("sectile-fleet-{}", uuid::Uuid::new_v4()));
        let fleet_root = root.join("fleet");
        let credentials = FleetCredentials {
            server: "https://fleet.example.invalid".into(),
            api_key: "private-api-key".into(),
            device_id: "device-1".into(),
        };
        let runtime = FleetRuntime {
            server: credentials.server.clone(),
            desktop_token: "private-desktop-token".into(),
            user_id: "user-1".into(),
        };
        write_fleet_credentials(&root, &credentials, &runtime).unwrap();
        let settings: Value =
            serde_json::from_slice(&fs::read(fleet_root.join("settings.json")).unwrap()).unwrap();
        assert_eq!(settings["apiKey"], "private-api-key");
        assert!(settings.get("apiToken").is_none());
        let runtime_value: Value =
            serde_json::from_slice(&fs::read(fleet_root.join("fleet-runtime.json")).unwrap())
                .unwrap();
        assert_eq!(runtime_value["desktopToken"], "private-desktop-token");
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            assert_eq!(
                fs::metadata(fleet_root.join("settings.json"))
                    .unwrap()
                    .permissions()
                    .mode()
                    & 0o777,
                0o600
            );
            assert_eq!(
                fs::metadata(fleet_root).unwrap().permissions().mode() & 0o777,
                0o700
            );
        }
        fs::remove_dir_all(root).unwrap();
    }

    #[test]
    fn committed_generation_owns_reads_and_profile_writes() {
        let root = std::env::temp_dir().join(format!("sectile-selection-{}", uuid::Uuid::new_v4()));
        private_dir(&root).unwrap();
        let mut installation = Installation {
            port: 52963,
            key: "01234567890123456789012345678901".into(),
            token: "old-token".into(),
            device_id: "device".into(),
            agent_device_id: String::new(),
            user_id: "user".into(),
            email: "owner@example.com".into(),
        };
        save(&root, &installation).unwrap();
        let original = fs::read(root.join("installation.json")).unwrap();
        let candidate = root.join("generations/companion-v1");
        private_dir(&candidate).unwrap();
        fs::write(candidate.join("installation.json"), &original).unwrap();
        fs::write(
            root.join("runtime-selection.json"),
            r#"{"protocol":1,"kind":"companion","generation":"generations/companion-v1"}"#,
        )
        .unwrap();
        installation.token = "new-token".into();
        save(&root, &installation).unwrap();
        assert_eq!(read_installation(&root).unwrap().token, "new-token");
        assert_eq!(fs::read(root.join("installation.json")).unwrap(), original);
        fs::remove_dir_all(root).unwrap();
    }

    #[test]
    fn invalid_selection_never_falls_back_to_the_source() {
        let root = std::env::temp_dir().join(format!("sectile-selection-{}", uuid::Uuid::new_v4()));
        private_dir(&root).unwrap();
        fs::write(
            root.join("runtime-selection.json"),
            r#"{"protocol":1,"kind":"companion","generation":"../../other"}"#,
        )
        .unwrap();
        assert!(active_root(&root).is_err());
        fs::remove_dir_all(root).unwrap();
    }
}

#[cfg(test)]
mod fleet_mcp_tests {
    use super::*;

    #[test]
    fn managed_fleet_uses_a_private_endpoint_without_global_mcp_refresh() {
        let mut command = Command::new("fixture-agent");
        command.env_clear();
        scope_fleet_mcp(&mut command, "https://fleet.example.test");
        let environment: std::collections::HashMap<_, _> = command.get_envs().collect();
        assert_eq!(
            environment.get(std::ffi::OsStr::new("SECTILE_DESKTOP_MCP_URL")),
            Some(&Some(std::ffi::OsStr::new("https://fleet.example.test")))
        );
        assert_eq!(
            environment.get(std::ffi::OsStr::new("SECTILE_DISABLE_MCP_REFRESH")),
            Some(&Some(std::ffi::OsStr::new("1")))
        );
        assert_eq!(command.get_args().count(), 0);
    }
}

#[cfg(test)]
mod update_preparation_tests {
    use super::validate_update_result;

    #[test]
    fn update_requires_both_successful_exit_and_explicit_readiness() {
        assert!(validate_update_result(true, br#"{"ready":true}"#).is_ok());
        for (success, payload) in [
            (false, br#"{"ready":true}"#.as_slice()),
            (true, br#"{"ready":false}"#.as_slice()),
            (true, b"not json".as_slice()),
        ] {
            assert!(validate_update_result(success, payload).is_err());
        }
    }

    #[test]
    fn update_errors_use_safe_guidance_instead_of_service_output() {
        assert!(
            validate_update_result(false, br#"{"reason":"busy"}"#)
                .unwrap_err()
                .contains("active runs")
        );
        let error =
            validate_update_result(false, br#"{"reason":"private-credential"}"#).unwrap_err();
        assert!(!error.contains("private-credential"));
    }
}

#[cfg(test)]
mod tracker_edit_tests {
    use super::{TrackerInput, edited_tracker};
    use serde_json::json;

    #[test]
    fn editing_source_fields_keeps_board_and_sync_settings() {
        let previous = json!({"name":"Old","provider":"gitlab","site":"https://gitlab.com/api/v4",
            "scope":"team/old","boardId":"board-9","stageColumns":{"active":["Doing"]},
            "autoSyncEnabled":false,"autoSyncIntervalMin":30,"ticketCount":4});
        let updated = edited_tracker(previous, TrackerInput {
            name: "New".into(), provider: "gitlab".into(),
            site: "https://gitlab.com/api/v4".into(), scope: "team/old".into(),
            issue_types: None,
        }).unwrap();
        assert_eq!(updated["name"], "New");
        assert_eq!(updated["boardId"], "board-9");
        assert_eq!(updated["stageColumns"]["active"], json!(["Doing"]));
        assert_eq!(updated["autoSyncEnabled"], false);
        assert_eq!(updated["autoSyncIntervalMin"], 30);
        assert_eq!(updated["ticketCount"], 4);
    }
}
