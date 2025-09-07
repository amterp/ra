package ra

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/amterp/color"
)

// HelpInvokedErr is returned by ParseOrError when help is invoked (via -h, --help, or auto-help).
// Users can compare against this constant to detect when help was shown instead of a parsing error.
var HelpInvokedErr = errors.New("help invoked")

// DumpInvokedErr is returned by ParseOrError when dump is invoked (via WithDump(true)).
// Users can compare against this constant to detect when dump was shown instead of parsing.
var DumpInvokedErr = errors.New("dump invoked")

// Internal error wrapper to carry exit code for ParseOrExit
type helpInvokedError struct {
	output         string // The usage text that was/would be output (empty if not yet generated)
	exitCode       int    // The exit code (0 for help, 1 for error)
	useStdout      bool   // Whether to output to stdout (true for help requests) or stderr (false for errors)
	isLongHelp     bool   // true for --help, false for -h or auto-help
	isAutoHelp     bool   // true if triggered by auto-help (no args with required flags)
	useCustomUsage bool   // true if custom usage function should be used
}

func (e *helpInvokedError) Error() string {
	return HelpInvokedErr.Error()
}

func (e *helpInvokedError) Unwrap() error {
	return HelpInvokedErr
}

// Internal error wrapper for dump invocation
type dumpInvokedError struct {
	output   string // The dump output (empty if not yet generated)
	exitCode int    // The exit code (0 for successful dump)
}

func (e *dumpInvokedError) Error() string {
	return DumpInvokedErr.Error()
}

func (e *dumpInvokedError) Unwrap() error {
	return DumpInvokedErr
}

// ProgrammingError wraps errors caused by incorrect library setup/configuration.
// These are bugs in the code using Ra, not user input errors.
type ProgrammingError struct {
	msg string
}

func (e *ProgrammingError) Error() string {
	return e.msg
}

// NewProgrammingError creates a new programming error
func NewProgrammingError(msg string) *ProgrammingError {
	return &ProgrammingError{msg: msg}
}

func (c *Cmd) ParseOrExit(args []string, opts ...ParseOpt) {
	err := c.parse(args, opts...)

	// Call PostParse hook after parsing, before any output (success or error)
	if c.parseHooks != nil && c.parseHooks.PostParse != nil {
		c.parseHooks.PostParse(c, err)
	}

	if err != nil {
		// Check if this is a help invoked error
		if helpErr, ok := err.(*helpInvokedError); ok {
			// Generate help output now, after PostParse hook has been called
			var output string
			if helpErr.useCustomUsage {
				if c.customUsage != nil {
					c.customUsage(helpErr.isLongHelp)
					output = "" // Custom usage handles output directly
				}
			} else {
				if helpErr.isLongHelp {
					output = c.GenerateLongUsage()
				} else {
					output = c.GenerateShortUsage()
				}
			}

			// Route output to stdout for help requests, stderr for errors
			if output != "" {
				if helpErr.useStdout {
					fmt.Fprint(stdoutWriter, output)
				} else {
					fmt.Fprint(stderrWriter, output)
				}
			}
			osExit(helpErr.exitCode)
		} else if dumpErr, ok := err.(*dumpInvokedError); ok {
			// Generate dump output now, after PostParse hook has been called
			output := c.generateDump(args, opts...)
			if output != "" {
				fmt.Fprint(stdoutWriter, output)
			}
			osExit(dumpErr.exitCode)
		} else if _, ok := err.(*ProgrammingError); ok {
			// Programming error - show only error message (no usage)
			fmt.Fprintln(stderrWriter, err.Error())
			osExit(1)
		} else {
			// Regular error - show error message and usage
			fmt.Fprintln(stderrWriter, err.Error())
			fmt.Fprintln(stderrWriter)
			fmt.Fprint(stderrWriter, c.GenerateLongUsage())
			osExit(1)
		}
	}
}

func (c *Cmd) ParseOrError(args []string, opts ...ParseOpt) error {
	err := c.parse(args, opts...)

	// Call PostParse hook after parsing, before any output (success or error)
	if c.parseHooks != nil && c.parseHooks.PostParse != nil {
		c.parseHooks.PostParse(c, err)
	}

	if err != nil {
		// Check if this is a help invoked error
		if helpErr, ok := err.(*helpInvokedError); ok {
			// Call custom usage function if it exists, even though ParseOrError doesn't display output
			// This maintains backward compatibility for custom usage functions that may have side effects
			if helpErr.useCustomUsage && c.customUsage != nil {
				c.customUsage(helpErr.isLongHelp)
			}
			return HelpInvokedErr
		} else if _, ok := err.(*dumpInvokedError); ok {
			// Dump was invoked - ParseOrError doesn't display output, but we return the standard error
			return DumpInvokedErr
		}
	}
	return err
}

func (c *Cmd) parse(args []string, opts ...ParseOpt) error {
	return c.parseWithPreserveState(args, false, opts...)
}

