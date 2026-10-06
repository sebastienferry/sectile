package agent

import (
	"os"
	"reflect"
	"testing"

	"tasks/internal/agentconfig"
	"tasks/internal/models"
)

// The folders of a run are those the engine was told about, the run's own
// directory first, each named and given its role (#762).
func TestRunFoldersListTheFolderMap(t *testing.T) {
	entries := []models.FolderMapEntry{
		{Identity: "github.com/o/docs", Role: models.FolderRoleContext, Path: "/src/docs"},
		{Identity: "github.com/o/app", Role: models.FolderRolePrimary, Path: "/src/app", Worktree: "/src/app/.tasks/worktrees/issue-1"},
		{Identity: "github.com/o/lib", Role: models.FolderRoleChanged, Path: "/src/lib", Worktree: "/src/lib/.tasks/worktrees/issue-1"},
		{Identity: "github.com/o/infra", Role: models.FolderRoleContext},
		{Identity: "github.com/o/api", Role: models.FolderRoleChanged, Path: "/src/api"},
		{Path: "/notes", Role: models.FolderRoleLocal, Kind: folderKindFolder, Attached: true},
		{Path: "/gone", Role: models.FolderRoleContext, Kind: folderKindMissing, Attached: true},
		{Identity: "gitlab.com/g/tools", Path: "/src/tools", Role: models.FolderRoleContext, Kind: folderKindGit, Attached: true},
		{Role: models.FolderRoleSpec, Path: "/specs", Worktree: "/specs/.tasks/worktrees/issue-1"},
		{Role: models.FolderRoleSpec, Path: "/src/docs"},
	}
	got := runFolders("/src/app/.tasks/worktrees/issue-1", entries)
	want := []runFolder{
		{Path: "/src/app/.tasks/worktrees/issue-1", Name: "app", Role: "primary"},
		{Path: "/src/docs", Name: "docs", Role: "context"},
		{Path: "/src/lib/.tasks/worktrees/issue-1", Name: "lib", Role: "changed"},
		{Path: "/src/api", Name: "api", Role: "changed"},
		{Path: "/notes", Name: "notes", Role: "local", Attached: true},
		{Path: "/src/tools", Name: "tools", Role: "context", Attached: true},
		{Path: "/specs/.tasks/worktrees/issue-1", Name: "specifications", Role: "spec"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("folders:\n got %+v\nwant %+v", got, want)
	}
}

func TestRunFoldersOfASingleFolderAreNone(t *testing.T) {
	if got := runFolders("/src/app", nil); got != nil {
		t.Errorf("no map: %+v", got)
	}
	// A free console's map names the project root as its primary worktree.
	single := []models.FolderMapEntry{{Identity: "github.com/o/app", Role: models.FolderRolePrimary, Path: "/src/app", Worktree: "/src/app"}}
	if got := runFolders("/src/app/", single); got != nil {
		t.Errorf("the directory alone: %+v", got)
	}
	unmapped := append(single, models.FolderMapEntry{Identity: "github.com/o/infra", Role: models.FolderRoleContext})
	if got := runFolders("/src/app", unmapped); got != nil {
		t.Errorf("an unmapped repository is not a folder: %+v", got)
	}
}

// A ticket whose code worktree is prepared lazily has no primary entry: its
// directory still comes first, named after itself.
func TestRunFoldersWithoutAPrimaryEntry(t *testing.T) {
	got := runFolders("/specs/.tasks/worktrees/issue-1", []models.FolderMapEntry{
		{Identity: "github.com/o/app", Role: models.FolderRoleContext, Path: "/src/app"},
	})
	want := []runFolder{
		{Path: "/specs/.tasks/worktrees/issue-1", Name: "issue-1", Role: "primary"},
		{Path: "/src/app", Name: "app", Role: "context"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("folders: %+v", got)
	}
}

// A worktree prepared during a ticket's runs joins the list of every run of
// that ticket still going, once, and of no other run.
func TestAPreparedWorktreeJoinsTheTaskRuns(t *testing.T) {
	d := &agentDaemon{}
	listed := &controlledRun{exited: make(chan struct{}), taskID: "t1",
		desktop: desktopRun{TaskID: "t1", Directory: "/src/app/wt", Folders: []runFolder{{Path: "/src/app/wt", Name: "app", Role: "primary"}, {Path: "/notes", Name: "notes", Role: "local"}}}}
	bare := &controlledRun{exited: make(chan struct{}), desktop: desktopRun{TaskKey: "#1", Directory: "/src/app/wt"}}
	ended := &controlledRun{exited: make(chan struct{}), taskID: "t1", desktop: desktopRun{TaskID: "t1", Directory: "/src/app/wt"}}
	close(ended.exited)
	restored := &controlledRun{exited: make(chan struct{}), restored: true, desktop: desktopRun{TaskID: "t1", Directory: "/src/app/wt"}}
	other := &controlledRun{exited: make(chan struct{}), taskID: "t2", desktop: desktopRun{TaskID: "t2", Directory: "/src/x"}}
	d.queue.runs = map[string]*controlledRun{"listed": listed, "bare": bare, "ended": ended, "restored": restored, "other": other}
	copied := append([]runFolder(nil), listed.desktop.Folders...)

	d.recordTaskFolder(models.Task{ID: "t1", Key: "#1"}, "github.com/o/lib", "/src/lib/wt")
	d.recordTaskFolder(models.Task{ID: "t1", Key: "#1"}, "github.com/o/lib", "/src/lib/wt/")

	lib := runFolder{Path: "/src/lib/wt", Name: "lib", Role: "changed"}
	if want := append(copied, lib); !reflect.DeepEqual(listed.desktop.Folders, want) {
		t.Errorf("listed run: %+v", listed.desktop.Folders)
	}
	if want := []runFolder{{Path: "/src/app/wt", Name: "wt", Role: "primary"}, lib}; !reflect.DeepEqual(bare.desktop.Folders, want) {
		t.Errorf("a run with no list yet: %+v", bare.desktop.Folders)
	}
	for name, run := range map[string]*controlledRun{"ended": ended, "restored": restored, "other task": other} {
		if run.desktop.Folders != nil {
			t.Errorf("%s run was given %+v", name, run.desktop.Folders)
		}
	}
	if len(copied) != 2 {
		t.Errorf("a list handed out before was grown in place: %+v", copied)
	}
}

// A conversation reads the project's folders at each turn: a project
// conversation's list becomes them, a ticket's keeps its own and gains them.
func TestAConversationTurnRefreshesItsFolders(t *testing.T) {
	project := &controlledRun{desktop: desktopRun{Directory: "/src/app", Folders: []runFolder{{Path: "/src/app", Name: "app", Role: "primary"}, {Path: "/old", Name: "old", Role: "local"}}}}
	ticket := &controlledRun{desktop: desktopRun{TaskID: "t1", Directory: "/src/app/wt", Folders: []runFolder{{Path: "/src/app/wt", Name: "app", Role: "primary"}, {Path: "/src/lib/wt", Name: "lib", Role: "changed"}}}}
	entries := func(directory string) []models.FolderMapEntry {
		return []models.FolderMapEntry{
			{Identity: "github.com/o/app", Role: models.FolderRolePrimary, Path: "/src/app", Worktree: directory},
			{Path: "/new", Role: models.FolderRoleLocal, Attached: true},
		}
	}
	refreshConversationFolders(project, entries("/src/app"))
	refreshConversationFolders(ticket, entries("/src/app/wt"))
	if want := []runFolder{{Path: "/src/app", Name: "app", Role: "primary"}, {Path: "/new", Name: "new", Role: "local", Attached: true}}; !reflect.DeepEqual(project.desktop.Folders, want) {
		t.Errorf("project conversation: %+v", project.desktop.Folders)
	}
	if want := []runFolder{{Path: "/src/app/wt", Name: "app", Role: "primary"}, {Path: "/src/lib/wt", Name: "lib", Role: "changed"}, {Path: "/new", Name: "new", Role: "local", Attached: true}}; !reflect.DeepEqual(ticket.desktop.Folders, want) {
		t.Errorf("ticket conversation: %+v", ticket.desktop.Folders)
	}
}

// A folder added to a quiet run is written to the store all the same, so a
// restart restores it.
func TestAFolderAddedToAQuietRunIsStored(t *testing.T) {
	d := &agentDaemon{store: testRunStore(t)}
	if _, err := d.enqueueRun("task-1", agentconfig.Dispatch{RunID: "quiet", TaskKey: "#1", SkillID: "implement"}, "project", "/work", 1, false); err != nil {
		t.Fatal(err)
	}
	d.queue.read("quiet", func(run *controlledRun) { run.desktop.Status = "running"; run.desktop.TaskID = "task-1" })
	d.persistRuns()
	_ = os.Remove(d.store.path("quiet"))
	d.recordTaskFolder(models.Task{ID: "task-1"}, "github.com/o/lib", "/lib/wt")
	d.persistRuns()
	loaded := openRunStore(d.store.dir).load()
	if len(loaded) != 1 {
		t.Fatalf("the run with a new folder was not written: %d runs", len(loaded))
	}
	if want := []runFolder{{Path: "/work", Name: "work", Role: "primary"}, {Path: "/lib/wt", Name: "lib", Role: "changed"}}; !reflect.DeepEqual(loaded[0].Run.Folders, want) {
		t.Fatalf("restored folders: %+v", loaded[0].Run.Folders)
	}
}
