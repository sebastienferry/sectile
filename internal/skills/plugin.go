package skills

import (
	"encoding/json"
	"fmt"
	"regexp"

	"tasks/internal/models"
)

// PluginName is the name of the Claude plugin distributing Sectile: its skills
// are invoked as /sectile:<dir> and its MCP server is named after it too.
const PluginName = "sectile"

// pluginVersion is SemVer without a leading "v", which is what plugin.json and
// marketplace.json carry.
var pluginVersion = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$`)

func validatePluginVersion(version string) error {
	if !pluginVersion.MatchString(version) {
		return fmt.Errorf("plugin version %q is not SemVer without a leading v, for example 1.4.0", version)
	}
	return nil
}

// RenderPlugin returns the Claude plugin distributing Sectile's workflow
// skills and its MCP server declaration, keyed by path relative to the plugin
// root. The output depends only on the catalogue and the version: the server
// URL and the workstation API key are asked by Claude at install time, the key
// kept in its secure storage.
func RenderPlugin(version string) (map[string][]byte, error) {
	if err := validatePluginVersion(version); err != nil {
		return nil, err
	}
	manifest := map[string]any{
		"name":        PluginName,
		"version":     version,
		"description": "Sectile workflow skills and the Sectile MCP server, for every project of a Sectile server.",
		"author":      map[string]any{"name": "Sectile"},
		"skills":      "./skills/",
		"userConfig": map[string]any{
			"server_url": map[string]any{
				"type":        "string",
				"title":       "Sectile server URL",
				"description": "Sectile server URL, for example https://sectile.example.com",
				"required":    true,
			},
			"api_key": map[string]any{
				"type":        "string",
				"title":       "Workstation API key",
				"description": "Workstation API key, from the desktop Connection settings. Kept in Claude Code secure storage.",
				"required":    true,
				"sensitive":   true,
			},
		},
	}
	// Same shape as the direct setup's Claude entry (agentconfig.mcpEntry).
	mcp := map[string]any{
		"mcpServers": map[string]any{
			PluginName: map[string]any{
				"type":    "http",
				"url":     "${user_config.server_url}/mcp",
				"headers": map[string]any{"Authorization": "Bearer ${user_config.api_key}"},
			},
		},
	}

	files := map[string][]byte{}
	var err error
	if files[".claude-plugin/plugin.json"], err = marshalPluginJSON(manifest); err != nil {
		return nil, err
	}
	if files[".mcp.json"], err = marshalPluginJSON(mcp); err != nil {
		return nil, err
	}
	for _, s := range StageSkills {
		files["skills/"+s.DirName+"/SKILL.md"] = []byte(RenderGenericSkillContent(s))
		if legacy := models.LegacySkillDirs[s.DirName]; legacy != "" {
			files["skills/"+legacy+"/SKILL.md"] = []byte(renderPluginAlias(legacy, s.DirName))
		}
	}
	return files, nil
}

// renderPluginAlias is a skill kept under a former name (#608): it only hands
// its arguments to the skill that replaced it, so a command typed with the old
// name keeps running the same stage.
func renderPluginAlias(legacy, target string) string {
	return fmt.Sprintf("---\nname: %s\ndescription: %s\n---\n"+
		"# %s (alias)\n\n"+
		"`%s` is the former name of `%s`. Invoke `%s:%s` with the same arguments (in Claude Code, the Skill tool). "+
		"If the running agent cannot invoke skills, read the sibling `../%s/SKILL.md` and follow it as written.\n",
		legacy, YAMLString("Former name of "+target+", kept as an alias."), legacy,
		legacy, target, PluginName, target, target)
}

// MarketplacePluginDir is where RenderMarketplace expects the plugin, relative
// to the marketplace root.
const MarketplacePluginDir = "plugins/" + PluginName

// RenderMarketplace returns a one-plugin marketplace around RenderPlugin, for
// the tests and for trying the plugin out from a local folder. How the
// published marketplace repository is laid out is not decided here.
func RenderMarketplace(version string) (map[string][]byte, error) {
	plugin, err := RenderPlugin(version)
	if err != nil {
		return nil, err
	}
	marketplace := map[string]any{
		"name":        PluginName,
		"owner":       map[string]any{"name": "Sectile"},
		"description": "The Sectile plugin for Claude Code.",
		"plugins": []any{map[string]any{
			"name":        PluginName,
			"source":      "./" + MarketplacePluginDir,
			"description": "Sectile workflow skills and MCP server.",
			"version":     version,
		}},
	}
	files := map[string][]byte{}
	if files[".claude-plugin/marketplace.json"], err = marshalPluginJSON(marketplace); err != nil {
		return nil, err
	}
	for name, content := range plugin {
		files[MarketplacePluginDir+"/"+name] = content
	}
	return files, nil
}

// marshalPluginJSON is indented and newline-terminated; encoding/json sorts
// map keys, which keeps the output byte-identical between runs.
func marshalPluginJSON(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