func (c *Cmd) parseWithPreserveState(args []string, preserveConfigured bool, opts ...ParseOpt) error {
	initializeColorFromEnv()

	cfg := &parseCfg{}
	for _, opt := range opts {
		opt(cfg)
	}

	// reset state in case this is called multiple times
	if !preserveConfigured {
		c.configured = make(map[string]bool)
	}
	c.unknownArgs = []string{}
	c.lastVariadicFlag = ""
	c.sawFlag = false

	// Add help flags if enabled
	if c.helpEnabled {
		if _, exists := c.flags["help"]; !exists {
			NewBool(
				"help",
			).SetShort("h").
				SetUsage("Print usage string.").
				SetOptional(true).
				Register(c, WithGlobal(true))
		}
	}

	if err := c.validateBeforeParsing(); err != nil {
		return err
	}

	// Set defaults first
	if err := c.setDefaults(); err != nil {
		return err
	}

	// Check for dump mode - if enabled, generate dump output and return
	if cfg.dump {
		return &dumpInvokedError{
			output:   "", // Will be generated later, after PostParse hook
			exitCode: 0,
		}
	}

	// Check for auto-help: if enabled, no args provided, and command has required flags
	if c.autoHelpOnNoArgs && len(args) == 0 && c.hasRequiredFlags() {
		return &helpInvokedError{
			output:         "", // Will be generated later, after PostParse hook
			exitCode:       0,
			useStdout:      true,
			isLongHelp:     false, // Auto-help uses short help
			isAutoHelp:     true,
			useCustomUsage: c.customUsage != nil,
		}
	}

	// Check if we have number shorts mode
	numberShortsMode := c.hasNumberShorts()

	// Parse arguments
	i := 0
	seenDashDash := false // Track if we've seen -- and should treat everything as positional

	for i < len(args) {
		arg := args[i]

		// Check for -- (end of flags marker)
		if arg == "--" && !seenDashDash {
			seenDashDash = true
			i++
			continue
		}

		// If we're in positional-only mode, treat everything as positional
		if seenDashDash {
			if err := c.assignPositionalWithMode(arg, true); err != nil {
				if cfg.ignoreUnknown {
					c.unknownArgs = append(c.unknownArgs, arg)
				} else {
					return err
				}
			}
			i++
			continue
		}

		// Check for subcommand first (only if not in positional-only mode)
		if !strings.HasPrefix(arg, "-") {
			if subCmd, exists := c.subCmds[arg]; exists {
				*subCmd.used = true
				// Apply global flags to subcommand before parsing
				if err := c.applyGlobalFlags(subCmd); err != nil {
					return err
				}
				// Apply global configured state before parsing
				for _, globalFlagName := range c.globalFlags {
					if c.configured[globalFlagName] {
						subCmd.configured[globalFlagName] = true
					}
				}
				return subCmd.parseWithPreserveState(args[i+1:], true, opts...)
			}
		}

		// Handle flags (only if not in positional-only mode)
		if strings.HasPrefix(arg, "-") {
			consumed, err := c.parseFlag(args, i, numberShortsMode)
			if err != nil {
				if err.Error() == "not a flag: "+arg {
					// This is a negative number, treat as positional
					if err := c.assignPositional(arg); err != nil {
						if cfg.ignoreUnknown {
							c.unknownArgs = append(c.unknownArgs, arg)
						} else {
							return err
						}
					}
					i++
					continue
				}
				if cfg.ignoreUnknown {
					c.unknownArgs = append(c.unknownArgs, arg)
					i++
					continue
				}
				return err
			}
			c.sawFlag = true
			c.lastVariadicFlag = "" // Reset variadic state when we see a flag
			i += consumed
		} else {
			// Handle positional argument
			if err := c.assignPositional(arg); err != nil {
				if cfg.ignoreUnknown {
					c.unknownArgs = append(c.unknownArgs, arg)
				} else {
					return err
				}
			}
			i++
		}
	}

	// Check for help flags after parsing (only if helpEnabled is true)
	if c.helpEnabled {
		for _, arg := range args {
			if arg == "--help" {
				return &helpInvokedError{
					output:         "", // Will be generated later, after PostParse hook
					exitCode:       0,
					useStdout:      true,
					isLongHelp:     true,
					isAutoHelp:     false,
					useCustomUsage: c.customUsage != nil,
				}
			}
			if arg == "-h" {
				return &helpInvokedError{
					output:         "", // Will be generated later, after PostParse hook
					exitCode:       0,
					useStdout:      true,
					isLongHelp:     false,
					isAutoHelp:     false,
					useCustomUsage: c.customUsage != nil,
				}
			}
		}
	}

	// Validate required flags
	return c.validateRequired()
}

func (c *Cmd) setDefaults() error {
	for _, flag := range c.flags {
		switch f := flag.(type) {
		case *BoolFlag:
			if f.Default != nil && !c.configured[f.Name] {
				*f.Value = *f.Default
			}
		case *StringFlag:
			if f.Default != nil && !c.configured[f.Name] {
				*f.Value = *f.Default
			}
		case *IntFlag:
			if f.Default != nil && !c.configured[f.Name] {
				*f.Value = *f.Default
			}
		case *Int64Flag:
			if f.Default != nil && !c.configured[f.Name] {
				*f.Value = *f.Default
			}
		case *Float64Flag:
			if f.Default != nil && !c.configured[f.Name] {
				*f.Value = *f.Default
			}
		case *StringSliceFlag:
			if !c.configured[f.Name] {
				if f.Default != nil {
					*f.Value = *f.Default
				} else {
					*f.Value = []string{}
				}
			}
		case *IntSliceFlag:
			if !c.configured[f.Name] {
				if f.Default != nil {
					*f.Value = *f.Default
				} else {
					*f.Value = []int{}
				}
			}
		case *Int64SliceFlag:
			if !c.configured[f.Name] {
				if f.Default != nil {
					*f.Value = *f.Default
				} else {
					*f.Value = []int64{}
				}
			}
		case *Float64SliceFlag:
			if !c.configured[f.Name] {
				if f.Default != nil {
					*f.Value = *f.Default
				} else {
					*f.Value = []float64{}
				}
			}
		case *BoolSliceFlag:
			if !c.configured[f.Name] {
				if f.Default != nil {
					*f.Value = *f.Default
				} else {
					*f.Value = []bool{}
				}
			}
		}
	}
	return nil
}
func (c *Cmd) hasNumberShorts() bool {
	for _, flag := range c.flags {
		var short string
		switch f := flag.(type) {
		case *IntFlag:
			short = f.Short
		case *Int64Flag:
			short = f.Short
		case *Float64Flag:
			short = f.Short
		case *StringFlag:
			short = f.Short
		case *BoolFlag:
			short = f.Short
		case *StringSliceFlag:
			short = f.Short
		case *IntSliceFlag:
			short = f.Short
		case *Int64SliceFlag:
			short = f.Short
		case *Float64SliceFlag:
			short = f.Short
		case *BoolSliceFlag:
			short = f.Short
		}
		if short != "" && len(short) == 1 && isDigit(short[0]) {
			return true
		}
	}
	return false
}

