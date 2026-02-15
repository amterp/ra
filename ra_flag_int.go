package ra

import "fmt"

type IntFlag struct {
	Flag[int]
	min          *int
	max          *int
	minInclusive *bool
	maxInclusive *bool
}

func NewInt(name string) *IntFlag {
	return &IntFlag{Flag: Flag[int]{BaseFlag: BaseFlag{Name: name, Optional: false}}}
}

func (f *IntFlag) SetShort(s string) *IntFlag {
	f.Short = s
	return f
}

func (f *IntFlag) SetUsage(u string) *IntFlag {
	f.Usage = u
	return f
}

func (f *IntFlag) SetDefault(v int) *IntFlag {
	f.Default = &v
	return f
}

func (f *IntFlag) SetOptional(b bool) *IntFlag {
	f.Optional = b
	return f
}

func (f *IntFlag) SetHidden(b bool) *IntFlag {
	f.Hidden = b
	return f
}

func (f *IntFlag) SetHiddenInShortHelp(b bool) *IntFlag {
	f.HiddenInShortHelp = b
	return f
}

func (f *IntFlag) SetPositionalOnly(b bool) *IntFlag {
	f.PositionalOnly = b
	return f
}

func (f *IntFlag) SetFlagOnly(b bool) *IntFlag {
	f.FlagOnly = b
	return f
}

func (f *IntFlag) SetExcludes(flags []string) *IntFlag {
	f.Excludes = &flags
	return f
}

func (f *IntFlag) SetRequires(flags []string) *IntFlag {
	f.Requires = &flags
	return f
}

func (f *IntFlag) SetMin(min int, inclusive bool) *IntFlag {
	f.min = &min
	f.minInclusive = &inclusive
	return f
}

func (f *IntFlag) SetMax(max int, inclusive bool) *IntFlag {
	f.max = &max
	f.maxInclusive = &inclusive
	return f
}

func (f *IntFlag) SetCustomUsageType(customType string) *IntFlag {
	f.CustomUsageType = customType
	return f
}

func (f *IntFlag) SetCompletionFunc(fn CompletionFunc) *IntFlag {
	f.CompletionFunc = fn
	return f
}

func (f *IntFlag) Register(cmd *Cmd, opts ...RegisterOption) (*int, error) {
	ptr := new(int)
	return ptr, f.RegisterWithPtr(cmd, ptr, opts...)
}

func (f *IntFlag) RegisterWithPtr(cmd *Cmd, ptr *int, opts ...RegisterOption) error {
	regConf := &registerConfig{}
	for _, opt := range opts {
		opt(regConf)
	}

	// Validate flag name is not empty
	if f.Name == "" {
		return fmt.Errorf("flag name cannot be empty")
	}

	// Validate mutually exclusive configuration
	if f.PositionalOnly && f.FlagOnly {
		return fmt.Errorf("flag %q cannot be both PositionalOnly and FlagOnly (mutually exclusive)", f.Name)
	}

	// Validate default value against constraints
	if f.Default != nil {
		if err := f.validateDefaultValue(*f.Default); err != nil {
			return fmt.Errorf("invalid default value for flag %q: %w", f.Name, err)
		}
	}

	if _, err := cmd.checkForGlobalFlagOverride(f.Name, f.Short, regConf.global); err != nil {
		return err
	}

	if regConf.global {
		cmd.globalFlags = append(cmd.globalFlags, f.Name)
	}

	// Create copy and set value pointer
	flag := *f
	flag.Value = ptr
	flag.BypassValidation = regConf.bypassValidation

	// Global flags should be flag-only (not positional)
	if regConf.global {
		flag.FlagOnly = true
	}

	// Add to short mapping
	if f.Short != "" {
		if _, exists := cmd.shortToName[f.Short]; exists {
			return fmt.Errorf("short flag %q already defined", f.Short)
		}
		cmd.shortToName[f.Short] = f.Name
	}

	cmd.flags[f.Name] = &flag
	if !flag.FlagOnly {
		// Check for positional-only after variadic error
		if flag.PositionalOnly {
			if err := cmd.validatePositionalOnlyAfterVariadic(f.Name); err != nil {
				return err
			}
		}
		cmd.positional = append(cmd.positional, f.Name)
	} else {
		cmd.nonPositional = append(cmd.nonPositional, f.Name)
	}

	return nil
}

// validateDefaultValue validates that a default value satisfies all constraints
func (f *IntFlag) validateDefaultValue(value int) error {
	// Check min constraint
	if f.min != nil {
		inclusive := f.minInclusive == nil || *f.minInclusive // default to inclusive
		if (inclusive && value < *f.min) || (!inclusive && value <= *f.min) {
			if inclusive {
				return fmt.Errorf("value %d is < minimum %d", value, *f.min)
			} else {
				return fmt.Errorf("value %d is <= minimum (exclusive) %d", value, *f.min)
			}
		}
	}

	// Check max constraint
	if f.max != nil {
		inclusive := f.maxInclusive == nil || *f.maxInclusive // default to inclusive
		if (inclusive && value > *f.max) || (!inclusive && value >= *f.max) {
			if inclusive {
				return fmt.Errorf("value %d is > maximum %d", value, *f.max)
			} else {
				return fmt.Errorf("value %d is >= maximum (exclusive) %d", value, *f.max)
			}
		}
	}

	return nil
}
