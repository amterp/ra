package ra

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// GenerateDump creates a comprehensive dump of the command structure and parsing context
func (c *Cmd) GenerateDump(args []string, opts ...ParseOpt) string {
	return c.generateDump(args, opts...)
}

// generateDump creates a comprehensive dump of the command structure and parsing context
func (c *Cmd) generateDump(args []string, opts ...ParseOpt) string {
	return c.generateDumpWithDepth(args, 0, opts...)
}

// generateDumpWithDepth creates a comprehensive dump with recursion depth tracking
func (c *Cmd) generateDumpWithDepth(args []string, depth int, opts ...ParseOpt) string {
	var sb strings.Builder

	// Root command header
	if depth == 0 {
		sb.WriteString(GreenBoldS("Ra Command Dump") + "\n")
		sb.WriteString(strings.Repeat("=", 50) + "\n\n")
	} else {
		// Subcommand header with indentation
		indent := strings.Repeat("  ", depth)
		sb.WriteString(fmt.Sprintf("%s%s (%s)\n", indent, GreenBoldS("Subcommand Dump"), BoldS(c.name)))
		sb.WriteString(fmt.Sprintf("%s%s\n\n", indent, strings.Repeat("-", 30)))
	}

	// Generate sections with appropriate indentation
	sb.WriteString(c.generateParseConfigSectionWithIndent(depth, opts...))
	sb.WriteString(c.generateCommandInfoSectionWithIndent(depth))
	if depth == 0 { // Only show arguments for root command
		sb.WriteString(c.generateArgumentsToParseSection(args))
	}
	sb.WriteString(c.generateFlagsStructureSectionWithIndent(depth))
	if depth == 0 { // Only show environment for root command
		sb.WriteString(c.generateEnvironmentSection())
	}

	// Recursively dump subcommands
	if len(c.subCmds) > 0 {
		// Add separator before subcommands
		if depth == 0 {
			sb.WriteString("\n" + GreenBoldS("Subcommand Details:") + "\n")
			sb.WriteString(strings.Repeat("=", 50) + "\n\n")
		}

		// Get sorted subcommand names for consistent output
		names := make([]string, 0, len(c.subCmds))
		for name := range c.subCmds {
			names = append(names, name)
		}
		sort.Strings(names)

		for _, name := range names {
			subCmd := c.subCmds[name]
			subDump := subCmd.generateDumpWithDepth(nil, depth+1, opts...)
			sb.WriteString(subDump)
			if name != names[len(names)-1] { // Add separator between subcommands
				sb.WriteString("\n")
			}
		}
	}

	return sb.String()
}

// generateParseConfigSection generates information about parse configuration
func (c *Cmd) generateParseConfigSection(opts ...ParseOpt) string {
	return c.generateParseConfigSectionWithIndent(0, opts...)
}

// generateParseConfigSectionWithIndent generates parse config section with indentation
func (c *Cmd) generateParseConfigSectionWithIndent(depth int, opts ...ParseOpt) string {
	var sb strings.Builder
	indent := strings.Repeat("  ", depth)

	sb.WriteString(fmt.Sprintf("%s%s\n", indent, GreenBoldS("Parse Configuration:")))

	// Build the config to see what options are set
	cfg := &parseCfg{}
	for _, opt := range opts {
		opt(cfg)
	}

	sb.WriteString(fmt.Sprintf("%s  Ignore Unknown: %s\n", indent, BoldS(fmt.Sprintf("%t", cfg.ignoreUnknown))))
	sb.WriteString(fmt.Sprintf("%s  Dump Enabled: %s\n", indent, BoldS(fmt.Sprintf("%t", cfg.dump))))
	sb.WriteString("\n")

	return sb.String()
}

// generateCommandInfoSection generates information about the command structure
func (c *Cmd) generateCommandInfoSection() string {
	return c.generateCommandInfoSectionWithIndent(0)
}