func (c *Cmd) parseFlag(args []string, index int, numberShortsMode bool) (int, error) {
	arg := args[index]

	if strings.HasPrefix(arg, "--") {
		// Long flag
		return c.parseLongFlag(args, index)
	} else if strings.HasPrefix(arg, "-") {
		// Short flag(s)
		return c.parseShortFlag(args, index, numberShortsMode)
	}

	return 0, fmt.Errorf("invalid flag: %s", arg)
}

func (c *Cmd) parseLongFlag(args []string, index int) (int, error) {
	arg := args[index]
	flagName := arg[2:] // remove --

	// Check for = syntax
	var value string
	var hasValue bool
	if idx := strings.Index(flagName, "="); idx != -1 {
		value = flagName[idx+1:]
		flagName = flagName[:idx]
		hasValue = true
	}

	flag, exists := c.flags[flagName]
	if !exists {
		return 0, fmt.Errorf("unknown flag: --%s", flagName)
	}

	c.configured[flagName] = true

	switch f := flag.(type) {
	case *BoolFlag:
		if hasValue {
			val, err := c.parseBoolValue(value)
			if err != nil {
				return 0, fmt.Errorf("invalid value for flag --%s: %s", flagName, err.Error())
			}
			*f.Value = val
		} else {
			*f.Value = true
		}
		return 1, nil
	case *StringFlag:
		if hasValue {
			err := c.setStringValue(f, value)
			return 1, err
		}
		if index+1 >= len(args) {
			return 0, fmt.Errorf("flag --%s requires a value", flagName)
		}
		err := c.setStringValue(f, args[index+1])
		return 2, err
	case *IntFlag:
		if hasValue {
			err := c.setIntValue(f, value)
			return 1, err
		}
		if index+1 >= len(args) {
			return 0, fmt.Errorf("flag --%s requires a value", flagName)
		}
		err := c.setIntValue(f, args[index+1])
		return 2, err
	case *Int64Flag:
		if hasValue {
			err := c.setInt64Value(f, value)
			return 1, err
		}
		if index+1 >= len(args) {
			return 0, fmt.Errorf("flag --%s requires a value", flagName)
		}
		err := c.setInt64Value(f, args[index+1])
		return 2, err
	case *Float64Flag:
		if hasValue {
			err := c.setFloat64Value(f, value)
			return 1, err
		}
		if index+1 >= len(args) {
			return 0, fmt.Errorf("flag --%s requires a value", flagName)
		}
		err := c.setFloat64Value(f, args[index+1])
		return 2, err
	case *StringSliceFlag:
		if hasValue {
			_, err := c.appendStringSliceValue(f, value)
			if err == nil && f.Variadic {
				c.lastVariadicFlag = flagName
			}
			return 1, err
		}
		consumed, err := c.parseSliceFlag(args, index, f)
		if err == nil && f.Variadic {
			c.lastVariadicFlag = flagName
		}
		return consumed, err
	case *IntSliceFlag:
		if hasValue {
			_, err := c.appendIntSliceValue(f, value)
			return 1, err
		}
		return c.parseIntSliceFlag(args, index, f)
	case *Int64SliceFlag:
		if hasValue {
			_, err := c.appendInt64SliceValue(f, value)
			return 1, err
		}
		return c.parseInt64SliceFlag(args, index, f)
	case *Float64SliceFlag:
		if hasValue {
			_, err := c.appendFloat64SliceValue(f, value)
			return 1, err
		}
		return c.parseFloat64SliceFlag(args, index, f)
	case *BoolSliceFlag:
		if hasValue {
			_, err := c.appendBoolSliceValue(f, value)
			return 1, err
		}
		return c.parseBoolSliceFlag(args, index, f)
	}

	return 0, NewProgrammingError(fmt.Sprintf("unsupported flag type for: %s", flagName))
}

