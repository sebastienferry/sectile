package trackerapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"tasks/internal/models"
	"time"
	"unicode"
)

// A Jira site names its priorities as it pleases. "Highest", "High", "Medium",
// "Low" is Atlassian's default scheme, not a guarantee: a site running the
// Blocker/Major/Minor scheme inherited from Jira Server, a site whose scheme
// was renamed to P1…P4, and a site served in French all answer
// "priority: The priority selected is invalid" to a write that assumes those
// four names — which is what this adapter sent on every creation and every
// update. So the scheme is read from the site, a write sends the option's id,
// and a read maps the site's own name back.

// jiraPriorityOption is one option of a site's priority scheme.
type jiraPriorityOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// jiraPriorityScheme is what a site offers, in Jira's own order: most urgent
// first. That order is what the rank fallback below counts on, and it is the
// order both priority endpoints serve.
type jiraPriorityScheme []jiraPriorityOption

// jiraPriorityTTL is how long a discovered scheme stands. Same reasoning as
// the custom field ids: a scheme does not change within a site, but an
// administrator changing one must not need a server restart to be seen.
const jiraPriorityTTL = 10 * time.Minute

type jiraPriorityEntry struct {
	scheme jiraPriorityScheme
	expiry time.Time
}

var jiraPriorityCache sync.Map // base URL -> jiraPriorityEntry

// resetJiraPriorityCache forgets every discovered scheme, for tests that
// rebuild a site under the same URL.
func resetJiraPriorityCache() { jiraPriorityCache = sync.Map{} }

// jiraPriorities is the site's priority scheme, discovered once per site and
// remembered for a while.
//
// It answers nil rather than an error: a site that will not say what its
// priorities are is no reason to refuse a write. The caller then sends the
// default name, which is what the adapter always did, and the site decides.
func (c *Client) jiraPriorities(ctx context.Context) jiraPriorityScheme {
	if cached, ok := jiraPriorityCache.Load(c.JiraURL); ok {
		if entry, ok := cached.(jiraPriorityEntry); ok && time.Now().Before(entry.expiry) {
			return entry.scheme
		}
	}
	scheme, err := c.readJiraPriorities(ctx)
	if err != nil {
		log.Printf("[jira] the site did not say which priorities it has, falling back on the default names: %v", err)
		return nil
	}
	jiraPriorityCache.Store(c.JiraURL, jiraPriorityEntry{scheme: scheme, expiry: time.Now().Add(jiraPriorityTTL)})
	return scheme
}

// readJiraPriorities asks the two endpoints a site may serve: the paginated
// search Cloud offers, then the bare list, which is a plain array and is what
// an older site answers. A site serving neither is reported as an error, never
// as a site with no priorities: an empty scheme and an unreachable one lead to
// different writes.
func (c *Client) readJiraPriorities(ctx context.Context) (jiraPriorityScheme, error) {
	pages, searchErr := c.jiraAgilePages(ctx, "/rest/api/3/priority/search", nil)
	if searchErr == nil && len(pages) > 0 {
		return decodeJiraPriorities(pages), nil
	}
	var list []json.RawMessage
	if err := c.jira(ctx, http.MethodGet, "/rest/api/3/priority", nil, nil, &list); err != nil {
		if searchErr != nil {
			return nil, searchErr
		}
		return nil, err
	}
	return decodeJiraPriorities(list), nil
}

func decodeJiraPriorities(items []json.RawMessage) jiraPriorityScheme {
	scheme := make(jiraPriorityScheme, 0, len(items))
	for _, raw := range items {
		var option jiraPriorityOption
		if json.Unmarshal(raw, &option) != nil {
			continue
		}
		option.ID, option.Name = strings.TrimSpace(option.ID), strings.TrimSpace(option.Name)
		if option.ID == "" || option.Name == "" {
			continue
		}
		scheme = append(scheme, option)
	}
	return scheme
}

// jiraPriorityValue is what a write puts in the "priority" field: the id of an
// option the site actually has, and the default name only when the scheme
// could not be read.
func (c *Client) jiraPriorityValue(ctx context.Context, p models.Priority) map[string]string {
	if option, ok := c.jiraPriorities(ctx).option(p); ok {
		return map[string]string{"id": option.ID}
	}
	return map[string]string{"name": jiraPriorityName(p)}
}

