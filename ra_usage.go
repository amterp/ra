package ra

import (
	"fmt"
	"sort"
	"strings"

	"github.com/amterp/color"
)

var (
	greenBold  = color.New(color.FgGreen, color.Bold)
	cyan       = color.New(color.FgCyan)
	bold       = color.New(color.Bold)
	GreenBoldS = greenBold.SprintfFunc()
	CyanS      = cyan.SprintfFunc()
	BoldS      = bold.SprintfFunc()
)

func (c *Cmd) GenerateUsage(isLongHelp bool) string {
	return c.generateUsage(isLongHelp)
}

func (c *Cmd) GenerateShortUsage() string {
	return c.generateUsage(false)
}

func (c *Cmd) GenerateLongUsage() string {
	return c.generateUsage(true)
}

func (c *Cmd) GenerateDescription() string {
	return c.generateDescription()
}

func (c *Cmd) GenerateSynopsis(isLongHelp bool) string {
	return c.generateSynopsis(isLongHelp)
}

func (c *Cmd) GenerateShortSynopsis() string {
	return c.generateSynopsis(false)
}

func (c *Cmd) GenerateLongSynopsis() string {
	return c.generateSynopsis(true)
}

func (c *Cmd) GenerateCommandsSection(isLongHelp bool) string {
	return c.generateCommandsSection(isLongHelp)
}

func (c *Cmd) GenerateArgumentsSection(isLongHelp bool) string {
	return c.generateArgumentsSection(isLongHelp)
}

func (c *Cmd) GenerateGlobalOptionsSection(isLongHelp bool) string {
	return c.generateGlobalOptionsSection(isLongHelp)
}

func (c *Cmd) GenerateShortCommandsSection() string {
	return c.generateCommandsSection(false)
}

func (c *Cmd) GenerateLongCommandsSection() string {
	return c.generateCommandsSection(true)
}

func (c *Cmd) GenerateShortArgumentsSection() string {
	return c.generateArgumentsSection(false)
}

func (c *Cmd) GenerateLongArgumentsSection() string {
	return c.generateArgumentsSection(true)
}

func (c *Cmd) GenerateShortGlobalOptionsSection() string {
	return c.generateGlobalOptionsSection(false)
}

func (c *Cmd) GenerateLongGlobalOptionsSection() string {
	return c.generateGlobalOptionsSection(true)
}

func (c *Cmd) generateDescription() string {
	if c.description == "" {
		return ""
	}
	return c.description + "\n\n"
}

func (c *Cmd) generateCommandsSection(isLongHelp bool) string {
	if len(c.subCmds) == 0 {
		return ""
	}

	var sb strings.Builder
	headers := c.getUsageHeaders()

	sb.WriteString("\n" + GreenBoldS(headers.Commands) + "\n")
	// Sort subcommand names for consistent output
	var subCmdNames []string
	for name := range c.subCmds {
		subCmdNames = append(subCmdNames, name)
	}
	sort.Strings(subCmdNames)
	for _, name := range subCmdNames {
		subCmd := c.subCmds[name]
		if subCmd.description != "" {
			sb.WriteString(fmt.Sprintf("  %-30s%s\n", name, subCmd.description))
		} else {
			sb.WriteString(fmt.Sprintf("  %s\n", name))
		}
	}

	return sb.String()
}

