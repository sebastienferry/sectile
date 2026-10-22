package db

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"tasks/internal/models"
	"tasks/internal/tracker"
)

// packageFiles parses the non-test sources of this package.
func packageFiles(t *testing.T) (*token.FileSet, []*ast.File) {
	t.Helper()
	fset := token.NewFileSet()
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var files []*ast.File
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		file, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, file)
	}
	return fset, files
}

// Every kind of tracker operation declares the credential it goes out with
// (ADR 0058), and the table names no kind that does not exist: a new kind
// added without a route fails here before it can borrow somebody's credential.
func TestEveryTrackerOpKindHasARoute(t *testing.T) {
	_, files := packageFiles(t)
	declared := map[TrackerOpKind]bool{}
	for _, file := range files {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				value := spec.(*ast.ValueSpec)
				ident, ok := value.Type.(*ast.Ident)
				if !ok || ident.Name != "TrackerOpKind" {
					continue
				}
				for i := range value.Names {
					lit, ok := value.Values[i].(*ast.BasicLit)
					if !ok {
						t.Fatalf("%s is not a literal", value.Names[i].Name)
					}
					declared[TrackerOpKind(strings.Trim(lit.Value, `"`))] = true
				}
			}
		}
	}
	if len(declared) == 0 {
		t.Fatal("no TrackerOpKind constant found")
	}
	for kind := range declared {
		if _, ok := trackerOpRoutes[kind]; !ok {
			t.Errorf("%s declares no credential route in trackerOpRoutes", kind)
		}
	}
	for kind := range trackerOpRoutes {
		if !declared[kind] {
			t.Errorf("trackerOpRoutes names %s, which is no TrackerOpKind constant", kind)
		}
	}
}

func TestAnOperationWithoutARouteIsRefused(t *testing.T) {
	if _, _, err := buildTrackerOpJob(TrackerOp{Kind: "invented"}); err == nil {
		t.Fatal("a kind with no declared route must not be queued")
	}
	_, job, err := buildTrackerOpJob(TrackerOp{Kind: TrackerOpStage, UserID: "usr_ada"})
	if err != nil || !job.Op.ServiceCredential {
		t.Fatalf("a stage is Sectile's own write: %+v %v", job.Op, err)
	}
	_, job, err = buildTrackerOpJob(TrackerOp{Kind: TrackerOpAssign, UserID: "usr_ada"})
	if err != nil || job.Op.ServiceCredential {
		t.Fatalf("an assignment is the person's own action: %+v %v", job.Op, err)
	}
}

// directWriteRoutes is every function of this package that writes to a tracker
// outside the operation queue, with the credential it goes out with. A
// function marked "service" sets tracker.WithServiceCredential itself or is
// only reached from one that does; "direct" is a ticket action the person
// performs, made with their own credential unless the call comes from a skill
// run (ADR 0058).
var directWriteRoutes = map[string]string{
	// Ticket actions.
	"CreateTaskAs":                "direct",
	"ConvertTaskToRemote":         "direct",
	"createStoryInRoadmapProject": "direct",
	"createStoryUnder":            "direct",
	"applyTaskMacro":              "direct",
	"MigrateTasks":                "direct",
	"AddTaskCommentAs":            "direct",
	"processTrackerUpdateJob":     "direct",
	"writeGitlabMacroLabels":      "direct",
	// The macro lifecycle and its mirrors.
	"CreateMacro":              "service",
	"UpdateMacro":              "service",
	"DeleteMacro":              "service",
	"MigrateMacro":             "service",
	"writeMilestoneTodosBlock": "service",
	"pushMacroCopy":            "service",
	"pushEpicAxisField":        "service",
	"PushMacroLabels":          "service",
	"pushMacroLabels":          "service",
	// Sprints are planning objects, not tickets.
	"CreateProjectSprints": "service",
	"UpdateProjectSprint":  "service",
	"DeleteProjectSprint":  "service",
	"moveOpenWork":         "service",
}

// writeCalls are the names whose call is a tracker write made outside the
// queue: the write client of the store, and the write methods of a
// TicketingSystem.
var writeCalls = map[string]bool{
	"trackerForWrite": true, "CreateIssue": true, "UpdateIssue": true, "DeleteIssue": true,
	"AddComment": true, "UpdateLabels": true, "Assign": true, "SetParent": true, "SetTeam": true,
	"SetSprint": true, "CreateSprint": true, "UpdateSprint": true, "DeleteSprint": true,
	"SetEpicAxisField": true, "UpsertMarkedComment": true, "Transition": true,
}

// Every tracker write made outside the queue is listed in directWriteRoutes,
// so a new one cannot appear without somebody deciding whose credential it
// uses.
func TestEveryTrackerWriteSiteHasARoute(t *testing.T) {
	_, files := packageFiles(t)
	found := map[string]bool{}
	for _, file := range files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			name := fn.Name.Name
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				var called string
				switch fun := call.Fun.(type) {
				case *ast.SelectorExpr:
					called = fun.Sel.Name
				case *ast.Ident:
					called = fun.Name
				}
				// A run* function executes a queued operation, whose route
				// trackerOpRoutes declares: it is not a site of its own.
				if writeCalls[called] && !strings.HasPrefix(name, "run") {
					found[name] = true
				}
				return true
			})
		}
	}
	var missing []string
	for name := range found {
		if _, ok := directWriteRoutes[name]; !ok {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("tracker writes with no declared credential route: %v", missing)
	}
	for name, route := range directWriteRoutes {
		if !found[name] {
			t.Errorf("directWriteRoutes names %s, which writes to no tracker", name)
		}
		if route != "direct" && route != "service" {
			t.Errorf("%s: unknown route %q", name, route)
		}
	}
}

// A ticket action queued from a skill run goes out with the server credential,
// still recorded as the person's; the same action queued by the person
// themself carries their own token (ADR 0058).
func TestADirectOperationFromASkillRunUsesTheServerCredential(t *testing.T) {
	f := newIsolationFixture(t)
	task := f.remoteTask(t, "Labels")
	for _, round := range []struct {
		ctx   context.Context
		token string
	}{
		{f.ada, "ada-token"},
		{tracker.WithServiceCredential(f.ada), "server-token"},
		{tracker.WithServiceCredential(f.grace), "server-token"},
	} {
		before := f.github.count()
		act, err := f.d.EnqueueTrackerOp(round.ctx, TrackerOp{
			Kind: TrackerOpTaskLabels, ProjectID: f.project.ID, TaskID: task.ID, TaskKey: task.Key, Labels: []string{"bug"},
		})
		if err != nil {
			t.Fatal(err)
		}
		done := f.finished(t, act.ID)
		if done.Status != string(models.ActivityStatusCompleted) || done.UserID != tracker.ActingUser(round.ctx) {
			t.Fatalf("the labels must be written under their author's name: %+v", done)
		}
		signedOnlyBy(t, f.github.since(before), round.token)
	}
}
