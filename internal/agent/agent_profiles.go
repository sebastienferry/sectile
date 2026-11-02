package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/google/uuid"
)

const agentProfileLimit = 64 * 1024

var agentProfileID = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
var reservedAgentProfileID = regexp.MustCompile(`^(con|prn|aux|nul|com[1-9]|lpt[1-9])$`)
var agentProfilesMu sync.Mutex

// invalidAgentProfile marks a profile whose content or file type cannot be
// used, as opposed to a repository that cannot be reached.
type invalidAgentProfile string

func (e invalidAgentProfile) Error() string { return string(e) }

type agentProfile struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type agentProfileWarning struct {
	ID      string `json:"id"`
	Message string `json:"message"`
}
type agentProfileList struct {
	Profiles []agentProfile        `json:"profiles"`
	Warnings []agentProfileWarning `json:"warnings"`
}

// profileSnapshot stays local to the controlled run, never in server reports.
type profileSnapshot struct {
	ID                    string
	Prompt                string
	DeveloperInstructions string
}

func validAgentProfileID(id string) bool {
	return len(id) <= 64 && agentProfileID.MatchString(id) && !reservedAgentProfileID.MatchString(id)
}
func validateAgentProfile(prompt string) error {
	if len(prompt) > agentProfileLimit || !utf8.ValidString(prompt) || strings.ContainsRune(prompt, 0) || strings.TrimSpace(prompt) == "" {
		return invalidAgentProfile("Agent instructions must be nonempty UTF-8 text without NUL, at most 64 KiB")
	}
	return nil
}

// checkedProfileRoot pins each directory handle and checks its identity after
// opening. A concurrent symlink substitution is rejected, even inside the root.
func checkedProfileRoot(parent *os.Root, name string) (*os.Root, error) {
	before, err := parent.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("Agent profile directories must not be symbolic links")
	}
	child, err := parent.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	file, err := child.Open(".")
	if err != nil {
		child.Close()
		return nil, err
	}
	after, err := file.Stat()
	file.Close()
	if err != nil || !os.SameFile(before, after) {
		child.Close()
		return nil, fmt.Errorf("Agent profile directory changed while opening")
	}
	return child, nil
}

func openAgentProfiles(root string, create bool) (*os.Root, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	// The mapped checkout path is the user's choice: links above or at it
	// (/tmp to /private/tmp on macOS, a linked home) are resolved once here.
	// Only the walk from the resolved path down to the profiles is pinned.
	if absolute, err = filepath.EvalSymlinks(absolute); err != nil {
		return nil, err
	}
	volume := filepath.VolumeName(absolute)
	dir, err := os.OpenRoot(volume + string(filepath.Separator))
	if err != nil {
		return nil, err
	}
	for _, part := range strings.Split(strings.TrimPrefix(absolute, volume+string(filepath.Separator)), string(filepath.Separator)) {
		if part == "" {
			continue
		}
		next, err := checkedProfileRoot(dir, part)
		dir.Close()
		if err != nil {
			return nil, err
		}
		dir = next
	}
	for _, part := range []string{".sectile", "agents"} {
		if create {
			if err := dir.Mkdir(part, 0755); err != nil && !errors.Is(err, fs.ErrExist) {
				dir.Close()
				return nil, err
			}
		}
		next, err := checkedProfileRoot(dir, part)
		dir.Close()
		if err != nil {
			return nil, err
		}
		dir = next
	}
	return dir, nil
}

func readAgentProfileFile(dir *os.Root, id string) (string, error) {
	if !validAgentProfileID(id) {
		return "", fmt.Errorf("Invalid agent profile name")
	}
	name := id + ".md"
	before, err := dir.Lstat(name)
	if err != nil {
		return "", err
	}
	if !before.Mode().IsRegular() {
		return "", invalidAgentProfile("Agent profile must be a regular file")
	}
	file, err := dir.OpenFile(name, os.O_RDONLY|agentProfileOpenFlags, 0)
	if err != nil {
		return "", err
	}
	defer file.Close()
	after, err := file.Stat()
	if err != nil || !os.SameFile(before, after) {
		return "", fmt.Errorf("Agent profile changed while opening")
	}
	raw, err := io.ReadAll(io.LimitReader(file, agentProfileLimit+1))
	if err != nil {
		return "", err
	}
	if err := validateAgentProfile(string(raw)); err != nil {
		return "", err
	}
	return string(raw), nil
}

