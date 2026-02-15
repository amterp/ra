package ra

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// parseCompletion is a test helper that captures completion output.
// Returns the stdout output string and the error from ParseOrError.
func parseCompletion(cmd *Cmd, args []string) (string, error) {
	var stdout bytes.Buffer
	SetStdoutWriter(&stdout)
	defer SetStdoutWriter(os.Stdout)

	err := cmd.ParseOrError(args)
	return stdout.String(), err
}

// parseCompletionLines parses completion output into candidates and directive.
func parseCompletionLines(output string) ([]string, string) {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	if len(lines) == 0 {
		return nil, ""
	}
	directive := lines[len(lines)-1]
	candidates := lines[:len(lines)-1]
	// Filter out empty strings
	var filtered []string
	for _, c := range candidates {
		if c != "" {
			filtered = append(filtered, c)
		}
	}
	return filtered, directive
}

func TestCompletionDisabledByDefault(t *testing.T) {
	cmd := NewCmd("test")
	NewString("name").SetOptional(true).Register(cmd)
	NewString("extra").SetOptional(true).Register(cmd)

	err := cmd.ParseOrError([]string{"__complete", ""})
	// Without EnableCompletion, __complete is treated as a positional arg
	assert.Nil(t, err)
}

func TestCompletionReturnsCompletionInvokedErr(t *testing.T) {
	cmd := NewCmd("test").EnableCompletion()
	NewString("name").SetOptional(true).Register(cmd)

	_, err := parseCompletion(cmd, []string{"__complete", ""})
	assert.True(t, errors.Is(err, CompletionInvokedErr))
}

func TestCompletionParseOrExitExitsZero(t *testing.T) {
	cmd := NewCmd("test").EnableCompletion()
	NewString("name").SetOptional(true).Register(cmd)

	var stdout bytes.Buffer
	SetStdoutWriter(&stdout)
	defer SetStdoutWriter(os.Stdout)

	var exitCode int
	exitCalled := false
	SetExitFunc(func(code int) {
		exitCode = code
		exitCalled = true
	})
	defer SetExitFunc(os.Exit)

	cmd.ParseOrExit([]string{"__complete", ""})
	assert.True(t, exitCalled)
	assert.Equal(t, 0, exitCode)
}

func TestCompletionSubcommandNames(t *testing.T) {
	cmd := NewCmd("test").EnableCompletion()
	addCmd := NewCmd("add")
	removeCmd := NewCmd("remove")
	cmd.RegisterCmd(addCmd)
	cmd.RegisterCmd(removeCmd)

	output, _ := parseCompletion(cmd, []string{"__complete", ""})
	candidates, directive := parseCompletionLines(output)

	assert.Contains(t, candidates, "add")
	assert.Contains(t, candidates, "remove")
	assert.Equal(t, ":4", directive) // NoFileComp
}

func TestCompletionSubcommandPrefix(t *testing.T) {
	cmd := NewCmd("test").EnableCompletion()
	cmd.RegisterCmd(NewCmd("add"))
	cmd.RegisterCmd(NewCmd("apply"))
	cmd.RegisterCmd(NewCmd("remove"))

	output, _ := parseCompletion(cmd, []string{"__complete", "a"})
	candidates, _ := parseCompletionLines(output)

	assert.Contains(t, candidates, "add")
	assert.Contains(t, candidates, "apply")
	assert.NotContains(t, candidates, "remove")
}

func TestCompletionLongFlagNames(t *testing.T) {
	cmd := NewCmd("test").EnableCompletion()
	NewString("output").SetShort("o").SetOptional(true).Register(cmd)
	NewBool("verbose").SetShort("v").Register(cmd)

	output, _ := parseCompletion(cmd, []string{"__complete", "--"})
	candidates, directive := parseCompletionLines(output)

	assert.Contains(t, candidates, "--output")
	assert.Contains(t, candidates, "--verbose")
	assert.Contains(t, candidates, "--help")
	assert.Equal(t, ":4", directive)
}

func TestCompletionLongFlagPrefix(t *testing.T) {
	cmd := NewCmd("test").EnableCompletion()
	NewString("output").SetOptional(true).Register(cmd)
	NewBool("verbose").Register(cmd)
	NewBool("version").Register(cmd)

	output, _ := parseCompletion(cmd, []string{"__complete", "--ver"})
	candidates, _ := parseCompletionLines(output)

	assert.Contains(t, candidates, "--verbose")
	assert.Contains(t, candidates, "--version")
	assert.NotContains(t, candidates, "--output")
}

