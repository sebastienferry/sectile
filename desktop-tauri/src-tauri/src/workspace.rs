use crate::agent::request_agent;
use reqwest::Method;
use serde_json::{Value, json};

fn provider_check(provider: &str) -> Result<(), String> {
    if ["claude", "codex"].contains(&provider) {
        Ok(())
    } else {
        Err("Moteur non pris en charge".into())
    }
}
fn project_view(source: &Value) -> Value {
    // Deliberately omit provider templates, tokens and embedded skill instructions.
    let mut result = json!({});
    for field in [
        "configured",
        "path",
        "specPath",
        "specDefault",
        "specKind",
        "issueSpecPath",
        "issueSpecDefault",
        "issueSpecKind",
        "aiProvider",
        "aiModel",
        "issueTracker",
        "useWorktrees",
        "parallelism",
        "defaultEngine",
        "board",
    ] {
        if let Some(value) = source.get(field) {
            result[field] = value.clone();
        }
    }
    if let Some(value) = source.pointer("/server/issueTracker") {
        result["issueTracker"] = value.clone();
    }
    result["skills"] = Value::Array(
        source["skills"]
            .as_array()
            .map(|skills| {
                skills
                    .iter()
                    .filter_map(|skill| {
                        let id = skill["id"].as_str()?;
                        Some(json!({"id":id,"command":skill["command"].as_str()}))
                    })
                    .collect()
            })
            .unwrap_or_default(),
    );
    result["policy"] = json!({
        "prCreationStage":source.pointer("/server/prCreationStage"),
        "fullChainStopStage":source.pointer("/server/fullChainStopStage"),
        "defaultSkillMode":source.pointer("/server/defaultSkillMode"),
        "specFramework":source.pointer("/server/specFramework")
    });
    result
}
fn apply_mapped_folder_fallback(source: &mut Value, projects: &Value, project_id: &str) {
    if source["path"].as_str().is_some_and(|path| !path.is_empty()) {
        return;
    }
    let path = projects
        .as_array()
        .and_then(|items| {
            items
                .iter()
                .find(|item| item["id"] == project_id && item["disconnected"] != true)
        })
        .and_then(|item| item["path"].as_str())
        .filter(|path| !path.is_empty() && std::path::Path::new(path).is_absolute());
    if let Some(path) = path {
        source["path"] = Value::String(path.to_owned());
        for field in ["specDefault", "issueSpecDefault"] {
            if source[field].as_str().is_none_or(|value| value.is_empty()) {
                source[field] = Value::String(path.to_owned());
            }
        }
        // Keep configured=false: a folder can support prompts without being a Git workflow checkout.
    }
}
async fn project_metadata(app: tauri::AppHandle, project_id: &str) -> Result<Value, String> {
    let mut source = request_agent(
        app.clone(),
        Method::GET,
        "/desktop/project",
        &[("id", project_id)],
        None,
    )
    .await?;
    if source["path"].as_str().is_none_or(|path| path.is_empty()) {
        let projects = request_agent(app, Method::GET, "/desktop/projects", &[], None).await?;
        apply_mapped_folder_fallback(&mut source, &projects, project_id);
    }
    Ok(source)
}
#[tauri::command]
pub async fn project_context(app: tauri::AppHandle, project_id: String) -> Result<Value, String> {
    if project_id.trim().is_empty() {
        return Err("Projet requis".into());
    }
    Ok(project_view(&project_metadata(app, &project_id).await?))
}
#[tauri::command]
pub async fn project_repositories(
    app: tauri::AppHandle,
    project_id: String,
) -> Result<Value, String> {
    if project_id.trim().is_empty() {
        return Err("Projet requis".into());
    }
    request_agent(
        app,
        Method::GET,
        "/desktop/repositories",
        &[("projectId", &project_id)],
        None,
    )
    .await
}
#[tauri::command]
pub async fn provider_readiness(app: tauri::AppHandle, provider: String) -> Result<Value, String> {
    provider_check(&provider)?;
    request_agent(
        app,
        Method::GET,
        "/desktop/setup",
        &[("provider", &provider)],
        None,
    )
    .await
}
#[tauri::command]
pub async fn install_provider_skills(
    app: tauri::AppHandle,
    provider: String,
    project_id: String,
) -> Result<Value, String> {
    provider_check(&provider)?;
    if project_id.trim().is_empty() {
        return Err("Choisissez un projet source".into());
    }
    request_agent(
        app,
        Method::POST,
        "/desktop/project",
        &[
            ("id", &project_id),
            ("action", "provider-skills"),
            ("provider", &provider),
        ],
        Some(json!({})),
    )
    .await
}
#[tauri::command]
pub async fn configure_provider_mcp(
    app: tauri::AppHandle,
    provider: String,
    replace_existing: bool,
) -> Result<Value, String> {
    provider_check(&provider)?;
    let view = request_agent(
        app.clone(),
        Method::GET,
        "/desktop/setup",
        &[("provider", &provider)],
        None,
    )
    .await?;
    if view.pointer("/steps/mcp/configuration/present") == Some(&Value::Bool(true))
        && view.pointer("/steps/mcp/configuration/valid") != Some(&Value::Bool(true))
        && !replace_existing
    {
        return Err("Confirmez explicitement le remplacement de l’entrée MCP existante".into());
    }
    // Register this provider only. Do not remove project entries or auto-repair another desktop.
    request_agent(
        app,
        Method::POST,
        "/desktop/mcp",
        &[("provider", &provider)],
        Some(json!({"target":"local","transport":"http","repair":false})),
    )
    .await
}

