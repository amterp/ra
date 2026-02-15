package ra

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

// CompletionFunc is a function that provides custom completions for a flag or positional argument.
// It receives the prefix the user has typed so far and returns candidates plus a directive.
type CompletionFunc func(toComplete string) ([]string, CompletionDirective)

// CompletionDirective is a bitmask that tells the shell how to interpret completion results.
type CompletionDirective int

const (
	// CompletionDirectiveDefault indicates normal completion behavior with file completion fallback.
	CompletionDirectiveDefault CompletionDirective = 0
	// CompletionDirectiveError indicates an error occurred; results should be ignored.
	CompletionDirectiveError CompletionDirective = 1
	// CompletionDirectiveNoSpace tells the shell not to add a trailing space after the completion.
	CompletionDirectiveNoSpace CompletionDirective = 2
	// CompletionDirectiveNoFileComp tells the shell not to fall back to file completion.
	CompletionDirectiveNoFileComp CompletionDirective = 4
)

// CompletionInvokedErr is returned by ParseOrError when completion is invoked (via __complete).
var CompletionInvokedErr = errors.New("completion invoked")

type completionInvokedError struct{}

func (e *completionInvokedError) Error() string {
	return CompletionInvokedErr.Error()
}

func (e *completionInvokedError) Unwrap() error {
	return CompletionInvokedErr
}

// EnableCompletion enables shell completion support for this command.
// When enabled, the hidden __complete subcommand is recognized during parsing.
func (c *Cmd) EnableCompletion() *Cmd {
	c.completionEnabled = true
	return c
}

// handleCompletion processes a __complete invocation and writes candidates to stdout.
func (c *Cmd) handleCompletion(args []string) error {
	candidates, directive := c.computeCompletions(args)

	var sb strings.Builder
	for _, candidate := range candidates {
		sb.WriteString(candidate)
		sb.WriteByte('\n')
	}
	fmt.Fprintf(&sb, ":%d\n", int(directive))

	fmt.Fprint(stdoutWriter, sb.String())
	return &completionInvokedError{}
}

