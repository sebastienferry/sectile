package runner

import (
	"bytes"
	"encoding/xml"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Images referenced by the documents travel with their own bounds, apart from
// the patch and document budgets (#683). They are variables only so that tests
// can lower them; the reasons name the production values.
var diffImageLimit = 2 << 20
var diffImageBudget = 8 << 20

// diffImagePathChunk bounds the paths given to one ls-tree command line.
const diffImagePathChunk = 256

// DiffImageRef is one repository path a document's images resolve to. It
// carries exactly one of Image, a key into WorktreeDiff.Images, and
// OmittedReason.
type DiffImageRef struct {
	Path          string `json:"path"`
	Image         string `json:"image,omitempty"`
	OmittedReason string `json:"omittedReason,omitempty"`
}

// DiffImage is an image file's content and the type sniffed from it.
// encoding/json writes Data as Base64.
type DiffImage struct {
	MimeType string `json:"mimeType"`
	Data     []byte `json:"data"`
}

// markdownImageParser mirrors the Desktop renderer's markdown-it setup:
// CommonMark with tables and strikethrough, and no HTML (html:false), so an
// image written between HTML tags is an image to both parsers. goldmark's
// default lists, minus the HTML block and raw HTML parsers.
var markdownImageParser = parser.NewParser(
	parser.WithBlockParsers(
		util.Prioritized(parser.NewSetextHeadingParser(), 100),
		util.Prioritized(parser.NewThematicBreakParser(), 200),
		util.Prioritized(parser.NewListParser(), 300),
		util.Prioritized(parser.NewListItemParser(), 400),
		util.Prioritized(parser.NewCodeBlockParser(), 500),
		util.Prioritized(parser.NewATXHeadingParser(), 600),
		util.Prioritized(parser.NewFencedCodeBlockParser(), 700),
		util.Prioritized(parser.NewBlockquoteParser(), 800),
		util.Prioritized(parser.NewParagraphParser(), 1000),
	),
	parser.WithInlineParsers(
		util.Prioritized(parser.NewCodeSpanParser(), 100),
		util.Prioritized(parser.NewLinkParser(), 200),
		util.Prioritized(parser.NewAutoLinkParser(), 300),
		util.Prioritized(parser.NewEmphasisParser(), 500),
		util.Prioritized(extension.NewStrikethroughParser(), 500),
	),
	parser.WithParagraphTransformers(
		util.Prioritized(parser.LinkReferenceParagraphTransformer, 100),
		util.Prioritized(extension.NewTableParagraphTransformer(), 200),
	),
	parser.WithASTTransformers(util.Prioritized(extension.NewTableASTTransformer(), 0)),
)

// markdownImageTargets returns the destination of each Markdown image of the
// document, in document order, as markdown-it reports it: backslash escapes
// and character references resolved. Raw HTML is never searched.
func markdownImageTargets(content []byte) []string {
	var targets []string
	document := markdownImageParser.Parse(text.NewReader(content))
	_ = ast.Walk(document, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if image, ok := n.(*ast.Image); ok && entering {
			destination := util.UnescapePunctuations(image.Destination)
			destination = util.ResolveNumericReferences(destination)
			destination = util.ResolveEntityNames(destination)
			targets = append(targets, string(destination))
		}
		return ast.WalkContinue, nil
	})
	return targets
}