func TestCompletionShortAndLongFlags(t *testing.T) {
	cmd := NewCmd("test").EnableCompletion()
	NewString("output").SetShort("o").SetOptional(true).Register(cmd)
	NewBool("verbose").SetShort("v").Register(cmd)

	output, _ := parseCompletion(cmd, []string{"__complete", "-"})
	candidates, _ := parseCompletionLines(output)

	assert.Contains(t, candidates, "--output")
	assert.Contains(t, candidates, "-o")
	assert.Contains(t, candidates, "--verbose")
	assert.Contains(t, candidates, "-v")
}

func TestCompletionEnumValues(t *testing.T) {
	cmd := NewCmd("test").EnableCompletion()
	NewString("format").SetShort("f").
		SetEnumConstraint([]string{"json", "yaml", "toml"}).
		SetFlagOnly(true).
		Register(cmd)

	output, _ := parseCompletion(cmd, []string{"__complete", "--format", ""})
	candidates, directive := parseCompletionLines(output)

	assert.Contains(t, candidates, "json")
	assert.Contains(t, candidates, "yaml")
	assert.Contains(t, candidates, "toml")
	assert.Equal(t, ":4", directive)
}

func TestCompletionEnumValuesPrefix(t *testing.T) {
	cmd := NewCmd("test").EnableCompletion()
	NewString("format").SetShort("f").
		SetEnumConstraint([]string{"json", "yaml", "toml"}).
		SetFlagOnly(true).
		Register(cmd)

	output, _ := parseCompletion(cmd, []string{"__complete", "--format", "j"})
	candidates, _ := parseCompletionLines(output)

	assert.Contains(t, candidates, "json")
	assert.NotContains(t, candidates, "yaml")
	assert.NotContains(t, candidates, "toml")
}

func TestCompletionCustomFunc(t *testing.T) {
	cmd := NewCmd("test").EnableCompletion()
	NewString("branch").SetShort("b").
		SetFlagOnly(true).
		SetCompletionFunc(func(toComplete string) ([]string, CompletionDirective) {
			branches := []string{"main", "develop", "feature/foo"}
			var result []string
			for _, b := range branches {
				if strings.HasPrefix(b, toComplete) {
					result = append(result, b)
				}
			}
			return result, CompletionDirectiveNoFileComp
		}).
		Register(cmd)

	output, _ := parseCompletion(cmd, []string{"__complete", "--branch", "f"})
	candidates, _ := parseCompletionLines(output)

	assert.Contains(t, candidates, "feature/foo")
	assert.NotContains(t, candidates, "main")
	assert.NotContains(t, candidates, "develop")
}

func TestCompletionFuncPriorityOverEnum(t *testing.T) {
	cmd := NewCmd("test").EnableCompletion()
	NewString("format").
		SetEnumConstraint([]string{"json", "yaml"}).
		SetCompletionFunc(func(toComplete string) ([]string, CompletionDirective) {
			return []string{"custom1", "custom2"}, CompletionDirectiveNoFileComp
		}).
		SetFlagOnly(true).
		Register(cmd)

	output, _ := parseCompletion(cmd, []string{"__complete", "--format", ""})
	candidates, _ := parseCompletionLines(output)

	// CompletionFunc should win over EnumConstraint
	assert.Contains(t, candidates, "custom1")
	assert.Contains(t, candidates, "custom2")
	assert.NotContains(t, candidates, "json")
	assert.NotContains(t, candidates, "yaml")
}

func TestCompletionHiddenFlagsExcluded(t *testing.T) {
	cmd := NewCmd("test").EnableCompletion()
	NewString("visible").SetOptional(true).Register(cmd)
	NewString("hidden-flag").SetHidden(true).SetOptional(true).Register(cmd)

	output, _ := parseCompletion(cmd, []string{"__complete", "--"})
	candidates, _ := parseCompletionLines(output)

	assert.Contains(t, candidates, "--visible")
	assert.NotContains(t, candidates, "--hidden-flag")
}

