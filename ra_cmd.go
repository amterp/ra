package ra

import (
	"fmt"
)

type UsageHeaders struct {
	Usage         string
	Commands      string
	Arguments     string
	GlobalOptions string
}

func DefaultUsageHeaders() UsageHeaders {
	return UsageHeaders{
		Usage:         "Usage:",
		Commands:      "Commands:",
		Arguments:     "Arguments:",
		GlobalOptions: "Global options:",
	}
}

type ParseHooks struct {
	PostParse func(cmd *Cmd, err error) // Called after parsing, before any output
}

type Cmd struct {
	name                  string
	description           string
	flags                 map[string]any  // flag name -> flag itself (either a Flag[T] or SliceFlag[T])
	positional            []string        // positional flags, i.e. flags that are positional args
	nonPositional         []string        // non-positional flags, i.e. flags that are only named
	globalFlags           []string        // flags that will be applied to all subcommands
	overriddenGlobalFlags map[string]any  // global flags that were overridden by non-global flags (name collisions)
	shadowedShortFlags    map[string]bool // global flags that lost their short flag to non-global flags (short collisions)
	shadowedNameFlags     map[string]bool // global flags that lost their name to non-global flags (name collisions)
	subCmds               map[string]*Cmd
	shortToName           map[string]string // short flag -> full name mapping

	// options
	customUsage          func(bool)    // if set, this function will be called to print usage instead of the default
	parseHooks           *ParseHooks   // if set, hooks will be called after parsing
	helpEnabled          bool          // default true automatically adds a help flag
	excludeNameFromUsage bool          // if true, this command will not be included in usage output
	autoHelpOnNoArgs     bool          // if true, show help when no args provided and required args exist
	usageHeaders         *UsageHeaders // custom headers for usage output

	// state post-parse
	used             *bool           // after parsing, whether this command was invoked
	configured       map[string]bool // specified flags from flags.
	unknownArgs      []string        // unknown args when ignoreUnknown is true
	lastVariadicFlag string          // last variadic flag that was used
	sawFlag          bool            // true if we've seen a flag since the last variadic
}

func NewCmd(name string) *Cmd {
	c := &Cmd{
		name:                  name,
		flags:                 make(map[string]any),
		positional:            []string{},
		overriddenGlobalFlags: make(map[string]any),
		shadowedShortFlags:    make(map[string]bool),
		shadowedNameFlags:     make(map[string]bool),
		subCmds:               make(map[string]*Cmd),
		configured:            make(map[string]bool),
		helpEnabled:           true,
		shortToName:           make(map[string]string),
	}

	return c
}

func (c *Cmd) SetDescription(desc string) *Cmd {
	c.description = desc
	return c
}

func (c *Cmd) SetCustomUsage(fn func(isLongHelp bool)) *Cmd {
	c.customUsage = fn
	return c
}

func (c *Cmd) SetParseHooks(hooks *ParseHooks) *Cmd {
	c.parseHooks = hooks
	return c
}

func (c *Cmd) SetHelpEnabled(enable bool) *Cmd {
	c.helpEnabled = enable
	return c
}

func (c *Cmd) SetExcludeNameFromUsage(exclude bool) *Cmd {
	c.excludeNameFromUsage = exclude
	return c
}

func (c *Cmd) SetAutoHelpOnNoArgs(enable bool) *Cmd {
	c.autoHelpOnNoArgs = enable
	return c
}

func (c *Cmd) SetUsageHeaders(headers UsageHeaders) *Cmd {
	c.usageHeaders = &headers
	return c
}

func (c *Cmd) getUsageHeaders() UsageHeaders {
	if c.usageHeaders != nil {
		return *c.usageHeaders
	}
	return DefaultUsageHeaders()
}