// option picks the site's option for one Sectile priority: the first option
// whose name means that priority, else the option sitting at that priority's
// rank in the scheme.
//
// Name first, because a scheme this adapter can read names better than
// arithmetic can: Jira's default has five options for four Sectile levels, and
// "low" belongs on "Low" rather than on "Lowest". Rank second, because a
// scheme named P1…P4 or S1…S3 means nothing to the table and everything to its
// own order.
func (s jiraPriorityScheme) option(p models.Priority) (jiraPriorityOption, bool) {
	if len(s) == 0 || p == "" {
		return jiraPriorityOption{}, false
	}
	for _, option := range s {
		if named, ok := jiraPriorityOf(option.Name); ok && named == p {
			return option, true
		}
	}
	return s[jiraPriorityRank(p, len(s))], true
}

// priorityAt reads a rank back into a Sectile priority: the one whose own rank
// in a scheme of this size is nearest. Being the exact inverse of
// jiraPriorityRank is what keeps a write and the read that follows it
// agreeing on an unnamed scheme.
func (s jiraPriorityScheme) priorityAt(index int) models.Priority {
	nearest, distance := models.PriorityMedium, -1
	for _, p := range []models.Priority{models.PriorityUrgent, models.PriorityHigh, models.PriorityMedium, models.PriorityLow} {
		d := index - jiraPriorityRank(p, len(s))
		if d < 0 {
			d = -d
		}
		if distance < 0 || d < distance {
			nearest, distance = p, d
		}
	}
	return nearest
}

// indexOf finds an option by name. It answers -1 for a name the scheme does
// not carry, which is what a work item priority deleted from the scheme since
// looks like.
func (s jiraPriorityScheme) indexOf(name string) int {
	folded := foldPriorityName(name)
	if folded == "" {
		return -1
	}
	for i, option := range s {
		if foldPriorityName(option.Name) == folded {
			return i
		}
	}
	return -1
}

// jiraPriorityRank places a Sectile priority in a scheme of n options ordered
// most urgent first: urgent at the top, low at the bottom, high and medium
// spread in between. It is the answer for a scheme whose names mean nothing to
// this adapter — P1…P5, a translation it has never met — and it never answers
// an index the scheme does not have.
func jiraPriorityRank(p models.Priority, n int) int {
	if n <= 1 {
		return 0
	}
	share := 0.5
	switch p {
	case models.PriorityUrgent:
		share = 0
	case models.PriorityHigh:
		share = 0.25
	case models.PriorityLow:
		share = 1
	}
	index := int(math.Round(share * float64(n-1)))
	if index < 0 {
		index = 0
	}
	if index > n-1 {
		index = n - 1
	}
	return index
}

// jiraPriorityAliases is what the priority names met so far mean: Atlassian's
// default scheme, the Blocker/Critical/Major/Minor/Trivial one Jira Server
// shipped, and the French translations of both. Accents are folded before the
// lookup, so one spelling answers for every way a site writes it.
//
// The Server scheme has five levels for Sectile's four, and it is read the way
// the sites running it use it: "Major" is the level most of their work items
// sit at, so it is the middle one, and "Critical" the one above it rather than
// a second name for "Blocker". Reading Critical as urgent and Major as high
// instead put every ordinary work item on the board's high level and left
// Sectile's medium with nowhere to write.
//
// Numbered schemes (P1…P4, S1…S3) are deliberately absent: what "P4" means
// depends on how many options the scheme has, which is precisely what
// jiraPriorityRank reads from the scheme itself.
var jiraPriorityAliases = map[string]models.Priority{
	// Urgent.
	"highest":        models.PriorityUrgent,
	"blocker":        models.PriorityUrgent,
	"blocking":       models.PriorityUrgent,
	"immediate":      models.PriorityUrgent,
	"urgent":         models.PriorityUrgent,
	"urgente":        models.PriorityUrgent,
	"showstopper":    models.PriorityUrgent,
	"bloquant":       models.PriorityUrgent,
	"bloquante":      models.PriorityUrgent,
	"la plus elevee": models.PriorityUrgent,
	"la plus haute":  models.PriorityUrgent,
	"tres elevee":    models.PriorityUrgent,
	"tres haute":     models.PriorityUrgent,
	// High. Critical sits under Blocker in the Server scheme, which is one
	// level down, not the same one.
	"critical":   models.PriorityHigh,
	"critique":   models.PriorityHigh,
	"high":       models.PriorityHigh,
	"important":  models.PriorityHigh,
	"importante": models.PriorityHigh,
	"elevee":     models.PriorityHigh,
	"eleve":      models.PriorityHigh,
	"haute":      models.PriorityHigh,
	// Medium. Major is the Server scheme's ordinary level, not an elevated one.
	"major":    models.PriorityMedium,
	"majeure":  models.PriorityMedium,
	"medium":   models.PriorityMedium,
	"moderate": models.PriorityMedium,
	"normal":   models.PriorityMedium,
	"normale":  models.PriorityMedium,
	"standard": models.PriorityMedium,
	"moyenne":  models.PriorityMedium,
	"moyen":    models.PriorityMedium,
	"moderee":  models.PriorityMedium,
	// Low. Sectile has no fifth level, so a scheme's bottom two both land here.
	"low":            models.PriorityLow,
	"lowest":         models.PriorityLow,
	"minor":          models.PriorityLow,
	"trivial":        models.PriorityLow,
	"negligible":     models.PriorityLow,
	"basse":          models.PriorityLow,
	"bas":            models.PriorityLow,
	"faible":         models.PriorityLow,
	"mineure":        models.PriorityLow,
	"triviale":       models.PriorityLow,
	"negligeable":    models.PriorityLow,
	"la plus basse":  models.PriorityLow,
	"la plus faible": models.PriorityLow,
	"tres basse":     models.PriorityLow,
	"tres faible":    models.PriorityLow,
}

