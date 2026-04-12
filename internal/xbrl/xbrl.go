// Package xbrl parses iXBRL (Inline XBRL) .xhtml documents and extracts
// structured financial facts. It uses an HTML5 parser rather than a strict XML
// parser so that real-world iXBRL files with HTML entities and minor markup
// quirks are handled correctly. The HTML5 parser has no XML external-entity
// processing, which eliminates the XXE and entity-expansion attack surface.
//
// External taxonomy references embedded in iXBRL documents (schemaRef,
// linkbaseRef) are intentionally ignored — this package makes no network calls.
package xbrl

import (
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// MaxFileSizeBytes is the maximum iXBRL file size that ParseFacts will load
// into the HTML parser. Files larger than this limit are rejected to bound
// memory usage.
const MaxFileSizeBytes int64 = 50 * 1024 * 1024 // 50 MB

// MaxFacts is the maximum number of facts ParseFacts will return. This cap
// prevents memory exhaustion when parsing adversarial documents with an
// extreme number of tagged elements.
const MaxFacts = 2000

// maxScale and minScale bound the XBRL scale attribute. Values outside this
// range would produce float64 overflow or underflow when math.Pow10 is applied.
const maxScale = 15
const minScale = -15

// maxContexts and maxUnits cap the number of entries built into the context and
// unit maps. An adversarial document could contain hundreds of thousands of these
// elements within the 50 MB file size limit; the caps prevent map memory from
// growing far beyond the file size.
const maxContexts = 10_000
const maxUnits = 10_000

// maxTextFactLen is the maximum number of runes kept from any single ix:nonNumeric
// text value. Annual report notes sections can span many pages; truncating prevents
// a single fact from producing an oversized MCP response.
const maxTextFactLen = 500

// pdfRenderedTextRatio is the minimum fraction of non-empty text nodes that must
// be ≤ pdfRenderedMaxNodeLen runes for the fragmentation-ratio heuristic to trigger.
const pdfRenderedTextRatio = 0.8

// pdfRenderedMaxNodeLen is the rune-length threshold used by the fragmentation-ratio
// heuristic. Text nodes shorter than or equal to this length count as "short".
const pdfRenderedMaxNodeLen = 10

// RenderTypeNativeIXBRL indicates the document is a standard iXBRL filing with
// accessible narrative text.
const RenderTypeNativeIXBRL = "native_ixbrl"

// RenderTypePDFRendered indicates the document was produced by a PDF-to-HTML
// converter (e.g. pdf2htmlEX). Structured XBRL facts are extractable, but
// narrative prose is fragmented into character-level nodes or embedded images
// and is not reliably readable as text.
const RenderTypePDFRendered = "pdf_rendered"

// ParseResult holds the output of ParseFacts.
type ParseResult struct {
	// Facts is the slice of extracted XBRL facts. Always non-nil.
	Facts []Fact
	// Truncated is true when the document contained more facts than MaxFacts.
	Truncated bool
	// RenderType is RenderTypeNativeIXBRL or RenderTypePDFRendered.
	RenderType string
}

// Fact is a single XBRL fact extracted from an iXBRL document.
type Fact struct {
	Name   string `json:"name"`
	Value  any    `json:"value"`            // float64 for numeric facts, string for text facts
	Period string `json:"period,omitempty"` // ISO 8601 date or interval
	Unit   string `json:"unit,omitempty"`   // omitted for text facts
}

// Options controls which facts ParseFacts returns.
type Options struct {
	// NamePrefix, if non-empty, restricts output to facts whose Name starts
	// with this prefix. The match is case-sensitive.
	NamePrefix string

	// IncludeTextFacts controls whether ix:nonNumeric text facts are included
	// in the output in addition to the default ix:nonFraction numeric facts.
	// Text facts (director names, company descriptions, etc.) are omitted by
	// default to keep responses compact.
	IncludeTextFacts bool
}

// ParseFacts parses the iXBRL .xhtml file at path and returns a ParseResult.
// At most MaxFacts facts are returned; ParseResult.Truncated is true when the
// document contained more facts than the cap. Files larger than
// MaxFileSizeBytes are rejected without reading their content.
func ParseFacts(path string, opts Options) (ParseResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return ParseResult{}, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	// Stat the open file descriptor to avoid a TOCTOU race between a separate
	// os.Stat call and the subsequent read.
	info, err := f.Stat()
	if err != nil {
		return ParseResult{}, fmt.Errorf("stat file: %w", err)
	}
	if info.Size() > MaxFileSizeBytes {
		return ParseResult{}, fmt.Errorf("file size %d bytes exceeds limit of %d bytes", info.Size(), MaxFileSizeBytes)
	}

	// Wrap in LimitReader as a second line of defence: even if the file is
	// replaced after the size check, the parser will never read more than the limit.
	lr := io.LimitReader(f, MaxFileSizeBytes+1)
	root, err := html.Parse(lr)
	if err != nil {
		return ParseResult{}, fmt.Errorf("parse HTML: %w", err)
	}

	contexts := buildContextMap(root)
	units := buildUnitMap(root)
	renderType := detectRenderType(root)
	facts, truncated := collectFacts(root, contexts, units, opts)
	return ParseResult{Facts: facts, Truncated: truncated, RenderType: renderType}, nil
}

// buildContextMap walks the DOM and returns a map from xbrli:context id to a
// human-readable period string. Instant contexts produce "YYYY-MM-DD".
// Duration contexts produce "YYYY-MM-DD/YYYY-MM-DD" (ISO 8601 interval).
func buildContextMap(root *html.Node) map[string]string {
	m := make(map[string]string)
	walkTree(root, func(n *html.Node) {
		if len(m) >= maxContexts {
			return
		}
		if n.Type != html.ElementNode || n.Data != "xbrli:context" {
			return
		}
		id := attrVal(n, "id")
		if id == "" {
			return
		}
		// Find the xbrli:period child.
		var period *html.Node
		walkTree(n, func(c *html.Node) {
			if c.Type == html.ElementNode && c.Data == "xbrli:period" && period == nil {
				period = c
			}
		})
		if period == nil {
			return
		}
		var instant, start, end string
		walkTree(period, func(c *html.Node) {
			if c.Type != html.ElementNode {
				return
			}
			switch c.Data {
			case "xbrli:instant":
				instant = strings.TrimSpace(nodeText(c))
			case "xbrli:startdate":
				start = strings.TrimSpace(nodeText(c))
			case "xbrli:enddate":
				end = strings.TrimSpace(nodeText(c))
			}
		})
		switch {
		case instant != "":
			m[id] = instant
		case start != "" && end != "":
			m[id] = start + "/" + end
		}
	})
	return m
}

// buildUnitMap walks the DOM and returns a map from xbrli:unit id to a
// human-readable unit label. Namespace prefixes are stripped from measures
// (e.g. "iso4217:GBP" → "GBP"). For divide units, only the numerator measure
// is used (xbrli:unitDenominator is skipped).
func buildUnitMap(root *html.Node) map[string]string {
	m := make(map[string]string)
	walkTree(root, func(n *html.Node) {
		if len(m) >= maxUnits {
			return
		}
		if n.Type != html.ElementNode || n.Data != "xbrli:unit" {
			return
		}
		id := attrVal(n, "id")
		if id == "" {
			return
		}
		m[id] = stripPrefix(unitMeasure(n))
	})
	return m
}

// unitMeasure extracts the primary measure label from a xbrli:unit node.
// For simple units it returns the text of the first xbrli:measure child.
// For divide units it reads only the numerator subtree, ignoring the denominator.
func unitMeasure(unitNode *html.Node) string {
	for c := unitNode.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode {
			continue
		}
		switch c.Data {
		case "xbrli:measure":
			return strings.TrimSpace(nodeText(c))
		case "xbrli:divide":
			// Find xbrli:unitnumerator inside divide; skip xbrli:unitdenominator.
			for d := c.FirstChild; d != nil; d = d.NextSibling {
				if d.Type == html.ElementNode && d.Data == "xbrli:unitnumerator" {
					if m := firstMeasure(d); m != "" {
						return m
					}
				}
			}
		}
	}
	return ""
}

