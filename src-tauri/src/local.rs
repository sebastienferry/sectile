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

pub fn discovery_path(app: &tauri::AppHandle) -> Option<PathBuf> {
    let root = active_root(&root(app).ok()?).ok()?;
    root.join("installation.json")
        .exists()
        .then(|| root.join("agent-connection.json"))
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

pub async fn resume(app: &tauri::AppHandle) -> Result<(), String> {
    let state = app.state::<LocalRuntime>();
    let _lock = state.2.lock().await;
    if state.1.load(std::sync::atomic::Ordering::Acquire) {
        return Ok(());
    }
    let root = root(app)?;
    if !root.join("installation.json").exists() {
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
    ensure_agent(&app, &root, &installation).await
}
#[tauri::command]
pub async fn local_setup_status(app: tauri::AppHandle) -> Result<Value, String> {
    let state = app.state::<LocalRuntime>();
    let _lock = state.0.lock().await;
    let root = root(&app)?;
    if !root.join("installation.json").exists() {
        return Ok(json!({"enabled":false}));
    }
    let installation = read_installation(&root)?;
    if !probe(&installation).await {
        let _installation_lock = installation_lock(&root)?;
        ensure_server(&app, &root, &installation).await?;
        ensure_agent(&app, &root, &installation).await?;
    }
    if installation.token.is_empty() {
        return Ok(
            json!({"enabled":true,"ready":true,"signedIn":false,"dataDirectory":active_root(&root)?}),
        );
    }
    Ok(
        json!({"enabled":true,"ready":true,"signedIn":true,"agentReady":agent_ready(&root,&installation).await,
        "dataDirectory":active_root(&root)?,
        "profile":request(&installation,Method::GET,"/api/me",None).await?,
        "credentials":request(&installation,Method::GET,"/api/me/tracker-credentials",None).await?,
        "trackers":request(&installation,Method::GET,"/api/trackers",None).await?,
        "projects":request(&installation,Method::GET,"/api/projects",None).await?}),
    )
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
    let installation = read_installation(&root(&app)?)?;
    request(&installation,Method::PUT,"/api/me/tracker-credentials",Some(json!({"tracker":input.tracker,"siteUrl":input.site_url,"email":input.email,"token":input.token}))).await?;
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
    let installation = read_installation(&root(&app)?)?;
    request(&installation,Method::POST,"/api/admin/trackers",Some(json!({"name":input.name,"provider":input.provider,"site":input.site,"scope":input.scope,"autoSyncEnabled":true,"autoSyncIntervalMin":5}))).await
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
    let installation = read_installation(&root(&app)?)?;
    request(&installation,Method::POST,"/api/projects",Some(json!({"name":name,"trackers":[{"trackerId":tracker_id}],"defaultTrackerId":tracker_id}))).await
}
#[tauri::command]
pub async fn sync_local_tracker(app: tauri::AppHandle, tracker_id: String) -> Result<(), String> {
    if uuid::Uuid::parse_str(&tracker_id).is_err() {
        return Err("Identifiant du tracker invalide".into());
    }
    let state = app.state::<LocalRuntime>();
    let _lock = state.0.lock().await;
    let installation = read_installation(&root(&app)?)?;
    request(
        &installation,
        Method::POST,
        &format!("/api/trackers/{tracker_id}/sync"),
        Some(json!({})),
    )
    .await?;
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
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
    let installation = read_installation(&root(&app)?)?;
    let mut apps = serde_json::Map::new();
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
    let installation = read_installation(&root(&app)?)?;
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
fn browser_ticket_url(installation: &Installation, value: &Value) -> Result<String, String> {
    let path = value["startPath"]
        .as_str()
        .ok_or("Lien de connexion absent")?;
    let parsed = reqwest::Url::parse(&format!("{}{path}", installation.url()))
        .map_err(|_| "Lien de connexion invalide")?;
    let pairs: Vec<_> = parsed.query_pairs().collect();
    if parsed.origin().ascii_serialization() != installation.url()
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
    let installation = read_installation(&root(&app)?)?;
    let configured = request(
        &installation,
        Method::GET,
        &format!("/api/admin/{provider}-oauth"),
        None,
    )
    .await?;
    if configured["configured"] != true
        || configured["expectedRedirectUrl"]
            .as_str()
            .filter(|url| !url.is_empty())
            .is_none()
        || configured["redirectUrl"] != configured["expectedRedirectUrl"]
    {
        return Err(
            "Configurez l’application OAuth avec l’URL de rappel affichée avant de vous connecter."
                .into(),
        );
    }
    let flow = request(
        &installation,
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
    let url = browser_ticket_url(&installation, &flow)?;
    if app.opener().open_url(url, None::<&str>).is_err() {
        let _ = request(
            &installation,
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
    let installation = read_installation(&root(&app)?)?;
    request(
        &installation,
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
    let installation = read_installation(&root(&app)?)?;
    request(
        &installation,
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
    fn browser_opening_only_accepts_one_use_local_tickets() {
        let installation = Installation {
            port: 45678,
            key: uuid::Uuid::new_v4().to_string(),
            token: String::new(),
            device_id: String::new(),
            agent_device_id: String::new(),
            user_id: String::new(),
            email: String::new(),
        };
        let ticket = uuid::Uuid::new_v4();
        assert!(
            browser_ticket_url(
                &installation,
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
            assert!(browser_ticket_url(&installation, &json!({"startPath":path})).is_err());
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
    use super::*;

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
