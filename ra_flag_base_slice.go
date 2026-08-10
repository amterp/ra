package ra

import (
	"fmt"
	"regexp"
	"strings"
)

type SliceFlag[T any] struct {
	BaseFlag
	Separator *string
	Variadic  bool
	Default   *[]T
	Value     *[]T

	// Constraints on each element. Only the ones that suit the element type may
	// be set - Register rejects the rest rather than accepting and ignoring
	// them. Bounds are float64 for every numeric element type, matching how
	// callers already model a range.
	EnumConstraint  *[]string
	RegexConstraint *regexp.Regexp
	min             *float64
	max             *float64
	minInclusive    *bool
	maxInclusive    *bool

	// Constraints on how many elements there are, rather than on any one of
	// them. Valid for every element type.
	minLen          *int
	maxLen          *int
	minLenInclusive *bool
	maxLenInclusive *bool
}

// elementKind reports which constraints suit this flag's element type.
type elementKind int

const (
	elemString elementKind = iota
	elemNumber
	elemBool
)

func (f *SliceFlag[T]) elementKind() elementKind {
	var zero T
	switch any(zero).(type) {
	case string:
		return elemString
	case int, int64, float64:
		return elemNumber
	default:
		return elemBool
	}
}

func (f *SliceFlag[T]) elementKindName() string {
	switch f.elementKind() {
	case elemString:
		return "string"
	case elemNumber:
		return "number"
	default:
		return "bool"
	}
}

// validateConstraints rejects a constraint that cannot apply to this flag's
// element type. Silently ignoring one lets a caller believe input is being
// checked when it is not, which is the failure this whole feature exists to
// prevent - so it is an error at registration rather than a no-op at parse.
func (f *SliceFlag[T]) validateConstraints() error {
	kind := f.elementKind()
	if f.EnumConstraint != nil && kind != elemString {
		return fmt.Errorf("enum constraint on %q is not valid for %s elements", f.Name, f.elementKindName())
	}
	if f.RegexConstraint != nil && kind != elemString {
		return fmt.Errorf("regex constraint on %q is not valid for %s elements", f.Name, f.elementKindName())
	}
	if (f.min != nil || f.max != nil) && kind != elemNumber {
		return fmt.Errorf("range constraint on %q is not valid for %s elements", f.Name, f.elementKindName())
	}
	return nil
}

// checkStringElement validates one element of a string slice.
func (f *SliceFlag[T]) checkStringElement(value string) error {
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

	if f.RegexConstraint != nil && !f.RegexConstraint.MatchString(value) {
		return fmt.Errorf(
			"Invalid '%s' value: %s (must match regex: %s)",
			f.Name,
			value,
			f.RegexConstraint.String(),
		)
	}

	return nil
}

// checkNumberElement validates one element of a numeric slice. raw is the token
// as the user typed it, so the message echoes their input rather than a
// round-tripped rendering of it.
func (f *SliceFlag[T]) checkNumberElement(val float64, raw string) error {
	if f.min != nil {
		inclusive := f.minInclusive == nil || *f.minInclusive
		if (inclusive && val < *f.min) || (!inclusive && val <= *f.min) {
			if inclusive {
				return fmt.Errorf("'%s' value %s is < minimum %g", f.Name, raw, *f.min)
			}
			return fmt.Errorf("'%s' value %s is <= minimum (exclusive) %g", f.Name, raw, *f.min)
		}
	}

	if f.max != nil {
		inclusive := f.maxInclusive == nil || *f.maxInclusive
		if (inclusive && val > *f.max) || (!inclusive && val >= *f.max) {
			if inclusive {
				return fmt.Errorf("'%s' value %s is > maximum %g", f.Name, raw, *f.max)
			}
			return fmt.Errorf("'%s' value %s is >= maximum (exclusive) %g", f.Name, raw, *f.max)
		}
	}

	return nil
}

// rangeString renders the element bounds in the same interval notation the
// scalar flags use, e.g. "[0, 100]" or "(0, 100)". Empty when unbounded.
func (f *SliceFlag[T]) rangeString() string {
	if f.min == nil && f.max == nil {
		return ""
	}

	left := "("
	if f.min != nil {
		if f.minInclusive == nil || *f.minInclusive {
			left = fmt.Sprintf("[%g", *f.min)
		} else {
			left = fmt.Sprintf("(%g", *f.min)
		}
	}

	right := ")"
	if f.max != nil {
		if f.maxInclusive == nil || *f.maxInclusive {
			right = fmt.Sprintf("%g]", *f.max)
		} else {
			right = fmt.Sprintf("%g)", *f.max)
		}
	}

	return left + ", " + right
}

