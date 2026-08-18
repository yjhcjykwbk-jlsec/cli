// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package slides

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/larksuite/cli/errs"
)

// lintBlockDetail is the JSON the backend puts in the error message when the
// layout lint refuses a write. Only the fields this hint is built from are named;
// everything else the document carries — including the per-rule measurements on
// each finding — lands in the ignored remainder rather than breaking the parse,
// and reaches the caller through the message itself, which is left verbatim.
type lintBlockDetail struct {
	Message string `json:"message"`
	// Blocked counts what refused the write, which is every finding in the report:
	// the server has no severity threshold, because a level that did not block
	// would mean reporting a defect and writing the page anyway. It is read rather
	// than derived from len(Issues) because it is the server's own count, and a
	// disagreement between the two is the server's to resolve.
	Blocked int `json:"blocked"`
	// Issues is the whole report, and all of it has to go before a retry succeeds.
	Issues []lintBlockIssue `json:"issues"`
	// SchemaIssues is the schema-sanitize report, carried as its own field so the
	// message stays parseable as a whole.
	SchemaIssues string `json:"schema_issues"`
}

// lintBlockIssue names the three fields this file locates and validates by. A
// finding carries more than this — which numbers depends on which rule reported
// it — and the rest is not decoded here because nothing here would do anything
// with it. It is not lost: the document it came in is what the caller reads.
//
// Level is read only to recognise the document, not to sort it: every finding
// refuses the write regardless of level, so there is nothing here to sort into.
type lintBlockIssue struct {
	Level       string `json:"level"`
	Code        string `json:"code"`
	SlideNumber int    `json:"slide_number"`
}

// lintRemediationHint names the escape hatch. The backend cannot write this line
// itself: --no-lint is a CLI flag and the server has never heard of it.
// The opening says "the page" and not "nothing": on the per-page paths the
// refusal can arrive after the deck and some of its pages already exist, and a
// blanket "nothing was written" would contradict the progress hint sitting next
// to it. One page is the unit the gate refuses on every path, so it is what this
// says.
const lintRemediationHint = "the page was not written. Fix the issues listed in the message and retry;" +
	" if the lint is wrong about a page that has to ship as-is, re-run with --no-lint"

// enrichSlidesLintError adds what the CLI knows about a layout-lint refusal, and
// leaves every other error alone.
//
// The refusal arrives as a JSON document in the message field — the shape the
// backend also uses for nodeServer validation failures — and it stays there
// verbatim. The same refusal reaches callers two ways: through these shortcuts,
// and through `lark-cli api` calling the endpoint directly, where nothing
// rewrites it. Rendering the document to prose here would mean the same refusal
// has two different message formats depending on which command produced it, so a
// caller could not parse one field one way. Verbatim also matches
// ERROR_CONTRACT.md's "propagate typed errors unchanged": the server named the
// offending element, the page it sits on, and how to fix it.
//
// What the backend cannot say is added as a hint instead: the issue count, the
// pages involved, and --no-lint, which is a CLI flag the server has never heard
// of. That is the one thing this layer knows and the report does not, which is
// the only reason this enrichment exists at all.
//
// Detection is by document shape, not by status code, because the OpenAPI
// gateway remaps engine codes per its own table and the code the CLI sees is not
// the code the engine sent. A message that is not this document is returned
// untouched, so the helper is safe on write paths whose backend does not lint.
func enrichSlidesLintError(err error) error {
	if err == nil {
		return nil
	}
	p, ok := errs.ProblemOf(err)
	if !ok {
		return err
	}
	detail, ok := parseLintBlockDetail(p.Message)
	if !ok {
		return err
	}
	// Reuses the progress-hint helper for its append-preserving-classification
	// behaviour, not for its orchestration meaning.
	return appendSlidesProgressHint(err, lintBlockHint(detail))
}

// parseLintBlockDetail recognises the refusal document. It insists on a blocked
// count and on issues that carry both a level and a code, because a bare "has an
// issues array" test would also match unrelated backend errors that report
// through the same field — and a false positive here rewrites a message the CLI
// does not understand.
func parseLintBlockDetail(message string) (lintBlockDetail, bool) {
	var detail lintBlockDetail
	trimmed := strings.TrimSpace(message)
	if !strings.HasPrefix(trimmed, "{") {
		return detail, false
	}
	if err := json.Unmarshal([]byte(trimmed), &detail); err != nil {
		return detail, false
	}
	if detail.Blocked <= 0 || len(detail.Issues) == 0 {
		return detail, false
	}
	for _, issue := range detail.Issues {
		if issue.Level == "" || issue.Code == "" {
			return detail, false
		}
	}
	return detail, true
}

// lintBlockHint summarises the refusal without restating it. The findings
// themselves stay in the message, so this says only what a caller needs before
// deciding whether to read them: how many refused the write, which pages those
// sit on, and what to do next.
//
// The schema findings get their own note because they travel in their own field
// and are easy to miss inside a long document.
func lintBlockHint(detail lintBlockDetail) string {
	head := detail.Message
	if head == "" {
		head = "xml lint blocked"
	}
	parts := []string{fmt.Sprintf("%s: %d blocking issue(s)", head, detail.Blocked)}
	if pages := lintBlockPages(detail.Issues); pages != "" {
		parts = append(parts, "on "+pages)
	}
	if detail.SchemaIssues != "" {
		parts = append(parts, "the message also carries schema_issues")
	}
	return strings.Join(parts, ", ") + ". " + lintRemediationHint
}

// lintBlockPages names the pages the findings sit on, so a caller knows where to
// look before parsing anything. Document-scoped findings have no page number and
// are reported as such. The list is capped because a deck can fail on every page
// and a hint that names forty of them helps nobody — the message still has all
// of them.
func lintBlockPages(issues []lintBlockIssue) string {
	const maxNamed = 5
	var named []string
	seen := map[int]bool{}
	document := false
	for _, issue := range issues {
		if issue.SlideNumber <= 0 {
			document = true
			continue
		}
		if seen[issue.SlideNumber] {
			continue
		}
		seen[issue.SlideNumber] = true
		if len(named) < maxNamed {
			named = append(named, fmt.Sprintf("slide %d", issue.SlideNumber))
		}
	}
	if rest := len(seen) - len(named); rest > 0 {
		named = append(named, fmt.Sprintf("and %d more page(s)", rest))
	}
	if document {
		named = append(named, "the document itself")
	}
	return strings.Join(named, ", ")
}
