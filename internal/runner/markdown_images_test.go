package runner

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestMarkdownImageTargets(t *testing.T) {
	source, e := os.ReadFile("testdata/markdown_image_document.md")
	if e != nil {
		t.Fatal(e)
	}
	// desktop/tests/markdown-view.test.mjs expects the same targets from
	// markdown-it, which runs with html:false, so HTML is text and an image between
	// HTML tags, inline or in what would be an HTML block, is still an image.
	got := markdownImageTargets(source)
	want := []string{"a.png", "../logo.png", "collapsed.png", "cell.png", "list.png", "badge.svg", "inline-html.png", "block-html.png", "my_shot.png", "a&b.png", "my shot.png", "struck.png"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got := markdownImageTargets(nil); len(got) != 0 {
		t.Fatalf("empty document: %q", got)
	}
}

// The same table drives desktop/tests/markdown-view.test.mjs, so both
// resolvers map a target to the same repository path.
func TestResolveImageTargetSharedTable(t *testing.T) {
	raw, e := os.ReadFile("testdata/markdown_image_targets.json")
	if e != nil {
		t.Fatal(e)
	}
	var rows []struct{ Document, Target, Path, Reason string }
	if e = json.Unmarshal(raw, &rows); e != nil {
		t.Fatal(e)
	}
	if len(rows) < 20 {
		t.Fatalf("table too short: %d rows", len(rows))
	}
	for _, row := range rows {
		p, reason := resolveImageTarget(row.Document, row.Target)
		if p != row.Path || reason != row.Reason {
			t.Errorf("%s + %q: got (%q, %q), want (%q, %q)", row.Document, row.Target, p, reason, row.Path, row.Reason)
		}
	}
}

func TestSniffImage(t *testing.T) {
	for name, c := range map[string]struct {
		content string
		mime    string
	}{
		"png":               {"\x89PNG\r\n\x1a\nrest", "image/png"},
		"jpeg":              {"\xFF\xD8\xFF\xE0rest", "image/jpeg"},
		"gif87a":            {"GIF87a....", "image/gif"},
		"gif89a":            {"GIF89a....", "image/gif"},
		"webp":              {"RIFF\x10\x00\x00\x00WEBPVP8 ", "image/webp"},
		"svg":               {`<svg xmlns="http://www.w3.org/2000/svg"/>`, "image/svg+xml"},
		"svg prolog":        {"\xEF\xBB\xBF<?xml version=\"1.0\"?>\n<!-- drawn by hand -->\n<!DOCTYPE svg PUBLIC \"-//W3C//DTD SVG 1.1//EN\" \"http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd\">\n  <svg xmlns=\"http://www.w3.org/2000/svg\"><rect/></svg>", "image/svg+xml"},
		"svg namespace":     {`<s:svg xmlns:s="http://www.w3.org/2000/svg"/>`, "image/svg+xml"},
		"svg entity":        {"<!DOCTYPE svg [<!ENTITY a \"aaaaaaaaaa\"><!ENTITY b \"&a;&a;&a;&a;&a;\">]><svg>&b;</svg>", "image/svg+xml"},
		"html":              {"<!DOCTYPE html><html><body><svg></svg></body></html>", ""},
		"xml not svg":       {`<?xml version="1.0"?><root><svg/></root>`, ""},
		"text before svg":   {"hello <svg/>", ""},
		"lfs pointer":       {"version https://git-lfs.github.com/spec/v1\noid sha256:4d7a\nsize 12345\n", ""},
		"truncated png":     {"\x89PNG\r\n", ""},
		"truncated jpeg":    {"\xFF\xD8", ""},
		"truncated gif":     {"GIF8", ""},
		"truncated webp":    {"RIFF\x10\x00\x00\x00WEB", ""},
		"riff not webp":     {"RIFF\x10\x00\x00\x00WAVEfmt ", ""},
		"invalid utf-8":     {"<svg>\xff</svg>", ""},
		"malformed xml":     {"<svg", ""},
		"empty":             {"", ""},
		"closing first":     {"</svg>", ""},
		"undeclared entity": {"<svg>&nope;</svg>", "image/svg+xml"},
	} {
		mime, ok := sniffImage([]byte(c.content))
		if mime != c.mime || ok != (c.mime != "") {
			t.Errorf("%s: got (%q, %v), want %q", name, mime, ok, c.mime)
		}
	}
}