// lenString renders the element-count bounds, same notation as rangeString.
func (f *SliceFlag[T]) lenString() string {
	if f.minLen == nil && f.maxLen == nil {
		return ""
	}

	left := "("
	if f.minLen != nil {
		if f.minLenInclusive == nil || *f.minLenInclusive {
			left = fmt.Sprintf("[%d", *f.minLen)
		} else {
			left = fmt.Sprintf("(%d", *f.minLen)
		}
	}

	right := ")"
	if f.maxLen != nil {
		if f.maxLenInclusive == nil || *f.maxLenInclusive {
			right = fmt.Sprintf("%d]", *f.maxLen)
		} else {
			right = fmt.Sprintf("%d)", *f.maxLen)
		}
	}

	return left + ", " + right
}

// dumpConstraints renders this flag's constraints for the debug dump, reusing
// the encodings the scalar flags already use: {a,b,c} for an enum, ~re for a
// regex, and an interval for bounds. Length is labelled, because a slice can
// carry both an element range and a length bound at once.
func (f *SliceFlag[T]) dumpConstraints() string {
	var out string
	if f.EnumConstraint != nil {
		out += fmt.Sprintf("{%s}", strings.Join(*f.EnumConstraint, ","))
	}
	if f.RegexConstraint != nil {
		out += fmt.Sprintf("~%s", f.RegexConstraint.String())
	}
	if f.min != nil || f.max != nil {
		minStr := "(-∞"
		if f.min != nil {
			minStr = fmt.Sprintf("%g", *f.min)
			if f.minInclusive != nil && !*f.minInclusive {
				minStr = "(" + minStr
			} else {
				minStr = "[" + minStr
			}
		}
		maxStr := "+∞)"
		if f.max != nil {
			maxStr = fmt.Sprintf("%g", *f.max)
			if f.maxInclusive != nil && !*f.maxInclusive {
				maxStr += ")"
			} else {
				maxStr += "]"
			}
		}
		out += fmt.Sprintf("%s,%s", minStr, maxStr)
	}
	if f.minLen != nil || f.maxLen != nil {
		out += fmt.Sprintf(" len:%s", f.lenString())
	}
	return out
}

// checkLength validates the element count once parsing has finished. Unlike the
// element checks it cannot run during append, because a count is only final
// when there is nothing left to append.
func (f *SliceFlag[T]) checkLength() error {
	if f.Value == nil {
		return nil
	}
	n := len(*f.Value)

	if f.minLen != nil {
		inclusive := f.minLenInclusive == nil || *f.minLenInclusive
		if (inclusive && n < *f.minLen) || (!inclusive && n <= *f.minLen) {
			if inclusive {
				return fmt.Errorf("'%s' takes at least %d values, got %d", f.Name, *f.minLen, n)
			}
			return fmt.Errorf("'%s' takes more than %d values, got %d", f.Name, *f.minLen, n)
		}
	}

	if f.maxLen != nil {
		inclusive := f.maxLenInclusive == nil || *f.maxLenInclusive
		if (inclusive && n > *f.maxLen) || (!inclusive && n >= *f.maxLen) {
			if inclusive {
				return fmt.Errorf("'%s' takes at most %d values, got %d", f.Name, *f.maxLen, n)
			}
			return fmt.Errorf("'%s' takes fewer than %d values, got %d", f.Name, *f.maxLen, n)
		}
	}

	return nil
}

type StringSliceFlag = SliceFlag[string]
type IntSliceFlag = SliceFlag[int]
type Int64SliceFlag = SliceFlag[int64]
type Float64SliceFlag = SliceFlag[float64]
type BoolSliceFlag = SliceFlag[bool]

func NewStringSlice(name string) *StringSliceFlag {
	return &SliceFlag[string]{BaseFlag: BaseFlag{Name: name, Optional: false}}
}

func NewIntSlice(name string) *IntSliceFlag {
	return &SliceFlag[int]{BaseFlag: BaseFlag{Name: name, Optional: false}}
}

func NewInt64Slice(name string) *Int64SliceFlag {
	return &SliceFlag[int64]{BaseFlag: BaseFlag{Name: name, Optional: false}}
}

func NewFloat64Slice(name string) *Float64SliceFlag {
	return &SliceFlag[float64]{BaseFlag: BaseFlag{Name: name, Optional: false}}
}

