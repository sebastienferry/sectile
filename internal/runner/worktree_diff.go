package runner

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const diffMetadataLimit = 8 << 20
const diffPatchLimit = 256 << 10
const diffResponseLimit = 4 << 20
const diffFileLimit = 1000

// WorktreeDiff is a local inspection, never a saved execution snapshot.
type WorktreeDiff struct {
	RunID         string             `json:"runId"`
	TaskID        string             `json:"taskId"`
	ProjectID     string             `json:"projectId"`
	Directory     string             `json:"directory"`
	Branch        string             `json:"branch"`
	BaseRef       string             `json:"baseRef"`
	BaseCommit    string             `json:"baseCommit"`
	MergeBase     string             `json:"mergeBase"`
	HeadCommit    string             `json:"headCommit"`
	GeneratedAt   string             `json:"generatedAt"`
	IsClean       bool               `json:"isClean"`
	Complete      bool               `json:"complete"`
	CountsPartial bool               `json:"countsPartial"`
	FilesChanged  int                `json:"filesChanged"`
	Additions     int                `json:"additions"`
	Deletions     int                `json:"deletions"`
	Warnings      []DiffWarning      `json:"warnings"`
	Files         []WorktreeDiffFile `json:"files"`
}
type DiffWarning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
type WorktreeDiffFile struct {
	Path          string `json:"path"`
	OldPath       string `json:"oldPath,omitempty"`
	Status        string `json:"status"`
	Kind          string `json:"kind"`
	Additions     *int   `json:"additions"`
	Deletions     *int   `json:"deletions"`
	Patch         string `json:"patch"`
	OmittedReason string `json:"omittedReason,omitempty"`
}
type DiffError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *DiffError) Error() string         { return e.Message }
func diffError(code, message string) error { return &DiffError{code, message} }

var errDiffBound = errors.New("inspection limit exceeded")

type diffBuffer struct {
	buffer   bytes.Buffer
	limit    int
	exceeded bool
}

func (b *diffBuffer) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.buffer.Len() {
		b.exceeded = true
		return 0, errDiffBound
	}
	return b.buffer.Write(p)
}

type diffGit struct {
	ctx context.Context
	dir string
	env []string
}