// generateCommandInfoSectionWithIndent generates command info section with indentation
func (c *Cmd) generateCommandInfoSectionWithIndent(depth int) string {
	var sb strings.Builder
	indent := strings.Repeat("  ", depth)

	sb.WriteString(fmt.Sprintf("%s%s\n", indent, GreenBoldS("Command Information:")))

	sb.WriteString(fmt.Sprintf("%s  Name: %s\n", indent, BoldS(c.name)))
	if c.description != "" {
		sb.WriteString(fmt.Sprintf("%s  Description: %s\n", indent, BoldS(c.description)))
	} else {
		sb.WriteString(fmt.Sprintf("%s  Description: %s\n", indent, CyanS("<not set>")))
	}

	sb.WriteString(fmt.Sprintf("%s  Help Enabled: %s\n", indent, BoldS(fmt.Sprintf("%t", c.helpEnabled))))
	sb.WriteString(fmt.Sprintf("%s  Auto Help on No Args: %s\n", indent, BoldS(fmt.Sprintf("%t", c.autoHelpOnNoArgs))))
	sb.WriteString(
		fmt.Sprintf("%s  Exclude Name from Usage: %s\n", indent, BoldS(fmt.Sprintf("%t", c.excludeNameFromUsage))),
	)

	if c.customUsage != nil {
		sb.WriteString(fmt.Sprintf("%s  Custom Usage Function: %s\n", indent, BoldS("set")))
	} else {
		sb.WriteString(fmt.Sprintf("%s  Custom Usage Function: %s\n", indent, CyanS("not set")))
	}

	if c.parseHooks != nil {
		if c.parseHooks.PostParse != nil {
			sb.WriteString(fmt.Sprintf("%s  PostParse Hook: %s\n", indent, BoldS("set")))
		} else {
			sb.WriteString(fmt.Sprintf("%s  PostParse Hook: %s\n", indent, CyanS("not set")))
		}
	} else {
		sb.WriteString(fmt.Sprintf("%s  PostParse Hook: %s\n", indent, CyanS("not set")))
	}

	// Subcommands
	if len(c.subCmds) > 0 {
		sb.WriteString(fmt.Sprintf("%s  Subcommands (%d): %s\n", indent, len(c.subCmds), BoldS(c.getSubcommandNames())))
	} else {
		sb.WriteString(fmt.Sprintf("%s  Subcommands: %s\n", indent, CyanS("none")))
	}

	sb.WriteString("\n")
	return sb.String()
}

// generateArgumentsToParseSection generates information about the arguments provided for parsing
func (c *Cmd) generateArgumentsToParseSection(args []string) string {
	var sb strings.Builder
	sb.WriteString(GreenBoldS("Arguments to Parse:") + "\n")

	if len(args) == 0 {
		sb.WriteString("  " + CyanS("<no arguments>") + "\n")
	} else {
		for i, arg := range args {
			sb.WriteString(fmt.Sprintf("  [%d]: %s\n", i, BoldS(fmt.Sprintf("%q", arg))))
		}
	}

	sb.WriteString("\n")
	return sb.String()
}

// generateFlagsStructureSection generates comprehensive information about all flags
func (c *Cmd) generateFlagsStructureSection() string {
	return c.generateFlagsStructureSectionWithIndent(0)
}

