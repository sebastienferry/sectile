use std::path::Path;

#[tauri::command]
pub fn open_terminal_at(path: String) -> Result<(), String> {
    let folder = Path::new(&path);
    if !folder.is_absolute() || !folder.is_dir() {
        return Err("Dossier de travail introuvable".into());
    }
    // This is a terminal window explicitly requested by the user.
    #[cfg(target_os = "macos")]
    let mut command = {
        let mut command = std::process::Command::new("/usr/bin/open");
        command.arg("-a").arg("Terminal").arg(folder);
        command
    };
    #[cfg(target_os = "linux")]
    let mut command = {
        let mut command = std::process::Command::new("x-terminal-emulator");
        command.current_dir(folder);
        command
    };
    #[cfg(target_os = "windows")]
    let mut command = {
        let mut command = std::process::Command::new("cmd.exe");
        command.arg("/K").arg("cd").arg("/d").arg(folder);
        command
    };
    command.spawn().map(|_| ()).map_err(|_| "Impossible d’ouvrir le terminal".into())
}