func TestCompletionAlreadyUsedFlagExcluded(t *testing.T) {
	cmd := NewCmd("test").EnableCompletion()
	NewString("output").SetOptional(true).SetFlagOnly(true).Register(cmd)
	NewBool("verbose").Register(cmd)

	output, _ := parseCompletion(cmd, []string{"__complete", "--output", "foo", "--"})
	candidates, _ := parseCompletionLines(output)

	assert.NotContains(t, candidates, "--output")
	assert.Contains(t, candidates, "--verbose")
}

func TestCompletionSliceFlagNotExcludedAfterUse(t *testing.T) {
	cmd := NewCmd("test").EnableCompletion()
	NewStringSlice("items").SetFlagOnly(true).SetOptional(true).Register(cmd)
	NewBool("verbose").Register(cmd)

	output, _ := parseCompletion(cmd, []string{"__complete", "--items", "foo", "--"})
	candidates, _ := parseCompletionLines(output)

	// Slice flags can be used multiple times
	assert.Contains(t, candidates, "--items")
	assert.Contains(t, candidates, "--verbose")
}

func TestCompletionBoolFlagNoValue(t *testing.T) {
	cmd := NewCmd("test").EnableCompletion()
	NewBool("verbose").SetShort("v").Register(cmd)
	NewString("output").SetOptional(true).SetFlagOnly(true).Register(cmd)

	// After a bool flag, should offer more flags (not try to complete a value)
	output, _ := parseCompletion(cmd, []string{"__complete", "--verbose", "--"})
	candidates, _ := parseCompletionLines(output)

	assert.Contains(t, candidates, "--output")
	assert.NotContains(t, candidates, "--verbose") // already used
}

func TestCompletionEqualsValueSyntax(t *testing.T) {
	cmd := NewCmd("test").EnableCompletion()
	NewString("format").
		SetEnumConstraint([]string{"json", "yaml", "toml"}).
		SetFlagOnly(true).
		Register(cmd)

	output, _ := parseCompletion(cmd, []string{"__complete", "--format=j"})
	candidates, _ := parseCompletionLines(output)

	assert.Contains(t, candidates, "--format=json")
	assert.NotContains(t, candidates, "--format=yaml")
}

func TestCompletionNestedSubcommand(t *testing.T) {
	cmd := NewCmd("test").EnableCompletion()
	addCmd := NewCmd("add")
	NewString("name").Register(addCmd)
	NewBool("force").SetShort("f").Register(addCmd)
	cmd.RegisterCmd(addCmd)

	// Complete flags within the subcommand
	output, _ := parseCompletion(cmd, []string{"__complete", "add", "--"})
	candidates, _ := parseCompletionLines(output)

	assert.Contains(t, candidates, "--force")
	assert.Contains(t, candidates, "--help")
}

func TestCompletionNestedSubcommandWithGlobals(t *testing.T) {
	cmd := NewCmd("test").EnableCompletion()
	NewBool("verbose").SetShort("v").Register(cmd, WithGlobal(true))

	addCmd := NewCmd("add")
	NewString("name").Register(addCmd)
	cmd.RegisterCmd(addCmd)

	// Global flags should be available in subcommand
	output, _ := parseCompletion(cmd, []string{"__complete", "add", "--"})
	candidates, _ := parseCompletionLines(output)

	assert.Contains(t, candidates, "--verbose")
	assert.Contains(t, candidates, "--help")
}

func TestCompletionPositionalOnlyExcluded(t *testing.T) {
	cmd := NewCmd("test").EnableCompletion()
	NewString("pos-only").SetPositionalOnly(true).Register(cmd)
	NewString("normal").SetOptional(true).Register(cmd)

	output, _ := parseCompletion(cmd, []string{"__complete", "--"})
	candidates, _ := parseCompletionLines(output)

	// Positional-only flags should not appear in flag completion
	assert.NotContains(t, candidates, "--pos-only")
	assert.Contains(t, candidates, "--normal")
}

func TestCompletionPositionalEnum(t *testing.T) {
	cmd := NewCmd("test").EnableCompletion()
	NewString("action").
		SetEnumConstraint([]string{"start", "stop", "restart"}).
		Register(cmd)

	output, _ := parseCompletion(cmd, []string{"__complete", "st"})
	candidates, _ := parseCompletionLines(output)

	assert.Contains(t, candidates, "start")
	assert.Contains(t, candidates, "stop")
	assert.NotContains(t, candidates, "restart")
}