#[cfg(test)]
mod tests {
    #[test]
    fn non_git_folder_metadata_keeps_workflow_readiness_distinct() {
        let mut source =
            json!({"configured":false,"path":"","specDefault":"","issueSpecDefault":""});
        apply_mapped_folder_fallback(
            &mut source,
            &json!([{"id":"p","path":"/plain-folder","disconnected":false}]),
            "p",
        );
        assert_eq!(source["path"], "/plain-folder");
        assert_eq!(source["specDefault"], "/plain-folder");
        assert_eq!(source["configured"], false);
        let mut other = json!({"configured":false,"path":""});
        apply_mapped_folder_fallback(
            &mut other,
            &json!([{"id":"p","path":"/plain-folder","disconnected":true}]),
            "p",
        );
        assert_eq!(other["path"], "");
    }
    use super::*;
    #[test]
    fn project_metadata_never_exposes_credentials_or_skill_prompts() {
        let result = project_view(
            &json!({"path":"/project","configured":true,"skills":[{"id":"clarify","command":"clarify-issue","content":"private instructions"}],"server":{"token":"secret","issueTracker":"local","skills":[{"content":"private instructions"}],"prCreationStage":"implemented","fullChainStopStage":"reviewed"},"aiCommandTemplate":"secret template"}),
        );
        let text = result.to_string();
        assert!(!text.contains("secret") && !text.contains("private instructions"));
        assert_eq!(result["policy"]["fullChainStopStage"], "reviewed");
        assert_eq!(result["skills"][0]["id"], "clarify");
        assert_eq!(result["issueTracker"], "local");
    }
    #[test]
    fn provider_preparation_has_a_bounded_target() {
        assert!(provider_check("claude").is_ok());
        assert!(provider_check("codex").is_ok());
        assert!(provider_check("custom --shell").is_err());
    }
}

#[tauri::command]
pub async fn run_result(app: tauri::AppHandle, run_id: String) -> Result<Value, String> {
    if run_id.trim().is_empty() {
        return Err("Exécution requise".into());
    }
    request_agent(
        app,
        Method::GET,
        "/desktop/run-result",
        &[("id", &run_id)],
        None,
    )
    .await
}