// computeCompletions determines completion candidates for the given args.
func (c *Cmd) computeCompletions(args []string) ([]string, CompletionDirective) {
	// Walk subcommand tree, skipping flags (and their values) to find
	// the active subcommand. This mirrors how the parser handles
	// interleaved flags and subcommands (e.g., "mycmd --verbose add").
	//
	// We scan ahead with index i but only advance "consumed" when we
	// actually descend into a subcommand or hit "--". This ensures flags
	// that DON'T lead to a subcommand remain in 'remaining' for the
	// preceding args scan to process (e.g., detecting prevNeedsValue).
	activeCmd := c
	usedFlags := make(map[string]bool)
	sawDashDash := false
	consumed := 0

	for i := 0; i < len(args)-1; {
		arg := args[i]

		if arg == "--" {
			sawDashDash = true
			consumed = i + 1
			break
		}

		if strings.HasPrefix(arg, "-") {
			scanFlags(activeCmd, arg, usedFlags)
			i += flagConsumedArgs(activeCmd, arg)
			continue
		}

		if subCmd, exists := activeCmd.subCmds[arg]; exists {
			activeCmd.applyGlobalFlags(subCmd)
			activeCmd = subCmd
			consumed = i + 1
			i++
			continue
		}

		// Non-flag, non-subcommand: positional arg, stop looking
		break
	}

	remaining := args[consumed:]

	// Ensure help flags are registered on the active command (they're
	// normally added lazily during parseWithPreserveState)
	if activeCmd.helpEnabled {
		if _, exists := activeCmd.flags["help"]; !exists {
			NewBool("help").SetShort("h").
				SetUsage("Print usage string.").
				SetOptional(true).
				Register(activeCmd, WithGlobal(true))
		}
	}

	// The last element is the word being completed (may be empty string)
	var toComplete string
	var precedingArgs []string
	if len(remaining) > 0 {
		toComplete = remaining[len(remaining)-1]
		precedingArgs = remaining[:len(remaining)-1]
	}

	// Scan remaining preceding args for flag usage and positional count
	prevNeedsValue := false
	prevFlagName := ""
	positionalCount := 0

	for i := 0; i < len(precedingArgs); i++ {
		arg := precedingArgs[i]

		if arg == "--" {
			sawDashDash = true
			// All remaining args after -- are positional
			positionalCount += len(precedingArgs) - i - 1
			break
		}

		if strings.HasPrefix(arg, "--") {
			flagPart := arg[2:]
			if idx := strings.Index(flagPart, "="); idx != -1 {
				usedFlags[flagPart[:idx]] = true
				continue
			}

			flag, exists := activeCmd.flags[flagPart]
			if !exists {
				continue
			}
			usedFlags[flagPart] = true

			if !isBoolFlag(flag) {
				if i+1 < len(precedingArgs) {
					i++ // skip the value
				} else {
					prevNeedsValue = true
					prevFlagName = flagPart
				}
			}
		} else if strings.HasPrefix(arg, "-") && arg != "-" {
			shorts := arg[1:]
			if idx := strings.Index(shorts, "="); idx != -1 {
				// Mark all short flags in the cluster before the =
				for _, ch := range shorts[:idx] {
					if flagName, exists := activeCmd.shortToName[string(ch)]; exists {
						usedFlags[flagName] = true
					}
				}
				continue
			}

			for j, ch := range shorts {
				flagName, exists := activeCmd.shortToName[string(ch)]
				if !exists {
					continue
				}
				flag := activeCmd.flags[flagName]
				usedFlags[flagName] = true

				if !isBoolFlag(flag) && j == len(shorts)-1 {
					if i+1 < len(precedingArgs) {
						i++
					} else {
						prevNeedsValue = true
						prevFlagName = flagName
					}
				}
			}
		} else {
			positionalCount++
		}
	}

	// Determine what to complete

	// After --, everything is positional (no flags, no subcommands)
	if sawDashDash {
		return activeCmd.completeSubcommandsAndPositionals(toComplete, positionalCount, false)
	}

	// Case 1: Previous arg was a value-taking flag waiting for its value
	if prevNeedsValue {
		return activeCmd.completeFlagValue(prevFlagName, toComplete)
	}

	// Case 2: --flag=prefix syntax
	if strings.HasPrefix(toComplete, "--") && strings.Contains(toComplete, "=") {
		eqIdx := strings.Index(toComplete, "=")
		flagName := toComplete[2:eqIdx]
		valuePrefix := toComplete[eqIdx+1:]

		candidates, directive := activeCmd.completeFlagValue(flagName, valuePrefix)
		prefix := toComplete[:eqIdx+1]
		for i, c := range candidates {
			candidates[i] = prefix + c
		}
		return candidates, directive
	}

	// Case 3: -f=prefix syntax (short flag with equals)
	if strings.HasPrefix(toComplete, "-") && !strings.HasPrefix(toComplete, "--") &&
		strings.Contains(toComplete, "=") {
		eqIdx := strings.Index(toComplete, "=")
		shortPart := toComplete[1:eqIdx]
		valuePrefix := toComplete[eqIdx+1:]

		if len(shortPart) > 0 {
			lastChar := string(shortPart[len(shortPart)-1])
			if flagName, exists := activeCmd.shortToName[lastChar]; exists {
				candidates, directive := activeCmd.completeFlagValue(flagName, valuePrefix)
				prefix := toComplete[:eqIdx+1]
				for i, c := range candidates {
					candidates[i] = prefix + c
				}
				return candidates, directive
			}
		}
		return nil, CompletionDirectiveDefault
	}

	// Case 4: Completing a long flag name
	if strings.HasPrefix(toComplete, "--") {
		prefix := toComplete[2:]
		return activeCmd.completeFlagNames(prefix, usedFlags)
	}

	// Case 5: Completing a short or long flag name (single dash)
	if toComplete == "-" || (strings.HasPrefix(toComplete, "-") && !strings.HasPrefix(toComplete, "--")) {
		return activeCmd.completeShortAndLongFlags(toComplete, usedFlags)
	}

	// Case 6: Empty or non-dash - offer subcommands + positional completions
	return activeCmd.completeSubcommandsAndPositionals(toComplete, positionalCount, true)
}

