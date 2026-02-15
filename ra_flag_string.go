package ra

import (
	"fmt"
	"regexp"
)

type StringFlag struct {
	Flag[string]
	EnumConstraint  *[]string      // if set, the value must be one of these
	RegexConstraint *regexp.Regexp // if set, the value must match this regex
}

func NewString(name string) *StringFlag {
	return &StringFlag{Flag: Flag[string]{BaseFlag: BaseFlag{Name: name, Optional: false}}}
}
func (f *StringFlag) SetShort(s string) *StringFlag {
	f.Short = s
	return f
}

func (f *StringFlag) SetUsage(u string) *StringFlag {
	f.Usage = u
	return f
}

func (f *StringFlag) SetDefault(v string) *StringFlag {
	f.Default = &v
	return f
}

func (f *StringFlag) SetOptional(b bool) *StringFlag {
	f.Optional = b
	return f
}

func (f *StringFlag) SetHidden(b bool) *StringFlag {
	f.Hidden = b
	return f
}

func (f *StringFlag) SetHiddenInShortHelp(b bool) *StringFlag {
	f.HiddenInShortHelp = b
	return f
}

func (f *StringFlag) SetPositionalOnly(b bool) *StringFlag {
	f.PositionalOnly = b
	return f
}

func (f *StringFlag) SetFlagOnly(b bool) *StringFlag {
	f.FlagOnly = b
	return f
}

func (f *StringFlag) SetExcludes(flags []string) *StringFlag {
	f.Excludes = &flags
	return f
}

func (f *StringFlag) SetRequires(flags []string) *StringFlag {
	f.Requires = &flags
	return f
}

func (f *StringFlag) SetEnumConstraint(values []string) *StringFlag {
	if len(values) == 0 {
		f.EnumConstraint = nil
	} else {
		f.EnumConstraint = &values
	}
	return f
}

func (f *StringFlag) SetRegexConstraint(regex *regexp.Regexp) *StringFlag {
	f.RegexConstraint = regex
	return f
}

func (f *StringFlag) SetCustomUsageType(customType string) *StringFlag {
	f.CustomUsageType = customType
	return f
}

func (f *StringFlag) SetCompletionFunc(fn CompletionFunc) *StringFlag {
	f.CompletionFunc = fn
	return f
}

func (f *StringFlag) Register(cmd *Cmd, opts ...RegisterOption) (*string, error) {
	ptr := new(string)
	return ptr, f.RegisterWithPtr(cmd, ptr, opts...)
}

func (f *StringFlag) RegisterWithPtr(cmd *Cmd, ptr *string, opts ...RegisterOption) error {
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
		flag.Optional = true // TODO test, make clearer? Might be surprising/undesirable?
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
func (f *StringFlag) validateDefaultValue(value string) error {
	// Check enum constraint
	if f.EnumConstraint != nil {
		valid := false
		for _, allowed := range *f.EnumConstraint {
			if value == allowed {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("value %q not in allowed enum values %v", value, *f.EnumConstraint)
		}
	}

	// Check regex constraint
	if f.RegexConstraint != nil {
		if !f.RegexConstraint.MatchString(value) {
			return fmt.Errorf("value %q does not match regex pattern %s", value, f.RegexConstraint.String())
		}
	}

	return nil
}