func (c *Cmd) parseShortFlag(args []string, index int, numberShortsMode bool) (int, error) {
	arg := args[index]
	shorts := arg[1:] // remove -

	// Check if this is a negative number without number shorts mode
	if !numberShortsMode && len(shorts) > 0 && (isDigit(shorts[0]) || shorts[0] == '.') {
		// This is a negative number, treat as positional
		return 0, fmt.Errorf("not a flag: %s", arg)
	}

	// In number shorts mode, check if this is a negative number
	if numberShortsMode && len(shorts) > 0 && isDigit(shorts[0]) {
		// This is a number short flag
		if flagName, exists := c.shortToName[shorts]; exists {
			flag := c.flags[flagName]
			c.configured[flagName] = true

			switch f := flag.(type) {
			case *IntFlag:
				if len(shorts) > 1 {
					// Multiple occurrences like -aaa
					count := len(shorts)
					*f.Value = count
					return 1, nil
				}
				// Single occurrence, needs value
				if index+1 >= len(args) {
					return 0, fmt.Errorf("flag -%s requires a value", shorts)
				}
				err := c.setIntValue(f, args[index+1])
				return 2, err
			case *StringFlag:
				if index+1 >= len(args) {
					return 0, fmt.Errorf("flag -%s requires a value", shorts)
				}
				err := c.setStringValue(f, args[index+1])
				return 2, err
			}
		}
	}

	// Regular short flag processing
	consumed := 1

	// Check if all chars are the same (for int flag counting)
	if len(shorts) > 1 {
		firstChar := shorts[0]
		allSame := true
		for i := 1; i < len(shorts); i++ {
			if shorts[i] != firstChar {
				allSame = false
				break
			}
		}
		if allSame {
			// All chars are the same, check if it's an int flag
			if flagName, exists := c.shortToName[string(firstChar)]; exists {
				if flag, exists := c.flags[flagName]; exists {
					if intFlag, ok := flag.(*IntFlag); ok {
						// This is an int flag being repeated, set it to the count
						c.configured[flagName] = true
						*intFlag.Value = len(shorts)
						return 1, nil
					}
				}
			}
		}
	}

	for i, short := range shorts {
		shortStr := string(short)
		flagName, exists := c.shortToName[shortStr]
		if !exists {
			return 0, fmt.Errorf("unknown shorthand flag: '%s' in -%s", shortStr, shortStr)
		}

		flag := c.flags[flagName]
		c.configured[flagName] = true

		switch f := flag.(type) {
		case *BoolFlag:
			*f.Value = true
		case *StringFlag:
			if i == len(shorts)-1 {
				// Last flag in cluster, can take value
				if index+1 >= len(args) {
					return 0, fmt.Errorf("flag -%s requires a value", shortStr)
				}
				err := c.setStringValue(f, args[index+1])
				if err != nil {
					return 0, err
				}
				consumed = 2
			} else {
				return 0, fmt.Errorf("non-bool flag -%s must be last in cluster", shortStr)
			}
		case *IntFlag:
			if i == len(shorts)-1 {
				// Last flag in cluster, can take value
				if index+1 >= len(args) {
					return 0, fmt.Errorf("flag -%s requires a value", shortStr)
				}
				err := c.setIntValue(f, args[index+1])
				if err != nil {
					return 0, err
				}
				consumed = 2
			} else {
				return 0, fmt.Errorf("non-bool flag -%s must be last in cluster", shortStr)
			}
		case *StringSliceFlag:
			if i == len(shorts)-1 {
				// Last flag in cluster, can take value
				consumed, err := c.parseSliceFlag(args, index, f)
				if err != nil {
					return 0, err
				}
				if f.Variadic {
					c.lastVariadicFlag = flagName
				}
				return consumed, nil
			} else {
				return 0, fmt.Errorf("non-bool flag -%s must be last in cluster", shortStr)
			}
		}
	}

	return consumed, nil
}
func (c *Cmd) assignPositional(value string) error {
	return c.assignPositionalWithMode(value, false)
}

func (c *Cmd) assignPositionalWithMode(value string, positionalOnlyMode bool) error {
	// Find next unassigned positional flag
	for _, name := range c.positional {
		flag := c.flags[name]

		// Check if it's positional-only or can be positional
		switch f := flag.(type) {
		case *StringFlag:
			if f.FlagOnly {
				continue
			}
			if c.configured[name] {
				continue // Already assigned
			}
			c.configured[name] = true
			return c.setStringValue(f, value)
		case *IntFlag:
			if f.FlagOnly {
				continue
			}
			if c.configured[name] {
				continue // Already assigned
			}
			c.configured[name] = true
			return c.setIntValue(f, value)
		case *Int64Flag:
			if f.FlagOnly {
				continue
			}
			if c.configured[name] {
				continue // Already assigned
			}
			c.configured[name] = true
			return c.setInt64Value(f, value)
		case *Float64Flag:
			if f.FlagOnly {
				continue
			}
			if c.configured[name] {
				continue // Already assigned
			}
			c.configured[name] = true
			return c.setFloat64Value(f, value)
		case *BoolFlag:
			if f.FlagOnly {
				continue
			}
			if c.configured[name] {
				continue // Already assigned
			}
			c.configured[name] = true
			val, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("invalid bool value for %s: %s", name, value)
			}
			*f.Value = val
			return nil
		case *StringSliceFlag:
			if f.FlagOnly {
				continue
			}
			if f.Variadic {
				// Variadic positional - collect if this is the current one or no flag seen since last variadic
				if c.lastVariadicFlag == name {
					c.configured[name] = true
					_, err := c.appendStringSliceValue(f, value)
					return err
				}
				// In positional-only mode (after --), continue appending to any configured variadic flag
				if positionalOnlyMode && c.configured[name] {
					_, err := c.appendStringSliceValue(f, value)
					return err
				}
				// If we saw a flag since last variadic, skip variadic flags that have already been used
				if c.sawFlag && c.configured[name] {
					continue
				}
				// Start new variadic only if we haven't seen a flag or this is a new variadic
				if !c.sawFlag || c.lastVariadicFlag == "" {
					c.configured[name] = true
					c.lastVariadicFlag = name
					_, err := c.appendStringSliceValue(f, value)
					return err
				}
				// Skip this variadic if we've seen a flag
				continue
			}
			if c.configured[name] {
				continue // Already assigned
			}
			c.configured[name] = true
			_, err := c.appendStringSliceValue(f, value)
			return err
		}
	}

	return fmt.Errorf("Too many positional arguments. Unused: [%s]", value)
}