// generateFlagsStructureSectionWithIndent generates flags structure section with indentation
func (c *Cmd) generateFlagsStructureSectionWithIndent(depth int) string {
	var sb strings.Builder
	indent := strings.Repeat("  ", depth)

	sb.WriteString(fmt.Sprintf("%s%s\n", indent, GreenBoldS("Flags Structure:")))

	// Flag counts
	totalFlags := len(c.flags)
	positionalFlags := len(c.positional)
	nonPositionalFlags := len(c.nonPositional)
	globalFlags := len(c.globalFlags)

	sb.WriteString(fmt.Sprintf("%s  Total Flags: %s\n", indent, BoldS(fmt.Sprintf("%d", totalFlags))))
	sb.WriteString(fmt.Sprintf("%s  Positional Flags: %s\n", indent, BoldS(fmt.Sprintf("%d", positionalFlags))))
	sb.WriteString(fmt.Sprintf("%s  Non-Positional Flags: %s\n", indent, BoldS(fmt.Sprintf("%d", nonPositionalFlags))))
	sb.WriteString(fmt.Sprintf("%s  Global Flags: %s\n", indent, BoldS(fmt.Sprintf("%d", globalFlags))))
	sb.WriteString("\n")

	// Positional flags
	if len(c.positional) > 0 {
		sb.WriteString(fmt.Sprintf("%s%s\n", indent, GreenBoldS("  Positional Flags (in order):")))
		for i, name := range c.positional {
			flag := c.flags[name]
			sb.WriteString(fmt.Sprintf("%s    [%d] %s\n", indent, i, c.formatFlagForDump(name, flag)))
		}
		sb.WriteString("\n")
	}

	// Non-positional flags
	if len(c.nonPositional) > 0 {
		sb.WriteString(fmt.Sprintf("%s%s\n", indent, GreenBoldS("  Non-Positional Flags:")))
		for _, name := range c.nonPositional {
			flag := c.flags[name]
			sb.WriteString(fmt.Sprintf("%s    %s\n", indent, c.formatFlagForDump(name, flag)))
		}
		sb.WriteString("\n")
	}

	// Global flags
	if len(c.globalFlags) > 0 {
		sb.WriteString(fmt.Sprintf("%s%s\n", indent, GreenBoldS("  Global Flags:")))
		for _, name := range c.globalFlags {
			if flag, exists := c.flags[name]; exists {
				sb.WriteString(fmt.Sprintf("%s    %s\n", indent, c.formatFlagForDump(name, flag)))
			}
		}
		sb.WriteString("\n")
	}

	// Flag conflicts and shadows
	if len(c.overriddenGlobalFlags) > 0 || len(c.shadowedShortFlags) > 0 || len(c.shadowedNameFlags) > 0 {
		sb.WriteString(fmt.Sprintf("%s%s\n", indent, GreenBoldS("  Flag Conflicts:")))

		if len(c.overriddenGlobalFlags) > 0 {
			sb.WriteString(fmt.Sprintf("%s    Overridden Global Flags:\n", indent))
			for name := range c.overriddenGlobalFlags {
				sb.WriteString(fmt.Sprintf("%s      %s\n", indent, BoldS(name)))
			}
		}

		if len(c.shadowedShortFlags) > 0 {
			sb.WriteString(fmt.Sprintf("%s    Shadowed Short Flags:\n", indent))
			for name := range c.shadowedShortFlags {
				sb.WriteString(fmt.Sprintf("%s      %s\n", indent, BoldS(name)))
			}
		}

		if len(c.shadowedNameFlags) > 0 {
			sb.WriteString(fmt.Sprintf("%s    Shadowed Name Flags:\n", indent))
			for name := range c.shadowedNameFlags {
				sb.WriteString(fmt.Sprintf("%s      %s\n", indent, BoldS(name)))
			}
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

// generateEnvironmentSection generates information about the environment
func (c *Cmd) generateEnvironmentSection() string {
	var sb strings.Builder
	sb.WriteString(GreenBoldS("Environment:") + "\n")

	// Check RA_COLOR environment variable
	raColor := os.Getenv("RA_COLOR")
	if raColor != "" {
		sb.WriteString(fmt.Sprintf("  RA_COLOR: %s\n", BoldS(raColor)))
	} else {
		sb.WriteString(fmt.Sprintf("  RA_COLOR: %s\n", CyanS("not set")))
	}

	return sb.String()
}

// Helper functions

// getSubcommandNames returns a comma-separated list of subcommand names
func (c *Cmd) getSubcommandNames() string {
	names := make([]string, 0, len(c.subCmds))
	for name := range c.subCmds {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// formatFlagForDump formats a flag with comprehensive details for dump output
func (c *Cmd) formatFlagForDump(name string, flag any) string {
	base := getBaseFlag(flag)
	if base == nil {
		return fmt.Sprintf("%s: %s", BoldS(name), CyanS("unknown type"))
	}

	var parts []string

	// Name and short
	if base.Short != "" {
		parts = append(parts, fmt.Sprintf("%s (-%s)", BoldS(name), BoldS(base.Short)))
	} else {
		parts = append(parts, BoldS(name))
	}

	// Type with constraints
	flagType := c.getFlagTypeForDump(flag)
	parts = append(parts, fmt.Sprintf("type:%s", CyanS(flagType)))

	// Required/Optional with default value
	if base.Optional {
		parts = append(parts, CyanS("optional"))
	} else {
		hasDefault := c.flagHasDefault(flag)
		if hasDefault {
			defaultVal := c.getFlagDefaultValue(flag)
			parts = append(parts, fmt.Sprintf("%s %s", CyanS("optional"), CyanS(fmt.Sprintf("(default:%s)", defaultVal))))
		} else {
			parts = append(parts, CyanS("required"))
		}
	}

	// Current value
	currentVal := c.getFlagCurrentValue(flag)
	if currentVal != "" {
		parts = append(parts, fmt.Sprintf("current:%s", currentVal))
	}

	// Configured status
	if c.configured[name] {
		parts = append(parts, "configured")
	}

	// Relational constraints
	requires := c.getFlagRequires(flag)
	if len(requires) > 0 {
		parts = append(parts, fmt.Sprintf("requires:[%s]", strings.Join(requires, ",")))
	}

	excludes := c.getFlagExcludes(flag)
	if len(excludes) > 0 {
		parts = append(parts, fmt.Sprintf("excludes:[%s]", strings.Join(excludes, ",")))
	}

	// Flag properties
	var flags []string
	if base.Hidden {
		flags = append(flags, "hidden")
	}
	if base.HiddenInShortHelp {
		flags = append(flags, "hidden-in-short")
	}
	if base.PositionalOnly {
		flags = append(flags, "positional-only")
	}
	if base.FlagOnly {
		flags = append(flags, "flag-only")
	}
	if base.BypassValidation {
		flags = append(flags, "bypass-validation")
	}

	if len(flags) > 0 {
		parts = append(parts, fmt.Sprintf("flags:[%s]", strings.Join(flags, ",")))
	}

	// Usage
	if base.Usage != "" {
		parts = append(parts, fmt.Sprintf("usage:%s", fmt.Sprintf("%q", base.Usage)))
	}

	return strings.Join(parts, " ")
}

// getFlagTypeForDump returns a string representation of the flag type for dump output
func (c *Cmd) getFlagTypeForDump(flag any) string {
	switch f := flag.(type) {
	case *BoolFlag:
		return "bool"
	case *StringFlag:
		result := "string"
		if f.EnumConstraint != nil {
			result += fmt.Sprintf("{%s}", strings.Join(*f.EnumConstraint, ","))
		}
		if f.RegexConstraint != nil {
			result += fmt.Sprintf("~%s", f.RegexConstraint.String())
		}
		return result
	case *IntFlag:
		result := "int"
		if f.min != nil || f.max != nil {
			var minStr, maxStr string
			if f.min != nil {
				minStr = fmt.Sprintf("%d", *f.min)
				if f.minInclusive != nil && !*f.minInclusive {
					minStr = "(" + minStr
				} else {
					minStr = "[" + minStr
				}
			} else {
				minStr = "(-∞"
			}
			if f.max != nil {
				maxStr = fmt.Sprintf("%d", *f.max)
				if f.maxInclusive != nil && !*f.maxInclusive {
					maxStr = maxStr + ")"
				} else {
					maxStr = maxStr + "]"
				}
			} else {
				maxStr = "+∞)"
			}
			result += fmt.Sprintf("%s,%s", minStr, maxStr)
		}
		return result
	case *Int64Flag:
		result := "int64"
		if f.min != nil || f.max != nil {
			var minStr, maxStr string
			if f.min != nil {
				minStr = fmt.Sprintf("%d", *f.min)
				if f.minInclusive != nil && !*f.minInclusive {
					minStr = "(" + minStr
				} else {
					minStr = "[" + minStr
				}
			} else {
				minStr = "(-∞"
			}
			if f.max != nil {
				maxStr = fmt.Sprintf("%d", *f.max)
				if f.maxInclusive != nil && !*f.maxInclusive {
					maxStr = maxStr + ")"
				} else {
					maxStr = maxStr + "]"
				}
			} else {
				maxStr = "+∞)"
			}
			result += fmt.Sprintf("%s,%s", minStr, maxStr)
		}
		return result
	case *Float64Flag:
		result := "float64"
		if f.min != nil || f.max != nil {
			var minStr, maxStr string
			if f.min != nil {
				minStr = fmt.Sprintf("%g", *f.min)
				if f.minInclusive != nil && !*f.minInclusive {
					minStr = "(" + minStr
				} else {
					minStr = "[" + minStr
				}
			} else {
				minStr = "(-∞"
			}
			if f.max != nil {
				maxStr = fmt.Sprintf("%g", *f.max)
				if f.maxInclusive != nil && !*f.maxInclusive {
					maxStr = maxStr + ")"
				} else {
					maxStr = maxStr + "]"
				}
			} else {
				maxStr = "+∞)"
			}
			result += fmt.Sprintf("%s,%s", minStr, maxStr)
		}
		return result
	case *StringSliceFlag:
		result := "[]string"
		if f.Variadic {
			result += "(variadic)"
		}
		if f.Separator != nil {
			result += fmt.Sprintf(" sep:%q", *f.Separator)
		}
		return result
	case *IntSliceFlag:
		result := "[]int"
		if f.Variadic {
			result += "(variadic)"
		}
		if f.Separator != nil {
			result += fmt.Sprintf(" sep:%q", *f.Separator)
		}
		return result
	case *Int64SliceFlag:
		result := "[]int64"
		if f.Variadic {
			result += "(variadic)"
		}
		if f.Separator != nil {
			result += fmt.Sprintf(" sep:%q", *f.Separator)
		}
		return result
	case *Float64SliceFlag:
		result := "[]float64"
		if f.Variadic {
			result += "(variadic)"
		}
		if f.Separator != nil {
			result += fmt.Sprintf(" sep:%q", *f.Separator)
		}
		return result
	case *BoolSliceFlag:
		result := "[]bool"
		if f.Variadic {
			result += "(variadic)"
		}
		if f.Separator != nil {
			result += fmt.Sprintf(" sep:%q", *f.Separator)
		}
		return result
	}
	return "unknown"
}

// getFlagDefaultValue returns the default value of a flag as a string
func (c *Cmd) getFlagDefaultValue(flag any) string {
	switch f := flag.(type) {
	case *BoolFlag:
		if f.Default != nil {
			return fmt.Sprintf("%t", *f.Default)
		}
		return "false"
	case *StringFlag:
		if f.Default != nil {
			return fmt.Sprintf("%q", *f.Default)
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
	case *StringSliceFlag:
		if f.Default != nil {
			return fmt.Sprintf("%q", *f.Default)
		}
		return "[]"
	case *IntSliceFlag:
		if f.Default != nil {
			return fmt.Sprintf("%v", *f.Default)
		}
		return "[]"
	case *Int64SliceFlag:
		if f.Default != nil {
			return fmt.Sprintf("%v", *f.Default)
		}
		return "[]"
	case *Float64SliceFlag:
		if f.Default != nil {
			return fmt.Sprintf("%v", *f.Default)
		}
		return "[]"
	case *BoolSliceFlag:
		if f.Default != nil {
			return fmt.Sprintf("%v", *f.Default)
		}
		return "[]"
	}
	return "none"
}

// getFlagCurrentValue returns the current value of a flag as a string, only if interesting
func (c *Cmd) getFlagCurrentValue(flag any) string {
	switch f := flag.(type) {
	case *BoolFlag:
		// Only show bool current value if it's true or if it has an explicit default
		if f.Value != nil && (*f.Value || (f.Default != nil && *f.Default)) {
			return fmt.Sprintf("%t", *f.Value)
		}
	case *StringFlag:
		if f.Value != nil && *f.Value != "" {
			return fmt.Sprintf("%q", *f.Value)
		}
	case *IntFlag:
		if f.Value != nil && *f.Value != 0 {
			return fmt.Sprintf("%d", *f.Value)
		}
	case *Int64Flag:
		if f.Value != nil && *f.Value != 0 {
			return fmt.Sprintf("%d", *f.Value)
		}
	case *Float64Flag:
		if f.Value != nil && *f.Value != 0 {
			return fmt.Sprintf("%g", *f.Value)
		}
	case *StringSliceFlag:
		if f.Value != nil && len(*f.Value) > 0 {
			return fmt.Sprintf("%q", *f.Value)
		}
	case *IntSliceFlag:
		if f.Value != nil && len(*f.Value) > 0 {
			return fmt.Sprintf("%v", *f.Value)
		}
	case *Int64SliceFlag:
		if f.Value != nil && len(*f.Value) > 0 {
			return fmt.Sprintf("%v", *f.Value)
		}
	case *Float64SliceFlag:
		if f.Value != nil && len(*f.Value) > 0 {
			return fmt.Sprintf("%v", *f.Value)
		}
	case *BoolSliceFlag:
		if f.Value != nil && len(*f.Value) > 0 {
			return fmt.Sprintf("%v", *f.Value)
		}
	}
	return ""
}

// getFlagRequires returns the requires constraint of a flag
func (c *Cmd) getFlagRequires(flag any) []string {
	switch f := flag.(type) {
	case *BoolFlag:
		if f.Requires != nil {
			return *f.Requires
		}
	case *StringFlag:
		if f.Requires != nil {
			return *f.Requires
		}
	case *IntFlag:
		if f.Requires != nil {
			return *f.Requires
		}
	case *Int64Flag:
		if f.Requires != nil {
			return *f.Requires
		}
	case *Float64Flag:
		if f.Requires != nil {
			return *f.Requires
		}
	case *StringSliceFlag:
		if f.Requires != nil {
			return *f.Requires
		}
	case *IntSliceFlag:
		if f.Requires != nil {
			return *f.Requires
		}
	case *Int64SliceFlag:
		if f.Requires != nil {
			return *f.Requires
		}
	case *Float64SliceFlag:
		if f.Requires != nil {
			return *f.Requires
		}
	case *BoolSliceFlag:
		if f.Requires != nil {
			return *f.Requires
		}
	}
	return nil
}

// getFlagExcludes returns the excludes constraint of a flag
func (c *Cmd) getFlagExcludes(flag any) []string {
	switch f := flag.(type) {
	case *BoolFlag:
		if f.Excludes != nil {
			return *f.Excludes
		}
	case *StringFlag:
		if f.Excludes != nil {
			return *f.Excludes
		}
	case *IntFlag:
		if f.Excludes != nil {
			return *f.Excludes
		}
	case *Int64Flag:
		if f.Excludes != nil {
			return *f.Excludes
		}
	case *Float64Flag:
		if f.Excludes != nil {
			return *f.Excludes
		}
	case *StringSliceFlag:
		if f.Excludes != nil {
			return *f.Excludes
		}
	case *IntSliceFlag:
		if f.Excludes != nil {
			return *f.Excludes
		}
	case *Int64SliceFlag:
		if f.Excludes != nil {
			return *f.Excludes
		}
	case *Float64SliceFlag:
		if f.Excludes != nil {
			return *f.Excludes
		}
	case *BoolSliceFlag:
		if f.Excludes != nil {
			return *f.Excludes
		}
	}
	return nil
}
