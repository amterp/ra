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
			consumed, err := c.parseFlag(args, i, numberShortsMode, cfg)
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

				// Handle unknown flags
				if cfg.variadicUnknownFlags {
					// Check if we have an active variadic or an unassigned variadic positional.
					// This allows variadics to be "activated" in two ways:
					//   1. Already active (lastVariadicFlag != "") from consuming a previous value
					//   2. Newly activated by finding the first unassigned variadic positional
					// This enables: radd test.rad -U (where -U activates the variadic)
					variadicFlag := c.lastVariadicFlag
					if variadicFlag == "" {
						variadicFlag = c.findNextUnassignedVariadicPositional()
					}

					if variadicFlag != "" {
						// We have a variadic that can consume this unknown flag
						if err := c.assignPositional(arg); err != nil {
							// If variadic assignment fails, fall back to normal unknown handling
							if cfg.ignoreUnknown {
								c.unknownArgs = append(c.unknownArgs, arg)
							} else {
								return err
							}
						} else {
							// Successfully assigned - activate the variadic if it wasn't already
							c.lastVariadicFlag = variadicFlag
						}
						i++
						continue
					}
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

// findNextUnassignedVariadicPositional returns the name of the first unassigned variadic positional flag
// (in registration order), or an empty string if none exists
func (c *Cmd) findNextUnassignedVariadicPositional() string {
	for _, name := range c.positional {
		flag := c.flags[name]

		// Skip if already configured
		if c.configured[name] {
			continue
		}

		// Check if it's a variadic slice flag
		switch f := flag.(type) {
		case *StringSliceFlag:
			if f.Variadic && !f.FlagOnly {
				return name
			}
		case *IntSliceFlag:
			if f.Variadic && !f.FlagOnly {
				return name
			}
		case *Int64SliceFlag:
			if f.Variadic && !f.FlagOnly {
				return name
			}
		case *Float64SliceFlag:
			if f.Variadic && !f.FlagOnly {
				return name
			}
		case *BoolSliceFlag:
			if f.Variadic && !f.FlagOnly {
				return name
			}
		}
	}
	return ""
}

func (c *Cmd) parseFlag(args []string, index int, numberShortsMode bool, cfg *parseCfg) (int, error) {
	arg := args[index]

	if strings.HasPrefix(arg, "--") {
		// Long flag
		return c.parseLongFlag(args, index, cfg)
	} else if strings.HasPrefix(arg, "-") {
		// Short flag(s)
		return c.parseShortFlag(args, index, numberShortsMode, cfg)
	}

	return 0, fmt.Errorf("invalid flag: %s", arg)
}

func (c *Cmd) parseLongFlag(args []string, index int, cfg *parseCfg) (int, error) {
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
		// Before returning unknown flag error, check if help flags are present
		if c.helpEnabled && c.hasHelpFlags(args) {
			return 0, c.createHelpError(args)
		}
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
		consumed, err := c.parseSliceFlag(args, index, f, cfg)
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

func (c *Cmd) parseShortFlag(args []string, index int, numberShortsMode bool, cfg *parseCfg) (int, error) {
	arg := args[index]
	shorts := arg[1:] // remove -

	// Check for = syntax in short flags (e.g., -r=value)
	var value string
	var hasValue bool
	if idx := strings.Index(shorts, "="); idx != -1 {
		value = shorts[idx+1:]
		shorts = shorts[:idx]
		hasValue = true
	}

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
				// Single occurrence
				if hasValue {
					// Use equals value
					err := c.setIntValue(f, value)
					return 1, err
				} else {
					// Check if next argument exists and is a valid value (not a flag)
					if index+1 < len(args) && !strings.HasPrefix(args[index+1], "-") {
						// Next arg is a value, try to parse it
						err := c.setIntValue(f, args[index+1])
						return 2, err
					} else {
						// No value provided or next arg is a flag, treat as count of 1
						*f.Value = 1
						return 1, nil
					}
				}
			case *Int64Flag:
				if len(shorts) > 1 {
					// Multiple occurrences like -nnn
					count := len(shorts)
					*f.Value = int64(count)
					return 1, nil
				}
				// Single occurrence
				if hasValue {
					// Use equals value
					err := c.setInt64Value(f, value)
					return 1, err
				} else {
					// Check if next argument exists and is a valid value (not a flag)
					if index+1 < len(args) && !strings.HasPrefix(args[index+1], "-") {
						// Next arg is a value, try to parse it
						err := c.setInt64Value(f, args[index+1])
						return 2, err
					} else {
						// No value provided or next arg is a flag, treat as count of 1
						*f.Value = 1
						return 1, nil
					}
				}
			case *StringFlag:
				if hasValue {
					// Use equals value
					err := c.setStringValue(f, value)
					return 1, err
				} else {
					// Use next argument
					if index+1 >= len(args) {
						return 0, fmt.Errorf("flag -%s requires a value", shorts)
					}
					err := c.setStringValue(f, args[index+1])
					return 2, err
				}
			}
		}
	}

	// Handle equals syntax for short flags (e.g., -r=value or -fr=value)
	if hasValue {
		// For single short flag with equals, handle directly
		if len(shorts) == 1 {
			shortStr := string(shorts[0])
			flagName, exists := c.shortToName[shortStr]
			if !exists {
				return 0, fmt.Errorf("unknown shorthand flag: -%s", shortStr)
			}

			flag := c.flags[flagName]
			c.configured[flagName] = true

			switch f := flag.(type) {
			case *BoolFlag:
				val, err := c.parseBoolValue(value)
				if err != nil {
					return 0, fmt.Errorf("invalid value for flag -%s: %s", shortStr, err.Error())
				}
				*f.Value = val
				return 1, nil
			case *StringFlag:
				err := c.setStringValue(f, value)
				return 1, err
			case *IntFlag:
				err := c.setIntValue(f, value)
				return 1, err
			case *Int64Flag:
				err := c.setInt64Value(f, value)
				return 1, err
			case *Float64Flag:
				err := c.setFloat64Value(f, value)
				return 1, err
			case *StringSliceFlag:
				_, err := c.appendStringSliceValue(f, value)
				if err == nil && f.Variadic {
					c.lastVariadicFlag = flagName
				}
				return 1, err
			case *IntSliceFlag:
				_, err := c.appendIntSliceValue(f, value)
				return 1, err
			case *Int64SliceFlag:
				_, err := c.appendInt64SliceValue(f, value)
				return 1, err
			case *Float64SliceFlag:
				_, err := c.appendFloat64SliceValue(f, value)
				return 1, err
			case *BoolSliceFlag:
				_, err := c.appendBoolSliceValue(f, value)
				return 1, err
			}

			return 0, NewProgrammingError(fmt.Sprintf("unsupported flag type for: %s", flagName))
		}
		// For clustered flags with equals (e.g., -fr=value), fall through to regular clustering
		// but the equals value will be used by the last flag in the cluster
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
			// All chars are the same, check if it's an int or int64 flag
			if flagName, exists := c.shortToName[string(firstChar)]; exists {
				if flag, exists := c.flags[flagName]; exists {
					if intFlag, ok := flag.(*IntFlag); ok {
						c.configured[flagName] = true
						if hasValue {
							// Explicit equals value takes precedence over counting
							err := c.setIntValue(intFlag, value)
							return 1, err
						} else {
							// This is an int flag being repeated, set it to the count
							*intFlag.Value = len(shorts)
							return 1, nil
						}
					}
					if int64Flag, ok := flag.(*Int64Flag); ok {
						c.configured[flagName] = true
						if hasValue {
							// Explicit equals value takes precedence over counting
							err := c.setInt64Value(int64Flag, value)
							return 1, err
						} else {
							// This is an int64 flag being repeated, set it to the count
							*int64Flag.Value = int64(len(shorts))
							return 1, nil
						}
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
				if hasValue {
					// Use equals value
					err := c.setStringValue(f, value)
					if err != nil {
						return 0, err
					}
					consumed = 1
				} else {
					// Use next argument
					if index+1 >= len(args) {
						return 0, fmt.Errorf("flag -%s requires a value", shortStr)
					}
					err := c.setStringValue(f, args[index+1])
					if err != nil {
						return 0, err
					}
					consumed = 2
				}
			} else {
				return 0, fmt.Errorf("non-bool flag -%s must be last in cluster", shortStr)
			}
		case *IntFlag:
			if i == len(shorts)-1 {
				// Last flag in cluster, can take value
				if hasValue {
					// Use equals value
					err := c.setIntValue(f, value)
					if err != nil {
						return 0, err
					}
					consumed = 1
				} else {
					// Check if next argument exists and is a valid value (not a flag)
					if index+1 < len(args) && !strings.HasPrefix(args[index+1], "-") {
						// Next arg is a value, try to parse it
						err := c.setIntValue(f, args[index+1])
						if err != nil {
							return 0, err
						}
						consumed = 2
					} else {
						// No value provided or next arg is a flag, treat as count of 1
						*f.Value = 1
						consumed = 1
					}
				}
			} else {
				return 0, fmt.Errorf("non-bool flag -%s must be last in cluster", shortStr)
			}
		case *StringSliceFlag:
			if i == len(shorts)-1 {
				// Last flag in cluster, can take value
				if hasValue {
					// Use equals value
					_, err := c.appendStringSliceValue(f, value)
					if err != nil {
						return 0, err
					}
					if f.Variadic {
						c.lastVariadicFlag = flagName
					}
					consumed = 1
				} else {
					// Use parseSliceFlag for next argument(s)
					consumed, err := c.parseSliceFlag(args, index, f, cfg)
					if err != nil {
						return 0, err
					}
					if f.Variadic {
						c.lastVariadicFlag = flagName
					}
					return consumed, nil
				}
			} else {
				return 0, fmt.Errorf("non-bool flag -%s must be last in cluster", shortStr)
			}
		case *Int64Flag:
			if i == len(shorts)-1 {
				// Last flag in cluster, can take value
				if hasValue {
					// Use equals value
					err := c.setInt64Value(f, value)
					if err != nil {
						return 0, err
					}
					consumed = 1
				} else {
					// Check if next argument exists and is a valid value (not a flag)
					if index+1 < len(args) && !strings.HasPrefix(args[index+1], "-") {
						// Next arg is a value, try to parse it
						err := c.setInt64Value(f, args[index+1])
						if err != nil {
							return 0, err
						}
						consumed = 2
					} else {
						// No value provided or next arg is a flag, treat as count of 1
						*f.Value = 1
						consumed = 1
					}
				}
			} else {
				return 0, fmt.Errorf("non-bool flag -%s must be last in cluster", shortStr)
			}
		case *Float64Flag:
			if i == len(shorts)-1 {
				// Last flag in cluster, can take value
				if hasValue {
					// Use equals value
					err := c.setFloat64Value(f, value)
					if err != nil {
						return 0, err
					}
					consumed = 1
				} else {
					// Use next argument
					if index+1 >= len(args) {
						return 0, fmt.Errorf("flag -%s requires a value", shortStr)
					}
					err := c.setFloat64Value(f, args[index+1])
					if err != nil {
						return 0, err
					}
					consumed = 2
				}
			} else {
				return 0, fmt.Errorf("non-bool flag -%s must be last in cluster", shortStr)
			}
		case *IntSliceFlag:
			if i == len(shorts)-1 {
				// Last flag in cluster, can take value
				if hasValue {
					// Use equals value
					_, err := c.appendIntSliceValue(f, value)
					if err != nil {
						return 0, err
					}
					consumed = 1
				} else {
					// Use parseIntSliceFlag for next argument(s)
					consumed, err := c.parseIntSliceFlag(args, index, f)
					if err != nil {
						return 0, err
					}
					return consumed, nil
				}
			} else {
				return 0, fmt.Errorf("non-bool flag -%s must be last in cluster", shortStr)
			}
		case *Int64SliceFlag:
			if i == len(shorts)-1 {
				// Last flag in cluster, can take value
				if hasValue {
					// Use equals value
					_, err := c.appendInt64SliceValue(f, value)
					if err != nil {
						return 0, err
					}
					consumed = 1
				} else {
					// Use parseInt64SliceFlag for next argument(s)
					consumed, err := c.parseInt64SliceFlag(args, index, f)
					if err != nil {
						return 0, err
					}
					return consumed, nil
				}
			} else {
				return 0, fmt.Errorf("non-bool flag -%s must be last in cluster", shortStr)
			}
		case *Float64SliceFlag:
			if i == len(shorts)-1 {
				// Last flag in cluster, can take value
				if hasValue {
					// Use equals value
					_, err := c.appendFloat64SliceValue(f, value)
					if err != nil {
						return 0, err
					}
					consumed = 1
				} else {
					// Use parseFloat64SliceFlag for next argument(s)
					consumed, err := c.parseFloat64SliceFlag(args, index, f)
					if err != nil {
						return 0, err
					}
					return consumed, nil
				}
			} else {
				return 0, fmt.Errorf("non-bool flag -%s must be last in cluster", shortStr)
			}
		case *BoolSliceFlag:
			if i == len(shorts)-1 {
				// Last flag in cluster, can take value
				if hasValue {
					// Use equals value
					_, err := c.appendBoolSliceValue(f, value)
					if err != nil {
						return 0, err
					}
					consumed = 1
				} else {
					// Use parseBoolSliceFlag for next argument(s)
					consumed, err := c.parseBoolSliceFlag(args, index, f)
					if err != nil {
						return 0, err
					}
					return consumed, nil
				}
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
				handled, err := c.handleVariadicSliceFlag(name, value, positionalOnlyMode, func() error {
					_, err := c.appendStringSliceValue(f, value)
					return err
				})
				if handled {
					return err
				}
				continue // Skip this variadic if not handled
			}
			if c.configured[name] {
				continue // Already assigned
			}
			c.configured[name] = true
			_, err := c.appendStringSliceValue(f, value)
			return err
		case *IntSliceFlag:
			if f.FlagOnly {
				continue
			}
			if f.Variadic {
				handled, err := c.handleVariadicSliceFlag(name, value, positionalOnlyMode, func() error {
					_, err := c.appendIntSliceValue(f, value)
					return err
				})
				if handled {
					return err
				}
				continue // Skip this variadic if not handled
			}
			if c.configured[name] {
				continue // Already assigned
			}
			c.configured[name] = true
			_, err := c.appendIntSliceValue(f, value)
			return err
		case *Int64SliceFlag:
			if f.FlagOnly {
				continue
			}
			if f.Variadic {
				handled, err := c.handleVariadicSliceFlag(name, value, positionalOnlyMode, func() error {
					_, err := c.appendInt64SliceValue(f, value)
					return err
				})
				if handled {
					return err
				}
				continue // Skip this variadic if not handled
			}
			if c.configured[name] {
				continue // Already assigned
			}
			c.configured[name] = true
			_, err := c.appendInt64SliceValue(f, value)
			return err
		case *Float64SliceFlag:
			if f.FlagOnly {
				continue
			}
			if f.Variadic {
				handled, err := c.handleVariadicSliceFlag(name, value, positionalOnlyMode, func() error {
					_, err := c.appendFloat64SliceValue(f, value)
					return err
				})
				if handled {
					return err
				}
				continue // Skip this variadic if not handled
			}
			if c.configured[name] {
				continue // Already assigned
			}
			c.configured[name] = true
			_, err := c.appendFloat64SliceValue(f, value)
			return err
		case *BoolSliceFlag:
			if f.FlagOnly {
				continue
			}
			if f.Variadic {
				handled, err := c.handleVariadicSliceFlag(name, value, positionalOnlyMode, func() error {
					_, err := c.appendBoolSliceValue(f, value)
					return err
				})
				if handled {
					return err
				}
				continue // Skip this variadic if not handled
			}
			if c.configured[name] {
				continue // Already assigned
			}
			c.configured[name] = true
			_, err := c.appendBoolSliceValue(f, value)
			return err
		}
	}

	// Check if we have boolean flags that might have been intended to take this value
	boolFlagNames := []string{}
	for name, flag := range c.flags {
		if _, isBool := flag.(*BoolFlag); isBool {
			boolFlagNames = append(boolFlagNames, name)
		}
	}

	return fmt.Errorf("Too many positional arguments. Unused: [%s]", value)
}