// jiraPriorityOf reads a site's priority name. It answers false for a name the
// table does not carry, which is what sends the caller to the scheme's order.
func jiraPriorityOf(name string) (models.Priority, bool) {
	folded := foldPriorityName(name)
	if folded == "" {
		return "", false
	}
	if p, ok := jiraPriorityAliases[folded]; ok {
		return p, true
	}
	// "P1 - Critical", "Haute (High)": a scheme often spells one level twice,
	// and reading the words one by one recognises the pair when the whole name
	// does not. The full name is tried first, so "la plus elevee" stays urgent
	// rather than becoming the "elevee" it contains.
	for _, word := range strings.FieldsFunc(folded, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if p, ok := jiraPriorityAliases[word]; ok {
			return p, true
		}
	}
	return "", false
}

// priorityFolds are the accented letters a translated scheme carries. Folding
// them by hand keeps one table entry per name and adds no dependency for the
// twenty-odd runes involved.
var priorityFolds = map[rune]rune{
	'à': 'a', 'á': 'a', 'â': 'a', 'ã': 'a', 'ä': 'a', 'å': 'a',
	'ç': 'c', 'è': 'e', 'é': 'e', 'ê': 'e', 'ë': 'e',
	'ì': 'i', 'í': 'i', 'î': 'i', 'ï': 'i', 'ñ': 'n',
	'ò': 'o', 'ó': 'o', 'ô': 'o', 'õ': 'o', 'ö': 'o',
	'ù': 'u', 'ú': 'u', 'û': 'u', 'ü': 'u', 'ý': 'y', 'ÿ': 'y',
}

// foldPriorityName reduces a priority name to what the table is keyed on:
// lower case, no accents, single spaces.
func foldPriorityName(name string) string {
	var b strings.Builder
	space := false
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		if folded, ok := priorityFolds[r]; ok {
			r = folded
		}
		if unicode.IsSpace(r) {
			space = b.Len() > 0
			continue
		}
		if space {
			b.WriteRune(' ')
			space = false
		}
		b.WriteRune(r)
	}
	return b.String()
}

// The site's list is not what a write is judged against. A project has a
// priority *scheme*, a subset of the site's priorities, and a work item type
// has a screen that may not carry the field at all. Both are why a name the
// site knows is still answered with "The priority selected is invalid": the
// option exists, just not for this project. So a write asks the screen that
// will receive it — createmeta for a creation, editmeta for an update — and
// keeps the site's list for when neither can be read.

type jiraScreenPriorities struct {
	scheme  jiraPriorityScheme
	offered bool // whether the field is on that screen at all
}

var jiraCreatePriorityCache sync.Map // site|project|type -> jiraCreatePriorityEntry

type jiraCreatePriorityEntry struct {
	screen jiraScreenPriorities
	expiry time.Time
}

// resetJiraCreatePriorityCache forgets every discovered create screen, for
// tests that rebuild a site under the same URL.
func resetJiraCreatePriorityCache() { jiraCreatePriorityCache = sync.Map{} }

