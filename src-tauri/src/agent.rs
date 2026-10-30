use serde::Deserialize;
use serde_json::Value;
use std::time::Duration;

#[derive(Deserialize)]
struct Connection {
    url: String,
    token: String,
    #[serde(skip)]
    connection_id: String,
}

fn validate_connection(connection: &Connection) -> Result<reqwest::Url, String> {
    let url = reqwest::Url::parse(&connection.url).map_err(|_| "Adresse de l’agent invalide")?;
    if url.scheme() != "http"
        || url.host_str() != Some("127.0.0.1")
        || !url.username().is_empty()
        || url.password().is_some()
        || url.path() != "/"
        || url.query().is_some()
        || url.fragment().is_some()
        || connection.token.trim().is_empty()
    {
        return Err("Connexion locale à l’agent invalide".into());
    }
    Ok(url)
}

fn connection(app: &tauri::AppHandle) -> Result<Connection, String> {
    let path =
        crate::local::connection_path(app).ok_or_else(|| {
            match crate::local::runtime_mode(app) {
                Ok(Some(crate::local::RuntimeMode::Local)) => {
                    "Connexion au compagnon local introuvable".to_string()
                }
                Ok(Some(crate::local::RuntimeMode::Fleet)) => {
                    "Connexion Fleet introuvable. Associez un agent Fleet depuis les paramètres."
                        .to_string()
                }
                _ => "Choisissez le mode Local ou Fleet dans les paramètres pour continuer."
                    .to_string(),
            }
        })?;
    let contents = std::fs::read(path)
        .map_err(|_| "Agent introuvable. Démarrez et associez l’agent Sectile.")?;
    let mut connection: Connection =
        serde_json::from_slice(&contents).map_err(|_| "Fichier de connexion invalide")?;
    validate_connection(&connection)?;
    connection.connection_id = crate::local::runtime_connection_id(app)?;
    Ok(connection)
}

fn request_url(
    connection: &Connection,
    route: &str,
    query: &[(&str, &str)],
) -> Result<reqwest::Url, String> {
    let mut url = validate_connection(connection)?
        .join(route)
        .map_err(|_| "Route invalide")?;
    if !query.is_empty() {
        url.query_pairs_mut().extend_pairs(query.iter().copied());
    }
    Ok(url)
}

pub(crate) async fn request_agent(
    app: tauri::AppHandle,
    method: reqwest::Method,
    route: &str,
    query: &[(&str, &str)],
    body: Option<Value>,
) -> Result<Value, String> {
    crate::local::resume(&app).await?;
    let connection = connection(&app)?;
    let result = send_request(&connection, method, route, query, body).await;
    if result.is_err() {
        crate::local::invalidate(&app);
    }
    result
}

async fn send_request(
    connection: &Connection,
    method: reqwest::Method,
    route: &str,
    query: &[(&str, &str)],
    body: Option<Value>,
) -> Result<Value, String> {
    let url = request_url(connection, route, query)?;
    let mutation = method != reqwest::Method::GET;
    let client = reqwest::Client::builder()
        .no_proxy()
        .redirect(reqwest::redirect::Policy::none())
        .timeout(Duration::from_secs(if mutation { 65 } else { 20 }))
        .build()
        .map_err(|_| "Impossible de créer la connexion locale")?;
    let mut request = client.request(method, url).bearer_auth(&connection.token);
    request = request.header("X-Sectile-Connection-ID", &connection.connection_id);
    if let Some(body) = body {
        request = request.json(&body);
    }
    let response = request.send().await.map_err(|_| if mutation {
        "Réponse non reçue : l’action a peut-être été acceptée. Actualisez les exécutions avant de réessayer."
    } else { "L’agent local est inaccessible." })?;
    let status = response.status();
    if status == reqwest::StatusCode::NO_CONTENT {
        return Ok(Value::Null);
    }
    let text = response
        .text()
        .await
        .map_err(|_| "Réponse interrompue. Vérifiez les exécutions avant de réessayer.")?;
    if !status.is_success() {
        let detail = serde_json::from_str::<Value>(&text)
            .ok()
            .and_then(|v| v.get("error").and_then(Value::as_str).map(str::to_owned))
            .unwrap_or(text);
        // Redact the local bearer token even if a misbehaving agent echoes it.
        let detail = detail.replace(&connection.token, "[masqué]");
        return Err(format!(
            "Agent HTTP {} : {}",
            status.as_u16(),
            detail.chars().take(1600).collect::<String>()
        ));
    }
    if text.trim().is_empty() {
        return Ok(Value::Null);
    }
    serde_json::from_str(&text).map_err(|_| "Réponse de l’agent invalide".into())
}

fn required(value: &str) -> Result<(), String> {
    if value.trim().is_empty() || value.len() > 1024 {
        Err("Identifiant invalide".into())
    } else {
        Ok(())
    }
}