func (c *Cmd) separateScriptAndGlobalFlags() (scriptFlags, globalFlags []any) {
	// Use a map to keep track of added flags to avoid duplicates
	addedFlags := make(map[string]bool)

	// First, handle all script flags (positional and non-positional)
	// Process positional flags in registration order
	for _, name := range c.positional {
		if addedFlags[name] {
			continue
		}
		flag := c.flags[name]

		// Check if this is originally a global flag
		isOriginallyGlobal := false
		for _, gName := range c.globalFlags {
			if name == gName {
				isOriginallyGlobal = true
				break
			}
		}

		// Check if this flag represents a non-global override of a global flag
		isNonGlobalOverride := false
		if _, wasOverridden := c.overriddenGlobalFlags[name]; wasOverridden {
			// Check what type of override this is
			if _, hadNameShadowed := c.shadowedNameFlags[name]; hadNameShadowed {
				// Name shadowing case - the non-global flag took over the name
				// The flag stored under this name is the non-global flag
				isNonGlobalOverride = true
			} else if _, hadShortShadowed := c.shadowedShortFlags[name]; hadShortShadowed {
				// Short shadowing case - the global flag lost its short but kept its name
				// The flag stored under this name is still the global flag (modified)
				isNonGlobalOverride = false
			} else {
				// Complete override case - both name and short conflicted
				// The flag stored under this name is the non-global flag
				isNonGlobalOverride = true
			}
		}

		// Add to scriptFlags if:
		// 1. Not originally global, OR
		// 2. It's a non-global flag that overrode a global flag
		if !isOriginallyGlobal || isNonGlobalOverride {
			scriptFlags = append(scriptFlags, flag)
		}
		addedFlags[name] = true
	}

	// Process non-positional flags in registration order
	for _, name := range c.nonPositional {
		if addedFlags[name] {
			continue
		}
		flag := c.flags[name]

		// Check if this is originally a global flag
		isOriginallyGlobal := false
		for _, gName := range c.globalFlags {
			if name == gName {
				isOriginallyGlobal = true
				break
			}
		}

		// Check if this flag represents a non-global override of a global flag
		isNonGlobalOverride := false
		if _, wasOverridden := c.overriddenGlobalFlags[name]; wasOverridden {
			// Check what type of override this is
			if _, hadNameShadowed := c.shadowedNameFlags[name]; hadNameShadowed {
				// Name shadowing case - the non-global flag took over the name
				// The flag stored under this name is the non-global flag
				isNonGlobalOverride = true
			} else if _, hadShortShadowed := c.shadowedShortFlags[name]; hadShortShadowed {
				// Short shadowing case - the global flag lost its short but kept its name
				// The flag stored under this name is still the global flag (modified)
				isNonGlobalOverride = false
			} else {
				// Complete override case - both name and short conflicted
				// The flag stored under this name is the non-global flag
				isNonGlobalOverride = true
			}
		}

		// Add to scriptFlags if:
		// 1. Not originally global, OR
		// 2. It's a non-global flag that overrode a global flag
		if !isOriginallyGlobal || isNonGlobalOverride {
			scriptFlags = append(scriptFlags, flag)
		}
		addedFlags[name] = true
	}

	// Process global flags in their registration order to preserve ordering
	for _, globalFlagName := range c.globalFlags {
		// Check if this global flag was name-shadowed
		if _, hadNameShadowed := c.shadowedNameFlags[globalFlagName]; hadNameShadowed {
			// Add name-shadowed global flag (appears with only its short form)
			if originalFlag, exists := c.overriddenGlobalFlags[globalFlagName]; exists {
				if base := getBaseFlag(originalFlag); base != nil && base.Short != "" {
					// Look up the flag stored under its short key
					if shortFlag, shortExists := c.flags[base.Short]; shortExists {
						// Create a copy without the name for display
						flagCopy := deepCopyFlag(shortFlag)
						if baseCopy := getBaseFlag(flagCopy); baseCopy != nil {
							baseCopy.Name = "" // Remove name for display, keep only short
						}
						globalFlags = append(globalFlags, flagCopy)
					}
				}
			}
		} else {
			// Check if this global flag was completely overridden (both name and short conflicted)
			if _, wasOverridden := c.overriddenGlobalFlags[globalFlagName]; wasOverridden {
				// Check what type of override this was
				if _, hadShortShadowed := c.shadowedShortFlags[globalFlagName]; hadShortShadowed {
					// Short shadowing case - global flag lost its short but should still appear in Global options
					if flag, exists := c.flags[globalFlagName]; exists {
						globalFlags = append(globalFlags, flag)
					}
				} else {
					// Complete override case - global flag was completely replaced, don't show it
					// (the non-global override will appear in Arguments section)
					continue
				}
			} else {
				// Regular global flag - add if it exists
				if flag, exists := c.flags[globalFlagName]; exists {
					globalFlags = append(globalFlags, flag)
				}
			}
		}
	}

	return scriptFlags, globalFlags
}