func TestCompletionPositionalCompletionFunc(t *testing.T) {
	cmd := NewCmd("test").EnableCompletion()
	NewString("file").
		SetCompletionFunc(func(toComplete string) ([]string, CompletionDirective) {
			return []string{"readme.md", "main.go"}, CompletionDirectiveNoFileComp
		}).
		Register(cmd)

	output, _ := parseCompletion(cmd, []string{"__complete", ""})
	candidates, _ := parseCompletionLines(output)

	assert.Contains(t, candidates, "readme.md")
	assert.Contains(t, candidates, "main.go")
}

func TestCompletionDirectiveDefault(t *testing.T) {
	cmd := NewCmd("test").EnableCompletion()
	NewString("file").Register(cmd)

	// No enum, no CompletionFunc - should get file fallback directive
	output, _ := parseCompletion(cmd, []string{"__complete", "--file", ""})
	_, directive := parseCompletionLines(output)

	assert.Equal(t, ":0", directive) // Default = file fallback
}

func TestCompletionEmptyArgs(t *testing.T) {
	cmd := NewCmd("test").EnableCompletion()
	cmd.RegisterCmd(NewCmd("add"))
	cmd.RegisterCmd(NewCmd("remove"))

	output, _ := parseCompletion(cmd, []string{"__complete", ""})
	candidates, _ := parseCompletionLines(output)

	assert.Contains(t, candidates, "add")
	assert.Contains(t, candidates, "remove")
}

func TestCompletionGenBash(t *testing.T) {
	cmd := NewCmd("myapp").EnableCompletion()

	var buf bytes.Buffer
	err := cmd.GenBashCompletion(&buf)
	assert.NoError(t, err)

	script := buf.String()
	assert.Contains(t, script, "myapp")
	assert.Contains(t, script, "__complete")
	assert.Contains(t, script, "complete -o default")
}

func TestCompletionGenZsh(t *testing.T) {
	cmd := NewCmd("myapp").EnableCompletion()

	var buf bytes.Buffer
	err := cmd.GenZshCompletion(&buf)
	assert.NoError(t, err)

	script := buf.String()
	assert.Contains(t, script, "myapp")
	assert.Contains(t, script, "__complete")
	assert.Contains(t, script, "compdef")
}

func TestCompletionMultiplePositionals(t *testing.T) {
	cmd := NewCmd("test").EnableCompletion()
	NewString("action").
		SetEnumConstraint([]string{"start", "stop"}).
		Register(cmd)
	NewString("target").
		SetEnumConstraint([]string{"server", "worker"}).
		Register(cmd)

	// First positional - should get action completions
	output, _ := parseCompletion(cmd, []string{"__complete", ""})
	candidates, _ := parseCompletionLines(output)
	assert.Contains(t, candidates, "start")
	assert.Contains(t, candidates, "stop")
	assert.NotContains(t, candidates, "server")

	// Second positional (first already consumed) - should get target completions
	output2, _ := parseCompletion(cmd, []string{"__complete", "start", ""})
	candidates2, _ := parseCompletionLines(output2)
	assert.Contains(t, candidates2, "server")
	assert.Contains(t, candidates2, "worker")
	assert.NotContains(t, candidates2, "start")
}

func TestCompletionGlobalFlagBeforeSubcommand(t *testing.T) {
	cmd := NewCmd("test").EnableCompletion()
	NewBool("verbose").SetShort("v").Register(cmd, WithGlobal(true))

	addCmd := NewCmd("add")
	NewString("name").Register(addCmd)
	NewBool("force").SetShort("f").Register(addCmd)
	cmd.RegisterCmd(addCmd)

	// --verbose used before subcommand should descend into subcommand context
	// and exclude --verbose from completions (already used)
	output, _ := parseCompletion(cmd, []string{"__complete", "--verbose", "add", "--"})
	candidates, _ := parseCompletionLines(output)

	assert.NotContains(t, candidates, "--verbose")
	assert.Contains(t, candidates, "--help")
	assert.Contains(t, candidates, "--force") // subcommand's own flag
	assert.Contains(t, candidates, "--name")  // subcommand's own flag
}