#[tauri::command]
pub async fn list_projects(app: tauri::AppHandle) -> Result<Value, String> {
    request_agent(app, reqwest::Method::GET, "/desktop/projects", &[], None).await
}
#[tauri::command]
pub async fn list_tasks(app: tauri::AppHandle, project_id: String) -> Result<Value, String> {
    required(&project_id)?;
    request_agent(
        app,
        reqwest::Method::GET,
        "/desktop/tasks",
        &[("projectId", &project_id)],
        None,
    )
    .await
}
#[tauri::command]
pub async fn get_task(app: tauri::AppHandle, project_id: String, task_id: String) -> Result<Value, String> {
    required(&project_id)?;
    required(&task_id)?;
    request_agent(
        app,
        reqwest::Method::GET,
        "/desktop/tasks/detail",
        &[("projectId", &project_id), ("taskId", &task_id)],
        None,
    )
    .await
}
#[tauri::command]
pub async fn execution_snapshot(app: tauri::AppHandle) -> Result<Value, String> {
    let runtime_mode = runtime_mode_key(crate::local::runtime_mode(&app)?);
    let mut status = request_agent(
        app.clone(),
        reqwest::Method::GET,
        "/desktop/status",
        &[],
        None,
    )
    .await?;
    status["runtimeMode"] = serde_json::json!(runtime_mode);
    let runs = request_agent(app, reqwest::Method::GET, "/desktop/runs", &[], None).await?;
    Ok(serde_json::json!({"status": status, "runs": runs}))
}

fn runtime_mode_key(mode: Option<crate::local::RuntimeMode>) -> &'static str {
    match mode {
        Some(crate::local::RuntimeMode::Fleet) => "fleet",
        Some(crate::local::RuntimeMode::Local) => "local",
        None => "unconfigured",
    }
}
#[tauri::command]
pub async fn project_skills(app: tauri::AppHandle, project_id: String) -> Result<Value, String> {
    required(&project_id)?;
    let project = request_agent(
        app,
        reqwest::Method::GET,
        "/desktop/project",
        &[("id", &project_id)],
        None,
    )
    .await?;
    let skills = project
        .pointer("/server/skills")
        .and_then(Value::as_array)
        .ok_or("Liste des compétences indisponible")?;
    // Return only the launch catalogue, not configuration or embedded instructions.
    Ok(Value::Array(
        skills
            .iter()
            .filter_map(|s| {
                s.get("id")
                    .and_then(Value::as_str)
                    .map(|id| serde_json::json!({"id": id}))
            })
            .collect(),
    ))
}

#[derive(Deserialize, serde::Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct LaunchTask {
    task_id: String,
    skill_id: String,
    prompt: String,
    mode: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    batch_task_ids: Option<Vec<String>>,
}
fn launch_body(input: LaunchTask) -> Result<Value, String> {
    required(&input.task_id)?;
    required(&input.skill_id)?;
    if !["", "interactive", "autonomous"].contains(&input.mode.as_str()) {
        return Err("Mode d’exécution invalide".into());
    }
    if let Some(ids) = &input.batch_task_ids {
        if input.skill_id != "pickup_issues"
            || input.mode != "autonomous"
            || ids.len() < 2
            || ids.len() > 100
            || ids.first() != Some(&input.task_id)
            || ids.iter().any(|id| required(id).is_err())
            || ids.iter().collect::<std::collections::HashSet<_>>().len() != ids.len()
        {
            return Err("Lot de tickets invalide".into());
        }
    }
    let body = serde_json::to_value(input).map_err(|_| "Requête invalide")?;
    if body.to_string().len() > 8192 {
        return Err("Instructions trop longues (limite de 8 Ko pour la requête)".into());
    }
    Ok(body)
}
#[tauri::command]
pub async fn launch_task(
    app: tauri::AppHandle,
    project_id: String,
    input: LaunchTask,
) -> Result<Value, String> {
    required(&project_id)?;
    let mut body = launch_body(input)?;
    body["view"] = serde_json::json!("conversation");
    request_agent(
        app,
        reqwest::Method::POST,
        "/desktop/tasks",
        &[("projectId", &project_id)],
        Some(body),
    )
    .await
}
#[tauri::command]
pub async fn stop_run(app: tauri::AppHandle, run_id: String) -> Result<Value, String> {
    required(&run_id)?;
    request_agent(
        app,
        reqwest::Method::POST,
        "/desktop/stop",
        &[("id", &run_id)],
        None,
    )
    .await
}
#[tauri::command]
pub async fn orchestration(app: tauri::AppHandle, project_id: String) -> Result<Value, String> {
    required(&project_id)?;
    let mut view = request_agent(
        app.clone(),
        reqwest::Method::GET,
        "/desktop/orchestration",
        &[("projectId", &project_id)],
        None,
    )
    .await?;
    // Capacity is server-owned and can include runs outside this desktop.
    if let Ok(projects) =
        request_agent(app, reqwest::Method::GET, "/desktop/projects", &[], None).await
    {
        merge_orchestration_capacity(&mut view, &projects, &project_id);
    }
    Ok(view)
}
fn merge_orchestration_capacity(view: &mut Value, projects: &Value, project_id: &str) {
    if let Some(count) = projects
        .as_array()
        .and_then(|items| items.iter().find(|p| p["id"] == project_id))
        .and_then(|p| p["orchestratorActiveRuns"].as_u64())
    {
        view["orchestratorActiveRuns"] = Value::from(count);
    }
}
#[tauri::command]
pub async fn run_orchestration(
    app: tauri::AppHandle,
    project_id: String,
    agent_id: String,
) -> Result<Value, String> {
    required(&project_id)?;
    required(&agent_id)?;
    request_agent(
        app,
        reqwest::Method::POST,
        "/desktop/orchestration",
        &[("projectId", &project_id)],
        Some(serde_json::json!({"agentId": agent_id})),
    )
    .await
}

