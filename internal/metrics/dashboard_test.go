package metrics

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
)

// dashboardPath is the Grafana dashboard model the repository ships for these
// metrics. See deploy/grafana/ and specs/467-sectile-grafana-dashboard.
const dashboardPath = "../../deploy/grafana/sectile.json"

// dashboardQuery is one PromQL expression of the dashboard and where it sits.
type dashboardQuery struct {
	panel string
	expr  string
}

// datasourceRef is one datasource reference of the dashboard and where it sits.
type datasourceRef struct {
	panel string
	uid   any
}

type dashboardModel struct {
	queries     []dashboardQuery
	datasources []datasourceRef
}

func loadDashboard(t *testing.T) dashboardModel {
	t.Helper()
	raw, err := os.ReadFile(dashboardPath)
	if err != nil {
		t.Fatalf("reading the dashboard model: %v", err)
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		t.Fatalf("the dashboard model is not valid JSON: %v", err)
	}
	var model dashboardModel
	collectDashboard(root, "dashboard", &model)
	if len(model.queries) == 0 {
		t.Fatal("the dashboard model has no query")
	}
	return model
}

// collectDashboard walks the whole model, rows and variables included, and
// keeps every query and every datasource reference. panel is the title of the
// closest enclosing panel or variable, for the failure messages.
func collectDashboard(node any, panel string, model *dashboardModel) {
	switch value := node.(type) {
	case map[string]any:
		if title, ok := value["title"].(string); ok && title != "" {
			panel = title
		} else if name, ok := value["name"].(string); ok && name != "" {
			panel = "variable " + name
		}
		for key, child := range value {
			switch key {
			case "expr":
				if expr, ok := child.(string); ok {
					model.queries = append(model.queries, dashboardQuery{panel: panel, expr: expr})
				}
			case "query":
				// A query variable carries its PromQL here, as a string; the
				// datasource variable's "query" is a plugin type, not PromQL.
				if expr, ok := child.(string); ok && value["type"] == "query" {
					model.queries = append(model.queries, dashboardQuery{panel: panel, expr: expr})
				}
			case "datasource":
				if ref, ok := child.(map[string]any); ok {
					model.datasources = append(model.datasources, datasourceRef{panel: panel, uid: ref["uid"]})
				}
			}
			collectDashboard(child, panel, model)
		}
	case []any:
		for _, child := range value {
			collectDashboard(child, panel, model)
		}
	}
}

// registeredFamilies is what the server actually exposes, with every series
// present: a vector only appears once it has a child, so one failing request
// goes through the instrumentation first.
func registeredFamilies(t *testing.T) map[string]dto.MetricType {
	t.Helper()
	m := New(fakeBoard{users: 1, runs: map[string]int{"running": 1}}, time.Minute, Build{Version: "dev"})
	failing := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
	m.Instrument(failing, func(*http.Request) string { return "/api/tasks/" }).
		ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/tasks/1", nil))
	families, err := m.registry.Gather()
	if err != nil {
		t.Fatalf("gathering the registry: %v", err)
	}
	names := make(map[string]dto.MetricType, len(families))
	for _, family := range families {
		names[family.GetName()] = family.GetType()
	}
	return names
}

var sectileMetricName = regexp.MustCompile(`\bsectile_[a-z0-9_]+\b`)

// A renamed series would silently empty the panels that chart it.
func TestTheDashboardOnlyChartsRegisteredSectileMetrics(t *testing.T) {
	registered := registeredFamilies(t)
	for _, query := range loadDashboard(t).queries {
		for _, name := range sectileMetricName.FindAllString(query.expr, -1) {
			if _, ok := registered[name]; ok {
				continue
			}
			base := name
			for _, suffix := range []string{"_bucket", "_sum", "_count"} {
				if trimmed, ok := strings.CutSuffix(name, suffix); ok {
					base = trimmed
					break
				}
			}
			if registered[base] == dto.MetricType_HISTOGRAM {
				continue
			}
			t.Errorf("panel %q queries %s, which the server does not register", query.panel, name)
		}
	}
}

// Every replica reports the same board figures (ADR 0027): a sum would count
// each active user once per replica.
func TestTheDashboardNeverSumsTheBoardSeries(t *testing.T) {
	for _, query := range loadDashboard(t).queries {
		if !strings.Contains(query.expr, "sectile_active_") {
			continue
		}
		if strings.Contains(query.expr, "sum(") || strings.Contains(query.expr, "sum by") {
			t.Errorf("panel %q adds up a board series across replicas; take max() instead: %s", query.panel, query.expr)
		}
	}
}

// The repository is public and the model must import into any Grafana: it
// names no datasource of its own, only the one picked in its selector.
func TestTheDashboardUsesTheSelectedDatasource(t *testing.T) {
	model := loadDashboard(t)
	if len(model.datasources) == 0 {
		t.Fatal("the dashboard model references no datasource")
	}
	for _, ref := range model.datasources {
		if ref.uid == "${datasource}" || ref.uid == "-- Grafana --" {
			continue
		}
		t.Errorf("%q names the fixed datasource %v; use ${datasource}", ref.panel, ref.uid)
	}
}

var namespaceMatcher = regexp.MustCompile(`namespace\s*(=~|!=|!~|=)\s*"([^"]*)"`)

// A literal namespace would publish a deployment's name and pin the dashboard
// to it.
func TestTheDashboardOnlyFiltersOnTheSelectedNamespace(t *testing.T) {
	for _, query := range loadDashboard(t).queries {
		for _, match := range namespaceMatcher.FindAllStringSubmatch(query.expr, -1) {
			if match[1] != "=" || match[2] != "$namespace" {
				t.Errorf("panel %q filters on %s; use namespace=\"$namespace\"", query.panel, match[0])
			}
		}
	}
}