func TestCompletionSubcommandsPlusPositionals(t *testing.T) {
	cmd := NewCmd("test").EnableCompletion()
	cmd.RegisterCmd(NewCmd("add"))
	cmd.RegisterCmd(NewCmd("remove"))
	// Positional with no completions - should still show subcommands,
	// not switch to file fallback
	NewString("file").SetOptional(true).Register(cmd)

	output, _ := parseCompletion(cmd, []string{"__complete", "a"})
	candidates, directive := parseCompletionLines(output)

	assert.Contains(t, candidates, "add")
	assert.NotContains(t, candidates, "remove")
	// Even though positional has no enum/func, since subcommands were
	// found, we should not fall back to file completion
	assert.Equal(t, ":4", directive) // NoFileComp because there are subcommand matches
}

func TestCompletionValueFlagBeforeSubcommand(t *testing.T) {
	cmd := NewCmd("test").EnableCompletion()
	NewString("output").SetShort("o").SetFlagOnly(true).SetOptional(true).Register(cmd, WithGlobal(true))

	addCmd := NewCmd("add")
	NewBool("force").SetShort("f").Register(addCmd)
	cmd.RegisterCmd(addCmd)

	// A value-taking flag before the subcommand should be consumed,
	// then the subcommand should be found and descended into
	output, _ := parseCompletion(cmd, []string{"__complete", "--output", "file.txt", "add", "--"})
	candidates, _ := parseCompletionLines(output)

	assert.Contains(t, candidates, "--force")
	assert.Contains(t, candidates, "--help")
	assert.NotContains(t, candidates, "--output") // already used
}

func TestCompletionDashDashNoFlags(t *testing.T) {
	cmd := NewCmd("test").EnableCompletion()
	NewString("file").Register(cmd)
	NewBool("verbose").SetShort("v").Register(cmd)

	// After --, flag-like input should NOT trigger flag completion
	output, _ := parseCompletion(cmd, []string{"__complete", "--", "--"})
	candidates, directive := parseCompletionLines(output)

	// Should NOT offer --verbose or any flags
	assert.NotContains(t, candidates, "--verbose")
	assert.NotContains(t, candidates, "--help")
	// Should fall back to file completion (positional with no enum/func)
	assert.Equal(t, ":0", directive)
	assert.Empty(t, candidates)
}

func TestCompletionDashDashCountsPositionals(t *testing.T) {
	cmd := NewCmd("test").EnableCompletion()
	NewString("action").
		SetEnumConstraint([]string{"start", "stop"}).
		Register(cmd)
	NewString("target").
		SetEnumConstraint([]string{"server", "worker"}).
		Register(cmd)

	// After -- with one positional consumed, should complete second positional
	output, _ := parseCompletion(cmd, []string{"__complete", "--", "start", ""})
	candidates, _ := parseCompletionLines(output)

	assert.Contains(t, candidates, "server")
	assert.Contains(t, candidates, "worker")
	assert.NotContains(t, candidates, "start")
}

func TestCompletionVariadicPositional(t *testing.T) {
	cmd := NewCmd("test").EnableCompletion()
	NewStringSlice("files").
		SetVariadic(true).
		SetCompletionFunc(func(toComplete string) ([]string, CompletionDirective) {
			return []string{"a.txt", "b.txt"}, CompletionDirectiveNoFileComp
		}).
		Register(cmd)

	// First value
	output1, _ := parseCompletion(cmd, []string{"__complete", ""})
	candidates1, directive1 := parseCompletionLines(output1)
	assert.Contains(t, candidates1, "a.txt")
	assert.Equal(t, ":4", directive1)

	// Second value - variadic should still be active
	output2, _ := parseCompletion(cmd, []string{"__complete", "file1.txt", ""})
	candidates2, directive2 := parseCompletionLines(output2)
	assert.Contains(t, candidates2, "a.txt")
	assert.Contains(t, candidates2, "b.txt")
	assert.Equal(t, ":4", directive2)

	// Third value - still active
	output3, _ := parseCompletion(cmd, []string{"__complete", "file1.txt", "file2.txt", ""})
	candidates3, _ := parseCompletionLines(output3)
	assert.Contains(t, candidates3, "a.txt")
}