// scanFlags marks a single flag arg as used in the usedFlags map.
func scanFlags(cmd *Cmd, arg string, usedFlags map[string]bool) {
	if strings.HasPrefix(arg, "--") {
		flagPart := arg[2:]
		if idx := strings.Index(flagPart, "="); idx != -1 {
			usedFlags[flagPart[:idx]] = true
			return
		}
		if _, exists := cmd.flags[flagPart]; exists {
			usedFlags[flagPart] = true
		}
	} else if strings.HasPrefix(arg, "-") && arg != "-" {
		shorts := arg[1:]
		if idx := strings.Index(shorts, "="); idx != -1 {
			for _, ch := range shorts[:idx] {
				if flagName, exists := cmd.shortToName[string(ch)]; exists {
					usedFlags[flagName] = true
				}
			}
			return
		}
		for _, ch := range shorts {
			if flagName, exists := cmd.shortToName[string(ch)]; exists {
				usedFlags[flagName] = true
			}
		}
	}
}

// completeFlagValue completes the value for a specific flag.
func (c *Cmd) completeFlagValue(flagName string, toComplete string) ([]string, CompletionDirective) {
	flag, exists := c.flags[flagName]
	if !exists {
		return nil, CompletionDirectiveDefault
	}

	base := getBaseFlag(flag)
	if base == nil {
		return nil, CompletionDirectiveDefault
	}

	// Priority 1: CompletionFunc
	if base.CompletionFunc != nil {
		return base.CompletionFunc(toComplete)
	}

	// Priority 2: EnumConstraint (StringFlag only)
	if sf, ok := flag.(*StringFlag); ok && sf.EnumConstraint != nil {
		var candidates []string
		for _, val := range *sf.EnumConstraint {
			if strings.HasPrefix(val, toComplete) {
				candidates = append(candidates, val)
			}
		}
		return candidates, CompletionDirectiveNoFileComp
	}

	// Priority 3: File completion fallback
	return nil, CompletionDirectiveDefault
}

// completeFlagNames completes long flag names with the given prefix.
func (c *Cmd) completeFlagNames(prefix string, usedFlags map[string]bool) ([]string, CompletionDirective) {
	var candidates []string

	for name, flag := range c.flags {
		base := getBaseFlag(flag)
		if base == nil {
			continue
		}

		// Skip hidden flags
		if base.Hidden {
			continue
		}

		// Skip positional-only flags
		if base.PositionalOnly {
			continue
		}

		// Skip already-used non-slice flags
		if usedFlags[name] && !isSliceFlag(flag) {
			continue
		}

		if strings.HasPrefix(name, prefix) {
			candidates = append(candidates, "--"+name)
		}
	}

	sort.Strings(candidates)
	return candidates, CompletionDirectiveNoFileComp
}

// completeShortAndLongFlags offers both short and long flag completions.
func (c *Cmd) completeShortAndLongFlags(toComplete string, usedFlags map[string]bool) ([]string, CompletionDirective) {
	var candidates []string

	for name, flag := range c.flags {
		base := getBaseFlag(flag)
		if base == nil {
			continue
		}

		if base.Hidden {
			continue
		}

		if base.PositionalOnly {
			continue
		}

		if usedFlags[name] && !isSliceFlag(flag) {
			continue
		}

		// Add long form
		if strings.HasPrefix("--"+name, toComplete) {
			candidates = append(candidates, "--"+name)
		}

		// Add short form
		if base.Short != "" {
			short := "-" + base.Short
			if strings.HasPrefix(short, toComplete) {
				candidates = append(candidates, short)
			}
		}
	}

	sort.Strings(candidates)
	return candidates, CompletionDirectiveNoFileComp
}