func NewBoolSlice(name string) *BoolSliceFlag {
	return &SliceFlag[bool]{BaseFlag: BaseFlag{Name: name, Optional: false}}
}
func (f *SliceFlag[T]) SetShort(s string) *SliceFlag[T] {
	f.Short = s
	return f
}

func (f *SliceFlag[T]) SetUsage(u string) *SliceFlag[T] {
	f.Usage = u
	return f
}

func (f *SliceFlag[T]) SetDefault(v []T) *SliceFlag[T] {
	f.Default = &v
	return f
}

func (f *SliceFlag[T]) SetOptional(b bool) *SliceFlag[T] {
	f.Optional = b
	return f
}

func (f *SliceFlag[T]) SetHidden(b bool) *SliceFlag[T] {
	f.Hidden = b
	return f
}

func (f *SliceFlag[T]) SetHiddenInShortHelp(b bool) *SliceFlag[T] {
	f.HiddenInShortHelp = b
	return f
}

func (f *SliceFlag[T]) SetPositionalOnly(b bool) *SliceFlag[T] {
	f.PositionalOnly = b
	return f
}

func (f *SliceFlag[T]) SetFlagOnly(b bool) *SliceFlag[T] {
	f.FlagOnly = b
	return f
}

func (f *SliceFlag[T]) SetExcludes(flags []string) *SliceFlag[T] {
	f.Excludes = &flags
	return f
}

func (f *SliceFlag[T]) SetRequires(flags []string) *SliceFlag[T] {
	f.Requires = &flags
	return f
}

func (f *SliceFlag[T]) SetSeparator(sep string) *SliceFlag[T] {
	f.Separator = &sep
	return f
}

func (f *SliceFlag[T]) SetVariadic(b bool) *SliceFlag[T] {
	f.Variadic = b
	return f
}

func (f *SliceFlag[T]) SetCustomUsageType(customType string) *SliceFlag[T] {
	f.CustomUsageType = customType
	return f
}

func (f *SliceFlag[T]) SetCompletionFunc(fn CompletionFunc) *SliceFlag[T] {
	f.CompletionFunc = fn
	return f
}

// SetEnumConstraint restricts every element to one of values. String elements
// only; Register errors otherwise.
func (f *SliceFlag[T]) SetEnumConstraint(values []string) *SliceFlag[T] {
	f.EnumConstraint = &values
	return f
}

// SetRegexConstraint requires every element to match regex. String elements
// only; Register errors otherwise.
func (f *SliceFlag[T]) SetRegexConstraint(regex *regexp.Regexp) *SliceFlag[T] {
	f.RegexConstraint = regex
	return f
}

// SetMin bounds every element from below. Numeric elements only; Register
// errors otherwise.
func (f *SliceFlag[T]) SetMin(min float64, inclusive bool) *SliceFlag[T] {
	f.min = &min
	f.minInclusive = &inclusive
	return f
}

// SetMax bounds every element from above. Numeric elements only; Register
// errors otherwise.
func (f *SliceFlag[T]) SetMax(max float64, inclusive bool) *SliceFlag[T] {
	f.max = &max
	f.maxInclusive = &inclusive
	return f
}

// SetMinLen bounds how many elements the flag accepts, from below.
func (f *SliceFlag[T]) SetMinLen(min int, inclusive bool) *SliceFlag[T] {
	f.minLen = &min
	f.minLenInclusive = &inclusive
	return f
}

// SetMaxLen bounds how many elements the flag accepts, from above.
func (f *SliceFlag[T]) SetMaxLen(max int, inclusive bool) *SliceFlag[T] {
	f.maxLen = &max
	f.maxLenInclusive = &inclusive
	return f
}

func (f *SliceFlag[T]) Register(cmd *Cmd, opts ...RegisterOption) (*[]T, error) {
	ptr := new([]T)
	return ptr, f.RegisterWithPtr(cmd, ptr, opts...)
}

func (f *SliceFlag[T]) RegisterWithPtr(cmd *Cmd, ptr *[]T, opts ...RegisterOption) error {
	regConf := &registerConfig{}
	for _, opt := range opts {
		opt(regConf)
	}

	if err := f.validateConstraints(); err != nil {
		return err
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
			if err := cmd.validatePositionalOnlyAfterListPositional(f.Name); err != nil {
				return err
			}
		}
		cmd.positional = append(cmd.positional, f.Name)
	} else {
		cmd.nonPositional = append(cmd.nonPositional, f.Name)
	}

	return nil
}