func (c *Cmd) setStringValue(f *StringFlag, value string) error {
	if f.EnumConstraint != nil {
		valid := false
		for _, allowed := range *f.EnumConstraint {
			if value == allowed {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf(
				"Invalid '%s' value: %s (valid values: %s)",
				f.Name,
				value,
				strings.Join(*f.EnumConstraint, ", "),
			)
		}
	}

	if f.RegexConstraint != nil {
		if !f.RegexConstraint.MatchString(value) {
			return fmt.Errorf(
				"Invalid '%s' value: %s (must match regex: %s)",
				f.Name,
				value,
				f.RegexConstraint.String(),
			)
		}
	}

	*f.Value = value
	return nil
}

func (c *Cmd) setIntValue(f *IntFlag, value string) error {
	val, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("invalid integer value for %s: %s", f.Name, value)
	}

	if f.min != nil {
		inclusive := f.minInclusive == nil || *f.minInclusive // default to inclusive
		if (inclusive && val < *f.min) || (!inclusive && val <= *f.min) {
			if inclusive {
				return fmt.Errorf("'%s' value %d is < minimum %d", f.Name, val, *f.min)
			} else {
				return fmt.Errorf("'%s' value %d is <= minimum (exclusive) %d", f.Name, val, *f.min)
			}
		}
	}

	if f.max != nil {
		inclusive := f.maxInclusive == nil || *f.maxInclusive // default to inclusive
		if (inclusive && val > *f.max) || (!inclusive && val >= *f.max) {
			if inclusive {
				return fmt.Errorf("'%s' value %d is > maximum %d", f.Name, val, *f.max)
			} else {
				return fmt.Errorf("'%s' value %d is >= maximum (exclusive) %d", f.Name, val, *f.max)
			}
		}
	}

	*f.Value = val
	return nil
}

func (c *Cmd) setInt64Value(f *Int64Flag, value string) error {
	val, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid int64 value for %s: %s", f.Name, value)
	}

	if f.min != nil {
		inclusive := f.minInclusive == nil || *f.minInclusive // default to inclusive
		if (inclusive && val < *f.min) || (!inclusive && val <= *f.min) {
			if inclusive {
				return fmt.Errorf("'%s' value %d is < minimum %d", f.Name, val, *f.min)
			} else {
				return fmt.Errorf("'%s' value %d is <= minimum (exclusive) %d", f.Name, val, *f.min)
			}
		}
	}

	if f.max != nil {
		inclusive := f.maxInclusive == nil || *f.maxInclusive // default to inclusive
		if (inclusive && val > *f.max) || (!inclusive && val >= *f.max) {
			if inclusive {
				return fmt.Errorf("'%s' value %d is > maximum %d", f.Name, val, *f.max)
			} else {
				return fmt.Errorf("'%s' value %d is >= maximum (exclusive) %d", f.Name, val, *f.max)
			}
		}
	}

	*f.Value = val
	return nil
}

func (c *Cmd) setFloat64Value(f *Float64Flag, value string) error {
	val, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fmt.Errorf("invalid float64 value for %s: %s", f.Name, value)
	}

	if f.min != nil {
		inclusive := f.minInclusive == nil || *f.minInclusive // default to inclusive
		if (inclusive && val < *f.min) || (!inclusive && val <= *f.min) {
			if inclusive {
				return fmt.Errorf("'%s' value %g is < minimum %g", f.Name, val, *f.min)
			} else {
				return fmt.Errorf("'%s' value %g is <= minimum (exclusive) %g", f.Name, val, *f.min)
			}
		}
	}

	if f.max != nil {
		inclusive := f.maxInclusive == nil || *f.maxInclusive // default to inclusive
		if (inclusive && val > *f.max) || (!inclusive && val >= *f.max) {
			if inclusive {
				return fmt.Errorf("'%s' value %g is > maximum %g", f.Name, val, *f.max)
			} else {
				return fmt.Errorf("'%s' value %g is >= maximum (exclusive) %g", f.Name, val, *f.max)
			}
		}
	}

	*f.Value = val
	return nil
}

// parseBoolValue parses a string value as a boolean, supporting standard formats and 0/1
func (c *Cmd) parseBoolValue(value string) (bool, error) {
	val, err := strconv.ParseBool(value)
	if err != nil {
		// Try parsing as 0/1
		if value == "0" {
			return false, nil
		} else if value == "1" {
			return true, nil
		}
		return false, err
	}
	return val, nil
}

func (c *Cmd) parseSliceFlag(args []string, index int, f *StringSliceFlag) (int, error) {
	if !f.Variadic {
		// Single value
		if index+1 >= len(args) {
			return 1, nil // Empty slice
		}
		return c.appendStringSliceValue(f, args[index+1])
	}

	// Variadic - consume until next flag
	consumed := 1
	for i := index + 1; i < len(args); i++ {
		if strings.HasPrefix(args[i], "-") {
			break
		}
		if _, err := c.appendStringSliceValue(f, args[i]); err != nil {
			return 0, err
		}
		consumed++
	}

	return consumed, nil
}

func (c *Cmd) appendStringSliceValue(f *StringSliceFlag, value string) (int, error) {
	if f.Separator != nil {
		parts := strings.Split(value, *f.Separator)
		for _, part := range parts {
			*f.Value = append(*f.Value, part)
		}
	} else {
		*f.Value = append(*f.Value, value)
	}
	return 2, nil
}

func (c *Cmd) parseIntSliceFlag(args []string, index int, f *IntSliceFlag) (int, error) {
	if !f.Variadic {
		// Single value
		if index+1 >= len(args) {
			return 1, nil // Empty slice
		}
		return c.appendIntSliceValue(f, args[index+1])
	}

	// Variadic - consume until next flag
	consumed := 1
	for i := index + 1; i < len(args); i++ {
		if strings.HasPrefix(args[i], "-") {
			break
		}
		if _, err := c.appendIntSliceValue(f, args[i]); err != nil {
			return 0, err
		}
		consumed++
	}

	return consumed, nil
}

func (c *Cmd) appendIntSliceValue(f *IntSliceFlag, value string) (int, error) {
	if f.Separator != nil {
		parts := strings.Split(value, *f.Separator)
		for _, part := range parts {
			val, err := strconv.Atoi(part)
			if err != nil {
				return 0, fmt.Errorf("invalid integer value for %s: %s", f.Name, part)
			}
			*f.Value = append(*f.Value, val)
		}
	} else {
		val, err := strconv.Atoi(value)
		if err != nil {
			return 0, fmt.Errorf("invalid integer value for %s: %s", f.Name, value)
		}
		*f.Value = append(*f.Value, val)
	}
	return 2, nil
}