// completeSubcommandsAndPositionals offers subcommand names and positional arg completions.
// positionalCount is how many positional args have already been consumed.
// includeSubcmds controls whether subcommand names are offered (false after --).
func (c *Cmd) completeSubcommandsAndPositionals(
	toComplete string,
	positionalCount int,
	includeSubcmds bool,
) ([]string, CompletionDirective) {
	var candidates []string
	directive := CompletionDirectiveNoFileComp

	// Add matching subcommand names
	if includeSubcmds {
		for name := range c.subCmds {
			if strings.HasPrefix(name, toComplete) {
				candidates = append(candidates, name)
			}
		}
	}
	subCmdCount := len(candidates)

	// Find the positional arg at the current index (skipping already-consumed ones).
	// Variadic positionals absorb all remaining positions, so once we reach one
	// it's always the active positional regardless of how many values were consumed.
	skipped := 0
	for _, name := range c.positional {
		flag := c.flags[name]
		base := getBaseFlag(flag)
		if base == nil || base.FlagOnly {
			continue
		}

		if !isVariadicFlag(flag) && skipped < positionalCount {
			skipped++
			continue
		}

		// This is the positional we're completing
		if base.CompletionFunc != nil {
			vals, dir := base.CompletionFunc(toComplete)
			candidates = append(candidates, vals...)
			// Only use the CompletionFunc's directive when there are no
			// subcommand candidates, to avoid downgrading from NoFileComp
			if subCmdCount == 0 {
				directive = dir
			}
		} else if sf, ok := flag.(*StringFlag); ok && sf.EnumConstraint != nil {
			for _, val := range *sf.EnumConstraint {
				if strings.HasPrefix(val, toComplete) {
					candidates = append(candidates, val)
				}
			}
		} else {
			// No custom completion - use file fallback only if there
			// are no other candidates already collected.
			if len(candidates) == 0 {
				directive = CompletionDirectiveDefault
			}
		}
		break
	}

	sort.Strings(candidates)
	return candidates, directive
}

// GenBashCompletion writes the bash completion script for this command to the given writer.
func (c *Cmd) GenBashCompletion(w io.Writer) error {
	_, err := fmt.Fprintf(w, bashCompletionTemplate, c.name, c.name, c.name, c.name, c.name)
	return err
}

// GenZshCompletion writes the zsh completion script for this command to the given writer.
func (c *Cmd) GenZshCompletion(w io.Writer) error {
	_, err := fmt.Fprintf(w, zshCompletionTemplate, c.name, c.name, c.name, c.name, c.name)
	return err
}

func isSliceFlag(flag any) bool {
	switch flag.(type) {
	case *StringSliceFlag, *IntSliceFlag, *Int64SliceFlag, *Float64SliceFlag, *BoolSliceFlag:
		return true
	}
	return false
}

func isVariadicFlag(flag any) bool {
	switch f := flag.(type) {
	case *StringSliceFlag:
		return f.Variadic
	case *IntSliceFlag:
		return f.Variadic
	case *Int64SliceFlag:
		return f.Variadic
	case *Float64SliceFlag:
		return f.Variadic
	case *BoolSliceFlag:
		return f.Variadic
	}
	return false
}

// flagConsumedArgs returns how many args a flag token consumes (including itself).
// Returns 1 for bool flags, flags with inline values (=), and unknown flags.
// Returns 2 for known value-taking flags without inline values.
func flagConsumedArgs(cmd *Cmd, arg string) int {
	if strings.HasPrefix(arg, "--") {
		flagPart := arg[2:]
		if strings.Contains(flagPart, "=") {
			return 1
		}
		flag, exists := cmd.flags[flagPart]
		if !exists {
			return 1
		}
		if isBoolFlag(flag) {
			return 1
		}
		return 2
	}
	if strings.HasPrefix(arg, "-") && arg != "-" {
		shorts := arg[1:]
		if strings.Contains(shorts, "=") {
			return 1
		}
		if len(shorts) == 0 {
			return 1
		}
		// Last char in cluster determines if a value is needed
		lastChar := string(shorts[len(shorts)-1])
		flagName, exists := cmd.shortToName[lastChar]
		if !exists {
			return 1
		}
		flag, exists := cmd.flags[flagName]
		if !exists {
			return 1
		}
		if isBoolFlag(flag) {
			return 1
		}
		return 2
	}
	return 1
}