// priorityFieldFor answers what to put in the "priority" field of a write, and
// false when the write must not carry the field at all.
//
// The screen decides. A project whose creation screen has no priority — which
// is the shape of every issue type on some projects — answers false, and the
// creation goes out without it rather than being refused outright over a field
// the site was never going to accept. A screen that does carry the field names
// the options a write may use, and the site's own list is the last resort.
func (c *Client) priorityFieldFor(ctx context.Context, screen jiraScreenPriorities, readable bool, p models.Priority, what string) (map[string]string, bool) {
	if readable && !screen.offered {
		log.Printf("[jira] %s does not offer the priority field; %q left to the site", what, p)
		return nil, false
	}
	if option, ok := screen.scheme.option(p); ok {
		return map[string]string{"id": option.ID}, true
	}
	// Either the screen could not be read, or it carries the field without
	// enumerating its options. The site's list is a better guess than the
	// default names, and the default names are better than nothing.
	return c.jiraPriorityValue(ctx, p), true
}

// jiraCreatePriorities reads the priority options of one project's creation
// screen for one work item type, and remembers them for a while.
func (c *Client) jiraCreatePriorities(ctx context.Context, projectKey, issueType string) (jiraScreenPriorities, bool) {
	cacheKey := c.JiraURL + "|" + projectKey + "|" + strings.ToLower(issueType)
	if cached, ok := jiraCreatePriorityCache.Load(cacheKey); ok {
		if entry, ok := cached.(jiraCreatePriorityEntry); ok && time.Now().Before(entry.expiry) {
			return entry.screen, true
		}
	}
	screen, err := c.readJiraCreatePriorities(ctx, projectKey, issueType)
	if err != nil {
		log.Printf("[jira] the creation screen of %s/%s did not say which priorities it takes: %v", projectKey, issueType, err)
		return jiraScreenPriorities{}, false
	}
	jiraCreatePriorityCache.Store(cacheKey, jiraCreatePriorityEntry{screen: screen, expiry: time.Now().Add(jiraPriorityTTL)})
	return screen, true
}

func (c *Client) readJiraCreatePriorities(ctx context.Context, projectKey, issueType string) (jiraScreenPriorities, error) {
	types, err := c.jiraIssueTypes(ctx, projectKey)
	if err != nil {
		return jiraScreenPriorities{}, err
	}
	typeID := ""
	for _, it := range types {
		if strings.EqualFold(it.Name, issueType) {
			typeID = it.ID
			break
		}
	}
	if typeID == "" {
		return jiraScreenPriorities{}, fmt.Errorf("issue type %q does not exist on project %s", issueType, projectKey)
	}
	items, err := c.jiraCreateMetaPages(ctx, "/rest/api/3/issue/createmeta/"+url.PathEscape(projectKey)+"/issuetypes/"+url.PathEscape(typeID), "fields")
	if err != nil {
		return jiraScreenPriorities{}, err
	}
	for _, raw := range items {
		var field struct {
			FieldID       string            `json:"fieldId"`
			AllowedValues []json.RawMessage `json:"allowedValues"`
		}
		if json.Unmarshal(raw, &field) != nil || field.FieldID != "priority" {
			continue
		}
		return jiraScreenPriorities{scheme: decodeJiraPriorities(field.AllowedValues), offered: true}, nil
	}
	return jiraScreenPriorities{}, nil
}

// jiraEditPriorities reads the priority options of one work item's own edit
// screen. It is not cached: the answer belongs to a work item's project and
// type, which a key alone does not name, and a priority change is a thing
// somebody asks for rather than something a synchronisation repeats.
func (c *Client) jiraEditPriorities(ctx context.Context, key string) (jiraScreenPriorities, bool) {
	var meta struct {
		Fields map[string]struct {
			AllowedValues []json.RawMessage `json:"allowedValues"`
		} `json:"fields"`
	}
	if err := c.jira(ctx, http.MethodGet, "/rest/api/3/issue/"+url.PathEscape(key)+"/editmeta", nil, nil, &meta); err != nil {
		log.Printf("[jira] the edit screen of %s did not say which priorities it takes: %v", key, err)
		return jiraScreenPriorities{}, false
	}
	field, ok := meta.Fields["priority"]
	if !ok {
		return jiraScreenPriorities{}, true
	}
	return jiraScreenPriorities{scheme: decodeJiraPriorities(field.AllowedValues), offered: true}, true
}