func (c *Cmd) parseInt64SliceFlag(args []string, index int, f *Int64SliceFlag) (int, error) {
	if !f.Variadic {
		// Single value
		if index+1 >= len(args) {
			return 1, nil // Empty slice
		}
		return c.appendInt64SliceValue(f, args[index+1])
	}

	// Variadic - consume until next flag
	consumed := 1
	for i := index + 1; i < len(args); i++ {
		if strings.HasPrefix(args[i], "-") {
			break
		}
		if _, err := c.appendInt64SliceValue(f, args[i]); err != nil {
			return 0, err
		}
		consumed++
	}

	return consumed, nil
}

func (c *Cmd) appendInt64SliceValue(f *Int64SliceFlag, value string) (int, error) {
	if f.Separator != nil {
		parts := strings.Split(value, *f.Separator)
		for _, part := range parts {
			val, err := strconv.ParseInt(part, 10, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid int64 value for %s: %s", f.Name, part)
			}
			*f.Value = append(*f.Value, val)
		}
	} else {
		val, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid int64 value for %s: %s", f.Name, value)
		}
		*f.Value = append(*f.Value, val)
	}
	return 2, nil
}

func (c *Cmd) parseFloat64SliceFlag(args []string, index int, f *Float64SliceFlag) (int, error) {
	if !f.Variadic {
		// Single value
		if index+1 >= len(args) {
			return 1, nil // Empty slice
		}
		return c.appendFloat64SliceValue(f, args[index+1])
	}

	// Variadic - consume until next flag
	consumed := 1
	for i := index + 1; i < len(args); i++ {
		if strings.HasPrefix(args[i], "-") {
			break
		}
		if _, err := c.appendFloat64SliceValue(f, args[i]); err != nil {
			return 0, err
		}
		consumed++
	}

	return consumed, nil
}

func (c *Cmd) appendFloat64SliceValue(f *Float64SliceFlag, value string) (int, error) {
	if f.Separator != nil {
		parts := strings.Split(value, *f.Separator)
		for _, part := range parts {
			val, err := strconv.ParseFloat(part, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid float64 value for %s: %s", f.Name, part)
			}
			*f.Value = append(*f.Value, val)
		}
	} else {
		val, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid float64 value for %s: %s", f.Name, value)
		}
		*f.Value = append(*f.Value, val)
	}
	return 2, nil
}

func (c *Cmd) parseBoolSliceFlag(args []string, index int, f *BoolSliceFlag) (int, error) {
	if !f.Variadic {
		// Single value
		if index+1 >= len(args) {
			return 1, nil // Empty slice
		}
		return c.appendBoolSliceValue(f, args[index+1])
	}

	// Variadic - consume until next flag
	consumed := 1
	for i := index + 1; i < len(args); i++ {
		if strings.HasPrefix(args[i], "-") {
			break
		}
		if _, err := c.appendBoolSliceValue(f, args[i]); err != nil {
			return 0, err
		}
		consumed++
	}

	return consumed, nil
}

func (c *Cmd) appendBoolSliceValue(f *BoolSliceFlag, value string) (int, error) {
	if f.Separator != nil {
		parts := strings.Split(value, *f.Separator)
		for _, part := range parts {
			val, err := strconv.ParseBool(part)
			if err != nil {
				// Try parsing as 0/1
				if part == "0" {
					val = false
				} else if part == "1" {
					val = true
				} else {
					return 0, fmt.Errorf("invalid bool value for %s: %s", f.Name, part)
				}
			}
			*f.Value = append(*f.Value, val)
		}
	} else {
		val, err := strconv.ParseBool(value)
		if err != nil {
			// Try parsing as 0/1
			if value == "0" {
				val = false
			} else if value == "1" {
				val = true
			} else {
				return 0, fmt.Errorf("invalid bool value for %s: %s", f.Name, value)
			}
		}
		*f.Value = append(*f.Value, val)
	}
	return 2, nil
}

// getAllFlagsInRegistrationOrder returns all flag names in registration order
// (positional flags first, then non-positional flags)
func (c *Cmd) getAllFlagsInRegistrationOrder() []string {
	allFlags := make([]string, 0, len(c.positional)+len(c.nonPositional))
	allFlags = append(allFlags, c.positional...)
	allFlags = append(allFlags, c.nonPositional...)
	return allFlags
}

func initializeColorFromEnv() {
	colorValue := strings.ToLower(strings.TrimSpace(os.Getenv("RA_COLOR")))
	switch colorValue {
	case "never":
		color.NoColor = true
	case "always":
		color.NoColor = false
	case "", "auto":
		// default behavior
		// let amterp/color decide based on tty
	default:
		// invalid value - treat as auto
	}
}

func (c *Cmd) validateBeforeParsing() error {
	// Validate that all constraint references point to real arg names
	return c.validateConstraintReferences()
}

func (c *Cmd) validateConstraintReferences() error {
	// Get all valid flag names
	validFlags := make(map[string]bool)
	for name := range c.flags {
		validFlags[name] = true
	}

	// Check all requires/excludes constraints
	for _, flag := range c.flags {
		var requires *[]string
		var excludes *[]string

		switch f := flag.(type) {
		case *StringFlag:
			requires = f.Requires
			excludes = f.Excludes
		case *IntFlag:
			requires = f.Requires
			excludes = f.Excludes
		case *Int64Flag:
			requires = f.Requires
			excludes = f.Excludes
		case *Float64Flag:
			requires = f.Requires
			excludes = f.Excludes
		case *BoolFlag:
			requires = f.Requires
			excludes = f.Excludes
		case *StringSliceFlag:
			requires = f.Requires
			excludes = f.Excludes
		case *IntSliceFlag:
			requires = f.Requires
			excludes = f.Excludes
		case *Int64SliceFlag:
			requires = f.Requires
			excludes = f.Excludes
		case *Float64SliceFlag:
			requires = f.Requires
			excludes = f.Excludes
		case *BoolSliceFlag:
			requires = f.Requires
			excludes = f.Excludes
		}

		// Validate requires constraints
		if requires != nil {
			for _, reqName := range *requires {
				if !validFlags[reqName] {
					return NewProgrammingError(fmt.Sprintf("Undefined flag '%s'", reqName))
				}
			}
		}

		// Validate excludes constraints
		if excludes != nil {
			for _, excName := range *excludes {
				if !validFlags[excName] {
					return NewProgrammingError(fmt.Sprintf("Undefined flag '%s'", excName))
				}
			}
		}
	}

	return nil
}