#[derive(Default)]
pub struct Streams(
    pub std::sync::Mutex<std::collections::HashMap<String, tauri::async_runtime::JoinHandle<()>>>,
);
#[derive(Clone, serde::Serialize)]
#[serde(tag = "kind", content = "data", rename_all = "camelCase")]
pub enum Output {
    Bytes(Vec<u8>),
    Closed,
    Error(String),
}

#[tauri::command]
pub async fn watch_run(
    app: tauri::AppHandle,
    state: tauri::State<'_, Streams>,
    subscription: String,
    run_id: String,
    output: tauri::ipc::Channel<Output>,
) -> Result<(), String> {
    use futures_util::StreamExt;
    use tokio_tungstenite::tungstenite::client::IntoClientRequest;
    required(&run_id)?;
    required(&subscription)?;
    let connection = connection(&app)?;
    let mut url = request_url(&connection, "/desktop/terminal", &[("id", &run_id)])?;
    url.set_scheme("ws")
        .map_err(|_| "Adresse de flux invalide")?;
    let mut request = url
        .as_str()
        .into_client_request()
        .map_err(|_| "Connexion au flux impossible")?;
    request.headers_mut().insert(
        "Authorization",
        format!("Bearer {}", connection.token)
            .parse()
            .map_err(|_| "Jeton invalide")?,
    );
    let mut streams = state.0.lock().map_err(|_| "Flux indisponible")?;
    if streams.len() >= 4 || streams.contains_key(&subscription) {
        return Err("Trop de flux ouverts".into());
    }
    let handle = tauri::async_runtime::spawn(async move {
        let connected = tokio::time::timeout(
            Duration::from_secs(20),
            tokio_tungstenite::connect_async(request),
        )
        .await;
        match connected {
            Ok(Ok((mut socket, _))) => {
                while let Some(message) = socket.next().await {
                    match message {
                        Ok(tokio_tungstenite::tungstenite::Message::Binary(bytes)) => {
                            if output.send(Output::Bytes(bytes.to_vec())).is_err() {
                                break;
                            }
                        }
                        Ok(tokio_tungstenite::tungstenite::Message::Text(text)) => {
                            if output
                                .send(Output::Bytes(text.as_bytes().to_vec()))
                                .is_err()
                            {
                                break;
                            }
                        }
                        Ok(tokio_tungstenite::tungstenite::Message::Close(_)) => break,
                        Err(_) => {
                            let _ = output.send(Output::Error(
                                "Flux interrompu. Reconnectez la console.".into(),
                            ));
                            break;
                        }
                        _ => {}
                    }
                }
                let _ = socket.close(None).await;
                let _ = output.send(Output::Closed);
            }
            _ => {
                let _ = output.send(Output::Error(
                    "Console indisponible ou pas encore prête. Réessayez.".into(),
                ));
            }
        }
    });
    streams.insert(subscription, handle);
    Ok(())
}
#[tauri::command]
pub fn unwatch_run(state: tauri::State<'_, Streams>, subscription: String) -> Result<(), String> {
    if let Some(handle) = state
        .0
        .lock()
        .map_err(|_| "Flux indisponible")?
        .remove(&subscription)
    {
        handle.abort();
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    #[test]
    fn execution_snapshot_names_the_selected_runtime() {
        assert_eq!(
            super::runtime_mode_key(Some(crate::local::RuntimeMode::Fleet)),
            "fleet"
        );
        assert_eq!(
            super::runtime_mode_key(Some(crate::local::RuntimeMode::Local)),
            "local"
        );
        assert_eq!(super::runtime_mode_key(None), "unconfigured");
    }

    #[test]
    fn orchestration_capacity_never_falls_back_to_local_run_count() {
        let mut view = serde_json::json!({"orchestratorMaxConcurrent":3});
        super::merge_orchestration_capacity(
            &mut view,
            &serde_json::json!([{"id":"p","orchestratorActiveRuns":2}]),
            "p",
        );
        assert_eq!(view["orchestratorActiveRuns"], 2);
        let mut absent = serde_json::json!({"orchestratorMaxConcurrent":3});
        super::merge_orchestration_capacity(&mut absent, &serde_json::json!([{"id":"p"}]), "p");
        assert!(absent.get("orchestratorActiveRuns").is_none());
    }
    use super::*;

    fn mock_agent(response: &'static str) -> (Connection, std::thread::JoinHandle<String>) {
        use std::io::{Read, Write};
        let listener = std::net::TcpListener::bind("127.0.0.1:0").unwrap();
        let address = listener.local_addr().unwrap();
        let handle = std::thread::spawn(move || {
            let (mut stream, _) = listener.accept().unwrap();
            stream
                .set_read_timeout(Some(Duration::from_secs(5)))
                .unwrap();
            let mut bytes = Vec::new();
            loop {
                let mut chunk = [0u8; 2048];
                let count = stream.read(&mut chunk).unwrap();
                if count == 0 {
                    break;
                }
                bytes.extend_from_slice(&chunk[..count]);
                let text = String::from_utf8_lossy(&bytes);
                if let Some(end) = text.find("\r\n\r\n") {
                    let length = text[..end]
                        .lines()
                        .find_map(|line| {
                            line.to_ascii_lowercase()
                                .strip_prefix("content-length: ")
                                .and_then(|v| v.parse::<usize>().ok())
                        })
                        .unwrap_or(0);
                    if bytes.len() >= end + 4 + length {
                        break;
                    }
                }
            }
            stream.write_all(response.as_bytes()).unwrap();
            String::from_utf8(bytes).unwrap()
        });
        (
            Connection {
                url: format!("http://{address}"),
                token: "private-test-token".into(),
                connection_id: uuid::Uuid::nil().to_string(),
            },
            handle,
        )
    }

    fn send(
        connection: &Connection,
        method: reqwest::Method,
        route: &str,
        query: &[(&str, &str)],
        body: Option<Value>,
    ) -> Result<Value, String> {
        tokio::runtime::Builder::new_current_thread()
            .enable_all()
            .build()
            .unwrap()
            .block_on(send_request(connection, method, route, query, body))
    }

    #[test]
    fn launch_encodes_identifiers_and_preserves_mode_without_force() {
        let (connection, server) =
            mock_agent("HTTP/1.1 200 OK\r\nContent-Length: 2\r\nConnection: close\r\n\r\n{}");
        let body = launch_body(LaunchTask {
            task_id: "ticket".into(),
            skill_id: "implement_issue".into(),
            prompt: "Fix l'API".into(),
            mode: "".into(),
            batch_task_ids: None,
        })
        .unwrap();
        assert!(body.get("force").is_none());
        send(
            &connection,
            reqwest::Method::POST,
            "/desktop/tasks",
            &[("projectId", "project&other=1")],
            Some(body),
        )
        .unwrap();
        let request = server.join().unwrap();
        assert!(request.starts_with("POST /desktop/tasks?projectId=project%26other%3D1 HTTP/1.1"));
        assert!(
            request
                .to_lowercase()
                .contains("authorization: bearer private-test-token")
        );
        assert!(
            request
                .to_lowercase()
                .contains(&format!("x-sectile-connection-id: {}", uuid::Uuid::nil()))
        );
        let payload: Value =
            serde_json::from_str(request.split("\r\n\r\n").nth(1).unwrap()).unwrap();
        assert_eq!(payload["prompt"], "Fix l'API");
        assert_eq!(payload["mode"], "");
        assert_eq!(payload["skillId"], "implement_issue");
    }

    #[test]
    fn stop_accepts_no_content() {
        let (connection, server) =
            mock_agent("HTTP/1.1 204 No Content\r\nConnection: close\r\n\r\n");
        assert_eq!(
            send(
                &connection,
                reqwest::Method::POST,
                "/desktop/stop",
                &[("id", "run/1")],
                None
            )
            .unwrap(),
            Value::Null
        );
        assert!(
            server
                .join()
                .unwrap()
                .starts_with("POST /desktop/stop?id=run%2F1")
        );
    }

    #[test]
    fn errors_are_explained_and_credentials_redacted() {
        let (connection, server) = mock_agent(
            "HTTP/1.1 409 Conflict\r\nConnection: close\r\n\r\n{\"error\":\"Capacity full private-test-token\"}",
        );
        let error = send(
            &connection,
            reqwest::Method::POST,
            "/desktop/orchestration",
            &[],
            None,
        )
        .unwrap_err();
        assert!(error.contains("409") && error.contains("Capacity full"));
        assert!(!error.contains("private-test-token"));
        server.join().unwrap();
    }

    #[test]
    fn redirects_are_not_followed() {
        let (connection, server) = mock_agent(
            "HTTP/1.1 302 Found\r\nLocation: http://127.0.0.1:1/leak\r\nContent-Length: 0\r\nConnection: close\r\n\r\n",
        );
        assert!(
            send(
                &connection,
                reqwest::Method::GET,
                "/desktop/runs",
                &[],
                None
            )
            .unwrap_err()
            .contains("302")
        );
        server.join().unwrap();
    }

    #[test]
    fn launch_rejects_invalid_modes_unknown_fields_and_oversized_payloads() {
        assert!(serde_json::from_value::<LaunchTask>(serde_json::json!({"taskId":"t", "skillId":"s", "prompt":"", "mode":"", "force":true})).is_err());
        assert!(
            launch_body(LaunchTask {
                task_id: "t".into(),
                skill_id: "s".into(),
                prompt: "".into(),
                mode: "unsafe".into(),
                batch_task_ids: None,
            })
            .is_err()
        );
        assert!(
            launch_body(LaunchTask {
                task_id: "t".into(),
                skill_id: "s".into(),
                prompt: "x".repeat(8192),
                mode: "".into(),
                batch_task_ids: None,
            })
            .is_err()
        );
    }

    #[test]
    fn batch_launch_requires_a_distinct_ordered_autonomous_selection() {
        let valid = serde_json::json!({"taskId":"first","skillId":"pickup_issues","prompt":"","mode":"autonomous","batchTaskIds":["first","second"]});
        let body = launch_body(serde_json::from_value(valid.clone()).unwrap()).unwrap();
        assert_eq!(body["batchTaskIds"], valid["batchTaskIds"]);
        for invalid in [
            serde_json::json!({"taskId":"first","skillId":"pickup_issues","prompt":"","mode":"autonomous","batchTaskIds":["second","first"]}),
            serde_json::json!({"taskId":"first","skillId":"pickup_issues","prompt":"","mode":"autonomous","batchTaskIds":["first","first"]}),
            serde_json::json!({"taskId":"first","skillId":"clarify","prompt":"","mode":"autonomous","batchTaskIds":["first","second"]}),
        ] {
            assert!(launch_body(serde_json::from_value(invalid).unwrap()).is_err());
        }
    }

    #[test]
    fn only_accepts_local_agent_origin() {
        for url in [
            "https://127.0.0.1:9000",
            "http://example.com",
            "http://127.0.0.1.evil.test",
            "http://user@127.0.0.1",
            "http://127.0.0.1/path",
            "http://127.0.0.1?token=x",
        ] {
            assert!(
                validate_connection(&Connection {
                    url: url.into(),
                    token: "test".into(),
                    connection_id: uuid::Uuid::nil().to_string(),
                })
                .is_err(),
                "accepted {url}"
            );
        }
        assert!(
            validate_connection(&Connection {
                url: "http://127.0.0.1:9000".into(),
                token: "test".into(),
                connection_id: uuid::Uuid::nil().to_string(),
            })
            .is_ok()
        );
        assert!(
            validate_connection(&Connection {
                url: "http://127.0.0.1:9000".into(),
                token: "".into(),
                connection_id: uuid::Uuid::nil().to_string(),
            })
            .is_err()
        );
    }
}

#[tauri::command]
pub async fn create_conversation(
    app: tauri::AppHandle,
    project_id: String,
    provider: String,
    engine_id: Option<String>,
) -> Result<Value, String> {
    required(&project_id)?;
    if !["claude", "codex"].contains(&provider.as_str()) {
        return Err("Fournisseur non pris en charge".into());
    }
    request_agent(app, reqwest::Method::POST, "/desktop/consoles", &[], Some(serde_json::json!({"projectId": project_id, "provider": provider, "engineId": engine_id.unwrap_or_default(), "view": "conversation"}))).await
}

#[tauri::command]
pub async fn read_conversation(app: tauri::AppHandle, run_id: String) -> Result<Value, String> {
    required(&run_id)?;
    request_agent(
        app,
        reqwest::Method::GET,
        "/desktop/conversation",
        &[("id", &run_id)],
        None,
    )
    .await
}

#[derive(Deserialize, serde::Serialize)]
#[serde(rename_all = "camelCase")]
pub enum ConversationMode {
    Plan,
    Execute,
}

#[derive(Deserialize, serde::Serialize)]
#[serde(tag = "action", rename_all = "camelCase", deny_unknown_fields)]
pub enum ConversationAction {
    AdoptConnection,
    CheckMcp,
    Message {
        message: String,
        model: Option<String>,
        mode: Option<ConversationMode>,
    },
    Interrupt,
    Approval {
        id: String,
        decision: String,
    },
    Answer {
        id: String,
        answers: std::collections::BTreeMap<String, String>,
    },
}
fn conversation_body(action: ConversationAction) -> Result<Value, String> {
    match action {
        ConversationAction::AdoptConnection => Ok(serde_json::json!({"adoptConnection": true})),
        ConversationAction::Message {
            message,
            model,
            mode,
        } => {
            if message.trim().is_empty() || message.len() > 32000 {
                return Err("Message vide ou trop long (32 Ko maximum)".into());
            }
            let mut body = serde_json::json!({"message": message});
            if let Some(model) = model.filter(|value| !value.is_empty()) {
                if model.len() > 256
                    || !model.as_bytes()[0].is_ascii_alphanumeric()
                    || !model
                        .chars()
                        .all(|c| c.is_ascii_alphanumeric() || "._-:@/".contains(c))
                {
                    return Err("Identifiant du modèle invalide".into());
                }
                body["model"] = Value::String(model);
            }
            if let Some(mode) = mode {
                body["mode"] = serde_json::to_value(mode).map_err(|_| "Mode invalide")?;
            }
            Ok(body)
        }
        ConversationAction::Answer { id, answers } => {
            required(&id)?;
            let size: usize = answers
                .iter()
                .map(|(question, answer)| question.len() + answer.len())
                .sum();
            if answers.is_empty()
                || answers.len() > 32
                || size > 16 * 1024
                || answers.iter().any(|(question, answer)| {
                    question.trim().is_empty() || answer.trim().is_empty()
                })
            {
                return Err("Répondez à chaque question (16 Ko maximum au total)".into());
            }
            Ok(serde_json::json!({"approval":{"id":id,"decision":"answer","answers":answers}}))
        }
        ConversationAction::CheckMcp => Ok(serde_json::json!({"checkMcp":true})),
        ConversationAction::Interrupt => Ok(serde_json::json!({"interrupt": true})),
        ConversationAction::Approval { id, decision } => {
            required(&id)?;
            if !["allow", "deny"].contains(&decision.as_str()) {
                return Err("Décision non prise en charge".into());
            }
            Ok(serde_json::json!({"approval": {"id": id, "decision": decision}}))
        }
    }
}
#[tauri::command]
pub async fn control_conversation(
    app: tauri::AppHandle,
    run_id: String,
    input: ConversationAction,
) -> Result<Value, String> {
    required(&run_id)?;
    let mut body = conversation_body(input)?;
    if body.get("mode").is_some() {
        let current = request_agent(
            app.clone(),
            reqwest::Method::GET,
            "/desktop/conversation",
            &[("id", &run_id)],
            None,
        )
        .await?;
        body["mode"] = Value::String(
            resolve_conversation_mode(
                body["mode"].as_str().unwrap_or(""),
                current["provider"].as_str().unwrap_or(""),
            )?
            .into(),
        );
    }
    request_agent(
        app,
        reqwest::Method::POST,
        "/desktop/conversation",
        &[("id", &run_id)],
        Some(body),
    )
    .await
}

#[cfg(test)]
mod conversation_tests {
    #[test]
    fn mcp_verification_never_contains_a_provider_message() {
        let action: super::ConversationAction =
            serde_json::from_value(serde_json::json!({"action":"checkMcp"})).unwrap();
        assert_eq!(
            super::conversation_body(action).unwrap(),
            serde_json::json!({"checkMcp":true})
        );
    }
    use super::*;
    #[test]
    fn conversation_actions_cannot_override_permissions() {
        assert!(
            serde_json::from_value::<ConversationAction>(
                serde_json::json!({"action":"message","message":"hi","mode":"bypassPermissions"})
            )
            .is_err()
        );
        assert!(
            conversation_body(ConversationAction::Approval {
                id: "a".into(),
                decision: "always".into()
            })
            .is_err()
        );
        assert!(
            conversation_body(ConversationAction::Message {
                message: " ".into(),
                model: None,
                mode: None
            })
            .is_err()
        );
        assert_eq!(
            conversation_body(ConversationAction::Message {
                message: "explain this project".into(),
                model: None,
                mode: None
            })
            .unwrap(),
            serde_json::json!({"message":"explain this project"})
        );
    }
}

#[tauri::command]
pub async fn provider_status(app: tauri::AppHandle, provider: String) -> Result<Value, String> {
    provider_request(app, provider, false).await
}
#[tauri::command]
pub async fn provider_login(app: tauri::AppHandle, provider: String) -> Result<Value, String> {
    provider_request(app, provider, true).await
}
async fn provider_request(
    app: tauri::AppHandle,
    provider: String,
    login: bool,
) -> Result<Value, String> {
    if !["claude", "codex"].contains(&provider.as_str()) {
        return Err("Fournisseur invalide".into());
    }
    request_agent(
        app,
        if login {
            reqwest::Method::POST
        } else {
            reqwest::Method::GET
        },
        "/desktop/provider-login",
        &[("provider", &provider)],
        None,
    )
    .await
}

pub async fn map_folder_project(
    app: tauri::AppHandle,
    project_id: &str,
    path: &str,
) -> Result<Value, String> {
    request_agent(
        app,
        reqwest::Method::POST,
        "/desktop/projects",
        &[],
        Some(serde_json::json!({"projectId":project_id,"path":path,"folderOnly":true})),
    )
    .await
}

#[tauri::command]
pub async fn retry_folder_project(
    app: tauri::AppHandle,
    project_id: String,
    path: String,
) -> Result<Value, String> {
    required(&project_id)?;
    if crate::local::connection_path(&app).is_none()
        || !std::path::Path::new(&path).is_absolute()
        || !std::path::Path::new(&path).is_dir()
    {
        return Err("Dossier local indisponible".into());
    }
    map_folder_project(app, &project_id, &path).await
}

#[tauri::command]
pub fn open_conversation_link(app: tauri::AppHandle, url: String) -> Result<(), String> {
    use tauri_plugin_opener::OpenerExt;
    let parsed = conversation_link_url(&url)?;
    app.opener()
        .open_url(parsed.as_str(), None::<&str>)
        .map_err(|_| "Impossible d’ouvrir le lien".into())
}

fn conversation_link_url(url: &str) -> Result<reqwest::Url, String> {
    let parsed = reqwest::Url::parse(url).map_err(|_| "Lien invalide")?;
    if !["http", "https"].contains(&parsed.scheme())
        || !parsed.username().is_empty()
        || parsed.password().is_some()
    {
        return Err("Lien non pris en charge".into());
    }
    Ok(parsed)
}

#[cfg(test)]
mod conversation_link_tests {
    use super::*;
    #[test]
    fn generated_links_allow_only_web_urls_without_embedded_credentials() {
        for value in [
            "javascript:alert(1)",
            "file:///tmp/a",
            "data:text/html,hello",
            "/relative",
            "https://user:secret@example.com",
        ] {
            assert!(
                conversation_link_url(value).is_err(),
                "unsafe URL was accepted"
            );
        }
        assert!(conversation_link_url("https://example.com/docs?q=hello").is_ok());
    }
}

#[cfg(test)]
mod question_answer_tests {
    use super::*;
    #[test]
    fn answers_use_the_existing_go_approval_contract_and_validate_size() {
        let input = serde_json::from_value::<ConversationAction>(serde_json::json!({"action":"answer","id":"q","answers":{"Which scope?":"Small","Checks?":"Unit, Integration"}})).unwrap();
        assert_eq!(
            conversation_body(input).unwrap(),
            serde_json::json!({"approval":{"id":"q","decision":"answer","answers":{"Which scope?":"Small","Checks?":"Unit, Integration"}}})
        );
        for answers in [
            serde_json::json!({}),
            serde_json::json!({"Question":" "}),
            serde_json::json!({"Question":"x".repeat(16385)}),
        ] {
            let input = serde_json::from_value::<ConversationAction>(
                serde_json::json!({"action":"answer","id":"q","answers":answers}),
            )
            .unwrap();
            assert!(conversation_body(input).is_err());
        }
        assert!(serde_json::from_value::<ConversationAction>(serde_json::json!({"action":"answer","id":"q","answers":{"Q":"A"},"mode":"bypassPermissions"})).is_err());
    }
}

fn resolve_conversation_mode(mode: &str, provider: &str) -> Result<&'static str, String> {
    match (provider, mode) {
        ("claude" | "codex", "plan") => Ok("plan"),
        ("claude", "execute") => Ok("default"),
        ("codex", "execute") => Ok("workspace-write"),
        _ => Err("Mode ou fournisseur non pris en charge".into()),
    }
}
#[tauri::command]
pub async fn provider_models(app: tauri::AppHandle, provider: String) -> Result<Value, String> {
    if !["claude", "codex"].contains(&provider.as_str()) {
        return Err("Fournisseur invalide".into());
    }
    let catalogue = request_agent(app, reqwest::Method::GET, "/desktop/engines", &[], None).await?;
    Ok(catalogue
        .get("providerModels")
        .and_then(|models| models.get(&provider))
        .cloned()
        .unwrap_or(serde_json::json!([])))
}

