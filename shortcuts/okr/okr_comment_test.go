// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package okr

import (
	"bytes"
	"strings"
	"testing"

	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/core"
	"github.com/larksuite/cli/shortcuts/common"
	"github.com/spf13/cobra"
)

func commentTestConfig(t *testing.T) *core.CliConfig {
	t.Helper()
	return &core.CliConfig{AppID: "test-okr-comment", AppSecret: "secret-okr-comment", Brand: core.BrandFeishu}
}

func runCommentShortcut(t *testing.T, shortcut *common.Shortcut, args []string) (error, *bytes.Buffer) {
	t.Helper()
	f, stdout, _, _ := cmdutil.TestFactory(t, commentTestConfig(t))
	parent := &cobra.Command{Use: "okr"}
	shortcut.Mount(parent, f)
	parent.SetArgs(args)
	parent.SilenceErrors = true
	parent.SilenceUsage = true
	return parent.Execute(), stdout
}

func TestCommentValidationSelectionModes(t *testing.T) {
	args := []string{"+comment-create", "--target-id", "1", "--target-type", "objective", "--content", "{\"text\":\"x\"}"}
	err, _ := runCommentShortcut(t, &OKRCreateComment, args)
	if err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("error = %v", err)
	}
}

func TestCommentThreadsGroupSelectionAndSort(t *testing.T) {
	t.Parallel()
	selection := "77"
	threads := groupCommentThreads([]Comment{
		{ID: "2", CreateTime: "20", Selection: &CommentSelection{ID: selection}},
		{ID: "1", CreateTime: "10", Selection: &CommentSelection{ID: selection}},
		{ID: "3", CreateTime: "15"},
	})
	if len(threads) != 2 || threads[0][0].ID != "1" || threads[0][1].ID != "2" || threads[1][0].ID != "3" {
		t.Fatalf("threads = %#v", threads)
	}
}

func TestCommentThreadOutputUsesDetailShape(t *testing.T) {
	threads := commentThreadOutput("simple", []Comment{
		{ID: "2", CreateTime: "20", Selection: &CommentSelection{ID: "77"}, Content: &ContentBlock{}},
		{ID: "1", CreateTime: "10", Selection: &CommentSelection{ID: "77"}, Content: &ContentBlock{}},
		{ID: "3", CreateTime: "15", Content: &ContentBlock{}},
	})
	if len(threads) != 2 || len(threads[0]) != 2 || threads[0][0].ID != "1" || threads[0][1].ID != "2" || len(threads[1]) != 1 || threads[1][0].ID != "3" {
		t.Fatalf("threads = %#v", threads)
	}
}

func TestCommentCreateDryRunUsesWildcardSelection(t *testing.T) {
	f, stdout, _, _ := cmdutil.TestFactory(t, commentTestConfig(t))
	parent := &cobra.Command{Use: "okr"}
	OKRCreateComment.Mount(parent, f)
	parent.SetArgs([]string{
		"+comment-create",
		"--target-type", "objective",
		"--target-id", "1",
		"--content", "{\"text\":\"hello\"}",
		"--select-all",
		"--dry-run",
	})
	if err := parent.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := stdout.String()
	if !strings.Contains(output, "\"selected_text\": \"*****\"") {
		t.Fatalf("dry-run should use wildcard selection, got: %s", output)
	}
	if strings.Contains(output, "department_id_type") {
		t.Fatalf("dry-run must omit department_id_type, got: %s", output)
	}
}

func TestCommentReopenDryRunUsesReopenPath(t *testing.T) {
	f, stdout, _, _ := cmdutil.TestFactory(t, commentTestConfig(t))
	parent := &cobra.Command{Use: "okr"}
	OKRReopenComment.Mount(parent, f)
	parent.SetArgs([]string{"+comment-reopen", "--comment-id", "2", "--dry-run"})
	if err := parent.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := stdout.String()
	if !strings.Contains(output, "/open-apis/okr/v2/comments/2/reopen") {
		t.Fatalf("dry-run should use reopen endpoint, got: %s", output)
	}
	if strings.Contains(output, "department_id_type") {
		t.Fatalf("dry-run must omit department_id_type, got: %s", output)
	}
}