// firstMeasure returns the text content of the first xbrli:measure descendant of n.
func firstMeasure(n *html.Node) string {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "xbrli:measure" {
			return strings.TrimSpace(nodeText(c))
		}
		if m := firstMeasure(c); m != "" {
			return m
		}
	}
	return ""
}

// collectFacts walks the DOM and returns XBRL facts. Facts are capped at
// MaxFacts to bound memory usage from adversarial inputs. truncated is true
// when at least one eligible fact was skipped because the cap was reached.
// The returned slice is always non-nil so callers get [] rather than null in JSON.
func collectFacts(root *html.Node, contexts, units map[string]string, opts Options) (facts []Fact, truncated bool) {
	facts = []Fact{}
	walkTree(root, func(n *html.Node) {
		if n.Type != html.ElementNode {
			return
		}
		switch n.Data {
		case "ix:nonfraction":
			if f, ok := parseNumericFact(n, contexts, units, opts.NamePrefix); ok {
				if len(facts) >= MaxFacts {
					truncated = true
					return
				}
				facts = append(facts, f)
			}
		case "ix:nonnumeric":
			if opts.IncludeTextFacts {
				if f, ok := parseTextFact(n, contexts, opts.NamePrefix); ok {
					if len(facts) >= MaxFacts {
						truncated = true
						return
					}
					facts = append(facts, f)
				}
			}
		// Explicitly ignored taxonomy reference elements — no external fetching.
		case "link:schemaref", "link:linkbaseref", "link:rolelinkbaseref":
			// intentionally ignored
		}
	})
	return facts, truncated
}