#[cfg(test)]
mod turn_options_tests {
    use super::*;
    #[test]
    fn plan_and_execute_map_to_provider_modes_without_bypass() {
        assert_eq!(resolve_conversation_mode("plan", "claude").unwrap(), "plan");
        assert_eq!(
            resolve_conversation_mode("execute", "claude").unwrap(),
            "default"
        );
        assert_eq!(
            resolve_conversation_mode("execute", "codex").unwrap(),
            "workspace-write"
        );
        assert!(resolve_conversation_mode("bypassPermissions", "claude").is_err());
        assert!(resolve_conversation_mode("execute", "unknown").is_err());
    }
    #[test]
    fn model_selection_is_optional_and_cannot_inject_arguments() {
        for model in ["--bad", "model --dangerous", "$(whoami)", "model\nother"] {
            let input = serde_json::from_value::<ConversationAction>(
                serde_json::json!({"action":"message","message":"Hello","model":model}),
            )
            .unwrap();
            assert!(conversation_body(input).is_err());
        }
        let input = serde_json::from_value::<ConversationAction>(serde_json::json!({"action":"message","message":"Hello","model":"provider/model-v1","mode":"plan"})).unwrap();
        assert_eq!(
            conversation_body(input).unwrap(),
            serde_json::json!({"message":"Hello","model":"provider/model-v1","mode":"plan"})
        );
    }
}