func (c *Cmd) generateArgumentsSection(isLongHelp bool) string {
	scriptFlags, _ := c.separateScriptAndGlobalFlags()

	if len(scriptFlags) == 0 || !c.hasVisibleFlags(scriptFlags, isLongHelp) {
		return ""
	}

	var sb strings.Builder
	headers := c.getUsageHeaders()

	sb.WriteString("\n" + GreenBoldS(headers.Arguments) + "\n")
	sb.WriteString(c.formatFlags(scriptFlags, isLongHelp))

	return sb.String()
}

func (c *Cmd) generateGlobalOptionsSection(isLongHelp bool) string {
	_, globalFlags := c.separateScriptAndGlobalFlags()

	if len(globalFlags) == 0 || !c.hasVisibleFlags(globalFlags, isLongHelp) {
		return ""
	}

	var sb strings.Builder
	headers := c.getUsageHeaders()

	sb.WriteString("\n" + GreenBoldS(headers.GlobalOptions) + "\n")
	sb.WriteString(c.formatFlags(globalFlags, isLongHelp))

	return sb.String()
}

func (c *Cmd) generateUsage(isLongHelp bool) string {
	var sb strings.Builder
	headers := c.getUsageHeaders()

	description := c.generateDescription()
	if description != "" {
		sb.WriteString(description)
	}

	sb.WriteString(GreenBoldS(headers.Usage) + "\n  ")
	sb.WriteString(c.generateSynopsis(isLongHelp))
	sb.WriteString("\n")

	commandsSection := c.generateCommandsSection(isLongHelp)
	if commandsSection != "" {
		sb.WriteString(commandsSection)
	}

	argumentsSection := c.generateArgumentsSection(isLongHelp)
	if argumentsSection != "" {
		sb.WriteString(argumentsSection)
	}

	globalOptionsSection := c.generateGlobalOptionsSection(isLongHelp)
	if globalOptionsSection != "" {
		sb.WriteString(globalOptionsSection)
	}

	return sb.String()
}