#[derive(serde::Deserialize, serde::Serialize)]
#[serde(deny_unknown_fields)]
pub struct OrchestratorDefinition {
    id: String,
    name: String,
    file: String,
}
#[derive(serde::Deserialize, serde::Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct OrchestrationSettings {
    orchestrator_enabled: bool,
    orchestrator_max_concurrent: u8,
    agents: Vec<OrchestratorDefinition>,
}
#[tauri::command]
pub async fn save_orchestration(
    app: tauri::AppHandle,
    project_id: String,
    input: OrchestrationSettings,
) -> Result<(), String> {
    if project_id.trim().is_empty()
        || !(1..=10).contains(&input.orchestrator_max_concurrent)
        || input.agents.len() > 20
    {
        return Err("Configuration d’orchestration invalide".into());
    }
    request_agent(
        app,
        Method::PUT,
        "/desktop/orchestration",
        &[("projectId", &project_id)],
        Some(serde_json::to_value(input).map_err(|_| "Configuration invalide")?),
    )
    .await
    .map(|_| ())
}
#[derive(serde::Deserialize, serde::Serialize)]
#[serde(rename_all = "camelCase", deny_unknown_fields)]
pub struct WorkflowPolicy {
    full_chain_stop_stage: String,
    pr_creation_stage: String,
    default_skill_mode: String,
}
#[tauri::command]
pub async fn save_workflow_policy(
    app: tauri::AppHandle,
    project_id: String,
    input: WorkflowPolicy,
) -> Result<(), String> {
    if project_id.trim().is_empty()
        || !["implemented", "reviewed"].contains(&input.full_chain_stop_stage.as_str())
        || !["clarified", "specified", "implemented"].contains(&input.pr_creation_stage.as_str())
        || !["interactive", "autonomous"].contains(&input.default_skill_mode.as_str())
    {
        return Err("Politique de workflow invalide".into());
    }
    request_agent(
        app,
        Method::PUT,
        "/desktop/project",
        &[("id", &project_id)],
        Some(serde_json::to_value(input).map_err(|_| "Configuration invalide")?),
    )
    .await
    .map(|_| ())
}

#[tauri::command]
pub async fn run_diff(
    app: tauri::AppHandle,
    run_id: String,
    folder: Option<String>,
) -> Result<Value, String> {
    if run_id.trim().is_empty() {
        return Err("Exécution requise".into());
    }
    let mut query = vec![("id", run_id.as_str())];
    if let Some(path) = folder.as_deref() {
        query.push(("folder", path));
    }
    request_agent(app, Method::GET, "/desktop/git-diff", &query, None).await
}
#[tauri::command]
pub async fn open_run_editor(
    app: tauri::AppHandle,
    run_id: String,
    folder: Option<String>,
) -> Result<(), String> {
    if run_id.trim().is_empty() {
        return Err("Exécution requise".into());
    }
    request_agent(
        app,
        Method::POST,
        "/desktop/open-editor",
        &[],
        Some(json!({"runId":run_id,"folder":folder.unwrap_or_default()})),
    )
    .await
    .map(|_| ())
}
#[tauri::command]
pub fn open_external_link(app: tauri::AppHandle, url: String) -> Result<(), String> {
    use tauri_plugin_opener::OpenerExt;
    let parsed = reqwest::Url::parse(&url).map_err(|_| "Lien invalide")?;
    if !["https", "http"].contains(&parsed.scheme())
        || parsed.host_str().is_none()
        || !parsed.username().is_empty()
        || parsed.password().is_some()
    {
        return Err("Lien HTTP requis, sans identifiants".into());
    }
    app.opener()
        .open_url(parsed.as_str(), None::<&str>)
        .map_err(|_| "Impossible d’ouvrir le lien".into())
}
#[tauri::command]
pub async fn editor_preference(
    app: tauri::AppHandle,
    editor: Option<String>,
) -> Result<Value, String> {
    let view = request_agent(app.clone(), Method::GET, "/desktop/workstation", &[], None).await?;
    let mut defaults = view["defaults"].clone();
    if let Some(editor) = editor {
        if !["code", "cursor", "zed"].contains(&editor.as_str()) {
            return Err("Éditeur non pris en charge".into());
        }
        defaults["editorCommand"] = Value::String(editor);
        request_agent(
            app,
            Method::PUT,
            "/desktop/workstation",
            &[],
            Some(defaults.clone()),
        )
        .await?;
    }
    let configured = defaults["editorCommand"].as_str().unwrap_or_default();
    Ok(
        json!({"configured":!configured.trim().is_empty(),"selection":if ["code","cursor","zed"].contains(&configured){configured}else{""}}),
    )
}
#[tauri::command]
pub async fn declare_reviewed(
    app: tauri::AppHandle,
    project_id: String,
    task_id: String,
    note: String,
) -> Result<Value, String> {
    if project_id.trim().is_empty()
        || task_id.trim().is_empty()
        || note.trim().is_empty()
        || note.len() > 2000
    {
        return Err("Projet, ticket et justification requis".into());
    }
    request_agent(
        app,
        Method::POST,
        "/desktop/tasks/transition",
        &[("projectId", &project_id)],
        Some(json!({"taskId":task_id,"stage":"reviewed","note":note})),
    )
    .await
}