// parseNumericFact extracts a single ix:nonFraction fact from n.
// Returns (fact, true) on success, (Fact{}, false) if the fact should be skipped.
func parseNumericFact(n *html.Node, contexts, units map[string]string, namePrefix string) (Fact, bool) {
	name := stripPrefix(attrVal(n, "name"))
	if name == "" {
		return Fact{}, false
	}
	if namePrefix != "" && !strings.HasPrefix(name, namePrefix) {
		return Fact{}, false
	}
	// Skip nil facts — they carry no value. The iXBRL spec uses xsi:nil="true";
	// some producers omit the namespace prefix and write nil="true" directly.
	// The HTML5 parser preserves the prefix in the attribute key, so both forms
	// must be checked.
	if attrVal(n, "nil") == "true" || attrVal(n, "xsi:nil") == "true" {
		return Fact{}, false
	}

	raw := strings.ReplaceAll(collectText(n), ",", "")
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Fact{}, false
	}

	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return Fact{}, false
	}

	// Apply scale attribute: value × 10^scale.
	if scaleStr := attrVal(n, "scale"); scaleStr != "" {
		scale, err := strconv.Atoi(scaleStr)
		if err != nil || scale < minScale || scale > maxScale {
			return Fact{}, false
		}
		v *= math.Pow10(scale)
	}

	// Apply sign attribute: sign="-" negates the value.
	if attrVal(n, "sign") == "-" {
		v = -v
	}

	// Guard against Inf and NaN: strconv.ParseFloat("1e400") returns +Inf with
	// no error, and scale multiplication can also produce overflow. json.Marshal
	// cannot represent these values and would return an error.
	if math.IsInf(v, 0) || math.IsNaN(v) {
		return Fact{}, false
	}

	return Fact{
		Name:   name,
		Value:  v,
		Period: contexts[attrVal(n, "contextref")],
		Unit:   units[attrVal(n, "unitref")],
	}, true
}

