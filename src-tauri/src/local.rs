use reqwest::{Client, Method};
use serde::{Deserialize, Serialize};
use serde_json::{Value, json};
use std::{
    fs,
    io::Write,
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
        .map_err(|_| "Dossier de données indisponible".into())
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
        .map_err(|_| "Écriture du mode refusée")?;
    file.write_all(&serde_json::to_vec(&record).map_err(|_| "Mode invalide")?)
        .map_err(|_| "Écriture du mode interrompue")?;
    file.sync_all()
        .map_err(|_| "Enregistrement du mode interrompu")?;
    fs::rename(next, path).map_err(|_| "Enregistrement du mode refusé".into())
}

fn ensure_runtime_connection_id(root: &Path) -> Result<String, String> {
    let mode = runtime_mode_from_root(root)?
        .or_else(|| {
            active_root(root)
                .ok()
                .filter(|path| path.join("installation.json").exists())
                .map(|_| RuntimeMode::Local)
        })
        .ok_or_else(|| "Choisissez un mode d’exécution dans les paramètres.".to_string())?;
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
        serde_json::from_slice(&fs::read(path).map_err(|_| "Identité de connexion indisponible")?)
            .map_err(|_| "Identité de connexion invalide")?;
    value["connectionId"]
        .as_str()
        .map(str::to_owned)
        .ok_or_else(|| "Identité de connexion indisponible".into())
}

pub fn runtime_connection_id(app: &tauri::AppHandle) -> Result<String, String> {
    ensure_runtime_connection_id(&root(app)?)
}

fn selected_fleet_server(root: &Path) -> Result<String, String> {
    let value: Value =
        serde_json::from_slice(&fs::read(mode_path(root)).map_err(|_| "Mode Fleet introuvable")?)
            .map_err(|_| "Mode Fleet illisible")?;
    value["server"]
        .as_str()
        .filter(|server| !server.is_empty())
        .map(str::to_owned)
        .ok_or_else(|| "Serveur Fleet absent".into())
}

fn normalize_fleet_server_url(input: &str) -> Result<String, String> {
    let parsed =
        reqwest::Url::parse(input.trim()).map_err(|_| "Adresse du serveur Fleet invalide")?;
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
            "Utilisez l’adresse HTTPS du backend Fleet, sans identifiants ni chemin.".into(),
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
        serde_json::from_slice(&fs::read(path).map_err(|_| "Sélection du service illisible")?)
            .map_err(|_| "Sélection du service invalide")?;
    if value["protocol"] != 1
        || value["kind"] != "companion"
        || value["generation"] != "generations/companion-v1"
    {
        return Err("Version du service local incompatible".into());
    }
    Ok(root.join("generations/companion-v1"))
}

fn fleet_root(root: &Path) -> Result<PathBuf, String> {
    Ok(active_root(root)?.join("fleet"))
}

pub fn discovery_path(app: &tauri::AppHandle) -> Option<PathBuf> {
    if runtime_mode(app).ok().flatten() != Some(RuntimeMode::Local) {
        return None;
    }
    connection_path(app)
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
        .map_err(|_| "Verrou local indisponible")?;
    file.try_lock()
        .map_err(|_| "Une autre fenêtre prépare les services locaux. Réessayez dans un instant.")?;
    Ok(file)
}
fn read_installation(root: &Path) -> Result<Installation, String> {
    let root = active_root(root)?;
    let bytes = fs::read(root.join("installation.json"))
        .map_err(|_| "Activez d’abord l’installation locale")?;
    let installation: Installation =
        serde_json::from_slice(&bytes).map_err(|_| "Configuration locale illisible")?;
    if installation.port == 0 || installation.key.len() < 32 {
        return Err("Configuration locale invalide".into());
    }
    Ok(installation)
}
fn private_dir(path: &Path) -> Result<(), String> {
    fs::create_dir_all(path).map_err(|_| "Impossible de créer le dossier local")?;
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        fs::set_permissions(path, fs::Permissions::from_mode(0o700))
            .map_err(|_| "Permissions du dossier refusées")?;
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
        .map_err(|_| "Écriture de la configuration refusée")?;
    file.write_all(&serde_json::to_vec(value).map_err(|_| "Configuration invalide")?)
        .map_err(|_| "Écriture interrompue")?;
    file.sync_all().map_err(|_| "Enregistrement interrompu")?;
    fs::rename(path, root.join("installation.json"))
        .map_err(|_| "Enregistrement de la configuration refusé".into())
}
fn client() -> Result<Client, String> {
    Client::builder()
        .no_proxy()
        .redirect(reqwest::redirect::Policy::none())
        .timeout(Duration::from_secs(45))
        .build()
        .map_err(|_| "Connexion locale indisponible".into())
}