func (c *Cmd) validateRequired() error {
	// First pass: Check relational constraints (requires/excludes)
	// These are more specific and should take precedence over generic "required flag missing" errors
	// Iterate in registration order to ensure deterministic error messages
	for _, name := range c.getAllFlagsInRegistrationOrder() {
		flag, exists := c.flags[name]
		if !exists {
			continue
		}

		// Check requires constraints for flags that are configured for relational constraints
		if c.flagConfiguredForRelationalConstraints(name) {
			var requires *[]string
			switch f := flag.(type) {
			case *StringFlag:
				requires = f.Requires
			case *IntFlag:
				requires = f.Requires
			case *BoolFlag:
				requires = f.Requires
			case *Int64Flag:
				requires = f.Requires
			case *Float64Flag:
				requires = f.Requires
			case *SliceFlag[string]:
				requires = f.Requires
			case *SliceFlag[int]:
				requires = f.Requires
			case *SliceFlag[int64]:
				requires = f.Requires
			case *SliceFlag[float64]:
				requires = f.Requires
			case *SliceFlag[bool]:
				requires = f.Requires
			}

			if requires != nil {
				for _, req := range *requires {
					if !c.flagConfiguredForRelationalConstraints(req) {
						return fmt.Errorf("Invalid args: '%s' requires '%s', but '%s' was not set", name, req, req)
					}
				}
			}
		}

		// Check exclusion constraints using the new helper
		if err := c.checkExclusion(name); err != nil {
			return err
		}
	}

	// Check if any configured flags bypass validation
	for name := range c.configured {
		if flag, exists := c.flags[name]; exists {
			switch f := flag.(type) {
			case *BoolFlag:
				if f.BypassValidation {
					return nil // Skip validation entirely
				}
			case *StringFlag:
				if f.BypassValidation {
					return nil // Skip validation entirely
				}
			case *Int64Flag:
				if f.BypassValidation {
					return nil // Skip validation entirely
				}
			case *IntFlag:
				if f.BypassValidation {
					return nil // Skip validation entirely
				}
			case *Float64Flag:
				if f.BypassValidation {
					return nil // Skip validation entirely
				}
			case *SliceFlag[string]:
				if f.BypassValidation {
					return nil // Skip validation entirely
				}
			case *SliceFlag[int]:
				if f.BypassValidation {
					return nil // Skip validation entirely
				}
			case *SliceFlag[int64]:
				if f.BypassValidation {
					return nil // Skip validation entirely
				}
			case *SliceFlag[float64]:
				if f.BypassValidation {
					return nil // Skip validation entirely
				}
			case *SliceFlag[bool]:
				if f.BypassValidation {
					return nil // Skip validation entirely
				}
			}
		}
	}

	// Second pass: Check if required flags are missing
	// This runs after relational constraints so more specific errors take precedence
	// Check in registration order: positional flags first, then non-positional flags
	var missingRequired []string

	// Check positional flags first
	for _, name := range c.positional {
		if c.isFlagRequired(name) && !c.configured[name] && !c.isFlagExcludedByConfiguredFlag(name) {
			missingRequired = append(missingRequired, name)
		}
	}

	// Then check non-positional flags
	for _, name := range c.nonPositional {
		if c.isFlagRequired(name) && !c.configured[name] && !c.isFlagExcludedByConfiguredFlag(name) {
			missingRequired = append(missingRequired, name)
		}
	}

	if len(missingRequired) > 0 {
		return fmt.Errorf("Missing required arguments: [%s]", strings.Join(missingRequired, ", "))
	}
	return nil
}
func (c *Cmd) isFlagRequired(name string) bool {
	flag, exists := c.flags[name]
	if !exists {
		return false
	}

	switch f := flag.(type) {
	case *StringFlag:
		return !f.Optional && f.Default == nil
	case *IntFlag:
		return !f.Optional && f.Default == nil
	case *BoolFlag:
		// Boolean flags have implicit default of false, so never required
		return false
	case *Int64Flag:
		return !f.Optional && f.Default == nil
	case *Float64Flag:
		return !f.Optional && f.Default == nil
	case *SliceFlag[string]:
		// Variadic slice flags have implicit default of empty slice, so never required
		return !f.Variadic && !f.Optional && f.Default == nil
	case *SliceFlag[int]:
		// Variadic slice flags have implicit default of empty slice, so never required
		return !f.Variadic && !f.Optional && f.Default == nil
	case *SliceFlag[int64]:
		// Variadic slice flags have implicit default of empty slice, so never required
		return !f.Variadic && !f.Optional && f.Default == nil
	case *SliceFlag[float64]:
		// Variadic slice flags have implicit default of empty slice, so never required
		return !f.Variadic && !f.Optional && f.Default == nil
	case *SliceFlag[bool]:
		// Boolean slice flags have implicit default of empty slice, so never required
		return false
	}
	return false
}