#[tauri::command]
pub async fn conversation_profiles(
    app: tauri::AppHandle,
    project_id: String,
) -> Result<Value, String> {
    required(&project_id)?;
    request_agent(
        app,
        reqwest::Method::GET,
        "/desktop/conversation-profiles",
        &[("projectId", &project_id)],
        None,
    )
    .await
}

#[derive(Deserialize, serde::Serialize)]
#[serde(tag = "action", rename_all = "camelCase", deny_unknown_fields)]
pub enum ProfileAction {
    Select {
        id: String,
    },
    Delete {
        id: String,
    },
    Save {
        id: String,
        name: String,
        provider: String,
        model: String,
        mode: String,
        #[serde(rename = "makeDefault")]
        make_default: bool,
    },
}
#[tauri::command]
pub async fn control_conversation_profiles(
    app: tauri::AppHandle,
    project_id: String,
    input: ProfileAction,
) -> Result<Value, String> {
    required(&project_id)?;
    request_agent(
        app,
        reqwest::Method::POST,
        "/desktop/conversation-profiles",
        &[("projectId", &project_id)],
        Some(serde_json::to_value(input).map_err(|e| e.to_string())?),
    )
    .await
}
#[tauri::command]
pub async fn rename_conversation(
    app: tauri::AppHandle,
    run_id: String,
    title: String,
) -> Result<Value, String> {
    required(&run_id)?;
    required(&title)?;
    request_agent(
        app,
        reqwest::Method::POST,
        "/desktop/conversation/title",
        &[],
        Some(serde_json::json!({"id":run_id,"title":title})),
    )
    .await
}