func (g diffGit) command(input []byte, limit int, args ...string) ([]byte, error) {
	prefix := []string{"--literal-pathspecs", "-c", "core.fsmonitor=false", "-c", "core.untrackedCache=false", "-c", "diff.external=", "-c", "core.pager=cat", "-C", g.dir}
	cmd := exec.CommandContext(g.ctx, "git", append(prefix, args...)...)
	// Do not inherit caller-selected indexes, repositories, or external helpers.
	for _, v := range os.Environ() {
		if !strings.HasPrefix(v, "GIT_") {
			cmd.Env = append(cmd.Env, v)
		}
	}
	cmd.Env = append(cmd.Env, "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0", "LC_ALL=C")
	cmd.Env = append(cmd.Env, g.env...)
	cmd.Stdin = bytes.NewReader(input)
	out, stderr := &diffBuffer{limit: limit}, &diffBuffer{limit: 4096}
	cmd.Stdout = out
	cmd.Stderr = stderr
	cmd.WaitDelay = 100 * time.Millisecond
	err := cmd.Run()
	if g.ctx.Err() != nil {
		return nil, diffError("timeout", "Inspection timed out or was canceled. Refresh to retry.")
	}
	if out.exceeded || stderr.exceeded || errors.Is(err, errDiffBound) {
		return out.buffer.Bytes(), errDiffBound
	}
	var exit *exec.ExitError
	if len(args) > 0 && args[0] == "check-ignore" && errors.As(err, &exit) && exit.ExitCode() == 1 {
		err = nil
	}
	if err != nil {
		return nil, diffError("git_failed", "Git could not inspect this checkout. Check its local repository and refresh.")
	}
	return out.buffer.Bytes(), nil
}
func (g diffGit) text(args ...string) (string, error) {
	b, e := g.command(nil, diffMetadataLimit, args...)
	return strings.TrimSpace(string(b)), e
}
func canonical(p string) (string, error) {
	a, e := filepath.Abs(p)
	if e != nil {
		return "", e
	}
	return filepath.EvalSymlinks(a)
}
func (g diffGit) common() (string, error) {
	s, e := g.text("rev-parse", "--path-format=absolute", "--git-common-dir")
	if e != nil {
		return "", e
	}
	return canonical(s)
}
func (g diffGit) identity(branch, root string) error {
	top, e := g.text("rev-parse", "--show-toplevel")
	actual, ce := canonical(g.dir)
	resolved, re := canonical(top)
	if e != nil || ce != nil || re != nil || actual != resolved {
		return diffError("checkout_unavailable", "The execution checkout is unavailable. Restore it before refreshing.")
	}
	b, e := g.text("symbolic-ref", "--quiet", "--short", "HEAD")
	if e != nil || branch == "" || b != branch {
		return diffError("checkout_mismatch", "The checkout branch no longer matches this execution.")
	}
	common, e := g.common()
	other, oe := (diffGit{ctx: g.ctx, dir: root}).common()
	if e != nil || oe != nil || root == "" || common != other {
		return diffError("checkout_mismatch", "The checkout no longer belongs to the execution repository.")
	}
	return nil
}
func (g diffGit) baseline() (ref, base, head, ancestor string, err error) {
	head, err = g.text("rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		err = diffError("baseline_unavailable", "The checkout has no available HEAD commit.")
		return
	}
	ref, err = g.text("symbolic-ref", "--quiet", "refs/remotes/origin/HEAD")
	if err != nil {
		ref = ""
		// A broken/non-symbolic origin/HEAD is not permission to choose another base.
		gitdir, ge := g.common()
		if ge != nil {
			err = ge
			return
		}
		if _, se := os.Lstat(filepath.Join(gitdir, "refs/remotes/origin/HEAD")); se == nil {
			err = diffError("baseline_unavailable", "The recorded default branch is invalid.")
			return
		}
		for _, r := range []string{"refs/remotes/origin/main", "refs/remotes/origin/master", "refs/heads/main", "refs/heads/master"} {
			if _, e := g.text("rev-parse", "--verify", r+"^{commit}"); e == nil {
				ref = r
				break
			}
		}
	}
	if ref == "" {
		err = diffError("baseline_unavailable", "No local default branch is available. Fetch the repository outside this viewer and retry.")
		return
	}
	base, err = g.text("rev-parse", "--verify", ref+"^{commit}")
	if err != nil {
		err = diffError("baseline_unavailable", "The recorded default branch commit is unavailable.")
		return
	}
	ancestor, err = g.text("merge-base", "--all", base, head)
	if err != nil || len(strings.Fields(ancestor)) != 1 {
		err = diffError("baseline_unavailable", "A unique common ancestor is unavailable. Check for shallow, unrelated, or ambiguous history.")
	}
	return
}

type diffEntry struct{ mode, oid string }

func (g diffGit) tree(ref string) (map[string]diffEntry, error) {
	raw, e := g.command(nil, diffMetadataLimit, "ls-tree", "-rz", ref)
	if e != nil {
		return nil, e
	}
	result := map[string]diffEntry{}
	for _, line := range bytes.Split(raw, []byte{0}) {
		if len(line) == 0 {
			continue
		}
		h, p, ok := strings.Cut(string(line), "\t")
		fields := strings.Fields(h)
		if !ok || len(fields) != 3 || !utf8.ValidString(p) {
			return nil, diffError("unsupported_path", "A repository path cannot be represented as UTF-8.")
		}
		result[p] = diffEntry{fields[0], fields[2]}
	}
	return result, nil
}
func safeDiffPath(root, p string) (string, error) {
	if p == "" || filepath.IsAbs(p) || filepath.Clean(p) != p || p == ".." || strings.HasPrefix(p, "../") {
		return "", diffError("checkout_mismatch", "An invalid repository path was found.")
	}
	parent := filepath.Dir(p)
	for parent != "." {
		info, e := os.Lstat(filepath.Join(root, parent))
		if e == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
			return "", diffError("checkout_changed", "A parent directory changed. Refresh to retry.")
		}
		if e != nil && !os.IsNotExist(e) {
			return "", e
		}
		parent = filepath.Dir(parent)
	}
	return filepath.Join(root, p), nil
}
func fileStamp(p string) (string, error) {
	i, e := os.Lstat(p)
	if os.IsNotExist(e) {
		return "missing", nil
	}
	if e != nil {
		return "", e
	}
	s := fmt.Sprintf("%v:%d:%d", i.Mode(), i.Size(), i.ModTime().UnixNano())
	stat := reflect.Indirect(reflect.ValueOf(i.Sys()))
	if stat.IsValid() && stat.Kind() == reflect.Struct {
		for _, name := range []string{"Ino", "Ctim", "Ctimespec"} {
			field := stat.FieldByName(name)
			if field.IsValid() {
				s += fmt.Sprint(field.Interface())
			}
		}
	}
	if i.Mode()&os.ModeSymlink != 0 {
		l, e := os.Readlink(p)
		return s + ":" + l, e
	}
	return s, nil
}
func (g diffGit) state() (string, error) {
	r, b, h, a, e := g.baseline()
	if e != nil {
		return "", e
	}
	branch, e := g.text("symbolic-ref", "HEAD")
	if e != nil {
		return "", e
	}
	index, e := g.text("rev-parse", "--path-format=absolute", "--git-path", "index")
	if e != nil {
		return "", e
	}
	f, e := os.Open(index)
	if os.IsNotExist(e) {
		return strings.Join([]string{r, b, h, a, branch, "missing"}, ":"), nil
	}
	if e != nil {
		return "", e
	}
	defer f.Close()
	hash := sha256.New()
	n, e := io.Copy(hash, io.LimitReader(f, diffMetadataLimit+1))
	if e != nil {
		return "", e
	}
	if n > diffMetadataLimit {
		return "", errDiffBound
	}
	return fmt.Sprintf("%s:%s:%s:%s:%s:%x", r, b, h, a, branch, hash.Sum(nil)), nil
}