// parseTextFact extracts a single ix:nonNumeric fact from n.
// Returns (fact, true) on success, (Fact{}, false) if the fact should be skipped.
func parseTextFact(n *html.Node, contexts map[string]string, namePrefix string) (Fact, bool) {
	name := stripPrefix(attrVal(n, "name"))
	if name == "" {
		return Fact{}, false
	}
	if namePrefix != "" && !strings.HasPrefix(name, namePrefix) {
		return Fact{}, false
	}

	text := strings.TrimSpace(collectText(n))
	if text == "" {
		return Fact{}, false
	}
	// Truncate long text values — annual report notes sections can span entire
	// pages; returning them verbatim would produce an oversized MCP response.
	// Rune-based slicing is required to avoid splitting multi-byte UTF-8 sequences.
	if runes := []rune(text); len(runes) > maxTextFactLen {
		text = string(runes[:maxTextFactLen]) + "…"
	}

	return Fact{
		Name:   name,
		Value:  text,
		Period: contexts[attrVal(n, "contextref")],
	}, true
}

// detectRenderType inspects the DOM for signatures of PDF-to-HTML conversion
// and returns RenderTypePDFRendered or RenderTypeNativeIXBRL.
//
// Two heuristics are applied in order; the first that fires wins:
//  1. Structural: presence of a <div class="pf"> element — the canonical
//     per-page wrapper emitted by pdf2htmlEX.
//  2. Fragmentation ratio: if more than pdfRenderedTextRatio of all non-empty
//     text nodes are ≤ pdfRenderedMaxNodeLen runes, the document's text is
//     character-split and narrative is not readable as prose.
func detectRenderType(root *html.Node) string {
	// Both heuristics are collected in a single DOM walk.
	// Heuristic 1: pdf2htmlEX page wrapper (<div class="pf">).
	// Heuristic 2: high ratio of very short text nodes (character-level fragmentation).
	var hasPFDiv bool
	var total, short int
	walkTree(root, func(n *html.Node) {
		switch n.Type {
		case html.ElementNode:
			if !hasPFDiv && n.Data == "div" {
				for _, field := range strings.Fields(attrVal(n, "class")) {
					if field == "pf" {
						hasPFDiv = true
						break
					}
				}
			}
		case html.TextNode:
			trimmed := strings.TrimSpace(n.Data)
			if trimmed == "" {
				return
			}
			total++
			if len([]rune(trimmed)) <= pdfRenderedMaxNodeLen {
				short++
			}
		}
	})
	if hasPFDiv {
		return RenderTypePDFRendered
	}
	if total > 0 && float64(short)/float64(total) > pdfRenderedTextRatio {
		return RenderTypePDFRendered
	}
	return RenderTypeNativeIXBRL
}

// walkTree calls fn for every node in the subtree rooted at n, in depth-first
// pre-order. fn must not modify the tree during traversal.
//
// Recursion depth is bounded in practice by the 50 MB file size limit in
// ParseFacts: HTML documents should not be able to nest deeply enough within
// that budget to exhaust Go's dynamically-grown goroutine stack.
func walkTree(n *html.Node, fn func(*html.Node)) {
	fn(n)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walkTree(c, fn)
	}
}

// attrVal returns the value of attribute key on n, or "" if not present.
// The HTML5 parser lowercases all attribute names.
func attrVal(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// collectText returns the concatenated text content of all text nodes in n's
// subtree, skipping any ix:exclude subtrees (which contain display-only content
// not intended for machine reading).
func collectText(n *html.Node) string {
	var sb strings.Builder
	appendText(n, &sb)
	return sb.String()
}

// appendText recursively appends text content to sb, pruning ix:exclude subtrees.
func appendText(n *html.Node, sb *strings.Builder) {
	if n.Type == html.ElementNode && n.Data == "ix:exclude" {
		return // skip entire subtree
	}
	if n.Type == html.TextNode {
		sb.WriteString(n.Data)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		appendText(c, sb)
	}
}

// nodeText returns the direct text content of n (not its descendants), trimmed.
// Used for simple leaf elements like xbrli:instant where mixed content is not expected.
func nodeText(n *html.Node) string {
	var sb strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode {
			sb.WriteString(c.Data)
		}
	}
	return sb.String()
}

// stripPrefix removes the XML namespace prefix from a qualified name.
// "iso4217:GBP" → "GBP", "Revenue" → "Revenue".
func stripPrefix(name string) string {
	if _, local, ok := strings.Cut(name, ":"); ok {
		return local
	}
	return name
}
