package agent

import (
	"slices"
	"testing"
)

func TestADirectoryTerminalOpensOnTheFolderWithoutAttaching(t *testing.T) {
	dir := "/work/my repo"
	for _, tc := range []struct {
		goos, app string
		name      string
		args      []string
	}{
		{"darwin", "terminal", "open", []string{"-a", "Terminal", dir}},
		{"darwin", "iTerm2", "open", []string{"-a", "iTerm", dir}},
		{"darwin", "ghostty", "open", []string{"-na", "Ghostty", "--args", "--working-directory=" + dir}},
		{"windows", "wt", "wt.exe", []string{"-w", "0", "nt", "-d", dir}},
		{"windows", "cmd", "cmd.exe", []string{"/c", "start", "", "/D", dir, "cmd.exe"}},
		{"linux", "gnome-terminal", "gnome-terminal", []string{"--working-directory=" + dir}},
		{"linux", "kitty", "kitty", []string{"--directory", dir}},
		{"linux", "x-terminal-emulator", "x-terminal-emulator", nil},
		{"darwin", "wezterm start --cwd {dir}", "wezterm", []string{"start", "--cwd", dir}},
		{"linux", "foot", "foot", []string{dir}},
	} {
		launch, err := BuildDirectoryTerminal(tc.goos, tc.app, dir)
		if err != nil || launch.Name != tc.name || !slices.Equal(launch.Args, tc.args) {
			t.Errorf("%s %q = %s %q (%v), want %s %q", tc.goos, tc.app, launch.Name, launch.Args, err, tc.name, tc.args)
		}
		for _, arg := range launch.Args {
			if arg == "attach" {
				t.Errorf("%s %q attaches to a session: %q", tc.goos, tc.app, launch.Args)
			}
		}
	}
}