// InspectWorktree uses a private index/object directory for Git's net comparison.
// No source contents or Git objects are written into the inspected repository.
func InspectWorktree(ctx context.Context, directory, branch, repository string) (*WorktreeDiff, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	g := diffGit{ctx: ctx, dir: directory}
	for attempt := 0; attempt < 2; attempt++ {
		r, e := inspectWorktree(g, branch, repository)
		if ctx.Err() != nil {
			return nil, diffError("timeout", "Inspection timed out or was canceled. Refresh to retry.")
		}
		var de *DiffError
		if errors.As(e, &de) && de.Code == "checkout_changed" && ctx.Err() == nil {
			continue
		}
		if errors.Is(e, errDiffBound) {
			e = diffError("limit_exceeded", "Repository metadata exceeds the inspection limit.")
		}
		return r, e
	}
	return nil, diffError("checkout_changed", "The checkout changed during inspection. Refresh when edits settle.")
}
func inspectWorktree(g diffGit, branch, repository string) (*WorktreeDiff, error) {
	if e := g.identity(branch, repository); e != nil {
		return nil, e
	}
	before, e := g.state()
	if e != nil {
		return nil, e
	}
	ref, base, head, ancestor, e := g.baseline()
	if e != nil {
		return nil, e
	}
	unmerged, e := g.command(nil, diffMetadataLimit, "ls-files", "-u", "-z")
	if e != nil {
		return nil, e
	}
	if len(unmerged) > 0 {
		return nil, diffError("unmerged_index", "Resolve the unmerged index before inspecting changes.")
	}
	stagedRaw, se := g.command(nil, diffMetadataLimit, "ls-files", "--stage", "-z")
	if se != nil {
		return nil, se
	}
	stagedModes := map[string]string{}
	stagedOIDs := map[string]string{}
	for _, line := range bytes.Split(stagedRaw, []byte{0}) {
		if len(line) == 0 {
			continue
		}
		h, p, ok := strings.Cut(string(line), "\t")
		fields := strings.Fields(h)
		if !ok || len(fields) != 3 {
			return nil, errDiffBound
		}
		stagedModes[p] = fields[0]
		stagedOIDs[p] = fields[1]
	}
	baseline, e := g.tree(ancestor)
	if e != nil {
		return nil, e
	}
	// Inspect tracked identities and non-ignored untracked paths using NUL records.
	raw, e := g.command(nil, diffMetadataLimit, "ls-files", "--cached", "--others", "--exclude-standard", "-z")
	if e != nil {
		return nil, e
	}
	paths := map[string]bool{}
	for p := range baseline {
		paths[p] = true
	}
	for _, p := range bytes.Split(raw, []byte{0}) {
		if len(p) > 0 {
			if !utf8.Valid(p) {
				return nil, diffError("unsupported_path", "A repository path cannot be represented as UTF-8.")
			}
			paths[string(p)] = true
		}
	}
	specials, se := g.specialPaths(stagedModes)
	if se != nil {
		return nil, se
	}
	for _, p := range specials {
		paths[p] = true
	}
	ordered := make([]string, 0, len(paths))
	for p := range paths {
		ordered = append(ordered, p)
	}
	sort.Strings(ordered)
	stamps := map[string]string{}
	for _, p := range ordered {
		full, pe := safeDiffPath(g.dir, p)
		if pe != nil {
			return nil, pe
		}
		stamp, se := fileStamp(full)
		if se != nil {
			return nil, se
		}
		stamps[p] = stamp
	}
	// Git selects changed tracked paths without invoking external diff/textconv.
	changed, e := g.command(nil, diffMetadataLimit, "diff", "--name-only", "--no-renames", "--no-ext-diff", "--no-textconv", "-z", ancestor, "--")
	if e != nil {
		return nil, e
	}
	candidates := map[string]bool{}
	for _, p := range specials {
		candidates[p] = true
	}
	for _, p := range bytes.Split(changed, []byte{0}) {
		if len(p) > 0 {
			candidates[string(p)] = true
		}
	}
	untracked, e := g.command(nil, diffMetadataLimit, "ls-files", "--others", "--exclude-standard", "-z")
	if e != nil {
		return nil, e
	}
	for _, p := range bytes.Split(untracked, []byte{0}) {
		if len(p) > 0 {
			candidates[string(p)] = true
		}
	}
	temp, e := os.MkdirTemp("", "sectile-diff-")
	if e != nil {
		return nil, e
	}
	defer os.RemoveAll(temp)
	objects, e := g.text("rev-parse", "--path-format=absolute", "--git-path", "objects")
	if e != nil {
		return nil, e
	}
	if e = os.Mkdir(filepath.Join(temp, "objects"), 0700); e != nil {
		return nil, e
	}
	snapshot := g
	snapshot.env = []string{"GIT_INDEX_FILE=" + filepath.Join(temp, "index"), "GIT_OBJECT_DIRECTORY=" + filepath.Join(temp, "objects"), "GIT_ALTERNATE_OBJECT_DIRECTORIES=" + strconv.Quote(objects)}
	if _, e = snapshot.command(nil, 4096, "read-tree", ancestor); e != nil {
		return nil, e
	}
	result := &WorktreeDiff{Directory: g.dir, Branch: branch, BaseRef: ref, BaseCommit: base, MergeBase: ancestor, HeadCommit: head, Complete: true, Files: []WorktreeDiffFile{}, Warnings: []DiffWarning{}}
	omitted := map[string]WorktreeDiffFile{}
	submodules := map[string]bool{}
	var updates bytes.Buffer
	snapshotBytes := 0
	submoduleStates := map[string]string{}
	zero := strings.Repeat("0", len(head))
	for _, p := range ordered {
		if !candidates[p] {
			continue
		}
		if e := g.ctx.Err(); e != nil {
			return nil, diffError("timeout", "Inspection timed out. Refresh to retry.")
		}
		full, pe := safeDiffPath(g.dir, p)
		if pe != nil {
			return nil, pe
		}
		info, ie := os.Lstat(full)
		old := baseline[p]
		if os.IsNotExist(ie) {
			fmt.Fprintf(&updates, "0 %s\t%s%c", zero, p, 0)
			continue
		}
		if ie != nil {
			return nil, ie
		}
		mode := "100644"
		var content []byte
		if stagedModes[p] == "160000" && info.IsDir() {
			child := diffGit{ctx: g.ctx, dir: full}
			top, te := child.text("rev-parse", "--show-toplevel")
			resolved, _ := canonical(top)
			expected, _ := canonical(full)
			oid := stagedOIDs[p]
			if te == nil && resolved == expected {
				var oe error
				oid, oe = child.text("rev-parse", "--verify", "HEAD^{commit}")
				if oe != nil {
					return nil, oe
				}
				submoduleStates[p] = oid
			}
			fmt.Fprintf(&updates, "160000 %s\t%s%c", oid, p, 0)
			submodules[p] = true
			continue
		}
		if old.mode == "160000" && info.IsDir() {
			fmt.Fprintf(&updates, "0 %s\t%s%c", zero, p, 0)
			continue
		}

		reason := ""
		kind := "unsupported"
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			mode = "120000"
			link, le := os.Readlink(full)
			if le != nil {
				return nil, le
			}
			content = []byte(link)
		case info.Mode().IsRegular():
			if info.Mode()&0111 != 0 {
				mode = "100755"
			}
			if snapshotBytes+int(info.Size()) > 64<<20 {
				reason = "The 64 MiB snapshot inspection budget was reached."
				kind = "text"
				break
			}
			if info.Size() > diffMetadataLimit {
				reason = "File exceeds the 8 MiB inspection limit."
				kind = "text"
				break
			}
			f, fe := os.Open(full)
			if fe != nil {
				return nil, fe
			}
			opened, fe := f.Stat()
			if fe != nil || !os.SameFile(info, opened) {
				f.Close()
				return nil, diffError("checkout_changed", "A file changed during inspection. Refresh to retry.")
			}
			content, fe = io.ReadAll(io.LimitReader(f, diffMetadataLimit+1))
			f.Close()
			if fe != nil {
				return nil, fe
			}
			if len(content) > diffMetadataLimit {
				reason = "File exceeds the 8 MiB inspection limit."
				kind = "text"
			}
		default:
			reason = "Unsupported filesystem entry."
		}
		if reason != "" {
			status := "modified"
			if old.oid == "" {
				status = "added"
			}
			omitted[p] = WorktreeDiffFile{Path: p, Status: status, Kind: kind, OmittedReason: reason}
			continue
		}
		snapshotBytes += len(content)
		oid, oe := writeDiffObject(filepath.Join(temp, "objects"), content, len(head) == 64)
		if oe != nil {
			return nil, oe
		}
		fmt.Fprintf(&updates, "%s %s\t%s%c", mode, oid, p, 0)
	}
	if _, e = snapshot.command(updates.Bytes(), 4096, "update-index", "-z", "--index-info"); e != nil {
		return nil, e
	}
	tree, e := snapshot.text("write-tree")
	if e != nil {
		return nil, e
	}
	current, e := snapshot.tree(tree)
	if e != nil {
		return nil, e
	}
	names, e := snapshot.command(nil, diffMetadataLimit, "diff", "--name-status", "-z", "--find-renames=50%", "-l1000", "--no-ext-diff", "--no-textconv", ancestor, tree, "--")
	if e != nil {
		return nil, e
	}
	// One bounded Git process per output kind keeps large file lists responsive.
	stats, e := snapshot.command(nil, diffMetadataLimit, "diff", "--numstat", "-z", "--find-renames=50%", "-l1000", "--no-ext-diff", "--no-textconv", ancestor, tree, "--")
	if e != nil {
		return nil, e
	}
	statRecords := bytes.Split(stats, []byte{0})
	countsByPath := map[string][]string{}
	for i := 0; i < len(statRecords) && len(statRecords[i]) > 0; {
		fields := strings.SplitN(string(statRecords[i]), "\t", 3)
		i++
		if len(fields) != 3 {
			return nil, errDiffBound
		}
		p := fields[2]
		if p == "" {
			if i+1 >= len(statRecords) {
				return nil, errDiffBound
			}
			p = string(statRecords[i+1])
			i += 2
		}
		countsByPath[p] = fields[:2]
	}
	patchOutput, patchErr := snapshot.command(nil, diffResponseLimit, "diff", "--patch", "--find-renames=50%", "-l1000", "--no-ext-diff", "--no-textconv", "--no-color", ancestor, tree, "--")
	if patchErr != nil && !errors.Is(patchErr, errDiffBound) {
		return nil, patchErr
	}
	patches := bytes.Split(patchOutput, []byte("\ndiff --git "))
	if len(patchOutput) == 0 {
		patches = nil
	}
	for i := 1; i < len(patches); i++ {
		patches[i] = append([]byte("diff --git "), patches[i]...)
	}
	if patchErr != nil && len(patches) > 0 {
		patches = patches[:len(patches)-1]
	}
	patchIndex := 0
	tokens := bytes.Split(names, []byte{0})
	for i := 0; i < len(tokens) && len(tokens[i]) > 0; {
		code := string(tokens[i])
		i++
		if i >= len(tokens) {
			return nil, errDiffBound
		}
		p := string(tokens[i])
		i++
		f := WorktreeDiffFile{Path: p, Kind: "text", Status: map[byte]string{'A': "added", 'D': "deleted", 'M': "modified", 'T': "type-changed", 'R': "renamed"}[code[0]]}
		if code[0] == 'R' {
			if i >= len(tokens) {
				return nil, errDiffBound
			}
			f.OldPath = p
			f.Path = string(tokens[i])
			i++
		}
		if _, ok := omitted[f.Path]; ok {
			patchIndex++
			continue
		}
		oldPath := f.OldPath
		if oldPath == "" {
			oldPath = f.Path
		}
		if baseline[oldPath].mode == "160000" || current[f.Path].mode == "160000" {
			f.Kind = "submodule"
		} else if baseline[oldPath].mode == "120000" || current[f.Path].mode == "120000" {
			f.Kind = "symlink"
		}
		segmentCount := 1
		// Git renders a type change as deletion and addition under one raw identity.
		if f.Status == "type-changed" {
			segmentCount = 2
		}
		var patch []byte
		if patchIndex+segmentCount <= len(patches) {
			patch = bytes.Join(patches[patchIndex:patchIndex+segmentCount], []byte("\n"))
		}
		counts, ok := countsByPath[f.Path]
		if !ok {
			return nil, errDiffBound
		}
		if f.Kind == "submodule" {
			f.OmittedReason = "Submodule commit or dirty state changed."
		} else if counts[0] == "-" {
			f.Kind = "binary"
			f.OmittedReason = "Binary contents are not displayed."
		} else {
			a, ae := strconv.Atoi(counts[0])
			d, de := strconv.Atoi(counts[1])
			if ae != nil || de != nil {
				return nil, errDiffBound
			}
			f.Additions = &a
			f.Deletions = &d
			if patchIndex+segmentCount > len(patches) {
				f.OmittedReason = "Patch omitted because the aggregate display limit was reached."
			} else if len(patch) > diffPatchLimit {
				f.OmittedReason = "Patch exceeds the 256 KiB display limit."
			} else if !utf8.Valid(patch) {
				f.Kind = "binary"
				f.OmittedReason = "Non-UTF-8 contents are not displayed."
			} else {
				f.Patch = string(patch)
			}
			if f.OmittedReason != "" {
				f.Additions = nil
				f.Deletions = nil
			}
		}
		patchIndex += segmentCount
		result.Files = append(result.Files, f)
		delete(submodules, f.Path)
	}
	for p := range submodules {
		result.Files = append(result.Files, WorktreeDiffFile{Path: p, Status: "modified", Kind: "submodule", OmittedReason: "Submodule dirty state changed."})
	}
	for _, f := range omitted {
		result.Files = append(result.Files, f)
	}
	sort.Slice(result.Files, func(i, j int) bool { return result.Files[i].Path < result.Files[j].Path })
	if len(result.Files) > diffFileLimit {
		result.Files = result.Files[:diffFileLimit]
		result.Complete = false
		result.Warnings = append(result.Warnings, DiffWarning{"file_limit", "Only the first 1,000 changed entries are shown; rename detection is bounded to 1,000 candidates."})
	}
	// Include JSON escaping and metadata in the aggregate byte budget.
	size := 0
	for i, f := range result.Files {
		encoded, _ := json.Marshal(f)
		if size+len(encoded) > diffResponseLimit-16384 {
			result.Files = result.Files[:i]
			result.Complete = false
			result.Warnings = append(result.Warnings, DiffWarning{"response_limit", "The result exceeds the 4 MiB response limit; only a prefix is shown."})
			break
		}
		size += len(encoded)
		if f.Additions == nil {
			result.CountsPartial = true
			if f.Kind != "binary" && f.Kind != "submodule" {
				result.Complete = false
			}
		} else {
			result.Additions += *f.Additions
			result.Deletions += *f.Deletions
		}
	}
	result.CountsPartial = result.CountsPartial || !result.Complete
	result.FilesChanged = len(result.Files)
	result.IsClean = result.Complete && len(result.Files) == 0
	after, e := g.state()
	if e != nil {
		return nil, e
	}
	if before != after {
		return nil, diffError("checkout_changed", "Git inputs changed during inspection. Refresh to retry.")
	}
	changedAfter, ce := g.command(nil, diffMetadataLimit, "diff", "--name-only", "--no-renames", "--no-ext-diff", "--no-textconv", "-z", ancestor, "--")
	if ce != nil {
		return nil, ce
	}
	if !bytes.Equal(changed, changedAfter) {
		return nil, diffError("checkout_changed", "Changed paths changed during inspection. Refresh to retry.")
	}
	again, e := g.command(nil, diffMetadataLimit, "ls-files", "--cached", "--others", "--exclude-standard", "-z")
	if e != nil {
		return nil, e
	}
	if !bytes.Equal(raw, again) {
		return nil, diffError("checkout_changed", "The file list changed during inspection. Refresh to retry.")
	}
	specialAfter, se := g.specialPaths(stagedModes)
	if se != nil {
		return nil, se
	}
	if strings.Join(specials, "\x00") != strings.Join(specialAfter, "\x00") {
		return nil, diffError("checkout_changed", "Special entries changed during inspection. Refresh to retry.")
	}
	for p, stamp := range stamps {
		full, pe := safeDiffPath(g.dir, p)
		if pe != nil {
			return nil, pe
		}
		now, se := fileStamp(full)
		if se != nil {
			return nil, se
		}
		if now != stamp {
			return nil, diffError("checkout_changed", "Files changed during inspection. Refresh to retry.")
		}
	}
	for p, oid := range submoduleStates {
		now, ne := (diffGit{ctx: g.ctx, dir: filepath.Join(g.dir, p)}).text("rev-parse", "--verify", "HEAD^{commit}")
		if ne != nil || now != oid {
			return nil, diffError("checkout_changed", "A submodule changed during inspection. Refresh to retry.")
		}
	}
	if e = g.identity(branch, repository); e != nil {
		return nil, e
	}
	result.GeneratedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return result, nil
}