func listAgentProfiles(root string) (agentProfileList, error) {
	agentProfilesMu.Lock()
	defer agentProfilesMu.Unlock()
	result := agentProfileList{Profiles: []agentProfile{}, Warnings: []agentProfileWarning{}}
	dir, err := openAgentProfiles(root, false)
	if errors.Is(err, fs.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	defer dir.Close()
	file, err := dir.Open(".")
	if err != nil {
		return result, err
	}
	defer file.Close()
	entries, err := file.ReadDir(-1)
	if err != nil {
		return result, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".md")
		if _, err := readAgentProfileFile(dir, id); err != nil {
			result.Warnings = append(result.Warnings, agentProfileWarning{ID: id, Message: "Invalid or unreadable agent profile"})
			continue
		}
		result.Profiles = append(result.Profiles, agentProfile{ID: id, Name: id})
	}
	return result, nil
}

func createAgentProfile(root, id, prompt string) error {
	if !validAgentProfileID(id) {
		return fmt.Errorf("Use 1–64 lowercase letters or digits, separated by single hyphens; avoid Windows device names such as con or nul")
	}
	prompt = strings.ReplaceAll(prompt, "\r\n", "\n")
	if err := validateAgentProfile(prompt); err != nil {
		return err
	}
	agentProfilesMu.Lock()
	defer agentProfilesMu.Unlock()
	dir, err := openAgentProfiles(root, true)
	if err != nil {
		return err
	}
	defer dir.Close()
	temporary := ".profile-" + uuid.NewString()
	file, err := dir.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	defer dir.Remove(temporary)
	_, err = file.WriteString(prompt)
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	// Hard-link publication is atomic and never replaces an existing name.
	return dir.Link(temporary, id+".md")
}

func loadAgentProfile(root, id string) (*profileSnapshot, error) {
	if id == "" {
		return nil, nil
	}
	if !validAgentProfileID(id) {
		return nil, fmt.Errorf("Invalid agent profile name")
	}
	agentProfilesMu.Lock()
	defer agentProfilesMu.Unlock()
	dir, err := openAgentProfiles(root, false)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	prompt, err := readAgentProfileFile(dir, id)
	if err != nil {
		return nil, err
	}
	return &profileSnapshot{ID: id, Prompt: prompt}, nil
}

func profileHTTPError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	message := "Agent profile could not be accessed or saved; check the repository path and permissions"
	if errors.Is(err, fs.ErrExist) {
		status = http.StatusConflict
		message = "An agent profile with this name already exists"
	}
	if errors.Is(err, fs.ErrNotExist) {
		status = http.StatusNotFound
		message = "The selected agent profile is no longer available; refresh the profile list"
	}
	if errors.As(err, new(invalidAgentProfile)) {
		status = http.StatusUnprocessableEntity
		message = "The selected agent profile is invalid or unreadable; fix its file in the repository or choose another profile"
	}
	// Paths and prompt text are not exposed in filesystem errors.
	http.Error(w, message, status)
}

func (d *agentDaemon) desktopAgentProfiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	var input struct {
		ProjectID string `json:"projectId"`
		ID        string `json:"id"`
		Prompt    string `json:"prompt"`
	}
	if r.Method == http.MethodGet {
		input.ProjectID = r.URL.Query().Get("projectId")
	} else {
		raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 80*1024))
		if err != nil || !utf8.Valid(raw) {
			http.Error(w, "Invalid profile request", 400)
			return
		}
		decoder := json.NewDecoder(strings.NewReader(string(raw)))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&input) != nil || decoder.Decode(new(any)) != io.EOF {
			http.Error(w, "Invalid profile request", 400)
			return
		}
		if !validAgentProfileID(input.ID) {
			http.Error(w, "Use 1–64 lowercase letters or digits, separated by single hyphens; avoid Windows device names such as con or nul", 400)
			return
		}
		if err := validateAgentProfile(strings.ReplaceAll(input.Prompt, "\r\n", "\n")); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
	}
	if strings.TrimSpace(input.ProjectID) == "" {
		http.Error(w, "Project required", 400)
		return
	}
	config, err := d.fetchConfig(r.Context(), input.ProjectID, "")
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	d.prepareMu.Lock()
	defer d.prepareMu.Unlock()
	root, _, err := d.localProjectRoot(r.Context(), config)
	if err != nil {
		http.Error(w, "Local project checkout unavailable", 409)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodGet {
		list, err := listAgentProfiles(root)
		if err != nil {
			profileHTTPError(w, err)
			return
		}
		_ = json.NewEncoder(w).Encode(list)
		return
	}
	if err := createAgentProfile(root, input.ID, input.Prompt); err != nil {
		profileHTTPError(w, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]agentProfile{"profile": {ID: input.ID, Name: input.ID}})
}
