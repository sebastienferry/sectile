package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

func writeMentionFile(t *testing.T, root, name string, modified time.Time) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(name), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(full, modified, modified); err != nil {
		t.Fatal(err)
	}
}

func mentionPaths(entries []mentionEntry) []string {
	var paths []string
	for _, entry := range entries {
		name := entry.Path
		if entry.Folder {
			name += "/"
		}
		paths = append(paths, name)
	}
	sort.Strings(paths)
	return paths
}

// In a checkout the index is Git's view: tracked and untracked files, never an
// ignored one, and never a symlink leading out of the root.
func TestMentionIndexFollowsGitAndDropsEscapingLinks(t *testing.T) {
	root := t.TempDir()
	if _, err := gitLocal(context.Background(), root, "init"); err != nil {
		t.Skip("git unavailable: ", err)
	}
	now := time.Now()
	writeMentionFile(t, root, ".gitignore", now)
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("secret.txt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeMentionFile(t, root, "main.go", now)
	writeMentionFile(t, root, "src/app/view.ts", now)
	writeMentionFile(t, root, "secret.txt", now)
	outside := filepath.Join(t.TempDir(), "passwd")
	if err := os.WriteFile(outside, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Skip("symlinks unavailable: ", err)
	}
	if err := os.Symlink(filepath.Join(root, "main.go"), filepath.Join(root, "inside")); err != nil {
		t.Fatal(err)
	}
	got := mentionPaths(indexMentionRoot(context.Background(), root))
	want := []string{".gitignore", "inside", "main.go", "src/", "src/app/", "src/app/view.ts"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("index = %q, want %q", got, want)
	}
}

// Outside a checkout the walk skips .git and node_modules.
func TestMentionIndexWalksAPlainFolder(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	writeMentionFile(t, root, "notes.md", now)
	writeMentionFile(t, root, "node_modules/lib/index.js", now)
	writeMentionFile(t, root, ".git/config", now)
	if got, want := mentionPaths(indexMentionRoot(context.Background(), root)), []string{"notes.md"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("index = %q, want %q", got, want)
	}
}

// A folder carries its newest descendant's time, and ranking puts a base name
// prefix before a path prefix before a substring, newest first in a tier.
func TestMentionRankingUsesTiersThenNewestTime(t *testing.T) {
	root := t.TempDir()
	old, recent, newest := time.Now().Add(-3*time.Hour), time.Now().Add(-time.Hour), time.Now()
	writeMentionFile(t, root, "docs/old.md", old)
	writeMentionFile(t, root, "docs/deep/new.md", newest)
	writeMentionFile(t, root, "readme-docs.md", recent)
	writeMentionFile(t, root, "src/docsgen.go", old)
	writeMentionFile(t, root, "src/docs.go", recent)
	entries := indexMentionRoot(context.Background(), root)
	for _, entry := range entries {
		if entry.Path == "docs" && !entry.ModTime.Equal(newest.Truncate(time.Second)) && !entry.ModTime.Equal(newest) {
			t.Fatalf("docs/ time = %v, want its newest descendant's %v", entry.ModTime, newest)
		}
	}
	roots := []mentionRoot{{Label: workingFolderLabel, Path: root}}
	var got []string
	for _, item := range rankMentionFiles(roots, [][]mentionEntry{entries}, "DOCS", 50) {
		got = append(got, item.entry.Path)
	}
	want := []string{"docs", "src/docs.go", "src/docsgen.go", "docs/deep", "docs/deep/new.md", "docs/old.md", "readme-docs.md"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ranking = %q, want %q", got, want)
	}
	if limited := rankMentionFiles(roots, [][]mentionEntry{entries}, "", 2); len(limited) != 2 || limited[0].entry.ModTime.Before(limited[1].entry.ModTime) {
		t.Fatalf("an empty query keeps the newest first within the limit: %+v", limited)
	}
}

func TestMentionsParseChipsAndFolders(t *testing.T) {
	message := "See @[plan.md](sectile:file/%2Ftmp%2Fa%20b%2Fplan.md), @[#4](sectile:ticket/gh-1-4) and @docs/ then @[plan.md](sectile:file/%2Ftmp%2Fa%20b%2Fplan.md) mail me@example.com @[Chat](sectile:conversation/run-1)"
	got := parseMentions(message)
	want := []mentionTarget{
		{kind: "file", label: "plan.md", id: "/tmp/a b/plan.md"},
		{kind: "ticket", label: "#4", id: "gh-1-4"},
		{kind: "conversation", label: "Chat", id: "run-1"},
		{kind: "folder", label: "docs/", id: "docs"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mentions = %+v, want %+v", got, want)
	}
}

// Folders side by side, or followed by punctuation, are each mentioned; a
// path with parentheses survives its encoding.
func TestMentionsParseAdjacentFoldersAndParentheses(t *testing.T) {
	got := parseMentions("@src/ @docs/, and @web/(app)/ not@this/ @[page](sectile:file/%2Frepo%2F%28marketing%29%2Fpage.tsx)")
	want := []mentionTarget{
		{kind: "file", label: "page", id: "/repo/(marketing)/page.tsx"},
		{kind: "folder", label: "src/", id: "src"},
		{kind: "folder", label: "docs/", id: "docs"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mentions = %+v, want %+v", got, want)
	}
}

// A request cancelled while the index builds leaves nothing cached.
func TestMentionIndexIsNotCachedWhenItsRequestEnds(t *testing.T) {
	root := t.TempDir()
	writeMentionFile(t, root, "a.md", time.Now())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cachedMentionIndex(ctx, root)
	mentionIndexes.Lock()
	_, cached := mentionIndexes.byRoot[root]
	mentionIndexes.Unlock()
	if cached {
		t.Fatal("an index built for a cancelled request was cached")
	}
	if got := mentionPaths(cachedMentionIndex(context.Background(), root)); !reflect.DeepEqual(got, []string{"a.md"}) {
		t.Fatalf("index = %q", got)
	}
}

// A path is resolved again by the agent: inside a root, of the right kind.
func TestMentionPathsStayInsideTheRoots(t *testing.T) {
	working, attached, outside := t.TempDir(), t.TempDir(), t.TempDir()
	writeMentionFile(t, working, "src/a.go", time.Now())
	writeMentionFile(t, attached, "notes/todo.md", time.Now())
	writeMentionFile(t, outside, "secret", time.Now())
	roots := mentionRootsFor(working, []string{attached, filepath.Join(outside, "missing")})
	if len(roots) != 2 || roots[0].Label != workingFolderLabel || roots[1].Label != filepath.Base(attached) {
		t.Fatalf("roots = %+v", roots)
	}
	if root, rel, ok := resolvedPath(roots, filepath.Join(attached, "notes", "todo.md"), false); !ok || rel != "notes/todo.md" || root.Path != filepath.Clean(attached) {
		t.Fatalf("attached file = %v %q %v", root, rel, ok)
	}
	if _, rel, ok := resolvedPath(roots, "src", true); !ok || rel != "src" {
		t.Fatalf("relative folder = %q %v", rel, ok)
	}
	for name, test := range map[string]struct {
		target string
		folder bool
	}{
		"outside":       {filepath.Join(outside, "secret"), false},
		"climbing":      {"../" + filepath.Base(outside) + "/secret", false},
		"folder a file": {"src/a.go", true},
		"file a folder": {filepath.Join(working, "src"), false},
	} {
		if _, _, ok := resolvedPath(roots, test.target, test.folder); ok {
			t.Errorf("%s: %q was resolved", name, test.target)
		}
	}
}

// Sending a message with mentions gives the provider a referenced-context
// block, resolving each kind and refusing what is not the project's, while the
// journal keeps the message as typed.
func TestAMessageWithMentionsSendsItsReferencedContext(t *testing.T) {
	d, config := disconnectFixture(t)
	fakeRecordingClaude(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/v1/agent/config":
			_ = json.NewEncoder(w).Encode(config)
		case r.URL.Path == "/api/tasks/gh-p-1":
			_, _ = w.Write([]byte(`{"id":"gh-p-1","key":"#1","title":"Plan the cache","status":"to_test","projectId":"p","externalUrl":"https://tracker.test/1"}`))
		case r.URL.Path == "/api/tasks/gh-o-2":
			_, _ = w.Write([]byte(`{"id":"gh-o-2","key":"#2","title":"Secret","status":"new","projectId":"other"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	d.link.serverURL = server.URL
	d.probeCommandsFn = func(*exec.Cmd) []conversationSlash { return nil }
	d.checkMCPFn = func(string, map[string]string) conversationMCP { return conversationMCP{Status: "unknown"} }
	t.Cleanup(d.stopConversations)
	addFormerSource(t, d, "source", "p", conversationEvent{Kind: "user", Text: "Design the cache"}, conversationEvent{Kind: "assistant", Text: "Use an LRU."})
	addFormerSource(t, d, "foreign", "other", conversationEvent{Kind: "user", Text: "Hidden"})

	rec := disconnectRequest(d, "POST", "/desktop/consoles", `{"projectId":"p","provider":"claude","view":"conversation"}`)
	var entry desktopRun
	if rec.Code != 202 || json.Unmarshal(rec.Body.Bytes(), &entry) != nil {
		t.Fatalf("conversation = %d %s", rec.Code, rec.Body.String())
	}
	d.queue.mu.Lock()
	run := d.queue.runs[entry.ID]
	directory := run.desktop.Directory
	d.queue.mu.Unlock()
	writeMentionFile(t, directory, "docs/plan.md", time.Now())

	if w := conversationRequest(d, "GET", "/desktop/mentions/files?runId="+entry.ID+"&query=plan", "", "private"); w.Code != 200 || !strings.Contains(w.Body.String(), `"path":"docs/plan.md"`) || !strings.Contains(w.Body.String(), `"rootLabel":"Working folder"`) {
		t.Fatalf("mention files = %d %s", w.Code, w.Body.String())
	}

	plan := filepath.Join(directory, "docs", "plan.md")
	typed := "Review @[plan.md](sectile:file/" + url.PathEscape(plan) + ") with @[#1](sectile:ticket/gh-p-1), @[#2](sectile:ticket/gh-o-2), @[Old chat](sectile:conversation/source), @[Foreign](sectile:conversation/foreign), @docs/ and @[passwd](sectile:file/%2Fetc%2Fpasswd)"
	body, _ := json.Marshal(map[string]string{"message": typed})
	if w := conversationRequest(d, "POST", "/desktop/conversation?id="+entry.ID, string(body), "private"); w.Code != http.StatusAccepted {
		t.Fatalf("send: %d %s", w.Code, w.Body.String())
	}
	waitConversationIdle(t, d, run)
	sent := recordedMessages(t, directory)
	if len(sent) != 1 {
		t.Fatalf("turns = %q", sent)
	}
	for _, part := range []string{
		typed + "\n\n<referenced-context>",
		"- File `docs/plan.md` in Working folder: " + plan,
		"- Folder `docs/` in Working folder: " + filepath.Join(directory, "docs"),
		`- Ticket #1 (task ID gh-p-1): "Plan the cache" · stage: to_test · https://tracker.test/1. Read its description and comments with the Sectile MCP get_task tool.`,
		`- Ticket "#2": refused, it belongs to another project.`,
		`<conversation title="Old chat">`, "User: Design the cache", "Assistant: Use an LRU.",
		`- Conversation "Foreign": belongs to another project.`,
		`- File "passwd": unavailable`,
		"</referenced-context>",
	} {
		if !strings.Contains(sent[0], part) {
			t.Errorf("provider message lacks %q:\n%s", part, sent[0])
		}
	}
	if strings.Contains(sent[0], "Hidden") || strings.Contains(sent[0], "Secret") {
		t.Fatalf("another project's content leaked:\n%s", sent[0])
	}
	_, lines := sourceState(d, run)
	if got := userEvents(lines); !reflect.DeepEqual(got, []string{typed}) {
		t.Fatalf("journaled user messages = %q", got)
	}
}