// handleVariadicSliceFlag handles the common logic for variadic slice flags
// Returns (handled, error) where handled indicates if the flag was processed
func (c *Cmd) handleVariadicSliceFlag(
	name string,
	value string,
	positionalOnlyMode bool,
	appendFunc func() error,
) (bool, error) {
	// Variadic positional - collect if this is the current one or no flag seen since last variadic
	if c.lastVariadicFlag == name {
		c.configured[name] = true
		return true, appendFunc()
	}
	// In positional-only mode (after --), continue appending to any configured variadic flag
	if positionalOnlyMode && c.configured[name] {
		return true, appendFunc()
	}
	// If we saw a flag since last variadic, skip variadic flags that have already been used
	if c.sawFlag && c.configured[name] {
		return false, nil // Skip this variadic
	}
	// Start new variadic only if we haven't seen a flag or this is a new variadic
	if !c.sawFlag || c.lastVariadicFlag == "" {
		c.configured[name] = true
		c.lastVariadicFlag = name
		return true, appendFunc()
	}
	// Skip this variadic if we've seen a flag
	return false, nil
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
	// Parse as int64 first to detect overflow
	val64, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid integer value for %s: %s", f.Name, value)
	}

	// Check for platform-specific int overflow
	if val64 < int64(int(^uint(0)>>1)*-1-1) || val64 > int64(int(^uint(0)>>1)) {
		return fmt.Errorf("integer overflow for %s: %s (value exceeds platform int range)", f.Name, value)
	}

	val := int(val64)

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

