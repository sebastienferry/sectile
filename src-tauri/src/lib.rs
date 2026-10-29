mod agent;
mod local;
mod workspace;

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_opener::init())
        .plugin(tauri_plugin_dialog::init())
        .manage(agent::Streams::default())
        .manage(local::LocalRuntime::default())
        .setup(|app| {
            let handle = app.handle().clone();
            tauri::async_runtime::spawn(async move {
                let _ = local::resume(&handle).await;
            });
            Ok(())
        })
        .invoke_handler(tauri::generate_handler![
            local::open_folder_project,
            local::oauth_apps,
            local::save_oauth_app,
            local::begin_oauth,
            local::oauth_status,
            local::cancel_oauth,
            local::enable_local_runtime,
            local::local_setup_status,
            local::save_local_profile,
            local::save_local_credential,
            local::create_local_tracker,
            local::create_local_project,
            local::sync_local_tracker,
            agent::retry_folder_project,
            agent::open_conversation_link,
            agent::provider_models,
            agent::provider_status,
            agent::provider_login,
            agent::create_conversation,
            agent::conversation_profiles,
            agent::control_conversation_profiles,
            agent::rename_conversation,
            agent::read_conversation,
            agent::control_conversation,
            agent::map_local_project,
            local::choose_project_folder,
            local::choose_attached_folder,
            agent::project_folders,
            workspace::project_context,
            workspace::run_result,
            workspace::run_diff,
            workspace::open_run_editor,
            workspace::open_external_link,
            workspace::editor_preference,
            workspace::declare_reviewed,
            workspace::save_orchestration,
            workspace::save_workflow_policy,
            workspace::provider_readiness,
            workspace::save_provider_permissions,
            workspace::install_provider_skills,
            workspace::configure_provider_mcp,
            workspace::project_repositories,
            workspace::choose_repository_folder,
            workspace::choose_specification_folder,
            agent::detach_project_folder,
            agent::list_projects,
            agent::list_tasks,
            agent::execution_snapshot,
            agent::project_skills,
            agent::launch_task,
            agent::stop_run,
            agent::orchestration,
            agent::run_orchestration,
            agent::watch_run,
            agent::unwatch_run
        ])
        .run(tauri::generate_context!())
        .expect("Impossible de démarrer Sectile Tauri");
}