func (c *Cmd) applyGlobalFlags(subCmd *Cmd) error {
	// Apply global flags
	for _, globalFlagName := range c.globalFlags {
		var flag any
		var exists bool

		// Check if we have an overridden version (original with short intact)
		if overriddenFlag, overriddenExists := c.overriddenGlobalFlags[globalFlagName]; overriddenExists {
			flag = overriddenFlag
			exists = true
		} else {
			flag, exists = c.flags[globalFlagName]
		}

		if exists {
			// Only add flag if it doesn't already exist in subcommand
			if _, exists := subCmd.flags[globalFlagName]; !exists {
				subCmd.flags[globalFlagName] = flag
				if base := getBaseFlag(flag); base != nil && base.Short != "" {
					subCmd.shortToName[base.Short] = base.Name
				}
				// Also add to subcommand's global flags list and non-positional list
				subCmd.globalFlags = append(subCmd.globalFlags, globalFlagName)
				subCmd.nonPositional = append(subCmd.nonPositional, globalFlagName)
			}
		}
	}

	return nil
}

// ResetParseState resets all parsing-related state to a clean slate, allowing the command
// to be parsed again from scratch.
//
// ADVANCED: This is for multi-parse scenarios. Most applications should parse once.
// All flag values are reset to defaults - cache any needed values before calling this.
func (c *Cmd) ResetParseState() {
	if c.used != nil {
		*c.used = false
	}
	c.configured = make(map[string]bool)
	c.unknownArgs = []string{}
	c.lastVariadicFlag = ""
	c.sawFlag = false

	// Reset all flag values to their defaults
	_ = c.setDefaults()

	// Recursively reset all subcommands
	for _, subCmd := range c.subCmds {
		if subCmd.used != nil {
			*subCmd.used = false
		}
		subCmd.ResetParseState()
	}
}

// Whether a flag was explicitly configured by the user.
func (c *Cmd) Configured(name string) bool {
	// Check if flag is configured in this command
	if configured, exists := c.configured[name]; exists && configured {
		return true
	}

	// Check all invoked subcommands recursively
	for _, subCmd := range c.subCmds {
		if subCmd.used != nil && *subCmd.used {
			if subCmd.Configured(name) {
				return true
			}
		}
	}

	return false
}

func (c *Cmd) GetUnknownArgs() []string {
	return c.unknownArgs
}

func (c *Cmd) RegisterCmd(subCmd *Cmd) (*bool, error) {
	if _, exists := c.subCmds[subCmd.name]; exists {
		return nil, fmt.Errorf("command %q already defined", subCmd.name)
	}

	c.subCmds[subCmd.name] = subCmd
	subCmd.used = new(bool)

	// Apply global flags to subcommand for usage generation
	if err := c.applyGlobalFlags(subCmd); err != nil {
		return nil, err
	}

	return subCmd.used, nil
}

func (c *Cmd) validatePositionalOnlyAfterVariadic(flagName string) error {
	// Check if there's already a variadic positional flag
	for _, existingName := range c.positional {
		existingFlag := c.flags[existingName]

		// Check if this existing flag is variadic
		switch f := existingFlag.(type) {
		case *StringSliceFlag:
			if f.Variadic {
				return fmt.Errorf("cannot register positional-only flag %q after variadic positional flag %q (positional-only flags cannot be set after variadic flags)", flagName, existingName)
			}
		case *IntSliceFlag:
			if f.Variadic {
				return fmt.Errorf("cannot register positional-only flag %q after variadic positional flag %q (positional-only flags cannot be set after variadic flags)", flagName, existingName)
			}
		case *Int64SliceFlag:
			if f.Variadic {
				return fmt.Errorf("cannot register positional-only flag %q after variadic positional flag %q (positional-only flags cannot be set after variadic flags)", flagName, existingName)
			}
		case *Float64SliceFlag:
			if f.Variadic {
				return fmt.Errorf("cannot register positional-only flag %q after variadic positional flag %q (positional-only flags cannot be set after variadic flags)", flagName, existingName)
			}
		case *BoolSliceFlag:
			if f.Variadic {
				return fmt.Errorf("cannot register positional-only flag %q after variadic positional flag %q (positional-only flags cannot be set after variadic flags)", flagName, existingName)
			}
		}
	}
	return nil
}