func (c *Cmd) parseSliceFlag(args []string, index int, f *StringSliceFlag, cfg *parseCfg) (int, error) {
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
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			// Check if this might be an unknown flag that should be collected
			if cfg.variadicUnknownFlags {
				// Try to parse as flag - if it fails, it's unknown and should be collected
				_, err := c.parseFlag(args, i, false, cfg)
				if err != nil {
					// Unknown flag - collect it into variadic
					if _, err := c.appendStringSliceValue(f, arg); err != nil {
						return 0, err
					}
					consumed++
					continue
				}
			}
			// Known flag or not collecting unknown flags - stop variadic collection
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
	// Check if this is the first user-provided value and we should replace defaults
	shouldReplace := false
	if f.Default != nil {
		// Check if current value equals the default value - if so, this is the first user input
		if len(*f.Value) == len(*f.Default) {
			equal := true
			for i, v := range *f.Value {
				if v != (*f.Default)[i] {
					equal = false
					break
				}
			}
			if equal {
				shouldReplace = true
			}
		}
	}

	if f.Separator != nil {
		parts := strings.Split(value, *f.Separator)
		if shouldReplace {
			*f.Value = make([]string, 0, len(parts))
		}
		for _, part := range parts {
			*f.Value = append(*f.Value, part)
		}
	} else {
		if shouldReplace {
			*f.Value = make([]string, 0, 1)
		}
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
	// Check if this is the first user-provided value and we should replace defaults
	shouldReplace := false
	if f.Default != nil {
		// Check if current value equals the default value - if so, this is the first user input
		if len(*f.Value) == len(*f.Default) {
			equal := true
			for i, v := range *f.Value {
				if v != (*f.Default)[i] {
					equal = false
					break
				}
			}
			if equal {
				shouldReplace = true
			}
		}
	}

	if f.Separator != nil {
		parts := strings.Split(value, *f.Separator)
		if shouldReplace {
			*f.Value = make([]int, 0, len(parts))
		}
		for _, part := range parts {
			val, err := strconv.Atoi(part)
			if err != nil {
				return 0, fmt.Errorf("invalid integer value for %s: %s", f.Name, part)
			}
			*f.Value = append(*f.Value, val)
		}
	} else {
		if shouldReplace {
			*f.Value = make([]int, 0, 1)
		}
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
	// Check if this is the first user-provided value and we should replace defaults
	shouldReplace := false
	if f.Default != nil {
		// Check if current value equals the default value - if so, this is the first user input
		if len(*f.Value) == len(*f.Default) {
			equal := true
			for i, v := range *f.Value {
				if v != (*f.Default)[i] {
					equal = false
					break
				}
			}
			if equal {
				shouldReplace = true
			}
		}
	}

	if f.Separator != nil {
		parts := strings.Split(value, *f.Separator)
		if shouldReplace {
			*f.Value = make([]int64, 0, len(parts))
		}
		for _, part := range parts {
			val, err := strconv.ParseInt(part, 10, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid int64 value for %s: %s", f.Name, part)
			}
			*f.Value = append(*f.Value, val)
		}
	} else {
		if shouldReplace {
			*f.Value = make([]int64, 0, 1)
		}
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
	// Check if this is the first user-provided value and we should replace defaults
	shouldReplace := false
	if f.Default != nil {
		// Check if current value equals the default value - if so, this is the first user input
		if len(*f.Value) == len(*f.Default) {
			equal := true
			for i, v := range *f.Value {
				if v != (*f.Default)[i] {
					equal = false
					break
				}
			}
			if equal {
				shouldReplace = true
			}
		}
	}

	if f.Separator != nil {
		parts := strings.Split(value, *f.Separator)
		if shouldReplace {
			*f.Value = make([]float64, 0, len(parts))
		}
		for _, part := range parts {
			val, err := strconv.ParseFloat(part, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid float64 value for %s: %s", f.Name, part)
			}
			*f.Value = append(*f.Value, val)
		}
	} else {
		if shouldReplace {
			*f.Value = make([]float64, 0, 1)
		}
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
	// Check if this is the first user-provided value and we should replace defaults
	shouldReplace := false
	if f.Default != nil {
		// Check if current value equals the default value - if so, this is the first user input
		if len(*f.Value) == len(*f.Default) {
			equal := true
			for i, v := range *f.Value {
				if v != (*f.Default)[i] {
					equal = false
					break
				}
			}
			if equal {
				shouldReplace = true
			}
		}
	}

	if f.Separator != nil {
		parts := strings.Split(value, *f.Separator)
		if shouldReplace {
			*f.Value = make([]bool, 0, len(parts))
		}
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
		if shouldReplace {
			*f.Value = make([]bool, 0, 1)
		}
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

// hasHelpFlags checks if help flags are present in args
func (c *Cmd) hasHelpFlags(args []string) bool {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			return true
		}
	}
	return false
}

// createHelpError creates a help invoked error, determining if it's long or short help
func (c *Cmd) createHelpError(args []string) error {
	isLongHelp := false
	for _, arg := range args {
		if arg == "--help" {
			isLongHelp = true
			break
		}
	}

	return &helpInvokedError{
		output:         "", // Will be generated later, after PostParse hook
		exitCode:       0,
		useStdout:      true,
		isLongHelp:     isLongHelp,
		isAutoHelp:     false,
		useCustomUsage: c.customUsage != nil,
	}
}