#[tauri::command]
pub async fn choose_repository_folder(
    app: tauri::AppHandle,
    project_id: String,
    repository: String,
) -> Result<Option<String>, String> {
    if project_id.trim().is_empty() || repository.trim().is_empty() {
        return Err("Projet et dépôt requis".into());
    }
    let Some(path) = crate::local::pick_project_folder(&app).await? else {
        return Ok(None);
    };
    request_agent(
        app,
        Method::POST,
        "/desktop/repositories",
        &[],
        Some(json!({"projectId":project_id,"repository":repository,"path":path})),
    )
    .await?;
    Ok(Some(path))
}
#[tauri::command]
pub async fn choose_specification_folder(
    app: tauri::AppHandle,
    project_id: String,
    kind: String,
    use_primary: bool,
) -> Result<Option<String>, String> {
    if project_id.trim().is_empty() || !["macro", "issue"].contains(&kind.as_str()) {
        return Err("Projet et type de spécification requis".into());
    }
    let project = project_metadata(app.clone(), &project_id).await?;
    let root = project["path"]
        .as_str()
        .filter(|s| !s.is_empty())
        .ok_or("Associez d’abord le dossier de travail du projet")?;
    let selected = if use_primary {
        String::new()
    } else {
        let Some(path) = crate::local::pick_project_folder(&app).await? else {
            return Ok(None);
        };
        path
    };
    let mut body = json!({"projectId":project_id,"path":root,"folderOnly":true});
    body[if kind == "macro" {
        "specPath"
    } else {
        "issueSpecPath"
    }] = Value::String(selected.clone());
    request_agent(app, Method::POST, "/desktop/projects", &[], Some(body)).await?;
    Ok(Some(selected))
}

#[derive(serde::Deserialize, serde::Serialize)]
#[serde(rename_all = "camelCase")]
pub enum ClaudePermissionMode {
    Default,
    AcceptEdits,
    Plan,
    Auto,
}
#[derive(serde::Deserialize, serde::Serialize)]
pub enum CodexReviewer {
    #[serde(rename = "user")]
    User,
    #[serde(rename = "auto_review")]
    AutoReview,
}
#[derive(serde::Deserialize)]
#[serde(tag = "provider", rename_all = "lowercase", deny_unknown_fields)]
pub enum ProviderPermissions {
    Claude {
        mode: ClaudePermissionMode,
    },
    Codex {
        #[serde(rename = "approvalsReviewer")]
        approvals_reviewer: CodexReviewer,
    },
}
fn permission_request(input: ProviderPermissions) -> (&'static str, Value) {
    match input {
        ProviderPermissions::Claude { mode } => {
            ("/desktop/conversation-mode", json!({"mode":mode}))
        }
        ProviderPermissions::Codex { approvals_reviewer } => (
            "/desktop/codex-settings",
            json!({"approvalsReviewer":approvals_reviewer}),
        ),
    }
}
#[tauri::command]
pub async fn save_provider_permissions(
    app: tauri::AppHandle,
    input: ProviderPermissions,
) -> Result<Value, String> {
    let (route, body) = permission_request(input);
    request_agent(app, Method::PUT, route, &[], Some(body)).await
}
#[cfg(test)]
mod permission_tests {
    use super::*;
    #[test]
    fn permission_changes_are_bounded_and_do_not_bypass_native_rules() {
        for invalid in [
            json!({"provider":"claude","mode":"bypassPermissions"}),
            json!({"provider":"codex","approvalsReviewer":"shell"}),
            json!({"provider":"claude","mode":"default","command":"run"}),
        ] {
            assert!(serde_json::from_value::<ProviderPermissions>(invalid).is_err());
        }
        let valid = serde_json::from_value::<ProviderPermissions>(
            json!({"provider":"claude","mode":"acceptEdits"}),
        )
        .unwrap();
        let (route, body) = permission_request(valid);
        assert_eq!(route, "/desktop/conversation-mode");
        assert_eq!(body, json!({"mode":"acceptEdits"}));
    }
}