// checkExclusion checks if a flag excludes or is excluded by another flag
func (c *Cmd) checkExclusion(flagName string) error {
	if !c.flagConfiguredForRelationalConstraints(flagName) {
		return nil
	}

	// Check if this flag excludes any other configured flags
	if flag, exists := c.flags[flagName]; exists {
		var excludes *[]string
		switch f := flag.(type) {
		case *StringFlag:
			excludes = f.Excludes
		case *IntFlag:
			excludes = f.Excludes
		case *BoolFlag:
			excludes = f.Excludes
		case *Int64Flag:
			excludes = f.Excludes
		case *Float64Flag:
			excludes = f.Excludes
		case *SliceFlag[string]:
			excludes = f.Excludes
		case *SliceFlag[int]:
			excludes = f.Excludes
		case *SliceFlag[int64]:
			excludes = f.Excludes
		case *SliceFlag[float64]:
			excludes = f.Excludes
		case *SliceFlag[bool]:
			excludes = f.Excludes
		}

		if excludes != nil {
			for _, excluded := range *excludes {
				if c.flagConfiguredForRelationalConstraints(excluded) {
					return fmt.Errorf(
						"Invalid args: '%s' excludes '%s', but '%s' was set",
						flagName,
						excluded,
						excluded,
					)
				}
			}
		}
	}

	// Check if any other configured flag excludes this flag
	for _, otherName := range c.getAllFlagsInRegistrationOrder() {
		if otherName == flagName || !c.flagConfiguredForRelationalConstraints(otherName) {
			continue
		}

		otherFlag, exists := c.flags[otherName]
		if !exists {
			continue
		}

		var otherExcludes *[]string
		switch f := otherFlag.(type) {
		case *StringFlag:
			otherExcludes = f.Excludes
		case *IntFlag:
			otherExcludes = f.Excludes
		case *BoolFlag:
			otherExcludes = f.Excludes
		case *Int64Flag:
			otherExcludes = f.Excludes
		case *Float64Flag:
			otherExcludes = f.Excludes
		case *SliceFlag[string]:
			otherExcludes = f.Excludes
		case *SliceFlag[int]:
			otherExcludes = f.Excludes
		case *SliceFlag[int64]:
			otherExcludes = f.Excludes
		case *SliceFlag[float64]:
			otherExcludes = f.Excludes
		case *SliceFlag[bool]:
			otherExcludes = f.Excludes
		}

		if otherExcludes != nil {
			for _, excluded := range *otherExcludes {
				if excluded == flagName {
					return fmt.Errorf(
						"Invalid args: '%s' excludes '%s', but '%s' was set",
						otherName,
						flagName,
						flagName,
					)
				}
			}
		}
	}

	return nil
}

// flagConfiguredForRelationalConstraints returns true if the flag should be considered configured
// for the purposes of relational constraints (requires/excludes).
// For boolean flags, this only returns true when the flag's value is true.
// For other flag types, this returns true if the flag has a value (configured or default).
func (c *Cmd) flagConfiguredForRelationalConstraints(name string) bool {
	if flag, exists := c.flags[name]; exists {
		switch f := flag.(type) {
		case *BoolFlag:
			// For boolean flags, only consider them configured for relational constraints when true
			if f.Value != nil && *f.Value {
				return true
			}
			return false
		default:
			// For all other flag types, use the normal flagHasValue logic
			return c.flagHasValue(name)
		}
	}
	return false
}

// flagHasValue returns true if the flag has a value (either configured by user or has a default)
func (c *Cmd) flagHasValue(name string) bool {
	// First check if it was explicitly configured
	if c.configured[name] {
		return true
	}

	// Then check if it has a default value
	if flag, exists := c.flags[name]; exists {
		switch f := flag.(type) {
		case *StringFlag:
			return f.Default != nil
		case *IntFlag:
			return f.Default != nil
		case *BoolFlag:
			return f.Default != nil
		case *Int64Flag:
			return f.Default != nil
		case *Float64Flag:
			return f.Default != nil
		case *SliceFlag[string]:
			return f.Default != nil
		case *SliceFlag[int]:
			return f.Default != nil
		case *SliceFlag[int64]:
			return f.Default != nil
		case *SliceFlag[float64]:
			return f.Default != nil
		case *SliceFlag[bool]:
			return f.Default != nil
		}
	}

	return false
}

// isFlagExcludedByConfiguredFlag returns true if the given flag is excluded by any configured flag
func (c *Cmd) isFlagExcludedByConfiguredFlag(flagName string) bool {
	// Check if any configured flag excludes the given flag
	for configuredFlagName, isConfigured := range c.configured {
		if !isConfigured || configuredFlagName == flagName {
			continue
		}

		// Check if this configured flag excludes the target flag
		if flag, exists := c.flags[configuredFlagName]; exists {
			var excludes *[]string
			switch f := flag.(type) {
			case *StringFlag:
				excludes = f.Excludes
			case *IntFlag:
				excludes = f.Excludes
			case *BoolFlag:
				// For boolean flags, only consider exclusion when the flag is true
				if f.Value != nil && *f.Value {
					excludes = f.Excludes
				}
			case *Int64Flag:
				excludes = f.Excludes
			case *Float64Flag:
				excludes = f.Excludes
			case *SliceFlag[string]:
				excludes = f.Excludes
			case *SliceFlag[int]:
				excludes = f.Excludes
			case *SliceFlag[int64]:
				excludes = f.Excludes
			case *SliceFlag[float64]:
				excludes = f.Excludes
			case *SliceFlag[bool]:
				excludes = f.Excludes
			}

			if excludes != nil {
				for _, excluded := range *excludes {
					if excluded == flagName {
						return true
					}
				}
			}
		}
	}
	return false
}

// hasRequiredFlags returns true if the command has any required flags
func (c *Cmd) hasRequiredFlags() bool {
	// Check positional flags first
	for _, name := range c.positional {
		if c.isFlagRequired(name) {
			return true
		}
	}

	// Then check non-positional flags
	for _, name := range c.nonPositional {
		if c.isFlagRequired(name) {
			return true
		}
	}

	return false
}