func (c *Cmd) generateSynopsis(isLongHelp bool) string {
	var sb strings.Builder
	sb.WriteString(BoldS(c.name))

	if len(c.subCmds) > 0 {
		sb.WriteString(" " + CyanS("[subcommand]"))
		// Still show parent command flags in synopsis even when subcommands exist
		for _, name := range c.positional {
			flag := c.flags[name]
			base := getBaseFlag(flag)
			if base.Hidden {
				continue
			}
			if !isLongHelp && base.HiddenInShortHelp {
				continue
			}

			// Show positional-only flags or non-bool flags in synopsis (bools never appear)
			if base.PositionalOnly || !isBoolFlag(flag) {
				var argName string

				// Check if it's a variadic slice
				isVariadic := false
				switch f := flag.(type) {
				case *StringSliceFlag:
					isVariadic = f.Variadic
				case *IntSliceFlag:
					isVariadic = f.Variadic
				case *Int64SliceFlag:
					isVariadic = f.Variadic
				case *Float64SliceFlag:
					isVariadic = f.Variadic
				case *BoolSliceFlag:
					isVariadic = f.Variadic
				}

				if isVariadic {
					argName = name + "..."
				} else {
					argName = name
				}

				// Determine if flag should show as required or optional in synopsis
				shouldBeOptional := c.shouldFlagBeOptionalInSynopsis(flag)

				if shouldBeOptional {
					sb.WriteString(" " + CyanS("[%s]", argName))
				} else {
					sb.WriteString(" " + CyanS("<%s>", argName))
				}
			}
		}
		sb.WriteString(" " + CyanS("[OPTIONS]"))
		return sb.String()
	}

	// First pass: collect positional-only flags
	var positionalOnlyFlags []string
	var nonPositionalFlags []string

	for _, name := range c.positional {
		flag := c.flags[name]
		base := getBaseFlag(flag)
		if base.Hidden {
			continue
		}
		if !isLongHelp && base.HiddenInShortHelp {
			continue
		}

		if isBoolFlag(flag) {
			continue // Bools never appear in synopsis
		}

		if base.PositionalOnly {
			positionalOnlyFlags = append(positionalOnlyFlags, name)
		} else {
			// If a flag is in the positional list, it should appear in synopsis
			// regardless of whether it's optional or not
			nonPositionalFlags = append(nonPositionalFlags, name)
		}
	}

	// Add non-positional flags from nonPositional list (they might not be in positional)
	for _, name := range c.nonPositional {
		flag := c.flags[name]
		base := getBaseFlag(flag)
		if base.Hidden {
			continue
		}
		if !isLongHelp && base.HiddenInShortHelp {
			continue
		}

		// Skip global flags - they don't appear in synopsis
		isGlobal := false
		for _, gName := range c.globalFlags {
			if name == gName {
				isGlobal = true
				break
			}
		}
		if isGlobal {
			continue
		}

		if isBoolFlag(flag) || base.Optional {
			continue // Bools and optional flags never appear in synopsis
		}

		// Check if already added
		found := false
		for _, existing := range nonPositionalFlags {
			if existing == name {
				found = true
				break
			}
		}
		if !found {
			nonPositionalFlags = append(nonPositionalFlags, name)
		}
	}

	// Process positional-only flags first, but stop after first variadic
	for _, name := range positionalOnlyFlags {
		flag := c.flags[name]
		shouldBeOptional := c.shouldFlagBeOptionalInSynopsis(flag)

		// Check if it's a variadic positional flag
		argName := name
		isVariadic := false
		switch f := flag.(type) {
		case *StringSliceFlag:
			if f.Variadic {
				argName = name + "..."
				isVariadic = true
			}
		case *IntSliceFlag:
			if f.Variadic {
				argName = name + "..."
				isVariadic = true
			}
		case *Int64SliceFlag:
			if f.Variadic {
				argName = name + "..."
				isVariadic = true
			}
		case *Float64SliceFlag:
			if f.Variadic {
				argName = name + "..."
				isVariadic = true
			}
		case *BoolSliceFlag:
			if f.Variadic {
				argName = name + "..."
				isVariadic = true
			}
		}

		if shouldBeOptional {
			sb.WriteString(" " + CyanS("[%s]", argName))
		} else {
			sb.WriteString(" " + CyanS("<%s>", argName))
		}

		// Stop after first variadic positional flag
		if isVariadic {
			sb.WriteString(" " + CyanS("[OPTIONS]"))
			return sb.String()
		}
	}

	// Then process non-positional flags, but stop after first variadic
	for _, name := range nonPositionalFlags {
		flag := c.flags[name]

		// Check if it's variadic
		isVariadic := false
		switch f := flag.(type) {
		case *StringSliceFlag:
			isVariadic = f.Variadic
		case *IntSliceFlag:
			isVariadic = f.Variadic
		case *Int64SliceFlag:
			isVariadic = f.Variadic
		case *Float64SliceFlag:
			isVariadic = f.Variadic
		case *BoolSliceFlag:
			isVariadic = f.Variadic
		}

		if isVariadic {
			// All variadic flags show as [name...]
			sb.WriteString(" " + CyanS("[%s...]", name))
			// Stop after first variadic flag
			sb.WriteString(" " + CyanS("[OPTIONS]"))
			return sb.String()
		} else {
			// Non-variadic required flags show as <name>
			shouldBeOptional := c.shouldFlagBeOptionalInSynopsis(flag)
			if shouldBeOptional {
				sb.WriteString(" " + CyanS("[%s]", name))
			} else {
				sb.WriteString(" " + CyanS("<%s>", name))
			}
		}
	}

	sb.WriteString(" " + CyanS("[OPTIONS]"))
	return sb.String()
}