#[tauri::command]
pub async fn project_folders(app: tauri::AppHandle, project_id: String) -> Result<Value, String> {
    required(&project_id)?;
    request_agent(
        app,
        reqwest::Method::GET,
        "/desktop/folders",
        &[("projectId", &project_id)],
        None,
    )
    .await
}

#[tauri::command]
pub async fn detach_project_folder(
    app: tauri::AppHandle,
    project_id: String,
    path: String,
) -> Result<Value, String> {
    required(&project_id)?;
    required(&path)?;
    // This removes a local association only; it never deletes filesystem content.
    request_agent(
        app,
        reqwest::Method::DELETE,
        "/desktop/folders",
        &[("projectId", &project_id), ("path", &path)],
        None,
    )
    .await
}

pub async fn attach_chosen_folder(
    app: tauri::AppHandle,
    project_id: &str,
    run_id: Option<&str>,
    path: &str,
) -> Result<Value, String> {
    required(project_id)?;
    if let Some(id) = run_id {
        required(id)?;
        let runs = request_agent(
            app.clone(),
            reqwest::Method::GET,
            "/desktop/runs",
            &[],
            None,
        )
        .await?;
        if !runs.as_array().is_some_and(|items| {
            items
                .iter()
                .any(|run| run["id"] == id && run["projectId"] == project_id)
        }) {
            return Err("Cette conversation n’appartient pas au projet".into());
        }
        request_agent(
            app,
            reqwest::Method::POST,
            "/desktop/run-folder",
            &[],
            Some(serde_json::json!({"runId":id,"path":path})),
        )
        .await
    } else {
        request_agent(
            app,
            reqwest::Method::POST,
            "/desktop/folders",
            &[],
            Some(serde_json::json!({"projectId":project_id,"path":path})),
        )
        .await
    }
}