// Git loose objects use a typed header and zlib stream. These objects live only
// in the request's temporary directory; no filters or repository hooks run.
func writeDiffObject(objects string, content []byte, sha256Repo bool) (string, error) {
	var digest hash.Hash = sha1.New()
	if sha256Repo {
		digest = sha256.New()
	}
	header := []byte(fmt.Sprintf("blob %d\x00", len(content)))
	digest.Write(header)
	digest.Write(content)
	oid := fmt.Sprintf("%x", digest.Sum(nil))
	dir := filepath.Join(objects, oid[:2])
	if e := os.MkdirAll(dir, 0700); e != nil {
		return "", e
	}
	f, e := os.OpenFile(filepath.Join(dir, oid[2:]), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if os.IsExist(e) {
		return oid, nil
	}
	if e != nil {
		return "", e
	}
	z := zlib.NewWriter(f)
	_, e = z.Write(header)
	if e == nil {
		_, e = z.Write(content)
	}
	ze := z.Close()
	fe := f.Close()
	if e != nil {
		return "", e
	}
	if ze != nil {
		return "", ze
	}
	return oid, fe
}

// Git omits special untracked entries (such as FIFOs) from ls-files. Enumerate
// their names without opening them, pruning ignored directories and submodules.
func (g diffGit) specialPaths(modes map[string]string) ([]string, error) {
	raw, e := g.command(nil, diffMetadataLimit, "ls-files", "--others", "--ignored", "--exclude-standard", "--directory", "-z")
	if e != nil {
		return nil, e
	}
	ignored := map[string]bool{}
	for _, p := range bytes.Split(raw, []byte{0}) {
		ignored[strings.TrimSuffix(string(p), "/")] = true
	}
	paths := []string{}
	visited := 0
	e = filepath.WalkDir(g.dir, func(full string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if g.ctx.Err() != nil {
			return g.ctx.Err()
		}
		if full == g.dir {
			return nil
		}
		p, err := filepath.Rel(g.dir, full)
		if err != nil {
			return err
		}
		visited++
		if visited > 200000 {
			return errDiffBound
		}
		if p == ".git" || ignored[p] || modes[p] == "160000" {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.IsDir() && !entry.Type().IsRegular() && entry.Type()&os.ModeSymlink == 0 {
			paths = append(paths, p)
		}
		return nil
	})
	if e != nil {
		return nil, e
	}
	if len(paths) == 0 {
		return paths, nil
	}
	check, e := g.command([]byte(strings.Join(paths, "\x00")+"\x00"), diffMetadataLimit, "check-ignore", "--no-index", "-z", "--stdin")
	if e != nil {
		return nil, e
	}
	for _, p := range bytes.Split(check, []byte{0}) {
		ignored[string(p)] = true
	}
	result := []string{}
	for _, p := range paths {
		if !ignored[p] {
			result = append(result, p)
		}
	}
	sort.Strings(result)
	return result, nil
}