// checkForGlobalFlagOverride checks if a non-global flag can override an existing global flag.
// Returns true if the override is allowed, false if not allowed.
func (c *Cmd) checkForGlobalFlagOverride(flagName string, flagShort string, isGlobal bool) (bool, error) {
	// Check for name collision
	if existingFlag, exists := c.flags[flagName]; exists {
		// Allow non-global flag to override global flag
		if !isGlobal {
			// Check if existing flag is global
			isExistingGlobal := false
			for _, globalFlagName := range c.globalFlags {
				if globalFlagName == flagName {
					isExistingGlobal = true
					break
				}
			}
			if isExistingGlobal {
				// Check if the non-global flag also conflicts on short
				base := getBaseFlag(existingFlag)
				hasShortConflict := base != nil && base.Short != "" && flagShort == base.Short

				if hasShortConflict {
					// Both name and short conflict - let non-global completely override
					c.overriddenGlobalFlags[flagName] = existingFlag

					// Remove short flag mapping
					if base.Short != "" {
						delete(c.shortToName, base.Short)
					}

					// Remove from positional/nonPositional lists
					for i, name := range c.positional {
						if name == flagName {
							c.positional = append(c.positional[:i], c.positional[i+1:]...)
							break
						}
					}
					for i, name := range c.nonPositional {
						if name == flagName {
							c.nonPositional = append(c.nonPositional[:i], c.nonPositional[i+1:]...)
							break
						}
					}

					return true, nil
				} else {
					// Only name conflicts - apply name shadowing logic
					c.overriddenGlobalFlags[flagName] = existingFlag
					c.shadowedNameFlags[flagName] = true

					// Store the global flag under a special key so it can still be parsed
					if base != nil && base.Short != "" {
						// Update shortToName to point to the short key for the global flag
						c.shortToName[base.Short] = base.Short // "-v" maps to "v"
						// Store global flag under its short name
						c.flags[base.Short] = existingFlag
					}
					// The non-global flag will be stored under the original name "verbose"

					// Remove the global flag from positional/nonPositional lists
					for i, name := range c.positional {
						if name == flagName {
							c.positional = append(c.positional[:i], c.positional[i+1:]...)
							break
						}
					}
					for i, name := range c.nonPositional {
						if name == flagName {
							c.nonPositional = append(c.nonPositional[:i], c.nonPositional[i+1:]...)
							break
						}
					}

					return true, nil
				}
			} else {
				return false, fmt.Errorf("flag %q already defined", flagName)
			}
		} else {
			return false, fmt.Errorf("flag %q already defined", flagName)
		}
	}

	// Check for short flag collision if we have a short flag
	if flagShort != "" && !isGlobal {
		if existingFlagName, exists := c.shortToName[flagShort]; exists {
			// Check if the existing flag with this short is global
			isExistingGlobal := false
			for _, globalFlagName := range c.globalFlags {
				if globalFlagName == existingFlagName {
					isExistingGlobal = true
					break
				}
			}
			if isExistingGlobal {
				// For short flag collisions, we need to keep the global flag as global
				// but remove its short flag from the parent command

				// Store the original global flag (with short) for subcommands
				existingFlag := c.flags[existingFlagName]
				c.overriddenGlobalFlags[existingFlagName] = existingFlag

				// Track that this global flag had its short shadowed
				c.shadowedShortFlags[existingFlagName] = true

				// Remove the short flag mapping - global flag will only be available by full name
				delete(c.shortToName, flagShort)

				// Create a copy of the global flag without the short for the parent command
				flagCopy := deepCopyFlag(existingFlag)
				if base := getBaseFlag(flagCopy); base != nil {
					base.Short = "" // Remove short flag from the parent command's copy
				}
				c.flags[existingFlagName] = flagCopy

				return true, nil
			} else {
				return false, fmt.Errorf("short flag %q already defined", flagShort)
			}
		}
	}

	return false, nil
}