func (c *Cmd) hasVisibleFlags(flags []any, isLongHelp bool) bool {
	for _, flag := range flags {
		base := getBaseFlag(flag)
		if base.Hidden {
			continue
		}
		if !isLongHelp && base.HiddenInShortHelp {
			continue
		}
		// Found at least one visible flag
		return true
	}
	return false
}

func (c *Cmd) formatFlags(flags []any, isLongHelp bool) string {
	// Use flags in the order they were passed (already correctly ordered by generateUsage)
	allFlags := flags

	// First pass: calculate maximum width for alignment
	maxWidth := 0
	var flagParts []string

	for _, flag := range allFlags {
		base := getBaseFlag(flag)
		if base.Hidden {
			flagParts = append(flagParts, "")
			continue
		}
		if !isLongHelp && base.HiddenInShortHelp {
			flagParts = append(flagParts, "")
			continue
		}

		var flagPart string
		if base.PositionalOnly {
			// Positional-only flags show without dashes
			flagPart = fmt.Sprintf("  %s", base.Name)
		} else if base.Name == "" && base.Short != "" {
			// Name-shadowed flag - show only short form
			flagPart = fmt.Sprintf("  -%s", base.Short)
		} else if base.Short != "" && base.Name != "" {
			// Normal flag with both short and name
			flagPart = fmt.Sprintf("  -%s, --%s", base.Short, base.Name)
		} else if base.Name != "" {
			// Flag with only name (no short)
			flagPart = fmt.Sprintf("      --%s", base.Name)
		} else {
			// Shouldn't happen - flag with neither name nor short
			flagPart = fmt.Sprintf("  (unnamed flag)")
		}

		typeStr := getFlagType(flag)
		if typeStr != "bool" {
			flagPart = fmt.Sprintf("%s %s", flagPart, typeStr)
		}

		flagParts = append(flagParts, flagPart)
		if len(flagPart) > maxWidth {
			maxWidth = len(flagPart)
		}
	}

	// Use dynamic alignment: longest left side + 3 spaces
	maxWidth = maxWidth + 3

	// Second pass: generate aligned output
	var sb strings.Builder
	for i, flag := range allFlags {
		flagPart := flagParts[i]
		if flagPart == "" {
			continue // hidden flag
		}

		base := getBaseFlag(flag)
		sb.WriteString(flagPart)

		// Check if we have usage text or constraints to display
		hasUsage := base.Usage != ""
		constraints := c.getConstraintString(flag)
		hasConstraints := constraints != ""

		if hasUsage || hasConstraints {
			// Calculate padding to align descriptions
			padding := maxWidth - len(flagPart)
			if padding < 1 {
				padding = 1
			}
			sb.WriteString(strings.Repeat(" ", padding))

			// Add optional marker for flags that should be optional
			// but not for variadic flags (their type already indicates optionality)
			isVariadic := false
			switch f := flag.(type) {
			case *StringSliceFlag:
				isVariadic = f.Variadic
			case *IntSliceFlag:
				isVariadic = f.Variadic
			case *Int64SliceFlag:
				isVariadic = f.Variadic
			case *Float64SliceFlag:
				isVariadic = f.Variadic
			case *BoolSliceFlag:
				isVariadic = f.Variadic
			}

			// Show status markers for non-variadic flags:
			// For positional-only flags: show (optional) if explicitly optional, otherwise no marker
			// For other flags: (optional) if explicitly optional AND no default, (required) if required AND no default
			hasDefault := c.flagHasDefault(flag)
			var shouldShowOptional bool
			if base.PositionalOnly {
				shouldShowOptional = base.Optional
			} else {
				shouldShowOptional = base.Optional && !hasDefault
			}

			if shouldShowOptional && !isVariadic {
				sb.WriteString("(optional) ")
			}

			// Add usage text if it exists
			if hasUsage {
				usage := base.Usage
				sb.WriteString(usage)
				// Add period after usage text if there are constraints and usage doesn't end with period
				if hasConstraints {
					if !strings.HasSuffix(usage, ".") {
						sb.WriteString(". ")
					} else {
						sb.WriteString(" ")
					}
				}
			}

			// Add constraints (including defaults)
			if hasConstraints {
				if !hasUsage {
					// If no usage text, don't add extra space
					sb.WriteString(constraints)
				} else {
					// Usage text already added period and space above
					sb.WriteString(constraints)
				}
			}
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func isBoolFlag(flag any) bool {
	_, ok := flag.(*BoolFlag)
	return ok
}

func getFlagType(flag any) string {
	// Check for custom type override first
	if base := getBaseFlag(flag); base != nil && base.CustomUsageType != "" {
		return base.CustomUsageType
	}

	switch f := flag.(type) {
	case *BoolFlag:
		return "bool"
	case *StringFlag:
		return "str"
	case *IntFlag:
		return "int"
	case *Int64Flag:
		return "int64"
	case *Float64Flag:
		return "float"
	case *BoolSliceFlag:
		if f.Variadic {
			return "[bools...]"
		}
		return "bools"
	case *StringSliceFlag:
		if f.Variadic {
			return "[strs...]"
		}
		return "strs"
	case *IntSliceFlag:
		if f.Variadic {
			return "[ints...]"
		}
		return "ints"
	case *Int64SliceFlag:
		if f.Variadic {
			return "[int64s...]"
		}
		return "int64s"
	case *Float64SliceFlag:
		if f.Variadic {
			return "[floats...]"
		}
		return "floats"
	}
	return ""
}

func (c *Cmd) getConstraintString(flag any) string {
	var parts []string

	// Add range constraints
	if rangeStr := c.getRangeString(flag); rangeStr != "" {
		parts = append(parts, "Range: "+rangeStr)
	}

	// Add enum and regex constraints (both can be present)
	if enumStr := c.getEnumString(flag); enumStr != "" {
		parts = append(parts, "Valid values: "+enumStr)
	}

	if regexStr := c.getRegexString(flag); regexStr != "" {
		parts = append(parts, "Regex: "+regexStr)
	}

	// Add separator for slices
	if sepStr := c.getSeparatorString(flag); sepStr != "" {
		parts = append(parts, "Separator: "+sepStr)
	}

	// Add relationship constraints
	if reqStr := c.getRequiresString(flag); reqStr != "" {
		parts = append(parts, "Requires: "+reqStr)
	}

	if exclStr := c.getExcludesString(flag); exclStr != "" {
		parts = append(parts, "Excludes: "+exclStr)
	}

	// Join constraint parts with periods
	constraintStr := strings.Join(parts, ". ")

	// Add default value last (if present)
	if defaultStr := c.getDefaultString(flag); defaultStr != "" {
		if constraintStr != "" {
			constraintStr += " " + fmt.Sprintf("(default %s)", defaultStr)
		} else {
			constraintStr = fmt.Sprintf("(default %s)", defaultStr)
		}
	}

	return constraintStr
}
func (c *Cmd) getDefaultString(flag any) string {
	switch f := flag.(type) {
	case *StringFlag:
		if f.Default != nil {
			return *f.Default
		}
	case *IntFlag:
		if f.Default != nil {
			return fmt.Sprintf("%d", *f.Default)
		}
	case *Int64Flag:
		if f.Default != nil {
			return fmt.Sprintf("%d", *f.Default)
		}
	case *Float64Flag:
		if f.Default != nil {
			return fmt.Sprintf("%g", *f.Default)
		}
	case *BoolFlag:
		if f.Default != nil && *f.Default {
			return "true" // Show (default true) but not (default false)
		}
		return ""
	case *StringSliceFlag:
		if f.Default != nil && len(*f.Default) > 0 {
			return fmt.Sprintf("[%s]", strings.Join(*f.Default, ", "))
		}
	case *IntSliceFlag:
		if f.Default != nil && len(*f.Default) > 0 {
			var strs []string
			for _, v := range *f.Default {
				strs = append(strs, fmt.Sprintf("%d", v))
			}
			return fmt.Sprintf("[%s]", strings.Join(strs, ", "))
		}
	case *Int64SliceFlag:
		if f.Default != nil && len(*f.Default) > 0 {
			var strs []string
			for _, v := range *f.Default {
				strs = append(strs, fmt.Sprintf("%d", v))
			}
			return fmt.Sprintf("[%s]", strings.Join(strs, ", "))
		}
	case *Float64SliceFlag:
		if f.Default != nil && len(*f.Default) > 0 {
			var strs []string
			for _, v := range *f.Default {
				strs = append(strs, fmt.Sprintf("%g", v))
			}
			return fmt.Sprintf("[%s]", strings.Join(strs, ", "))
		}
	case *BoolSliceFlag:
		if f.Default != nil && len(*f.Default) > 0 {
			var strs []string
			for _, v := range *f.Default {
				strs = append(strs, fmt.Sprintf("%t", v))
			}
			return fmt.Sprintf("[%s]", strings.Join(strs, ", "))
		}
	}
	return ""
}

func (c *Cmd) getRangeString(flag any) string {
	switch f := flag.(type) {
	case *IntFlag:
		if f.min != nil || f.max != nil {
			var left, right string

			if f.min != nil {
				inclusive := f.minInclusive == nil || *f.minInclusive // default to inclusive
				if inclusive {
					left = fmt.Sprintf("[%d", *f.min)
				} else {
					left = fmt.Sprintf("(%d", *f.min)
				}
			} else {
				left = "("
			}

			if f.max != nil {
				inclusive := f.maxInclusive == nil || *f.maxInclusive // default to inclusive
				if inclusive {
					right = fmt.Sprintf("%d]", *f.max)
				} else {
					right = fmt.Sprintf("%d)", *f.max)
				}
			} else {
				right = ")"
			}

			return left + ", " + right
		}
	case *Int64Flag:
		if f.min != nil || f.max != nil {
			var left, right string

			if f.min != nil {
				inclusive := f.minInclusive == nil || *f.minInclusive // default to inclusive
				if inclusive {
					left = fmt.Sprintf("[%d", *f.min)
				} else {
					left = fmt.Sprintf("(%d", *f.min)
				}
			} else {
				left = "("
			}

			if f.max != nil {
				inclusive := f.maxInclusive == nil || *f.maxInclusive // default to inclusive
				if inclusive {
					right = fmt.Sprintf("%d]", *f.max)
				} else {
					right = fmt.Sprintf("%d)", *f.max)
				}
			} else {
				right = ")"
			}

			return left + ", " + right
		}
	case *Float64Flag:
		if f.min != nil || f.max != nil {
			var left, right string

			if f.min != nil {
				inclusive := f.minInclusive == nil || *f.minInclusive // default to inclusive
				if inclusive {
					left = fmt.Sprintf("[%g", *f.min)
				} else {
					left = fmt.Sprintf("(%g", *f.min)
				}
			} else {
				left = "("
			}

			if f.max != nil {
				inclusive := f.maxInclusive == nil || *f.maxInclusive // default to inclusive
				if inclusive {
					right = fmt.Sprintf("%g]", *f.max)
				} else {
					right = fmt.Sprintf("%g)", *f.max)
				}
			} else {
				right = ")"
			}

			return left + ", " + right
		}
	}
	return ""
}

func (c *Cmd) getEnumString(flag any) string {
	switch f := flag.(type) {
	case *StringFlag:
		if f.EnumConstraint != nil && len(*f.EnumConstraint) > 0 {
			return fmt.Sprintf("[%s]", strings.Join(*f.EnumConstraint, ", "))
		}
	}
	return ""
}

func (c *Cmd) getRegexString(flag any) string {
	switch f := flag.(type) {
	case *StringFlag:
		if f.RegexConstraint != nil {
			return f.RegexConstraint.String()
		}
	}
	return ""
}

func (c *Cmd) getSeparatorString(flag any) string {
	switch f := flag.(type) {
	case *StringSliceFlag:
		if f.Separator != nil {
			return fmt.Sprintf("\"%s\"", *f.Separator)
		}
	case *IntSliceFlag:
		if f.Separator != nil {
			return fmt.Sprintf("\"%s\"", *f.Separator)
		}
	case *Int64SliceFlag:
		if f.Separator != nil {
			return fmt.Sprintf("\"%s\"", *f.Separator)
		}
	case *Float64SliceFlag:
		if f.Separator != nil {
			return fmt.Sprintf("\"%s\"", *f.Separator)
		}
	case *BoolSliceFlag:
		if f.Separator != nil {
			return fmt.Sprintf("\"%s\"", *f.Separator)
		}
	}
	return ""
}

func (c *Cmd) getRequiresString(flag any) string {
	base := getBaseFlag(flag)
	if base != nil && base.Requires != nil && len(*base.Requires) > 0 {
		return strings.Join(*base.Requires, ", ")
	}
	return ""
}

func (c *Cmd) getExcludesString(flag any) string {
	base := getBaseFlag(flag)
	if base != nil && base.Excludes != nil && len(*base.Excludes) > 0 {
		return strings.Join(*base.Excludes, ", ")
	}
	return ""
}

func (c *Cmd) shouldFlagBeOptionalInSynopsis(flag any) bool {
	base := getBaseFlag(flag)

	// Check if flag has a default value (makes it optional)
	hasDefault := false
	switch f := flag.(type) {
	case *StringFlag:
		hasDefault = f.Default != nil
	case *IntFlag:
		hasDefault = f.Default != nil
	case *Int64Flag:
		hasDefault = f.Default != nil
	case *Float64Flag:
		hasDefault = f.Default != nil
	case *BoolFlag:
		hasDefault = f.Default != nil
	case *StringSliceFlag:
		hasDefault = f.Default != nil
	case *IntSliceFlag:
		hasDefault = f.Default != nil
	case *Int64SliceFlag:
		hasDefault = f.Default != nil
	case *Float64SliceFlag:
		hasDefault = f.Default != nil
	case *BoolSliceFlag:
		hasDefault = f.Default != nil
	}

	// Flag is optional if it has a default OR was explicitly set optional
	if hasDefault || base.Optional {
		return true
	}

	return false
}

func (c *Cmd) flagHasDefault(flag any) bool {
	switch f := flag.(type) {
	case *StringFlag:
		return f.Default != nil
	case *IntFlag:
		return f.Default != nil
	case *Int64Flag:
		return f.Default != nil
	case *Float64Flag:
		return f.Default != nil
	case *BoolFlag:
		return true // Boolean flags have implicit default of false
	case *StringSliceFlag:
		return f.Default != nil
	case *IntSliceFlag:
		return f.Default != nil
	case *Int64SliceFlag:
		return f.Default != nil
	case *Float64SliceFlag:
		return f.Default != nil
	case *BoolSliceFlag:
		return true // Boolean slice flags have implicit default of empty slice
	}
	return false
}