var imageScheme = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:`)

// escapeLoosePercent encodes a "%" that starts no escape sequence, as
// markdown-it does before handing a target to the renderer.
func escapeLoosePercent(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && (i+2 >= len(s) || !isHex(s[i+1]) || !isHex(s[i+2])) {
			b.WriteString("%25")
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func isHex(c byte) bool {
	return '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F'
}

// resolveImageTarget turns an image target of the document into a repository
// path. An empty path with no reason means the target is not a repository
// image (a scheme, an empty or undecodable target); a reason says why a
// repository image is refused. desktop/src/markdownView.mjs implements the
// same steps, and testdata/markdown_image_targets.json keeps both in step.
func resolveImageTarget(documentPath, target string) (string, string) {
	if target == "" || imageScheme.MatchString(target) {
		return "", ""
	}
	if i := strings.IndexAny(target, "?#"); i >= 0 {
		target = target[:i]
	}
	decoded, e := url.PathUnescape(escapeLoosePercent(target))
	if e != nil || decoded == "" || !utf8.ValidString(decoded) {
		return "", ""
	}
	joined := strings.TrimLeft(decoded, "/")
	if !strings.HasPrefix(decoded, "/") {
		joined = path.Join(path.Dir(documentPath), decoded)
	}
	resolved := path.Clean(joined)
	if resolved == "." || resolved == ".." || strings.HasPrefix(resolved, "../") {
		return "", "This image path leaves the repository."
	}
	if strings.ContainsAny(resolved, "\n\x00") {
		return resolved, "This path cannot be rendered."
	}
	return resolved, ""
}

// sniffImage identifies a supported image from its leading bytes, never from
// its name.
func sniffImage(content []byte) (string, bool) {
	switch {
	case bytes.HasPrefix(content, []byte("\x89PNG\r\n\x1a\n")):
		return "image/png", true
	case bytes.HasPrefix(content, []byte("\xFF\xD8\xFF")):
		return "image/jpeg", true
	case bytes.HasPrefix(content, []byte("GIF87a")), bytes.HasPrefix(content, []byte("GIF89a")):
		return "image/gif", true
	case len(content) >= 12 && string(content[:4]) == "RIFF" && string(content[8:12]) == "WEBP":
		return "image/webp", true
	case isSVG(content):
		return "image/svg+xml", true
	}
	return "", false
}

// isSVG reads tokens up to the first element, which must be svg. The decoder
// is strict and knows no entity, so a declared entity is never expanded.
func isSVG(content []byte) bool {
	content = bytes.TrimPrefix(content, []byte("\xEF\xBB\xBF"))
	if !utf8.Valid(content) {
		return false
	}
	decoder := xml.NewDecoder(bytes.NewReader(content))
	decoder.Strict = true
	for {
		token, e := decoder.Token()
		if e != nil {
			return false
		}
		switch t := token.(type) {
		case xml.StartElement:
			return t.Name.Local == "svg"
		case xml.CharData:
			if len(bytes.TrimSpace(t)) > 0 {
				return false
			}
		case xml.EndElement:
			return false
		}
	}
}

// attachImages is attachDiffImages; tests replace it to inspect without images.
var attachImages = attachDiffImages

// attachDiffImages reads the repository images the documents reference from
// the trees the documents were read from: the inspected tree for a "new"
// document, the merge-base for an "old" one. Images are taken in file order,
// then reference order, until the image budget is spent; an image referenced
// several times on one side is read and counted once. An image problem is the
// reference's reason; only a Git failure or a changing object fails.
func attachDiffImages(snapshot diffGit, result *WorktreeDiff, omitted map[string]WorktreeDiffFile, ancestor, tree string) error {
	type image struct {
		side, path, object, reason string
		size                       int
	}
	images := map[string]*image{}
	var order []*image
	for i := range result.Files {
		document := result.Files[i].Document
		if document == nil || document.OmittedReason != "" || document.Content == "" {
			continue
		}
		seen := map[string]bool{}
		for _, target := range markdownImageTargets([]byte(document.Content)) {
			p, reason := resolveImageTarget(result.Files[i].Path, target)
			// Desktop resolves targets with the same steps and explains a
			// refused one itself: only readable paths are listed.
			if p == "" || reason != "" || seen[p] {
				continue
			}
			seen[p] = true
			document.Images = append(document.Images, DiffImageRef{Path: p})
			key := document.Side + ":" + p
			if images[key] == nil {
				treeish := tree
				if document.Side == "old" {
					treeish = ancestor
				}
				images[key] = &image{side: document.Side, path: p, object: treeish + ":" + p}
				order = append(order, images[key])
			}
		}
	}
	if len(order) == 0 {
		return nil
	}
	modes := map[string]map[string]string{}
	for _, side := range []string{"new", "old"} {
		treeish := tree
		if side == "old" {
			treeish = ancestor
		}
		var paths []string
		for _, img := range order {
			if img.side == side {
				paths = append(paths, img.path)
			}
		}
		modes[side] = map[string]string{}
		for start := 0; start < len(paths); start += diffImagePathChunk {
			end := min(start+diffImagePathChunk, len(paths))
			listed, e := snapshot.command(nil, diffMetadataLimit, append([]string{"ls-tree", "-z", treeish, "--"}, paths[start:end]...)...)
			if e != nil {
				return e
			}
			for _, record := range bytes.Split(listed, []byte{0}) {
				meta, p, ok := bytes.Cut(record, []byte{'\t'})
				fields := strings.Fields(string(meta))
				if ok && len(fields) == 3 {
					modes[side][string(p)] = fields[0] + " " + fields[1]
				}
			}
		}
	}
	var candidates []*image
	for _, img := range order {
		if skipped, ok := omitted[img.path]; ok && img.side == "new" {
			// Its content never reached the snapshot tree.
			img.reason = skipped.OmittedReason
			continue
		}
		switch modes[img.side][img.path] {
		case "100644 blob", "100755 blob":
			candidates = append(candidates, img)
		case "120000 blob":
			img.reason = "Symbolic links are not followed."
		default:
			img.reason = "Image not found in the inspected state."
		}
	}
	var wanted []*image
	spent := 0
	if len(candidates) > 0 {
		var input bytes.Buffer
		for _, img := range candidates {
			input.WriteString(img.object + "\n")
		}
		checked, e := snapshot.command(input.Bytes(), diffMetadataLimit, "cat-file", "--batch-check=%(objecttype) %(objectsize)")
		if e != nil {
			return e
		}
		lines := strings.Split(strings.TrimSuffix(string(checked), "\n"), "\n")
		if len(lines) != len(candidates) {
			return errDiffBound
		}
		exhausted := false
		for i, img := range candidates {
			fields := strings.Fields(lines[i])
			size := -1
			if len(fields) == 2 && fields[0] == "blob" {
				size, _ = strconv.Atoi(fields[1])
			}
			switch {
			case size < 0:
				img.reason = "Image not found in the inspected state."
			case size > diffImageLimit:
				img.reason = "Image exceeds the 2 MiB display limit."
			case exhausted || spent+size > diffImageBudget:
				// Once the budget is spent, every later image is skipped, as
				// for the documents, even one small enough to fit.
				exhausted = true
				img.reason = "Image skipped: the 8 MiB image budget was reached."
			default:
				// The reservation holds even if the content is then refused:
				// the budget bounds the bytes read, not only those sent.
				spent += size
				img.size = size
				wanted = append(wanted, img)
			}
		}
	}
	if len(wanted) > 0 {
		var input bytes.Buffer
		for _, img := range wanted {
			input.WriteString(img.object + "\n")
		}
		// Each record is "<oid> blob <size>\n<content>\n"; 128 bytes cover a header.
		raw, e := snapshot.command(input.Bytes(), spent+len(wanted)*128, "cat-file", "--batch")
		if e != nil {
			return e
		}
		result.Images = map[string]DiffImage{}
		for _, img := range wanted {
			header, rest, ok := bytes.Cut(raw, []byte{'\n'})
			fields := strings.Fields(string(header))
			if !ok || len(fields) != 3 || fields[2] != strconv.Itoa(img.size) || len(rest) < img.size+1 {
				return diffError("checkout_changed", "Git objects changed during inspection. Refresh to retry.")
			}
			content := rest[:img.size]
			raw = rest[img.size+1:]
			mime, ok := sniffImage(content)
			if !ok {
				img.reason = "This file is not a supported image."
				continue
			}
			result.Images[img.side+":"+img.path] = DiffImage{MimeType: mime, Data: content}
		}
		if len(result.Images) == 0 {
			result.Images = nil
		}
	}
	for i := range result.Files {
		document := result.Files[i].Document
		if document == nil {
			continue
		}
		for j := range document.Images {
			ref := &document.Images[j]
			img := images[document.Side+":"+ref.Path]
			if img.reason != "" {
				ref.OmittedReason = img.reason
			} else {
				ref.Image = document.Side + ":" + ref.Path
			}
		}
	}
	return nil
}