async fn exchange_fleet_pairing(
    client: &Client,
    server: &str,
    pairing_code: &str,
) -> Result<FleetPairing, String> {
    let endpoint = reqwest::Url::parse(server)
        .map_err(|_| "Adresse du serveur Fleet invalide")?
        .join("/api/v1/agent/pair")
        .map_err(|_| "Adresse du serveur Fleet invalide")?;
    let response = client
        .post(endpoint)
        .json(&json!({"code":pairing_code,"label":"Sectile Desktop"}))
        .timeout(Duration::from_secs(12))
        .send()
        .await
        .map_err(|_| "Impossible de joindre le serveur Fleet")?;
    if response.status() != reqwest::StatusCode::CREATED {
        return Err(if response.status() == reqwest::StatusCode::UNAUTHORIZED {
            "Code de jumelage invalide ou expiré. Générez-en un nouveau depuis votre profil Fleet."
                .into()
        } else {
            format!(
                "Le serveur Fleet a refusé l’association (HTTP {}).",
                response.status().as_u16()
            )
        });
    }
    let payload: Value = response
        .json()
        .await
        .map_err(|_| "Réponse de jumelage Fleet illisible")?;
    let (Some(api_token), Some(device_id), Some(user_id)) = (
        payload["token"].as_str(),
        payload["deviceId"].as_str(),
        payload["userId"].as_str(),
    ) else {
        return Err("Le serveur Fleet n’a pas renvoyé une association valide.".into());
    };
    if api_token.is_empty() || device_id.is_empty() || user_id.is_empty() {
        return Err("Le serveur Fleet n’a pas renvoyé une association complète.".into());
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
    let packaged = app
        .path()
        .resource_dir()
        .map_err(|_| "Ressources indisponibles")?
        .join("binaries")
        .join(&name);
    if packaged.is_file() {
        return Ok(packaged);
    }
    #[cfg(debug_assertions)]
    {
        let development = PathBuf::from(env!("CARGO_MANIFEST_DIR"))
            .join("binaries")
            .join(name);
        if development.is_file() {
            return Ok(development);
        }
    }
    Err(format!(
        "Service {service} absent. Exécutez npm run prepare:services puis reconstruisez l’application."
    ))
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
        .map_err(|_| "Journal local indisponible")?;
    cmd.stdin(Stdio::null())
        .stdout(log.try_clone().map_err(|_| "Journal indisponible")?)
        .stderr(log);
    Ok(cmd)
}
// Discover the fixed transport contract from Go without starting a server or DB.
fn runtime_info(app: &tauri::AppHandle, root: &Path) -> Result<Value, String> {
    let output = command(app, root, "companion")?
        .arg("--local-runtime-info")
        .stdout(Stdio::piped())
        .output()
        .map_err(|_| "Configuration du serveur Go indisponible")?;
    let info: Value = serde_json::from_slice(&output.stdout)
        .map_err(|_| "Configuration du serveur Go invalide")?;
    if !output.status.success()
        || info["protocol"] != 1
        || info["port"]
            .as_u64()
            .filter(|p| *p > 0 && *p <= 65535)
            .is_none()
        || info["origin"] != format!("http://127.0.0.1:{}", info["port"])
    {
        return Err("Configuration du serveur Go incompatible".into());
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
    let info = runtime_info(app, root)?;
    if info["port"].as_u64() != Some(u64::from(installation.port)) {
        return Err("Port local incompatible avec le rappel OAuth enregistré".into());
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
            .map_err(|_| "Migration locale interrompue")?
            .map_err(|_| "Impossible de préparer la migration locale")?;
        if !result.success() {
            return Err("Migration locale suspendue. Terminez les exécutions actives, puis réessayez. Les données d’origine sont conservées ; consultez companion.log.".into());
        }
    }
    if probe(installation).await {
        return Ok(());
    }
    let data = active_root(root)?;
    let reservation = TcpListener::bind(("127.0.0.1", installation.port))
        .map_err(|_| "Le port local est occupé par un autre service")?;
    let mut cmd = command(app, root, "companion")?;
    cmd.arg("--data-dir")
        .arg(&data)
        .env("SECTILE_LOCAL_RUNTIME_KEY", &installation.key)
        .env("SECTILE_COMPANION_MCP_URL", installation.url());
    drop(reservation);
    let mut child = cmd
        .spawn()
        .map_err(|_| "Impossible de démarrer le compagnon Go")?;
    for _ in 0..120 {
        if probe(installation).await {
            std::thread::spawn(move || {
                let _ = child.wait();
            });
            return Ok(());
        }
        if child
            .try_wait()
            .map_err(|_| "État du compagnon indisponible")?
            .is_some()
        {
            return Err("Le compagnon Go s’est arrêté. Consultez companion.log.".into());
        }
        tokio::time::sleep(Duration::from_millis(250)).await;
    }
    let _ = child.kill();
    let _ = child.wait();
    Err("Le compagnon local ne répond pas. Consultez companion.log.".into())
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
        return Err("L’agent Fleet répond avec une autre identité de serveur".into());
    }
    let bytes = fs::read(fleet_root.join("settings.json")).map_err(
        |_| "L’agent Fleet associé est arrêté. Redémarrez-le depuis son installation habituelle.",
    )?;
    let credentials: FleetCredentials =
        serde_json::from_slice(&bytes).map_err(|_| "Configuration de l’agent Fleet illisible")?;
    let runtime: FleetRuntime = serde_json::from_slice(
        &fs::read(fleet_root.join("fleet-runtime.json"))
            .map_err(|_| "Configuration d’exécution Fleet absente")?,
    )
    .map_err(|_| "Configuration d’exécution Fleet illisible")?;
    if credentials.server != expected_server
        || runtime.server != expected_server
        || credentials.api_key.is_empty()
        || runtime.desktop_token.is_empty()
        || runtime.user_id.is_empty()
        || credentials.device_id.is_empty()
    {
        return Err("Configuration de l’agent Fleet incompatible".into());
    }
    private_dir(&fleet_root)?;
    let _installation_lock = installation_lock(root)?;
    if let Some(status) = fleet_status(&fleet_root).await {
        if status["server"] == expected_server {
            return Ok(());
        }
        return Err("L’agent Fleet répond avec une autre identité de serveur".into());
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
    let mut child = cmd
        .spawn()
        .map_err(|_| "Impossible de démarrer l’agent Fleet")?;
    for _ in 0..240 {
        if let Some(status) = fleet_status(&fleet_root).await {
            if status["server"] == expected_server && status["connected"] == true {
                std::thread::spawn(move || {
                    let _ = child.wait();
                });
                return Ok(());
            }
        }
        if child
            .try_wait()
            .map_err(|_| "État de l’agent Fleet indisponible")?
            .is_some()
        {
            return Err("L’agent Fleet s’est arrêté. Consultez agent.log.".into());
        }
        tokio::time::sleep(Duration::from_millis(250)).await;
    }
    let _ = child.kill();
    let _ = child.wait();
    Err("L’agent Fleet ne s’est pas connecté au serveur. Consultez agent.log.".into())
}

fn write_private_json(path: &Path, value: &impl Serialize) -> Result<(), String> {
    let parent = path.parent().ok_or("Dossier de configuration absent")?;
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
        .map_err(|_| "Écriture de la configuration refusée")?;
    file.write_all(&serde_json::to_vec(value).map_err(|_| "Configuration invalide")?)
        .map_err(|_| "Enregistrement de la configuration interrompu")?;
    file.sync_all()
        .map_err(|_| "Enregistrement de la configuration interrompu")?;
    fs::rename(next, path).map_err(|_| "Enregistrement de la configuration refusé".into())
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
    Err("L’agent intégré n’est pas prêt. Consultez companion.log.".into())
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
        .map_err(|_| "Réponse locale non reçue. Vérifiez l’état avant de répéter une création.")?;
    let status = response.status();
    if status == reqwest::StatusCode::NO_CONTENT {
        return Ok(Value::Null);
    }
    let value: Value = response
        .json()
        .await
        .map_err(|_| "Réponse locale illisible")?;
    if !status.is_success() {
        // Do not return payloads that could echo a submitted tracker credential.
        return Err(format!(
            "Action refusée par le serveur local (HTTP {}). Vérifiez les champs et la connexion.",
            status.as_u16()
        ));
    }
    Ok(value)
}

fn read_fleet_credentials(root: &Path) -> Result<FleetCredentials, String> {
    let fleet_root = fleet_root(root)?;
    let credentials: FleetCredentials = serde_json::from_slice(
        &fs::read(fleet_root.join("settings.json"))
            .map_err(|_| "Les identifiants API Fleet sont absents. Associez ce poste avec un code de jumelage.".to_string())?,
    )
    .map_err(|_| "Configuration API Fleet illisible".to_string())?;
    let normalized = normalize_fleet_server_url(&credentials.server)?;
    if normalized != credentials.server
        || credentials.api_key.is_empty()
        || credentials.device_id.is_empty()
        || credentials.server != selected_fleet_server(root)?
    {
        return Err("Configuration API Fleet incompatible".into());
    }
    Ok(credentials)
}

fn read_fleet_runtime(root: &Path) -> Result<FleetRuntime, String> {
    let runtime: FleetRuntime = serde_json::from_slice(
        &fs::read(fleet_root(root)?.join("fleet-runtime.json"))
            .map_err(|_| "Configuration d’identité Fleet absente".to_string())?,
    )
    .map_err(|_| "Configuration d’identité Fleet illisible".to_string())?;
    if runtime.server != selected_fleet_server(root)?
        || runtime.user_id.trim().is_empty()
        || runtime.desktop_token.trim().is_empty()
    {
        return Err("Configuration d’identité Fleet incompatible".into());
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
            "L’identité du backend Fleet ne correspond plus au jumelage de ce poste.".into(),
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
    let base = reqwest::Url::parse(&credentials.server).map_err(|_| "Adresse Fleet invalide")?;
    let endpoint = base
        .join(route.trim_start_matches('/'))
        .map_err(|_| "Route Fleet invalide")?;
    if endpoint.origin() != base.origin() || !endpoint.path().starts_with("/api/") {
        return Err("Route Fleet refusée".into());
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
        .map_err(|_| "Le backend Fleet est inaccessible. Vérifiez sa connexion.")?;
    let status = response.status();
    if status == reqwest::StatusCode::NO_CONTENT {
        return Ok(Value::Null);
    }
    if !status.is_success() {
        return Err(match status {
            reqwest::StatusCode::UNAUTHORIZED => {
                "Le jeton Fleet a expiré ou a été révoqué. Réassociez ce poste.".into()
            }
            reqwest::StatusCode::FORBIDDEN => {
                "Votre profil Fleet n’est pas autorisé à effectuer cette action.".into()
            }
            reqwest::StatusCode::NOT_FOUND => {
                "Cette opération n’est pas disponible sur cette version du backend Fleet.".into()
            }
            _ => format!(
                "Le backend Fleet a refusé cette action (HTTP {}).",
                status.as_u16()
            ),
        });
    }
    response
        .json()
        .await
        .map_err(|_| "Réponse du backend Fleet illisible".into())
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
        None => Err("Choisissez un mode d’exécution dans les paramètres.".into()),
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
        serde_json::from_slice(&fs::read(path).map_err(|_| "Mode d’exécution illisible")?)
            .map_err(|_| "Mode d’exécution invalide")?;
    if value["protocol"] != 1
        || (value["mode"] == "fleet" && value["server"].as_str().is_none_or(str::is_empty))
    {
        return Err("Configuration du mode incompatible".into());
    }
    serde_json::from_value(value["mode"].clone())
        .map(Some)
        .map_err(|_| "Configuration du mode incompatible".into())
}

fn require_unconfigured_runtime(mode: Option<RuntimeMode>) -> Result<(), String> {
    if mode.is_some() {
        return Err("Un mode d’exécution est déjà sélectionné. Sa migration doit être effectuée explicitement.".into());
    }
    Ok(())
}

pub async fn resume(app: &tauri::AppHandle) -> Result<(), String> {
    let state = app.state::<LocalRuntime>();
    let _lock = state.2.lock().await;
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
pub async fn enable_local_runtime(app: tauri::AppHandle) -> Result<(), String> {
    let state = app.state::<LocalRuntime>();
    let _lock = state.0.lock().await;
    let root = root(&app)?;
    if runtime_mode(&app)? == Some(RuntimeMode::Fleet) {
        return Err(
            "Le mode Fleet est sélectionné. Détachez-le avant d’activer une installation locale."
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
        let port = info["port"].as_u64().ok_or("Port Go absent")? as u16;
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
    ensure_server(&app, &root, &installation).await?;
    ensure_agent(&app, &root, &installation).await?;
    save_mode(&root, RuntimeMode::Local, None)
}
#[tauri::command]
pub async fn local_setup_status(app: tauri::AppHandle) -> Result<Value, String> {
    resume(&app).await?;
    let state = app.state::<LocalRuntime>();
    let _lock = state.0.lock().await;
    let root = root(&app)?;
    if runtime_mode(&app)? == Some(RuntimeMode::Fleet) {
        let path = connection_path(&app).ok_or("Connexion Fleet introuvable")?;
        let connection: Value =
            serde_json::from_slice(&fs::read(path).map_err(|_| "Connexion Fleet illisible")?)
                .map_err(|_| "Connexion Fleet invalide")?;
        let (Some(url), Some(token)) = (connection["url"].as_str(), connection["token"].as_str())
        else {
            return Err("Connexion Fleet incomplète".into());
        };
        let status = client()?
            .get(format!("{}/desktop/status", url.trim_end_matches('/')))
            .bearer_auth(token)
            .timeout(Duration::from_secs(3))
            .send()
            .await
            .map_err(|_| "L’agent Fleet associé est inaccessible.")?;
        if !status.status().is_success() {
            return Err("L’agent Fleet associé a refusé la connexion.".into());
        }
        let value: Value = status.json().await.map_err(|_| "État Fleet illisible")?;
        if value["server"] != selected_fleet_server(&root)? {
            return Err("L’agent Fleet répond avec une autre identité de serveur".into());
        }
        if !fleet_root(&root)?.join("settings.json").exists() {
            return Ok(json!({"enabled":true,"ready":true,"signedIn":false,
                "agentReady":value["connected"],"mode":"fleet","fleetServer":value["server"],
                "apiSetupAvailable":false}));
        }
        let profile = fleet_request(&root, Method::GET, "/api/me", None).await?;
        let runtime = read_fleet_runtime(&root)?;
        validate_fleet_profile_identity(&profile, &runtime.user_id)?;
        let credentials =
            fleet_request(&root, Method::GET, "/api/me/tracker-credentials", None).await?;
        let trackers = fleet_request(&root, Method::GET, "/api/trackers", None).await?;
        let projects = fleet_request(&root, Method::GET, "/api/projects", None).await?;
        return Ok(
            json!({"enabled":true,"ready":true,"signedIn":profile["signedIn"] == true,
            "agentReady":value["connected"],"mode":"fleet","fleetServer":value["server"],
            "apiSetupAvailable":true,
            "profile":profile,"credentials":credentials,"trackers":trackers,"projects":projects}),
        );
    }
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
        return Err("Code de jumelage requis.".into());
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
        .map_err(|_| "Dossier personnel introuvable")?
        .join(".taskflow/agent-connection.json");
    let bytes = fs::read(&legacy).map_err(|_| "Aucun agent Fleet déjà associé n’a été trouvé. Associez d’abord ce poste au serveur depuis l’agent Sectile.")?;
    let connection: Value =
        serde_json::from_slice(&bytes).map_err(|_| "Fichier de connexion Fleet invalide")?;
    let (Some(url), Some(token)) = (connection["url"].as_str(), connection["token"].as_str())
    else {
        return Err("Fichier de connexion Fleet incomplet".into());
    };
    let parsed = reqwest::Url::parse(url).map_err(|_| "Adresse d’agent invalide")?;
    if parsed.scheme() != "http"
        || parsed.host_str() != Some("127.0.0.1")
        || parsed.path() != "/"
        || token.trim().is_empty()
    {
        return Err("L’agent Fleet doit être un agent local associé à ce poste".into());
    }
    let status = client()?
        .get(format!("{}/desktop/status", url.trim_end_matches('/')))
        .bearer_auth(token)
        .timeout(Duration::from_secs(3))
        .send()
        .await
        .map_err(|_| "Impossible de joindre l’agent Fleet associé")?;
    if !status.status().is_success() {
        return Err("L’agent Fleet a refusé la vérification de connexion".into());
    }
    let value: Value = status
        .json()
        .await
        .map_err(|_| "État de l’agent Fleet illisible")?;
    if value["connected"] != true || value["server"].as_str().is_none_or(str::is_empty) {
        return Err("L’agent Fleet n’est pas connecté à son serveur".into());
    }
    let fleet_root = fleet_root(&root)?;
    private_dir(&fleet_root)?;
    let _installation_lock = installation_lock(&root)?;
    let path = fleet_root.join("agent-connection.json");
    let contents = serde_json::to_vec(&json!({"url":url,"token":token}))
        .map_err(|_| "Connexion Fleet invalide")?;
    let mut options = fs::OpenOptions::new();
    options.write(true).create(true).truncate(true);
    #[cfg(unix)]
    {
        use std::os::unix::fs::OpenOptionsExt;
        options.mode(0o600);
    }
    let mut file = options
        .open(path)
        .map_err(|_| "Écriture de la connexion Fleet refusée")?;
    file.write_all(&contents)
        .map_err(|_| "Enregistrement de la connexion Fleet interrompu")?;
    file.sync_all()
        .map_err(|_| "Enregistrement de la connexion Fleet interrompu")?;
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
            .map_err(|_| "Connexion locale interrompue")?;
        if !response.status().is_success() {
            return Err("Connexion refusée. Vérifiez votre adresse e-mail.".into());
        }
        let cookie = response
            .headers()
            .get(reqwest::header::SET_COOKIE)
            .and_then(|v| v.to_str().ok())
            .and_then(|v| v.split(';').next())
            .ok_or("Session locale absente")?
            .to_owned();
        let profile: Value = response.json().await.map_err(|_| "Profil illisible")?;
        let response = client()?
            .post(format!("{}/api/pairing-codes", installation.url()))
            .header(reqwest::header::COOKIE, cookie)
            .send()
            .await
            .map_err(|_| "Association interrompue")?;
        if !response.status().is_success() {
            return Err("Association refusée".into());
        }
        let code: Value = response
            .json()
            .await
            .map_err(|_| "Code d’association absent")?;
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
            .ok_or("Jeton d’association absent")?
            .to_owned();
        installation.device_id = pair["deviceId"]
            .as_str()
            .ok_or("Identité du poste absente")?
            .to_owned();
        installation.user_id = profile["userId"]
            .as_str()
            .ok_or("Profil absent")?
            .to_owned();
        installation.email = email;
        save(&root, &installation)?;
    } else if email != installation.email {
        return Err(
            "L’adresse identifie ce profil local et ne peut pas être remplacée ici.".into(),
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
        Err("Tracker distant inconnu".into())
    }
}
#[tauri::command]
pub async fn save_local_credential(
    app: tauri::AppHandle,
    input: CredentialInput,
) -> Result<(), String> {
    remote_provider(&input.tracker)?;
    if input.token.trim().is_empty() {
        return Err("Le jeton est requis".into());
    }
    let state = app.state::<LocalRuntime>();
    let _lock = state.0.lock().await;
    let root = root(&app)?;
    selected_request(&root,Method::PUT,"/api/me/tracker-credentials",Some(json!({"tracker":input.tracker,"siteUrl":input.site_url,"email":input.email,"token":input.token}))).await?;
    Ok(())
}
#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
pub struct TrackerInput {
    name: String,
    provider: String,
    site: String,
    scope: String,
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
pub async fn create_local_project(
    app: tauri::AppHandle,
    name: String,
    tracker_id: String,
) -> Result<Value, String> {
    if name.trim().is_empty() || tracker_id.is_empty() {
        return Err("Nom et tracker requis".into());
    }
    let state = app.state::<LocalRuntime>();
    let _lock = state.0.lock().await;
    let root = root(&app)?;
    selected_request(&root,Method::POST,"/api/projects",Some(json!({"name":name,"trackers":[{"trackerId":tracker_id}],"defaultTrackerId":tracker_id}))).await
}
#[tauri::command]
pub async fn sync_local_tracker(app: tauri::AppHandle, tracker_id: String) -> Result<Value, String> {
    if uuid::Uuid::parse_str(&tracker_id).is_err() {
        return Err("Identifiant du tracker invalide".into());
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
pub async fn tracker_sync_status(app: tauri::AppHandle, activity_id: String) -> Result<Value, String> {
    if uuid::Uuid::parse_str(&activity_id).is_err() {
        return Err("Identifiant de synchronisation invalide".into());
    }
    let state = app.state::<LocalRuntime>();
    let _lock = state.0.lock().await;
    let root = root(&app)?;
    selected_request(&root, Method::GET, &format!("/api/activities/{activity_id}"), None).await
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
                "Code de jumelage invalide ou expiré",
            ),
            (
                "201 Created",
                r#"{"token":"api-token","deviceId":"device-1"}"#,
                "association valide",
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
            "Les applications OAuth sont gérées par l’administrateur du backend Fleet.".into(),
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
            .ok_or("Identifiant de l’application requis")?
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
        .ok_or("Lien de connexion absent")?;
    let base = reqwest::Url::parse(server).map_err(|_| "Lien de connexion invalide")?;
    let parsed = base.join(path).map_err(|_| "Lien de connexion invalide")?;
    let pairs: Vec<_> = parsed.query_pairs().collect();
    if parsed.origin() != base.origin()
        || parsed.path() != "/auth/desktop-oauth/start"
        || parsed.fragment().is_some()
        || pairs.len() != 1
        || pairs[0].0 != "ticket"
        || uuid::Uuid::parse_str(&pairs[0].1).is_err()
    {
        return Err("Lien de connexion refusé".into());
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
        None => return Err("Choisissez un mode d’exécution dans les paramètres.".into()),
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
            "Configurez l’application OAuth sur le backend avant de vous connecter.".into(),
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
        .ok_or("Connexion incomplète")?
        .to_owned();
    if uuid::Uuid::parse_str(&id).is_err() {
        return Err("Identifiant de connexion invalide".into());
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
        return Err("Impossible d’ouvrir le navigateur. Réessayez depuis les paramètres.".into());
    }
    Ok(id)
}
#[tauri::command]
pub async fn oauth_status(app: tauri::AppHandle, id: String) -> Result<Value, String> {
    if uuid::Uuid::parse_str(&id).is_err() {
        return Err("Identifiant de connexion invalide".into());
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
        return Err("Identifiant de connexion invalide".into());
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
            .set_title("Choisir le dossier du projet")
            .blocking_pick_folder()
    })
    .await
    .map_err(|_| "Impossible d’ouvrir le sélecteur de dossier")?;
    let Some(selected) = selected else {
        return Ok(None);
    };
    let path = selected.into_path().map_err(|_| "Dossier local requis")?;
    let path = std::fs::canonicalize(path).map_err(|_| "Dossier introuvable")?;
    if !path.is_dir() {
        return Err("Sélectionnez un dossier".into());
    }
    let path = path.to_str().ok_or("Chemin non pris en charge")?.to_owned();
    Ok(Some(path))
}

#[tauri::command]
pub async fn choose_project_folder(
    app: tauri::AppHandle,
    project_id: String,
) -> Result<Option<String>, String> {
    if project_id.trim().is_empty() {
        return Err("Projet requis".into());
    }
    if discovery_path(&app).is_none() {
        return Err("Activez l’installation locale".into());
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
        .unwrap_or("Projet local")
        .to_owned();
    let state = app.state::<LocalRuntime>();
    let _lock = state.0.lock().await;
    let installation = read_installation(&root(&app)?)?;
    // Reopening a mapped folder selects its existing project.
    let projects = crate::agent::list_projects(app.clone()).await?;
    if let Some(project) = projects.as_array().and_then(|items| {
        items
            .iter()
            .find(|p| p.get("path").and_then(Value::as_str) == Some(path.as_str()))
    }) {
        return Ok(Some(project.clone()));
    }
    let project = request(
        &installation,
        Method::POST,
        "/api/projects",
        Some(json!({"name":name,"trackers":[],"folderOnly":true})),
    )
    .await?;
    let id = project
        .get("id")
        .and_then(Value::as_str)
        .ok_or("Identifiant du projet absent")?;
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
        return Err("Projet requis".into());
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