func TestCompletionVariadicWithPrecedingPositional(t *testing.T) {
	cmd := NewCmd("test").EnableCompletion()
	NewString("action").
		SetEnumConstraint([]string{"start", "stop"}).
		Register(cmd)
	NewStringSlice("files").
		SetVariadic(true).
		SetCompletionFunc(func(toComplete string) ([]string, CompletionDirective) {
			return []string{"a.txt", "b.txt"}, CompletionDirectiveNoFileComp
		}).
		Register(cmd)

	// First positional - should complete the non-variadic "action"
	output1, _ := parseCompletion(cmd, []string{"__complete", ""})
	candidates1, _ := parseCompletionLines(output1)
	assert.Contains(t, candidates1, "start")
	assert.Contains(t, candidates1, "stop")
	assert.NotContains(t, candidates1, "a.txt")

	// Second positional - action consumed, now variadic files
	output2, _ := parseCompletion(cmd, []string{"__complete", "start", ""})
	candidates2, _ := parseCompletionLines(output2)
	assert.Contains(t, candidates2, "a.txt")
	assert.Contains(t, candidates2, "b.txt")
	assert.NotContains(t, candidates2, "start")

	// Third positional - variadic still active
	output3, _ := parseCompletion(cmd, []string{"__complete", "start", "file1.txt", ""})
	candidates3, _ := parseCompletionLines(output3)
	assert.Contains(t, candidates3, "a.txt")
}

func TestCompletionVariadicFileCompletion(t *testing.T) {
	cmd := NewCmd("test").EnableCompletion()
	NewStringSlice("files").SetVariadic(true).Register(cmd)

	// Variadic with no CompletionFunc - should allow file completion fallback
	// even after consuming values
	output, _ := parseCompletion(cmd, []string{"__complete", "file1.txt", ""})
	_, directive := parseCompletionLines(output)
	assert.Equal(t, ":0", directive) // Default = file fallback
}

func TestCompletionShortFlagEquals(t *testing.T) {
	cmd := NewCmd("test").EnableCompletion()
	NewString("format").SetShort("f").
		SetEnumConstraint([]string{"json", "yaml", "toml"}).
		SetFlagOnly(true).
		Register(cmd)

	output, _ := parseCompletion(cmd, []string{"__complete", "-f=j"})
	candidates, _ := parseCompletionLines(output)

	assert.Contains(t, candidates, "-f=json")
	assert.NotContains(t, candidates, "-f=yaml")
}

func TestCompletionDirectiveMergeSubcmdsAndCompletionFunc(t *testing.T) {
	cmd := NewCmd("test").EnableCompletion()
	cmd.RegisterCmd(NewCmd("add"))
	// Positional with CompletionFunc that returns Default directive
	NewString("file").SetOptional(true).
		SetCompletionFunc(func(toComplete string) ([]string, CompletionDirective) {
			return []string{"readme.md"}, CompletionDirectiveDefault
		}).
		Register(cmd)

	// When subcommands match, we keep NoFileComp even if CompletionFunc
	// returns Default, to avoid mixing file completion with subcommands
	output, _ := parseCompletion(cmd, []string{"__complete", ""})
	candidates, directive := parseCompletionLines(output)

	assert.Contains(t, candidates, "add")
	assert.Contains(t, candidates, "readme.md")
	assert.Equal(t, ":4", directive) // NoFileComp because subcommands exist
}

func TestCompletionDashDashBeforeSubcommand(t *testing.T) {
	cmd := NewCmd("test").EnableCompletion()
	addCmd := NewCmd("add")
	NewBool("force").Register(addCmd)
	cmd.RegisterCmd(addCmd)

	// After --, "add" should be treated as positional, not a subcommand
	output, _ := parseCompletion(cmd, []string{"__complete", "--", "add", "--"})
	candidates, _ := parseCompletionLines(output)

	// Should NOT have subcommand flags (didn't descend into "add")
	assert.NotContains(t, candidates, "--force")
}

func TestCompletionShortFlagValueBeforeSubcommand(t *testing.T) {
	cmd := NewCmd("test").EnableCompletion()
	NewString("config").SetShort("c").SetFlagOnly(true).SetOptional(true).Register(cmd, WithGlobal(true))

	addCmd := NewCmd("add")
	NewBool("force").SetShort("f").Register(addCmd)
	cmd.RegisterCmd(addCmd)

	// Short flag with value before subcommand
	output, _ := parseCompletion(cmd, []string{"__complete", "-c", "myconfig", "add", "--"})
	candidates, _ := parseCompletionLines(output)

	assert.Contains(t, candidates, "--force")
	assert.Contains(t, candidates, "--help")
	assert.NotContains(t, candidates, "--config")
}
