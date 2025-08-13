package ra

import (
	"bytes"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Basic(t *testing.T) {
	fs := NewCmd("test")

	boolFlag, err := NewBool("foo").
		SetShort("f").
		SetUsage("foo usage here").
		SetDefault(true).
		Register(fs)
	assert.NoError(t, err)

	strFlag, err := NewString("bar").
		SetShort("b").
		SetUsage("bar usage here").
		SetDefault("alice").
		Register(fs)
	assert.NoError(t, err)

	parseErr := fs.ParseOrError([]string{})
	assert.Nil(t, parseErr)

	assert.Equal(t, true, *boolFlag)
	assert.Equal(t, "alice", *strFlag)
}

func Test_OptionalString(t *testing.T) {
	fs := NewCmd("test")

	strFlag, err := NewString("bar").
		SetShort("b").
		SetUsage("bar usage here").
		SetOptional(true).
		Register(fs)
	assert.NoError(t, err)

	parseErr := fs.ParseOrError([]string{})
	assert.Nil(t, parseErr)

	assert.NotNil(t, strFlag)
	assert.Equal(t, "", *strFlag)
}

func Test_StringSliceMultiple(t *testing.T) {
	fs := NewCmd("test")

	strSliceFlag, err := NewStringSlice("bar").
		SetShort("b").
		SetUsage("bar usage here").
		Register(fs)
	assert.NoError(t, err)

	parseErr := fs.ParseOrError([]string{"--bar", "alice", "--bar", "bob"})
	assert.Nil(t, parseErr)
	assert.Equal(t, []string{"alice", "bob"}, *strSliceFlag)
}

func Test_StringSliceSeparator(t *testing.T) {
	fs := NewCmd("test")

	strSliceFlag, err := NewStringSlice("bar").
		SetShort("b").
		SetUsage("bar usage here").
		SetSeparator("|").
		Register(fs)
	assert.NoError(t, err)

	parseErr := fs.ParseOrError([]string{"--bar", "alice|bob"})
	assert.Nil(t, parseErr)
	assert.Equal(t, []string{"alice", "bob"}, *strSliceFlag)
}

func Test_StringSliceVariadic(t *testing.T) {
	fs := NewCmd("test")

	strSliceFlag, err := NewStringSlice("bar").
		SetShort("b").
		SetUsage("bar usage here").
		SetVariadic(true).
		Register(fs)
	assert.NoError(t, err)

	parseErr := fs.ParseOrError([]string{"--bar", "alice", "bob"})
	assert.Nil(t, parseErr)
	assert.Equal(t, []string{"alice", "bob"}, *strSliceFlag)
}

func Test_StringSliceVariadicAndSeparator(t *testing.T) {
	fs := NewCmd("test")

	strSliceFlag, err := NewStringSlice("bar").
		SetShort("b").
		SetUsage("bar usage here").
		SetVariadic(true).
		SetSeparator(",").
		Register(fs)
	assert.NoError(t, err)

	parseErr := fs.ParseOrError([]string{"--bar", "alice", "bob,charlie"})
	assert.Nil(t, parseErr)
	assert.Equal(t, []string{"alice", "bob", "charlie"}, *strSliceFlag)
}

func Test_IntRangeConstraint(t *testing.T) {
	fs := NewCmd("test")

	intFlag, err := NewInt("foo").
		SetMin(5, true).
		SetMax(10, true).
		Register(fs)
	assert.NoError(t, err)

	parseErr := fs.ParseOrError([]string{"--foo", "7"})
	assert.Nil(t, parseErr)
	assert.Equal(t, 7, *intFlag)
}

func Test_IntRangeConstraintErrors(t *testing.T) {
	fs := NewCmd("test")

	_, err := NewInt("foo").
		SetMin(5, true).
		SetMax(10, true).
		Register(fs)
	assert.NoError(t, err)

	parseErr := fs.ParseOrError([]string{"--foo", "70"})
	assert.NotNil(t, parseErr)
}

func Test_Cmds(t *testing.T) {
	addCmd := NewCmd("add")
	addFile, err := NewString("file").
		Register(addCmd)
	assert.NoError(t, err)

	rmCmd := NewCmd("rm")
	rmName, err := NewInt("name").
		Register(rmCmd)
	assert.NoError(t, err)

	rootCmd := NewCmd("root")

	addInvoked, err := rootCmd.RegisterCmd(addCmd)
	assert.NoError(t, err)

	rmInvoked, err := rootCmd.RegisterCmd(rmCmd)
	assert.NoError(t, err)

	parseErr := rootCmd.ParseOrError([]string{"add", "--file", "test.txt"})
	assert.Nil(t, parseErr)
	assert.True(t, *addInvoked)
	assert.False(t, *rmInvoked)
	assert.Equal(t, 0, *rmName) // rmName should have default value since rm command not used
	assert.Equal(t, "test.txt", *addFile)
}

// --arg1 already set, so "bbb" falls into arg2
func Test_PositionalAssignmentLeftToRight(t *testing.T) {
	fs := NewCmd("test")

	arg1, _ := NewString("arg1").Register(fs)
	arg2, _ := NewString("arg2").Register(fs)

	err := fs.ParseOrError([]string{"--arg1=aaa", "bbb"})
	assert.Nil(t, err)

	assert.Equal(t, "aaa", *arg1)
	assert.Equal(t, "bbb", *arg2)
}

// positional assignment then named flag override - named flag wins
func Test_PositionalThenNamedFlagOverride(t *testing.T) {
	fs := NewCmd("test")

	arg1, _ := NewString("arg1").Register(fs)
	arg2, _ := NewString("arg2").SetOptional(true).Register(fs)

	err := fs.ParseOrError([]string{"aaa", "--arg1=bbb"})
	assert.Nil(t, err)
	assert.Equal(t, "bbb", *arg1) // named flag overrides positional
	assert.Equal(t, "", *arg2)    // no value assigned
}

// -bcd where b,c,d are bools; “aaa” becomes first positional arg
func Test_ShortBoolCluster(t *testing.T) {
	fs := NewCmd("test")

	pos, _ := NewString("arg1").Register(fs)
	b, _ := NewBool("b").SetShort("b").Register(fs)
	c, _ := NewBool("c").SetShort("c").Register(fs)
	d, _ := NewBool("d").SetShort("d").Register(fs)
	e, _ := NewBool("e").SetShort("e").Register(fs) // never set

	err := fs.ParseOrError([]string{"-bcd", "aaa"})
	assert.Nil(t, err)

	assert.Equal(t, "aaa", *pos)
	assert.True(t, *b)
	assert.True(t, *c)
	assert.True(t, *d)
	assert.False(t, *e)
}

// cluster terminates at non‑bool flag
func Test_ShortClusterEndsWithNonBool(t *testing.T) {
	fs := NewCmd("test")

	a, _ := NewBool("a").SetShort("a").Register(fs)
	b, _ := NewBool("b").SetShort("b").Register(fs)
	c, _ := NewString("c").SetShort("c").Register(fs)

	err := fs.ParseOrError([]string{"-abc", "ddd"})
	assert.Nil(t, err)

	assert.True(t, *a)
	assert.True(t, *b)
	assert.Equal(t, "ddd", *c)
}

// No int‑shorts defined → “-1” and “-2” are values, not flags.
func Test_NegativeIntsWithoutNumberShortMode(t *testing.T) {
	fs := NewCmd("test")

	val1, _ := NewInt("arg1").Register(fs)
	val2, _ := NewInt("arg2").Register(fs)

	err := fs.ParseOrError([]string{"-1", "--arg2", "-2"})
	assert.Nil(t, err)

	assert.Equal(t, -1, *val1)
	assert.Equal(t, -2, *val2)
}

// Defining an int‑short activates number‑shorts mode.
func Test_NumberShortsMode(t *testing.T) {
	fs := NewCmd("test")

	// arg1 is string to capture --arg1 value
	arg1, _ := NewString("arg1").Register(fs)
	// int short “2” – activates the mode
	arg2, _ := NewInt("arg2").SetShort("2").Register(fs)

	err := fs.ParseOrError([]string{"--arg1=-2", "-2", "42"})
	assert.Nil(t, err)

	assert.Equal(t, "-2", *arg1) // parsed via =, so literal -2
	assert.Equal(t, 42, *arg2)   // “-2” consumed flag, next token 42
}

func Test_PositionalAndFlagVariadics(t *testing.T) {
	fs := NewCmd("test")

	posVar, _ := NewStringSlice("arg1").SetVariadic(true).Register(fs) // positional variadic
	flagVar, _ := NewStringSlice("arg2").
		SetVariadic(true).
		SetShort("e").
		Register(fs)

	err := fs.ParseOrError([]string{"aaa", "bbb", "--arg2", "ccc", "ddd", "-e", "eee"})
	assert.Nil(t, err)

	assert.Equal(t, []string{"aaa", "bbb"}, *posVar)
	assert.Equal(t, []string{"ccc", "ddd", "eee"}, *flagVar)
}

func Test_ConfiguredAndDefaults(t *testing.T) {
	fs := NewCmd("test")

	str, _ := NewString("foo").SetDefault("bar").Register(fs)
	assert.False(t, fs.Configured("foo")) // default only

	err := fs.ParseOrError([]string{"--foo", "baz"})
	assert.Nil(t, err)

	assert.True(t, fs.Configured("foo"))
	assert.Equal(t, "baz", *str)
}

func Test_UnknownFlagProducesError(t *testing.T) {
	fs := NewCmd("test")

	err := fs.ParseOrError([]string{"--does-not-exist"})
	assert.NotNil(t, err)
}

func Test_Int64Flag(t *testing.T) {
	fs := NewCmd("test")

	intFlag, err := NewInt64("value").Register(fs)
	assert.NoError(t, err)

	err = fs.ParseOrError([]string{"--value", "9223372036854775807"})
	assert.Nil(t, err)
	assert.Equal(t, int64(9223372036854775807), *intFlag)
}

func Test_Float64Flag(t *testing.T) {
	fs := NewCmd("test")

	floatFlag, err := NewFloat64("value").Register(fs)
	assert.NoError(t, err)

	err = fs.ParseOrError([]string{"--value", "3.14159"})
	assert.Nil(t, err)
	assert.Equal(t, 3.14159, *floatFlag)
}

func Test_BoolSliceFlag(t *testing.T) {
	fs := NewCmd("test")

	boolSliceFlag, err := NewBoolSlice("flags").Register(fs)
	assert.NoError(t, err)

	err = fs.ParseOrError([]string{"--flags", "true", "--flags", "false", "--flags", "1", "--flags", "0"})
	assert.Nil(t, err)
	assert.Equal(t, []bool{true, false, true, false}, *boolSliceFlag)
}

func Test_StringEnumConstraint(t *testing.T) {
	fs := NewCmd("test")

	enumFlag, err := NewString("level").
		SetEnumConstraint([]string{"debug", "info", "warn", "error"}).
		Register(fs)
	assert.NoError(t, err)

	err = fs.ParseOrError([]string{"--level", "info"})
	assert.Nil(t, err)
	assert.Equal(t, "info", *enumFlag)
}

func Test_StringEnumConstraintError(t *testing.T) {
	fs := NewCmd("test")

	_, err := NewString("level").
		SetEnumConstraint([]string{"debug", "info", "warn", "error"}).
		Register(fs)
	assert.NoError(t, err)

	err = fs.ParseOrError([]string{"--level", "invalid"})
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "Invalid 'level' value: invalid (valid values: debug, info, warn, error)")
}

func Test_StringRegexConstraint(t *testing.T) {
	fs := NewCmd("test")

	regexFlag, err := NewString("email").
		SetRegexConstraint(regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)).
		Register(fs)
	assert.NoError(t, err)

	err = fs.ParseOrError([]string{"--email", "test@example.com"})
	assert.Nil(t, err)
	assert.Equal(t, "test@example.com", *regexFlag)
}

func Test_StringRegexConstraintError(t *testing.T) {
	fs := NewCmd("test")

	_, err := NewString("name").
		SetRegexConstraint(regexp.MustCompile(`^[A-Z][a-z]*$`)).
		Register(fs)
	assert.NoError(t, err)

	err = fs.ParseOrError([]string{"--name", "alice"})
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "Invalid 'name' value: alice")
	assert.Contains(t, err.Error(), "must match regex: ^[A-Z][a-z]*$")
}

func Test_Float64RangeConstraint(t *testing.T) {
	fs := NewCmd("test")

	floatFlag, err := NewFloat64("value").
		SetMin(0.0, true).
		SetMax(100.0, true).
		Register(fs)
	assert.NoError(t, err)

	err = fs.ParseOrError([]string{"--value", "42.5"})
	assert.Nil(t, err)
	assert.Equal(t, 42.5, *floatFlag)
}

func Test_Float64RangeConstraintError(t *testing.T) {
	fs := NewCmd("test")

	_, err := NewFloat64("value").
		SetMin(0.0, true).
		SetMax(100.0, true).
		Register(fs)
	assert.NoError(t, err)

	err = fs.ParseOrError([]string{"--value", "150.0"})
	assert.NotNil(t, err)
}

func Test_GlobalFlags(t *testing.T) {
	subCmd := NewCmd("sub")
	subArg, _ := NewString("subarg").Register(subCmd)

	rootCmd := NewCmd("root")
	globalFlag, err := NewBool("verbose").
		SetShort("v").
		SetDefault(false).
		Register(rootCmd, WithGlobal(true))
	assert.NoError(t, err)

	subInvoked, err := rootCmd.RegisterCmd(subCmd)
	assert.NoError(t, err)

	err = rootCmd.ParseOrError([]string{"--verbose", "sub", "--subarg", "test"})
	assert.Nil(t, err)
	assert.True(t, *globalFlag)
	assert.True(t, *subInvoked)
	assert.Equal(t, "test", *subArg)
}

func Test_MutuallyExclusiveFlags(t *testing.T) {
	fs := NewCmd("test")

	flag1, err := NewString("flag1").
		SetExcludes([]string{"flag2"}).
		SetOptional(true).
		Register(fs)
	assert.NoError(t, err)

	_, err = NewString("flag2").
		SetExcludes([]string{"flag1"}).
		SetOptional(true).
		Register(fs)
	assert.NoError(t, err)

	err = fs.ParseOrError([]string{"--flag1", "value1", "--flag2", "value2"})
	assert.NotNil(t, err)

	// Test that using just one works
	err = fs.ParseOrError([]string{"--flag1", "value1"})
	assert.Nil(t, err)
	assert.Equal(t, "value1", *flag1)
}

func Test_ExcludesFlags_OneWay(t *testing.T) {
	fs := NewCmd("test")

	// Only flag1 declares flag2 as excluded, not the other way around
	flag1, err := NewString("flag1").
		SetExcludes([]string{"flag2"}).
		SetOptional(true).
		Register(fs)
	assert.NoError(t, err)

	flag2, err := NewString("flag2").SetOptional(true).Register(fs) // flag2 does NOT declare flag1 as excluded
	assert.NoError(t, err)

	// Should error when flag1 is used with flag2
	err = fs.ParseOrError([]string{"--flag1", "value1", "--flag2", "value2"})
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "excludes")

	// Should also error when flag2 is used with flag1 (one-way constraint should work both ways)
	err = fs.ParseOrError([]string{"--flag2", "value2", "--flag1", "value1"})
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "excludes")

	// Test that using just flag1 works
	err = fs.ParseOrError([]string{"--flag1", "value1"})
	assert.Nil(t, err)
	assert.Equal(t, "value1", *flag1)

	// Test that using just flag2 works
	err = fs.ParseOrError([]string{"--flag2", "value2"})
	assert.Nil(t, err)
	assert.Equal(t, "value2", *flag2)
}

func Test_ErrorMessages_Format(t *testing.T) {
	fs := NewCmd("test")

	// Test requires error message format
	_, err := NewString("a").SetRequires([]string{"b"}).Register(fs)
	assert.NoError(t, err)
	_, err = NewString("b").Register(fs)
	assert.NoError(t, err)

	err = fs.ParseOrError([]string{"--a", "value1"})
	assert.NotNil(t, err)
	assert.Equal(t, "Invalid args: 'a' requires 'b', but 'b' was not set", err.Error())

	// Test excludes error message format
	fs2 := NewCmd("test2")
	_, err = NewString("file").SetExcludes([]string{"url"}).Register(fs2)
	assert.NoError(t, err)
	_, err = NewString("url").Register(fs2)
	assert.NoError(t, err)

	err = fs2.ParseOrError([]string{"--file", "test.txt", "--url", "http://example.com"})
	assert.NotNil(t, err)
	assert.Equal(t, "Invalid args: 'file' excludes 'url', but 'url' was set", err.Error())
}

func Test_RequiresWithDefaults_Scenario(t *testing.T) {
	fs := NewCmd("test")

	// Setup: a (required), b (has default "bob" and requires c), c (required)
	_, err := NewString("a").Register(fs) // required, no default
	assert.NoError(t, err)

	_, err = NewString("b").SetDefault("bob").SetRequires([]string{"c"}).Register(fs) // has default, requires c
	assert.NoError(t, err)

	_, err = NewString("c").Register(fs) // required, no default
	assert.NoError(t, err)

	// Test: invoke with --a alice
	// What happens:
	// - 'a' is explicitly configured ✓
	// - 'b' gets default value "bob" (so b has a value)
	// - 'c' is required but not provided ✗
	//
	// Key insight:
	// - 'b' has a value (from default), and 'b' requires 'c'
	// - Since 'b' has a value, its requires constraint DOES apply
	// - 'c' is not set, so this should fail with requires constraint violation

	err = fs.ParseOrError([]string{"--a", "alice"})
	assert.NotNil(t, err, "Should fail because b (with default) requires c, but c was not provided")
	assert.Equal(t, "Invalid args: 'b' requires 'c', but 'c' was not set", err.Error())
}

func Test_RequiredFlags(t *testing.T) {
	fs := NewCmd("test")

	flag1, err := NewString("flag1").
		SetRequires([]string{"flag2"}).
		Register(fs)
	assert.NoError(t, err)

	flag2, err := NewString("flag2").Register(fs)
	assert.NoError(t, err)

	// Should error if flag1 is used without flag2
	err = fs.ParseOrError([]string{"--flag1", "value1"})
	assert.NotNil(t, err)

	// Should work if both are provided
	err = fs.ParseOrError([]string{"--flag1", "value1", "--flag2", "value2"})
	assert.Nil(t, err)
	assert.Equal(t, "value1", *flag1)
	assert.Equal(t, "value2", *flag2)
}

func Test_FlagOnlyFlag(t *testing.T) {
	fs := NewCmd("test")

	flagOnly, err := NewString("flagonly").
		SetFlagOnly(true).
		Register(fs)
	assert.NoError(t, err)

	normalFlag, err := NewString("normal").Register(fs)
	assert.NoError(t, err)

	// Should work when used as flag
	err = fs.ParseOrError([]string{"--flagonly", "value1", "positional"})
	assert.Nil(t, err)
	assert.Equal(t, "value1", *flagOnly)
	assert.Equal(t, "positional", *normalFlag)
}

func Test_PositionalOnlyFlag(t *testing.T) {
	fs := NewCmd("test")

	posOnly, err := NewString("posonly").
		SetPositionalOnly(true).
		Register(fs)
	assert.NoError(t, err)

	normalFlag, err := NewString("normal").Register(fs)
	assert.NoError(t, err)

	// Should work when used positionally
	err = fs.ParseOrError([]string{"positional1", "--normal", "value2"})
	assert.Nil(t, err)
	assert.Equal(t, "positional1", *posOnly)
	assert.Equal(t, "value2", *normalFlag)
}

// Example: mycmd aaa bbb
func Test_ExampleBasicPositional(t *testing.T) {
	fs := NewCmd("mycmd")

	arg1, _ := NewString("arg1").Register(fs)
	arg2, _ := NewString("arg2").Register(fs)

	err := fs.ParseOrError([]string{"aaa", "bbb"})
	assert.Nil(t, err)
	assert.Equal(t, "aaa", *arg1)
	assert.Equal(t, "bbb", *arg2)
}

// Example: mycmd aaa --arg2 bbb -c ddd -f
func Test_ExampleMixedPositionalAndFlags(t *testing.T) {
	fs := NewCmd("mycmd")

	arg1, _ := NewString("arg1").Register(fs)
	arg2, _ := NewString("arg2").Register(fs)
	arg3, _ := NewString("arg3").SetShort("c").Register(fs)
	arg4, _ := NewBool("arg4").SetShort("f").Register(fs)

	err := fs.ParseOrError([]string{"aaa", "--arg2", "bbb", "-c", "ddd", "-f"})
	assert.Nil(t, err)
	assert.Equal(t, "aaa", *arg1)
	assert.Equal(t, "bbb", *arg2)
	assert.Equal(t, "ddd", *arg3)
	assert.True(t, *arg4)
}

// Example: mycmd --arg1=aaa bbb  # assigns 'bbb' to 'arg2' because 'arg1' already assigned
func Test_ExamplePositionalAssignmentAfterFlag(t *testing.T) {
	fs := NewCmd("mycmd")

	arg1, _ := NewString("arg1").Register(fs)
	arg2, _ := NewString("arg2").Register(fs)

	err := fs.ParseOrError([]string{"--arg1=aaa", "bbb"})
	assert.Nil(t, err)
	assert.Equal(t, "aaa", *arg1)
	assert.Equal(t, "bbb", *arg2)
}

// Example: mycmd -bcd aaa  # since flags are bools, 'aaa' gets interpreted as the first positional arg
func Test_ExampleBoolClusterWithPositional(t *testing.T) {
	fs := NewCmd("mycmd")

	arg1, _ := NewString("arg1").Register(fs)
	arg2, _ := NewBool("arg2").SetShort("b").Register(fs)
	arg3, _ := NewBool("arg3").SetShort("c").Register(fs)
	arg4, _ := NewBool("arg4").SetShort("d").Register(fs)
	arg5, _ := NewBool("arg5").SetShort("e").Register(fs)

	err := fs.ParseOrError([]string{"-bcd", "aaa"})
	assert.Nil(t, err)

	assert.Equal(t, "aaa", *arg1)
	assert.True(t, *arg2)
	assert.True(t, *arg3)
	assert.True(t, *arg4)
	assert.False(t, *arg5)
}

// Example: mycmd -abc ddd  # last flag 'c' is a non-bool and so will read 'ddd'
func Test_ExampleBoolClusterEndingWithNonBool(t *testing.T) {
	fs := NewCmd("mycmd")

	arg1, _ := NewBool("arg1").SetShort("a").Register(fs)
	arg2, _ := NewBool("arg2").SetShort("b").Register(fs)
	arg3, _ := NewString("arg3").SetShort("c").Register(fs)

	err := fs.ParseOrError([]string{"-abc", "ddd"})
	assert.Nil(t, err)

	assert.True(t, *arg1)
	assert.True(t, *arg2)
	assert.Equal(t, "ddd", *arg3)
}

// Example: mycmd -aaa (incrementing int shorts)
func Test_ExampleIncrementingIntShorts(t *testing.T) {
	fs := NewCmd("mycmd")

	arg1, _ := NewInt("arg1").SetShort("a").SetDefault(0).Register(fs)

	err := fs.ParseOrError([]string{"-aaa"})
	assert.Nil(t, err)
	assert.Equal(t, 3, *arg1)
}

// Example: mycmd -1 --arg2 -2 -3.4 (negative numbers without number shorts mode)
func Test_ExampleNegativeNumbers(t *testing.T) {
	fs := NewCmd("mycmd")

	arg1, _ := NewInt("arg1").Register(fs)
	arg2, _ := NewInt("arg2").Register(fs)
	arg3, _ := NewFloat64("arg3").Register(fs)

	err := fs.ParseOrError([]string{"-1", "--arg2", "-2", "-3.4"})
	assert.Nil(t, err)
	assert.Equal(t, -1, *arg1)
	assert.Equal(t, -2, *arg2)
	assert.Equal(t, -3.4, *arg3)
}

// Example: mycmd --arg1=-2 -2 aaa -a bbb ccc (number shorts mode)
func Test_ExampleNumberShortsMode(t *testing.T) {
	fs := NewCmd("mycmd")

	arg1, _ := NewInt("arg1").Register(fs)
	arg2, _ := NewString("arg2").SetShort("2").Register(fs)
	arg3, _ := NewInt("arg3").Register(fs)
	arg4, _ := NewString("arg4").SetShort("a").Register(fs)

	err := fs.ParseOrError([]string{"--arg1=-2", "-2", "aaa", "-a", "bbb", "123"})
	assert.Nil(t, err)
	assert.Equal(t, -2, *arg1)
	assert.Equal(t, "aaa", *arg2)
	assert.Equal(t, 123, *arg3) // positional assignment
	assert.Equal(t, "bbb", *arg4)
}

// Example: mycmd aaa (positional variadic - empty)
func Test_ExamplePositionalVariadicEmpty(t *testing.T) {
	fs := NewCmd("mycmd")

	arg1, _ := NewString("arg1").Register(fs)
	arg2, _ := NewStringSlice("arg2").SetVariadic(true).Register(fs)

	err := fs.ParseOrError([]string{"aaa"})
	assert.Nil(t, err)
	assert.Equal(t, "aaa", *arg1)
	assert.Equal(t, []string{}, *arg2)
}

// Example: mycmd aaa bbb (positional variadic - single item)
func Test_ExamplePositionalVariadicSingle(t *testing.T) {
	fs := NewCmd("mycmd")

	arg1, _ := NewString("arg1").Register(fs)
	arg2, _ := NewStringSlice("arg2").SetVariadic(true).Register(fs)

	err := fs.ParseOrError([]string{"aaa", "bbb"})
	assert.Nil(t, err)
	assert.Equal(t, "aaa", *arg1)
	assert.Equal(t, []string{"bbb"}, *arg2)
}

// Example: mycmd aaa bbb ccc (positional variadic - multiple items)
func Test_ExamplePositionalVariadicMultiple(t *testing.T) {
	fs := NewCmd("mycmd")

	arg1, _ := NewString("arg1").Register(fs)
	arg2, _ := NewStringSlice("arg2").SetVariadic(true).Register(fs)

	err := fs.ParseOrError([]string{"aaa", "bbb", "ccc"})
	assert.Nil(t, err)
	assert.Equal(t, "aaa", *arg1)
	assert.Equal(t, []string{"bbb", "ccc"}, *arg2)
}

// Example: mycmd aaa --arg2 (variadic flags - empty)
func Test_ExampleVariadicFlagEmpty(t *testing.T) {
	fs := NewCmd("mycmd")

	arg1, _ := NewString("arg1").Register(fs)
	arg2, _ := NewStringSlice("arg2").SetVariadic(true).Register(fs)

	err := fs.ParseOrError([]string{"aaa", "--arg2"})
	assert.Nil(t, err)
	assert.Equal(t, "aaa", *arg1)
	assert.Equal(t, []string{}, *arg2)
}

// Example: mycmd aaa --arg2 bbb ccc (variadic flags - multiple items)
func Test_ExampleVariadicFlagMultiple(t *testing.T) {
	fs := NewCmd("mycmd")

	arg1, _ := NewString("arg1").Register(fs)
	arg2, _ := NewStringSlice("arg2").SetVariadic(true).Register(fs)

	err := fs.ParseOrError([]string{"aaa", "--arg2", "bbb", "ccc"})
	assert.Nil(t, err)
	assert.Equal(t, "aaa", *arg1)
	assert.Equal(t, []string{"bbb", "ccc"}, *arg2)
}

// Example: mycmd --arg2 aaa bbb --arg1 ccc (variadic reads until next flag)
func Test_ExampleVariadicUntilNextFlag(t *testing.T) {
	fs := NewCmd("mycmd")

	arg1, _ := NewString("arg1").Register(fs)
	arg2, _ := NewStringSlice("arg2").SetVariadic(true).Register(fs)

	err := fs.ParseOrError([]string{"--arg2", "aaa", "bbb", "--arg1", "ccc"})
	assert.Nil(t, err)
	assert.Equal(t, "ccc", *arg1)
	assert.Equal(t, []string{"aaa", "bbb"}, *arg2)
}

// Example: mycmd aaa bbb --arg2 ccc ddd -e fff (multiple variadics)
func Test_ExampleMultipleVariadics(t *testing.T) {
	fs := NewCmd("mycmd")

	arg1, _ := NewStringSlice("arg1").SetVariadic(true).Register(fs)
	arg2, _ := NewStringSlice("arg2").SetVariadic(true).Register(fs)
	arg3, _ := NewBool("arg3").SetShort("e").Register(fs)
	arg4, _ := NewString("arg4").Register(fs)

	err := fs.ParseOrError([]string{"aaa", "bbb", "--arg2", "ccc", "ddd", "-e", "fff"})
	assert.Nil(t, err)
	assert.Equal(t, []string{"aaa", "bbb"}, *arg1)
	assert.Equal(t, []string{"ccc", "ddd"}, *arg2)
	assert.True(t, *arg3)
	assert.Equal(t, "fff", *arg4)
}

func Test_HiddenFlags(t *testing.T) {
	fs := NewCmd("test")

	visible, _ := NewString("visible").Register(fs)
	hidden, _ := NewString("hidden").SetHidden(true).Register(fs)

	// Both should work when parsed
	err := fs.ParseOrError([]string{"--visible", "value1", "--hidden", "value2"})
	assert.Nil(t, err)
	assert.Equal(t, "value1", *visible)
	assert.Equal(t, "value2", *hidden)
}

func Test_IgnoreUnknownArgs(t *testing.T) {
	fs := NewCmd("test")

	knownFlag, _ := NewString("known").Register(fs)

	err := fs.ParseOrError([]string{"--known", "value", "--unknown", "ignored", "positional"}, WithIgnoreUnknown(true))
	assert.Nil(t, err)
	assert.Equal(t, "value", *knownFlag)

	unknownArgs := fs.GetUnknownArgs()
	assert.Contains(t, unknownArgs, "--unknown")
	assert.Contains(t, unknownArgs, "ignored")
	assert.Contains(t, unknownArgs, "positional")
}

func Test_IntSliceFlag(t *testing.T) {
	fs := NewCmd("test")

	intSlice, _ := NewIntSlice("values").Register(fs)

	err := fs.ParseOrError([]string{"--values", "1", "--values", "2", "--values", "3"})
	assert.Nil(t, err)
	assert.Equal(t, []int{1, 2, 3}, *intSlice)
}

func Test_Int64SliceFlag(t *testing.T) {
	fs := NewCmd("test")

	int64Slice, _ := NewInt64Slice("values").Register(fs)

	err := fs.ParseOrError([]string{"--values", "1", "--values", "2", "--values", "3"})
	assert.Nil(t, err)
	assert.Equal(t, []int64{1, 2, 3}, *int64Slice)
}

func Test_Float64SliceFlag(t *testing.T) {
	fs := NewCmd("test")

	floatSlice, _ := NewFloat64Slice("values").Register(fs)

	err := fs.ParseOrError([]string{"--values", "1.1", "--values", "2.2", "--values", "3.3"})
	assert.Nil(t, err)
	assert.Equal(t, []float64{1.1, 2.2, 3.3}, *floatSlice)
}

func Test_DuplicateFlagRegistration(t *testing.T) {
	fs := NewCmd("test")
	_, err := NewString("flag").Register(fs)
	assert.NoError(t, err)

	_, err = NewString("flag").Register(fs)
	assert.Error(t, err)
}

func Test_DuplicateShortFlagRegistration(t *testing.T) {
	fs := NewCmd("test")
	_, err := NewString("flag1").SetShort("f").Register(fs)
	assert.NoError(t, err)

	_, err = NewString("flag2").SetShort("f").Register(fs)
	assert.Error(t, err)
}

func Test_MissingRequiredFlagValue(t *testing.T) {
	fs := NewCmd("test")
	_, err := NewString("flag").Register(fs)
	assert.NoError(t, err)

	err = fs.ParseOrError([]string{"--flag"})
	assert.NotNil(t, err)
}

func Test_InvalidFlagCluster(t *testing.T) {
	fs := NewCmd("test")
	_, err := NewString("flag1").SetShort("a").Register(fs)
	assert.NoError(t, err)
	_, err = NewBool("flag2").SetShort("b").Register(fs)
	assert.NoError(t, err)

	err = fs.ParseOrError([]string{"-ab"})
	assert.NotNil(t, err)
}

func Test_EmptyArgs(t *testing.T) {
	fs := NewCmd("test")
	_, err := NewString("flag").SetOptional(false).Register(fs)
	assert.NoError(t, err)

	err = fs.ParseOrError([]string{})
	assert.NotNil(t, err)
}

func Test_MultipleMissingRequiredArgs(t *testing.T) {
	fs := NewCmd("test")
	_, err := NewString("mandatory1").Register(fs)
	assert.NoError(t, err)
	_, err = NewString("mandatory2").Register(fs)
	assert.NoError(t, err)
	_, err = NewString("optional1").SetOptional(true).Register(fs)
	assert.NoError(t, err)

	err = fs.ParseOrError([]string{})
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "Missing required arguments: [mandatory1, mandatory2]")
}

func Test_SingleMissingRequiredArg(t *testing.T) {
	fs := NewCmd("test")
	_, err := NewString("mandatory2").Register(fs)
	assert.NoError(t, err)

	err = fs.ParseOrError([]string{})
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "Missing required arguments: [mandatory2]")
}

func Test_EmptyArgsWithDefault(t *testing.T) {
	fs := NewCmd("test")
	flag, err := NewString("flag").SetDefault("default").Register(fs)
	assert.NoError(t, err)

	err = fs.ParseOrError([]string{})
	assert.Nil(t, err)
	assert.Equal(t, "default", *flag)
}

// mockExitWriter is a test implementation of StderrWriter
type mockExitWriter struct {
	buffer bytes.Buffer
}

func (m *mockExitWriter) Write(p []byte) (int, error) {
	return m.buffer.Write(p)
}

// mockExit replaces the osExit function to test for calls to os.Exit.
func mockExit(t *testing.T) (func(), *int, *bytes.Buffer) {
	t.Helper()
	var exitCode int
	var mu sync.Mutex

	originalExit := osExit
	originalStderr := stderrWriter

	mockWriter := &mockExitWriter{}
	stderrWriter = mockWriter

	osExit = func(code int) {
		mu.Lock()
		defer mu.Unlock()
		exitCode = code
		// Use a panic to stop execution flow like os.Exit would.
		// The deferred function will recover from this.
		panic("os.Exit called")
	}

	cleanup := func() {
		osExit = originalExit
		stderrWriter = originalStderr
	}

	return cleanup, &exitCode, &mockWriter.buffer
}

func Test_ParseOrExit_ExitsOnError(t *testing.T) {
	cleanup, exitCode, stderr := mockExit(t)
	defer cleanup()

	// This will panic because we mocked os.Exit
	assert.PanicsWithValue(t, "os.Exit called", func() {
		fs := NewCmd("test")
		fs.ParseOrExit([]string{"--unknown-flag"})
	})

	assert.Equal(t, 1, *exitCode)
	assert.Contains(t, stderr.String(), "unknown flag")
	assert.Contains(t, stderr.String(), "Usage:")
}

func Test_ParseOrError_ReturnsError(t *testing.T) {
	cleanup, exitCode, _ := mockExit(t)
	defer cleanup()

	var err error
	assert.NotPanics(t, func() {
		fs := NewCmd("test")
		err = fs.ParseOrError([]string{"--unknown-flag"})
	})

	assert.NotNil(t, err)
	assert.Equal(t, 0, *exitCode) // os.Exit was not called
}

func Test_HelpFlags_Exit(t *testing.T) {
	// Test --help (long)
	cleanup, exitCode, _ := mockExit(t)
	assert.PanicsWithValue(t, "os.Exit called", func() {
		fs := NewCmd("test")
		NewString("my-flag").SetUsage("This is a test flag.").Register(fs)
		fs.ParseOrExit([]string{"--help"})
	})
	assert.Equal(t, 0, *exitCode)
	cleanup()

	// Test -h (short)
	cleanup, exitCode, _ = mockExit(t)
	assert.PanicsWithValue(t, "os.Exit called", func() {
		fs := NewCmd("test")
		NewString("my-flag").SetUsage("This is a test flag.").Register(fs)
		fs.ParseOrExit([]string{"-h"})
	})
	assert.Equal(t, 0, *exitCode)
	cleanup()
}

func Test_HiddenInShortHelp(t *testing.T) {
	cleanup, _, stderr := mockExit(t)
	defer cleanup()

	assert.Panics(t, func() {
		fs := NewCmd("test")
		NewString("visible-flag").Register(fs)
		NewString("hidden-flag").SetHiddenInShortHelp(true).Register(fs)
		fs.ParseOrExit([]string{"-h"})
	})

	output := stderr.String()
	assert.Contains(t, output, "visible-flag")
	assert.NotContains(t, output, "hidden-flag")
}

func Test_ShortHelpVsLongHelp(t *testing.T) {
	cleanup, _, longStderr := mockExit(t)
	assert.Panics(t, func() {
		fs := NewCmd("test")
		NewString("visible-flag").Register(fs)
		NewString("advanced-flag").SetHiddenInShortHelp(true).Register(fs)
		fs.ParseOrExit([]string{"--help"})
	})
	cleanup()

	cleanup, _, shortStderr := mockExit(t)
	assert.Panics(t, func() {
		fs := NewCmd("test")
		NewString("visible-flag").Register(fs)
		NewString("advanced-flag").SetHiddenInShortHelp(true).Register(fs)
		fs.ParseOrExit([]string{"-h"})
	})
	cleanup()

	// Per spec, HiddenInShortHelp flags are shown in long help but not in short help.
	// Long help should show both visible and advanced flags
	assert.Contains(t, longStderr.String(), "visible-flag")
	assert.Contains(t, longStderr.String(), "advanced-flag")

	// Short help should only show visible flags, not advanced ones
	assert.Contains(t, shortStderr.String(), "visible-flag")
	assert.NotContains(t, shortStderr.String(), "advanced-flag")
}

func Test_CustomUsage(t *testing.T) {
	var shortHelpCalled, longHelpCalled bool

	customUsageFunc := func(isLongHelp bool) {
		if isLongHelp {
			longHelpCalled = true
			stderrWriter.Write([]byte("Custom long help!"))
		} else {
			shortHelpCalled = true
			stderrWriter.Write([]byte("Custom short help!"))
		}
	}

	// Test long custom help
	cleanup, _, stderr := mockExit(t)
	assert.Panics(t, func() {
		fs := NewCmd("test")
		fs.SetCustomUsage(customUsageFunc)
		fs.ParseOrExit([]string{"--help"})
	})
	assert.True(t, longHelpCalled)
	assert.False(t, shortHelpCalled)
	assert.Contains(t, stderr.String(), "Custom long help!")
	cleanup()

	// Reset and test short custom help
	longHelpCalled, shortHelpCalled = false, false
	cleanup, _, stderr = mockExit(t)
	assert.Panics(t, func() {
		fs := NewCmd("test")
		fs.SetCustomUsage(customUsageFunc)
		fs.ParseOrExit([]string{"-h"})
	})
	assert.False(t, longHelpCalled)
	assert.True(t, shortHelpCalled)
	assert.Contains(t, stderr.String(), "Custom short help!")
	cleanup()
}

func Test_CustomUsageWithDefaultGenerator(t *testing.T) {
	cleanup, _, stderr := mockExit(t)
	defer cleanup()

	fs := NewCmd("test")
	NewString("my-flag").SetUsage("My flag usage.").Register(fs)
	fs.SetCustomUsage(func(isLongHelp bool) {
		// User captures fs in a closure
		if isLongHelp {
			stderrWriter.Write([]byte("--- Custom Header ---\n"))
			stderrWriter.Write([]byte(fs.GenerateLongUsage()))
			stderrWriter.Write([]byte("\n--- Custom Footer ---"))
		} else {
			stderrWriter.Write([]byte(fs.GenerateShortUsage()))
		}
	})

	assert.Panics(t, func() {
		fs.ParseOrExit([]string{"--help"})
	})

	output := stderr.String()
	assert.True(t, strings.HasPrefix(output, "--- Custom Header ---"))
	assert.True(t, strings.HasSuffix(output, "--- Custom Footer ---"))
	assert.Contains(t, output, "My flag usage.")
}

func Test_UsageStringFormat(t *testing.T) {
	cleanup, _, stderr := mockExit(t)
	defer cleanup()

	fs := NewCmd("hm")
	fs.SetDescription(
		"A rad-powered recreation of 'um', with the help of 'tldr'.\nAllows you to check the tldr for commands, but then also\nadd your own notes and customize the notes in their own\nentries.",
	)

	NewString("task").Register(fs)
	NewBool("edit").SetShort("e").Register(fs)
	NewBool("list").SetShort("l").SetUsage("Lists stored entries. Exits after.").Register(fs)
	NewBool("reconfigure").SetUsage("Enable to reconfigure hm.").Register(fs)

	// Global flags (help is added automatically)
	NewBool("debug").
		SetShort("d").
		SetUsage("Enables debug output. Intended for Rad script developers.").
		Register(fs, WithGlobal(true))
	NewString("color").
		SetUsage("Control output colorization.").
		SetEnumConstraint([]string{"auto", "always", "never"}).
		SetDefault("auto").
		Register(fs, WithGlobal(true))
	NewBool("quiet").
		SetShort("q").
		SetUsage("Suppresses some output.").
		Register(fs, WithGlobal(true))
	NewBool("confirm-shell").
		SetUsage("Confirm all shell commands before running them.").
		Register(fs, WithGlobal(true))
	NewString("src").
		SetUsage("Instead of running the target script, just print it out").
		SetHiddenInShortHelp(true).
		Register(fs, WithGlobal(true))

	assert.Panics(t, func() {
		fs.ParseOrExit([]string{"--help"})
	})

	expected := `A rad-powered recreation of 'um', with the help of 'tldr'.
Allows you to check the tldr for commands, but then also
add your own notes and customize the notes in their own
entries.

Usage:
  hm <task> [OPTIONS]

Arguments:
      --task str
  -e, --edit
  -l, --list          Lists stored entries. Exits after.
      --reconfigure   Enable to reconfigure hm.

Global options:
  -d, --debug           Enables debug output. Intended for Rad script developers.
      --color str       Control output colorization. Valid values: [auto, always, never]. (default auto)
  -q, --quiet           Suppresses some output.
      --confirm-shell   Confirm all shell commands before running them.
      --src str         (optional) Instead of running the target script, just print it out
  -h, --help            Print usage string.
`
	// Compare the full output as a string
	assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(stderr.String()))

	// --src should now appear in long help (--help) since it's marked HiddenInShortHelp
	assert.Contains(t, stderr.String(), "--src")
}

func Test_UsageStringFormatWithSubcommands(t *testing.T) {
	cleanup, _, stderr := mockExit(t)
	defer cleanup()

	rootCmd := NewCmd("git")
	rootCmd.SetDescription("A dummy git command.")

	NewString("author").Register(rootCmd, WithGlobal(true))

	addCmd := NewCmd("add")
	NewString("patch").SetShort("p").Register(addCmd)
	rootCmd.RegisterCmd(addCmd)

	commitCmd := NewCmd("commit")
	NewString("message").SetShort("m").Register(commitCmd)
	rootCmd.RegisterCmd(commitCmd)

	assert.Panics(t, func() {
		rootCmd.ParseOrExit([]string{"--help"})
	})

	expected := `
A dummy git command.

Usage:
  git [subcommand] [OPTIONS]

Commands:
  add
  commit

Global options:
      --author str
  -h, --help         Print usage string.
`
	// Compare the full output as a string
	assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(stderr.String()))
}

func Test_GlobalFlagAfterSubcmdRegistration(t *testing.T) {
	rootCmd := NewCmd("root")
	subCmd := NewCmd("sub")

	// Register subcommand first
	subInvoked, err := rootCmd.RegisterCmd(subCmd)
	assert.NoError(t, err)

	// Register global flag after subcommand registration
	globalFlag, err := NewBool("global").
		SetShort("g").
		Register(rootCmd, WithGlobal(true))
	assert.NoError(t, err)

	// Parse with global flag before subcommand
	err = rootCmd.ParseOrError([]string{"--global", "sub"})
	assert.NoError(t, err)
	assert.True(t, *subInvoked)
	assert.True(t, *globalFlag, "global flag should be accessible in subcommand even when registered after subcommand")
}

func Test_ConfiguredFunctionWithSubcommands(t *testing.T) {
	rootCmd := NewCmd("root")
	subCmd := NewCmd("sub")

	// Register global flag on root
	globalFlag, err := NewBool("global").
		SetShort("g").
		Register(rootCmd, WithGlobal(true))
	assert.NoError(t, err)

	// Register subcommand-specific flag
	subFlag, err := NewString("subflag").
		SetShort("s").
		Register(subCmd)
	assert.NoError(t, err)

	// Register subcommand
	subInvoked, err := rootCmd.RegisterCmd(subCmd)
	assert.NoError(t, err)

	// Parse with global flag AFTER subcommand (i.e., during subcommand parsing)
	err = rootCmd.ParseOrError([]string{"sub", "--global", "--subflag", "test"})
	assert.NoError(t, err)
	assert.True(t, *subInvoked)
	assert.True(t, *globalFlag)
	assert.Equal(t, "test", *subFlag)

	// Test Configured function - both should report the flag as configured
	// Global flag set during subcommand parsing should be visible from root
	assert.True(t, subCmd.Configured("global"), "global flag should be configured in subcommand")
	assert.True(
		t,
		rootCmd.Configured("global"),
		"global flag should be configured in root too when set during subcommand parsing",
	)

	// Subcommand-specific flag should be visible from root via recursive check
	assert.True(t, subCmd.Configured("subflag"), "subflag should be configured in subcommand")
	assert.True(t, rootCmd.Configured("subflag"), "subflag should be configured in root via recursive check")
}

func Test_ConfiguredFunctionDoesNotCheckUnusedSubcommands(t *testing.T) {
	rootCmd := NewCmd("root")
	subCmd1 := NewCmd("used")
	subCmd2 := NewCmd("unused")

	// Register flag on unused subcommand
	unusedFlag, err := NewBool("unused-flag").Register(subCmd2)
	assert.NoError(t, err)

	// Register both subcommands
	_, err = rootCmd.RegisterCmd(subCmd1)
	assert.NoError(t, err)
	_, err = rootCmd.RegisterCmd(subCmd2)
	assert.NoError(t, err)

	// Parse only the used subcommand
	err = rootCmd.ParseOrError([]string{"used"})
	assert.NoError(t, err)

	// unused-flag should not be reported as configured since subCmd2 wasn't used
	assert.False(t, rootCmd.Configured("unused-flag"), "should not report flags from unused subcommands as configured")
	assert.False(t, *unusedFlag, "unused flag should have default value")
}

func Test_GlobalFlagsAreFlagOnly(t *testing.T) {
	rootCmd := NewCmd("root")

	// Register global flag - should automatically become flag-only
	globalFlag, err := NewString("global").
		SetShort("g").
		SetOptional(true).
		Register(rootCmd, WithGlobal(true))
	assert.NoError(t, err)

	// Register regular flag - should be positional by default
	regularFlag, err := NewString("regular").
		SetShort("r").
		Register(rootCmd)
	assert.NoError(t, err)

	// Test 1: Global flag should NOT be assignable positionally
	// This should assign to regular flag, not global flag
	err = rootCmd.ParseOrError([]string{"somevalue"})
	assert.NoError(t, err)
	assert.Equal(t, "somevalue", *regularFlag, "value should go to regular flag")
	assert.Equal(t, "", *globalFlag, "global flag should remain empty when not specified")

	// Reset
	*globalFlag = ""
	*regularFlag = ""

	// Test 2: Global flag should work when used as flag
	err = rootCmd.ParseOrError([]string{"--global", "globalvalue", "regularvalue"})
	assert.NoError(t, err)
	assert.Equal(t, "globalvalue", *globalFlag, "global flag should be set via flag syntax")
	assert.Equal(t, "regularvalue", *regularFlag, "regular flag should be set via positional")

	// Test 3: Verify global flag is actually in positional list or not
	// This is the real test - check if global flag is added to positional list
	foundGlobalInPositional := false
	foundRegularInPositional := false
	for _, name := range rootCmd.positional {
		if name == "global" {
			foundGlobalInPositional = true
		}
		if name == "regular" {
			foundRegularInPositional = true
		}
	}
	assert.False(t, foundGlobalInPositional, "global flag should NOT be in positional list")
	assert.True(t, foundRegularInPositional, "regular flag should be in positional list")
}

func Test_BoolFlagRequires_OnlyWhenTrue(t *testing.T) {
	fs := NewCmd("test")

	// authenticate is a bool flag that requires token
	authenticate, err := NewBool("authenticate").
		SetRequires([]string{"token"}).
		Register(fs)
	assert.NoError(t, err)

	token, err := NewString("token").Register(fs)
	assert.NoError(t, err)

	// Test 1: Only --token provided (authenticate defaults to false)
	// This should fail because authenticate is false and not considered configured for relational constraints
	err = fs.ParseOrError([]string{"--token", "mytoken"})
	assert.Nil(
		t,
		err,
		"should succeed when only token is provided since authenticate=false is not considered configured",
	)
	assert.False(t, *authenticate)
	assert.Equal(t, "mytoken", *token)
}

func Test_BoolFlagRequires_MutualRequirement(t *testing.T) {
	fs := NewCmd("test")

	// authenticate is a bool flag that requires token
	authenticate, err := NewBool("authenticate").
		SetRequires([]string{"token"}).
		Register(fs)
	assert.NoError(t, err)

	// token is a string flag that requires authenticate
	token, err := NewString("token").
		SetRequires([]string{"authenticate"}).
		Register(fs)
	assert.NoError(t, err)

	// Test 1: Only --token provided (authenticate defaults to false)
	// This should fail because token requires authenticate, but authenticate=false is not considered configured
	err = fs.ParseOrError([]string{"--token", "mytoken"})
	assert.NotNil(
		t,
		err,
		"should fail because token requires authenticate, but authenticate=false is not considered configured",
	)
	assert.Contains(t, err.Error(), "requires")
	assert.Contains(t, err.Error(), "token")
	assert.Contains(t, err.Error(), "authenticate")

	// Test 2: Both flags provided correctly should succeed
	err = fs.ParseOrError([]string{"--authenticate", "--token", "mytoken"})
	assert.Nil(t, err, "should succeed when both authenticate=true and token are provided")
	assert.True(t, *authenticate)
	assert.Equal(t, "mytoken", *token)
}

func Test_BoolFlagRequires_ExplicitlySetToFalse(t *testing.T) {
	fs := NewCmd("test")

	// authenticate is a bool flag that requires token
	authenticate, err := NewBool("authenticate").
		SetRequires([]string{"token"}).
		Register(fs)
	assert.NoError(t, err)

	token, err := NewString("token").Register(fs)
	assert.NoError(t, err)

	// Test: explicitly set authenticate to false with --authenticate=false
	// Even when explicitly set to false, bool flags should not be considered configured for relational constraints
	err = fs.ParseOrError([]string{"--authenticate=false", "--token", "mytoken"})
	assert.Nil(
		t,
		err,
		"should succeed when authenticate is explicitly set to false since false bools are not considered configured",
	)
	assert.False(t, *authenticate)
	assert.Equal(t, "mytoken", *token)
}

func Test_BoolFlagRequires_WhenTrue(t *testing.T) {
	fs := NewCmd("test")

	// authenticate is a bool flag that requires token
	authenticate, err := NewBool("authenticate").
		SetRequires([]string{"token"}).
		Register(fs)
	assert.NoError(t, err)

	token, err := NewString("token").SetOptional(true).Register(fs)
	assert.NoError(t, err)

	// Test 1: authenticate=true without token should fail
	err = fs.ParseOrError([]string{"--authenticate"})
	assert.NotNil(t, err, "should fail when authenticate=true but token is not provided")
	assert.Contains(t, err.Error(), "requires")

	// Test 2: authenticate=true with token should succeed
	err = fs.ParseOrError([]string{"--authenticate", "--token", "mytoken"})
	assert.Nil(t, err, "should succeed when both authenticate=true and token are provided")
	assert.True(t, *authenticate)
	assert.Equal(t, "mytoken", *token)
}

func Test_BoolFlagExcludes_OnlyWhenTrue(t *testing.T) {
	fs := NewCmd("test")

	// quiet is a bool flag that excludes verbose
	quiet, err := NewBool("quiet").
		SetExcludes([]string{"verbose"}).
		Register(fs)
	assert.NoError(t, err)

	verbose, err := NewBool("verbose").Register(fs)
	assert.NoError(t, err)

	// Test 1: Only --verbose provided (quiet defaults to false)
	// This should succeed because quiet=false is not considered configured for relational constraints
	err = fs.ParseOrError([]string{"--verbose"})
	assert.Nil(t, err, "should succeed when only verbose is provided since quiet=false is not considered configured")
	assert.False(t, *quiet)
	assert.True(t, *verbose)
}

func Test_BoolFlagExcludes_ExplicitlySetToFalse(t *testing.T) {
	fs := NewCmd("test")

	// quiet is a bool flag that excludes verbose
	quiet, err := NewBool("quiet").
		SetExcludes([]string{"verbose"}).
		Register(fs)
	assert.NoError(t, err)

	verbose, err := NewBool("verbose").Register(fs)
	assert.NoError(t, err)

	// Test: explicitly set quiet to false with --quiet=false
	// Even when explicitly set to false, bool flags should not be considered configured for relational constraints
	err = fs.ParseOrError([]string{"--quiet=false", "--verbose"})
	assert.Nil(
		t,
		err,
		"should succeed when quiet is explicitly set to false since false bools are not considered configured",
	)
	assert.False(t, *quiet)
	assert.True(t, *verbose)
}

func Test_BoolFlagExcludes_WhenTrue(t *testing.T) {
	// Test 1: both quiet=true and verbose=true should fail
	fs1 := NewCmd("test")
	_, err := NewBool("quiet").
		SetExcludes([]string{"verbose"}).
		Register(fs1)
	assert.NoError(t, err)
	_, err = NewBool("verbose").Register(fs1)
	assert.NoError(t, err)

	err = fs1.ParseOrError([]string{"--quiet", "--verbose"})
	assert.NotNil(t, err, "should fail when both quiet=true and verbose=true are provided")
	assert.Contains(t, err.Error(), "excludes")

	// Test 2: quiet=true alone should succeed
	fs2 := NewCmd("test")
	quiet2, err := NewBool("quiet").
		SetExcludes([]string{"verbose"}).
		Register(fs2)
	assert.NoError(t, err)
	verbose2, err := NewBool("verbose").Register(fs2)
	assert.NoError(t, err)

	err = fs2.ParseOrError([]string{"--quiet"})
	assert.Nil(t, err, "should succeed when only quiet=true is provided")
	assert.True(t, *quiet2)
	assert.False(t, *verbose2)
}

func Test_IntFlag_SetMin_Inclusive(t *testing.T) {
	fs := NewCmd("test")

	intFlag, err := NewInt("value").
		SetMin(5, true). // inclusive
		Register(fs)
	assert.NoError(t, err)

	// Test value == min (should pass with inclusive=true)
	err = fs.ParseOrError([]string{"--value", "5"})
	assert.Nil(t, err)
	assert.Equal(t, 5, *intFlag)
}

func Test_IntFlag_SetMin_Exclusive(t *testing.T) {
	fs := NewCmd("test")

	_, err := NewInt("value").
		SetMin(5, false). // exclusive
		Register(fs)
	assert.NoError(t, err)

	// Test value == min (should fail with exclusive=false)
	err = fs.ParseOrError([]string{"--value", "5"})
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "'value' value 5 is <= minimum (exclusive) 5")
}

func Test_IntFlag_SetMax_Inclusive(t *testing.T) {
	fs := NewCmd("test")

	intFlag, err := NewInt("value").
		SetMax(10, true). // inclusive
		Register(fs)
	assert.NoError(t, err)

	// Test value == max (should pass with inclusive=true)
	err = fs.ParseOrError([]string{"--value", "10"})
	assert.Nil(t, err)
	assert.Equal(t, 10, *intFlag)
}

func Test_IntFlag_SetMax_Exclusive(t *testing.T) {
	fs := NewCmd("test")

	_, err := NewInt("value").
		SetMax(10, false). // exclusive
		Register(fs)
	assert.NoError(t, err)

	// Test value == max (should fail with exclusive=false)
	err = fs.ParseOrError([]string{"--value", "10"})
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "'value' value 10 is >= maximum (exclusive) 10")
}

func Test_Int64Flag_SetMin_Inclusive(t *testing.T) {
	fs := NewCmd("test")

	intFlag, err := NewInt64("value").
		SetMin(5, true). // inclusive
		Register(fs)
	assert.NoError(t, err)

	// Test value == min (should pass with inclusive=true)
	err = fs.ParseOrError([]string{"--value", "5"})
	assert.Nil(t, err)
	assert.Equal(t, int64(5), *intFlag)
}

func Test_Int64Flag_SetMin_Exclusive(t *testing.T) {
	fs := NewCmd("test")

	_, err := NewInt64("value").
		SetMin(5, false). // exclusive
		Register(fs)
	assert.NoError(t, err)

	// Test value == min (should fail with exclusive=false)
	err = fs.ParseOrError([]string{"--value", "5"})
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "'value' value 5 is <= minimum (exclusive) 5")
}

func Test_Int64Flag_SetMax_Inclusive(t *testing.T) {
	fs := NewCmd("test")

	intFlag, err := NewInt64("value").
		SetMax(10, true). // inclusive
		Register(fs)
	assert.NoError(t, err)

	// Test value == max (should pass with inclusive=true)
	err = fs.ParseOrError([]string{"--value", "10"})
	assert.Nil(t, err)
	assert.Equal(t, int64(10), *intFlag)
}

func Test_Int64Flag_SetMax_Exclusive(t *testing.T) {
	fs := NewCmd("test")

	_, err := NewInt64("value").
		SetMax(10, false). // exclusive
		Register(fs)
	assert.NoError(t, err)

	// Test value == max (should fail with exclusive=false)
	err = fs.ParseOrError([]string{"--value", "10"})
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "'value' value 10 is >= maximum (exclusive) 10")
}

func Test_Float64Flag_SetMin_Inclusive(t *testing.T) {
	fs := NewCmd("test")

	floatFlag, err := NewFloat64("value").
		SetMin(5.0, true). // inclusive
		Register(fs)
	assert.NoError(t, err)

	// Test value == min (should pass with inclusive=true)
	err = fs.ParseOrError([]string{"--value", "5.0"})
	assert.Nil(t, err)
	assert.Equal(t, 5.0, *floatFlag)
}

func Test_Float64Flag_SetMin_Exclusive(t *testing.T) {
	fs := NewCmd("test")

	_, err := NewFloat64("value").
		SetMin(5.0, false). // exclusive
		Register(fs)
	assert.NoError(t, err)

	// Test value == min (should fail with exclusive=false)
	err = fs.ParseOrError([]string{"--value", "5.0"})
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "'value' value 5 is <= minimum (exclusive) 5")
}

func Test_Float64Flag_SetMax_Inclusive(t *testing.T) {
	fs := NewCmd("test")

	floatFlag, err := NewFloat64("value").
		SetMax(10.0, true). // inclusive
		Register(fs)
	assert.NoError(t, err)

	// Test value == max (should pass with inclusive=true)
	err = fs.ParseOrError([]string{"--value", "10.0"})
	assert.Nil(t, err)
	assert.Equal(t, 10.0, *floatFlag)
}

func Test_Float64Flag_SetMax_Exclusive(t *testing.T) {
	fs := NewCmd("test")

	_, err := NewFloat64("value").
		SetMax(10.0, false). // exclusive
		Register(fs)
	assert.NoError(t, err)

	// Test value == max (should fail with exclusive=false)
	err = fs.ParseOrError([]string{"--value", "10.0"})
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "'value' value 10 is >= maximum (exclusive) 10")
}

func Test_RangeError_ActualFormat(t *testing.T) {
	fs := NewCmd("test")

	_, err := NewInt("age").
		SetMin(1, false). // exclusive
		Register(fs)
	assert.NoError(t, err)

	// Test the exact format requested: 'age' value 0 is <= minimum (exclusive) 1
	err = fs.ParseOrError([]string{"--age", "0"})
	assert.NotNil(t, err)
	assert.Equal(t, "'age' value 0 is <= minimum (exclusive) 1", err.Error())
}

func Test_RangeError_InclusiveFormat(t *testing.T) {
	fs := NewCmd("test")

	_, err := NewInt("score").
		SetMax(100, true). // inclusive
		Register(fs)
	assert.NoError(t, err)

	// Test inclusive format: 'score' value 101 is > maximum 100
	err = fs.ParseOrError([]string{"--score", "101"})
	assert.NotNil(t, err)
	assert.Equal(t, "'score' value 101 is > maximum 100", err.Error())
}

func Test_HelpEnabled_False_CustomHelpFlag(t *testing.T) {
	cmd := NewCmd("test")
	cmd.SetHelpEnabled(false) // Disable automatic help handling
	cmd.SetDescription("Test help disabled behavior")

	// User registers their own help flag
	help, err := NewBool("help").
		SetShort("h").
		SetUsage("Custom help handler").
		Register(cmd)
	assert.NoError(t, err)

	// Should parse without triggering usage print and exit
	err = cmd.ParseOrError([]string{"--help"})
	assert.NoError(t, err)
	assert.True(t, *help, "Custom help flag should be set to true")

	// Also test short form
	cmd2 := NewCmd("test2")
	cmd2.SetHelpEnabled(false)

	help2, err := NewBool("help").
		SetShort("h").
		SetUsage("Custom help handler").
		Register(cmd2)
	assert.NoError(t, err)

	err = cmd2.ParseOrError([]string{"-h"})
	assert.NoError(t, err)
	assert.True(t, *help2, "Custom help flag via short form should be set to true")
}

func Test_HelpEnabled_True_AutomaticHelp(t *testing.T) {
	// This test verifies that when helpEnabled is true (default),
	// help flags are automatically registered during parsing.

	cmd := NewCmd("test")
	// helpEnabled is true by default
	cmd.SetDescription("Test automatic help behavior")

	someFlag, err := NewString("flag").
		SetUsage("Some flag").
		Register(cmd)
	assert.NoError(t, err)

	// Trigger parsing to register automatic help flags
	// We'll parse empty args to avoid triggering help behavior
	err = cmd.ParseOrError([]string{})
	assert.Error(t, err) // Should error due to missing required flag

	_ = someFlag

	// The automatic help flag should now be available in the usage
	usage := cmd.GenerateUsage(false)
	assert.Contains(t, usage, "--help")
	assert.Contains(t, usage, "-h")
	assert.Contains(t, usage, "Print usage string")
}

func Test_ExcludesFlags_RequiredFlagsShouldNotBeRequiredWhenExcluded(t *testing.T) {
	// This test reproduces the issue where mutually exclusive required flags
	// incorrectly fail validation when one excludes the other
	fs := NewCmd("test")

	// Two required flags that mutually exclude each other
	fileFlag, err := NewString("file").
		SetExcludes([]string{"url"}).
		Register(fs) // required by default
	assert.NoError(t, err)

	urlFlag, err := NewString("url").
		SetExcludes([]string{"file"}).
		Register(fs) // required by default
	assert.NoError(t, err)

	// This should succeed: specifying --file should make url no longer required
	// since file excludes url
	err = fs.ParseOrError([]string{"--file", "file.txt"})
	assert.Nil(t, err, "should succeed when file is provided (url should not be required due to exclusion)")
	assert.Equal(t, "file.txt", *fileFlag)
	assert.Equal(t, "", *urlFlag) // should remain empty/default

	// Reset and test the other direction
	fs2 := NewCmd("test")
	fileFlag2, err := NewString("file").
		SetExcludes([]string{"url"}).
		Register(fs2)
	assert.NoError(t, err)

	urlFlag2, err := NewString("url").
		SetExcludes([]string{"file"}).
		Register(fs2)
	assert.NoError(t, err)

	// This should also succeed: specifying --url should make file no longer required
	err = fs2.ParseOrError([]string{"--url", "https://example.com"})
	assert.Nil(t, err, "should succeed when url is provided (file should not be required due to exclusion)")
	assert.Equal(t, "", *fileFlag2) // should remain empty/default
	assert.Equal(t, "https://example.com", *urlFlag2)

	// Reset and test that both flags still cause an error when both are provided
	fs3 := NewCmd("test")
	_, err = NewString("file").
		SetExcludes([]string{"url"}).
		Register(fs3)
	assert.NoError(t, err)

	_, err = NewString("url").
		SetExcludes([]string{"file"}).
		Register(fs3)
	assert.NoError(t, err)

	// This should fail: both flags provided should trigger exclusion error
	err = fs3.ParseOrError([]string{"--file", "file.txt", "--url", "https://example.com"})
	assert.NotNil(t, err, "should fail when both mutually exclusive flags are provided")
	assert.Contains(t, err.Error(), "excludes")

	// Reset and test that neither flag provided should fail due to missing required args
	fs4 := NewCmd("test")
	_, err = NewString("file").
		SetExcludes([]string{"url"}).
		Register(fs4)
	assert.NoError(t, err)

	_, err = NewString("url").
		SetExcludes([]string{"file"}).
		Register(fs4)
	assert.NoError(t, err)

	// This should fail: neither flag provided, so both are missing (no exclusion applies)
	err = fs4.ParseOrError([]string{})
	assert.NotNil(t, err, "should fail when neither mutually exclusive required flag is provided")
	assert.Contains(t, err.Error(), "Missing required arguments")
}

func Test_AutoHelpOnNoArgs_CoreBehavior(t *testing.T) {
	cmd := NewCmd("test")
	cmd.SetAutoHelpOnNoArgs(true) // Enable auto-help

	// Add a required flag
	_, err := NewString("required-flag").Register(cmd)
	assert.NoError(t, err)

	// Call with no args - should return HelpInvokedErr
	err = cmd.ParseOrError([]string{})

	// Should return HelpInvokedErr
	assert.Equal(t, HelpInvokedErr, err, "should return HelpInvokedErr when auto-help is triggered")
}

func Test_AutoHelpOnNoArgs_OnlyWhenRequiredArgsExist(t *testing.T) {
	cmd := NewCmd("test")
	cmd.SetAutoHelpOnNoArgs(true) // Enable auto-help

	// Add only optional flags (no required flags)
	_, err := NewString("optional-flag").SetOptional(true).Register(cmd)
	assert.NoError(t, err)

	// Call with no args - should parse normally since no required args
	err = cmd.ParseOrError([]string{})
	assert.Nil(t, err, "should parse normally when no required args exist")
}

func Test_AutoHelpOnNoArgs_WorksForSubcommands(t *testing.T) {
	parentCmd := NewCmd("parent")
	subCmd := NewCmd("sub")
	subCmd.SetAutoHelpOnNoArgs(true) // Enable auto-help on subcommand

	// Add required flag to subcommand
	_, err := NewString("sub-required").Register(subCmd)
	assert.NoError(t, err)

	_, err = parentCmd.RegisterCmd(subCmd)
	assert.NoError(t, err)

	// Call subcommand with no args - should return HelpInvokedErr
	err = parentCmd.ParseOrError([]string{"sub"})

	// Should return HelpInvokedErr
	assert.Equal(t, HelpInvokedErr, err, "should return HelpInvokedErr when subcommand auto-help is triggered")
}

func Test_AutoHelpOnNoArgs_RespectsCustomUsage(t *testing.T) {
	// Track if custom usage was called
	var customUsageCalled bool
	var customUsageIsLongHelp bool

	cmd := NewCmd("test")
	cmd.SetAutoHelpOnNoArgs(true) // Enable auto-help
	cmd.SetCustomUsage(func(isLongHelp bool) {
		customUsageCalled = true
		customUsageIsLongHelp = isLongHelp
	})

	// Add a required flag
	_, err := NewString("required-flag").Register(cmd)
	assert.NoError(t, err)

	// Call with no args - should return HelpInvokedErr
	err = cmd.ParseOrError([]string{})

	// Should return HelpInvokedErr
	assert.Equal(t, HelpInvokedErr, err, "should return HelpInvokedErr when auto-help with custom usage is triggered")

	// Should have called custom usage function
	assert.True(t, customUsageCalled, "should call custom usage function")
	assert.False(t, customUsageIsLongHelp, "should call custom usage with isLongHelp=false (short help)")
}

func Test_ParseOrExit_HandlesHelpInvokedErr(t *testing.T) {
	// Mock stderr to capture output
	var capturedOutput bytes.Buffer
	originalStderrWriter := stderrWriter
	stderrWriter = &capturedOutput
	defer func() { stderrWriter = originalStderrWriter }()

	// Mock osExit to capture exit code
	var exitCode int
	var exitCalled bool
	originalOsExit := osExit
	osExit = func(code int) {
		exitCode = code
		exitCalled = true
	}
	defer func() { osExit = originalOsExit }()

	cmd := NewCmd("test")
	cmd.SetAutoHelpOnNoArgs(true)

	_, err := NewString("required-flag").Register(cmd)
	assert.NoError(t, err)

	// Call ParseOrExit which should handle help invoked error properly
	cmd.ParseOrExit([]string{})

	// Should have exited with code 0
	assert.True(t, exitCalled, "should have called osExit")
	assert.Equal(t, 0, exitCode, "should exit with code 0 for help")

	// Should have output the usage text
	output := capturedOutput.String()
	assert.Contains(t, output, "Usage:", "should output usage text")
	assert.Contains(t, output, "required-flag", "should show required flag")
}

func Test_ParseOrError_NeverCallsOsExit(t *testing.T) {
	// Mock osExit to ensure it's never called
	var exitCalled bool
	originalOsExit := osExit
	osExit = func(code int) {
		exitCalled = true
	}
	defer func() { osExit = originalOsExit }()

	cmd := NewCmd("test")

	// Test with --help flag
	err := cmd.ParseOrError([]string{"--help"})
	assert.False(t, exitCalled, "ParseOrError should never call osExit")
	assert.Equal(t, HelpInvokedErr, err, "should return HelpInvokedErr")

	// Test with -h flag
	exitCalled = false
	err = cmd.ParseOrError([]string{"-h"})
	assert.False(t, exitCalled, "ParseOrError should never call osExit for -h")
	assert.Equal(t, HelpInvokedErr, err, "should return HelpInvokedErr")
}

func Test_DoubleDash_BasicBehavior(t *testing.T) {
	cmd := NewCmd("test")

	verbose, err := NewBool("verbose").
		SetShort("v").
		SetUsage("Enable verbose output").
		Register(cmd)
	assert.NoError(t, err)

	file, err := NewString("file").
		SetUsage("Input file").
		SetPositionalOnly(true).
		Register(cmd)
	assert.NoError(t, err)

	output, err := NewString("output").
		SetUsage("Output file").
		SetPositionalOnly(true).
		SetOptional(true).
		Register(cmd)
	assert.NoError(t, err)

	// Test: arguments that look like flags after -- should be treated as positional
	err = cmd.ParseOrError([]string{"--verbose", "--", "--flag-like-arg", "-v"})
	assert.NoError(t, err)

	assert.True(t, *verbose)
	assert.Equal(t, "--flag-like-arg", *file)
	assert.Equal(t, "-v", *output)
}

func Test_DoubleDash_OnlyPositionalAfterDoubleDash(t *testing.T) {
	cmd := NewCmd("test")

	flag1, err := NewString("flag1").
		SetUsage("First flag").
		Register(cmd)
	assert.NoError(t, err)

	files, err := NewStringSlice("files").
		SetUsage("Input files").
		SetPositionalOnly(true).
		SetVariadic(true).
		SetOptional(true).
		Register(cmd)
	assert.NoError(t, err)

	// Test: everything after -- should be positional, even if it looks like flags
	err = cmd.ParseOrError([]string{"--flag1", "value1", "--", "--not-a-flag", "-a", "--another"})
	assert.NoError(t, err)

	assert.Equal(t, "value1", *flag1)
	assert.Equal(t, []string{"--not-a-flag", "-a", "--another"}, *files)
}

func Test_DoubleDash_EmptyAfterDoubleDash(t *testing.T) {
	cmd := NewCmd("test")

	verbose, err := NewBool("verbose").
		SetShort("v").
		Register(cmd)
	assert.NoError(t, err)

	file, err := NewString("file").
		SetPositionalOnly(true).
		SetOptional(true).
		Register(cmd)
	assert.NoError(t, err)

	// Test: -- with nothing after it should work
	err = cmd.ParseOrError([]string{"--verbose", "--"})
	assert.NoError(t, err)

	assert.True(t, *verbose)
	assert.Equal(t, "", *file) // Should remain empty
}

func Test_DoubleDash_OnlyDoubleDash(t *testing.T) {
	cmd := NewCmd("test")

	verbose, err := NewBool("verbose").
		SetShort("v").
		SetOptional(true).
		Register(cmd)
	assert.NoError(t, err)

	// Test: just -- by itself
	err = cmd.ParseOrError([]string{"--"})
	assert.NoError(t, err)

	assert.False(t, *verbose) // Should remain false
}

func Test_DoubleDash_WithRequiredFlags(t *testing.T) {
	cmd := NewCmd("test")

	required, err := NewString("required").
		SetUsage("Required flag").
		SetFlagOnly(true). // Make this flag-only so it can't be satisfied positionally
		Register(cmd)
	assert.NoError(t, err)

	file, err := NewString("file").
		SetPositionalOnly(true).
		SetOptional(true).
		Register(cmd)
	assert.NoError(t, err)

	// Test: required flags must still be satisfied before --
	err = cmd.ParseOrError([]string{"--", "positional-arg"})
	assert.Error(t, err)
	if err != nil {
		assert.Contains(t, err.Error(), "Missing required arguments: [required]")
	}

	// Test: providing required flag before -- should work
	err = cmd.ParseOrError([]string{"--required", "value", "--", "positional-arg"})
	assert.NoError(t, err)

	assert.Equal(t, "value", *required)
	assert.Equal(t, "positional-arg", *file)
}

func Test_DoubleDash_WithVariadicSlice(t *testing.T) {
	cmd := NewCmd("test")

	flag1, err := NewString("flag1").
		SetUsage("First flag").
		SetOptional(true).
		Register(cmd)
	assert.NoError(t, err)

	files, err := NewStringSlice("files").
		SetUsage("Input files").
		SetVariadic(true).
		Register(cmd)
	assert.NoError(t, err)

	// Test: variadic slice should consume everything after --
	err = cmd.ParseOrError([]string{"--flag1", "value", "--files", "file1", "--", "file2", "--flag-like", "file3"})
	assert.NoError(t, err)

	assert.Equal(t, "value", *flag1)
	assert.Equal(t, []string{"file1", "file2", "--flag-like", "file3"}, *files)
}

func Test_DoubleDash_MixedWithShortFlags(t *testing.T) {
	cmd := NewCmd("test")

	verbose, err := NewBool("verbose").
		SetShort("v").
		Register(cmd)
	assert.NoError(t, err)

	debug, err := NewBool("debug").
		SetShort("d").
		Register(cmd)
	assert.NoError(t, err)

	args, err := NewStringSlice("args").
		SetPositionalOnly(true).
		SetVariadic(true).
		SetOptional(true).
		Register(cmd)
	assert.NoError(t, err)

	// Test: short flags should work normally before --, but be treated as positional after
	err = cmd.ParseOrError([]string{"-vd", "--", "-v", "-d", "normal-arg"})
	assert.NoError(t, err)

	assert.True(t, *verbose)
	assert.True(t, *debug)
	assert.Equal(t, []string{"-v", "-d", "normal-arg"}, *args)
}

func Test_DoubleDash_WithSubcommands(t *testing.T) {
	cmd := NewCmd("main")

	verbose, err := NewBool("verbose").
		SetShort("v").
		Register(cmd)
	assert.NoError(t, err)

	// Create subcommand
	subCmd := NewCmd("sub")

	subFile, err := NewString("file").
		SetUsage("Sub file").
		SetPositionalOnly(true).
		Register(subCmd)
	assert.NoError(t, err)

	_, err = cmd.RegisterCmd(subCmd)
	assert.NoError(t, err)

	// Test: -- should work with subcommands
	err = cmd.ParseOrError([]string{"--verbose", "sub", "--", "--not-a-flag"})
	assert.NoError(t, err)

	assert.True(t, *verbose)
	assert.Equal(t, "--not-a-flag", *subFile)
}

func Test_DoubleDash_MultipleTimes(t *testing.T) {
	cmd := NewCmd("test")

	args, err := NewStringSlice("args").
		SetPositionalOnly(true).
		SetVariadic(true).
		SetOptional(true).
		Register(cmd)
	assert.NoError(t, err)

	// Test: multiple -- should be treated as regular arguments after the first
	err = cmd.ParseOrError([]string{"--", "--", "arg1", "--", "arg2"})
	assert.NoError(t, err)

	assert.Equal(t, []string{"--", "arg1", "--", "arg2"}, *args)
}

func Test_DoubleDash_WithEqualsFlag(t *testing.T) {
	cmd := NewCmd("test")

	flag1, err := NewString("flag1").
		Register(cmd)
	assert.NoError(t, err)

	args, err := NewStringSlice("args").
		SetPositionalOnly(true).
		SetVariadic(true).
		SetOptional(true).
		Register(cmd)
	assert.NoError(t, err)

	// Test: -- should work with = syntax flags
	err = cmd.ParseOrError([]string{"--flag1=value", "--", "--flag2=value2"})
	assert.NoError(t, err)

	assert.Equal(t, "value", *flag1)
	assert.Equal(t, []string{"--flag2=value2"}, *args)
}

func Test_ParentPositionalArgs_WithSubcommands_SubcommandTakesPrecedence(t *testing.T) {
	// Test that subcommands take precedence over parent positional args
	parentCmd := NewCmd("parent")

	// Parent has a positional argument
	parentArg, err := NewString("parent-arg").
		SetUsage("Parent positional argument").
		Register(parentCmd)
	assert.NoError(t, err)

	// Create subcommand with same name as a potential argument value
	subCmd := NewCmd("sub")
	subArg, err := NewString("sub-arg").
		SetUsage("Subcommand argument").
		Register(subCmd)
	assert.NoError(t, err)

	_, err = parentCmd.RegisterCmd(subCmd)
	assert.NoError(t, err)

	// Test: "sub" should be interpreted as subcommand, not parent positional arg
	err = parentCmd.ParseOrError([]string{"sub", "value"})
	assert.NoError(t, err)

	// Parent arg should not be set since subcommand was invoked
	assert.False(t, parentCmd.Configured("parent-arg"))
	assert.Equal(t, "", *parentArg) // default empty string

	// Subcommand arg should be set
	assert.True(t, subCmd.Configured("sub-arg"))
	assert.Equal(t, "value", *subArg)
}

func Test_ParentPositionalArgs_NoSubcommandMatch_ParentArgsFilled(t *testing.T) {
	// Test that parent positional args are filled when no subcommand matches
	parentCmd := NewCmd("parent")

	// Parent has positional arguments
	arg1, err := NewString("arg1").
		SetUsage("First argument").
		Register(parentCmd)
	assert.NoError(t, err)

	arg2, err := NewString("arg2").
		SetUsage("Second argument").
		Register(parentCmd)
	assert.NoError(t, err)

	// Create subcommand that won't match
	subCmd := NewCmd("subcmd")
	_, err = parentCmd.RegisterCmd(subCmd)
	assert.NoError(t, err)

	// Test: arguments that don't match subcommand names should be treated as positional
	err = parentCmd.ParseOrError([]string{"value1", "value2"})
	assert.NoError(t, err)

	// Parent args should be filled
	assert.True(t, parentCmd.Configured("arg1"))
	assert.True(t, parentCmd.Configured("arg2"))
	assert.Equal(t, "value1", *arg1)
	assert.Equal(t, "value2", *arg2)
}

func Test_ParentRequiredPositionalArgs_ErrorWhenSubcommandInvoked(t *testing.T) {
	// Test behavior when parent has required positional args but subcommand is invoked
	parentCmd := NewCmd("parent")

	// Parent has required positional argument
	parentArg, err := NewString("required-arg").
		SetUsage("Required parent argument").
		Register(parentCmd) // Not optional, so required
	assert.NoError(t, err)

	// Create subcommand
	subCmd := NewCmd("sub")
	subArg, err := NewString("sub-arg").
		SetUsage("Subcommand argument").
		SetOptional(true).
		Register(subCmd)
	assert.NoError(t, err)

	_, err = parentCmd.RegisterCmd(subCmd)
	assert.NoError(t, err)

	// Test: invoking subcommand should work even if parent has required args
	// This tests the current behavior - subcommands bypass parent validation
	err = parentCmd.ParseOrError([]string{"sub"})
	assert.NoError(t, err)

	// Parent required arg is not set, but no error since subcommand was invoked
	assert.False(t, parentCmd.Configured("required-arg"))
	assert.Equal(t, "", *parentArg)

	// Subcommand is invoked, but sub-arg is optional and not provided
	assert.False(t, subCmd.Configured("sub-arg"))
	assert.Equal(t, "", *subArg) // optional, so empty string default
}

func Test_ParentPositionalArgs_MixedWithFlags_SubcommandHandling(t *testing.T) {
	// Test complex scenario with parent flags, positional args, and subcommands
	parentCmd := NewCmd("parent")

	// Parent flag
	parentFlag, err := NewBool("verbose").
		SetShort("v").
		SetOptional(true).
		Register(parentCmd)
	assert.NoError(t, err)

	// Parent positional arg
	parentArg, err := NewString("input").
		SetUsage("Input file").
		SetOptional(true).
		Register(parentCmd)
	assert.NoError(t, err)

	// Subcommand
	subCmd := NewCmd("process")
	subArg, err := NewString("output").
		SetUsage("Output file").
		Register(subCmd)
	assert.NoError(t, err)

	_, err = parentCmd.RegisterCmd(subCmd)
	assert.NoError(t, err)

	// Test: parent flag + subcommand + subcommand args
	err = parentCmd.ParseOrError([]string{"--verbose", "process", "output.txt"})
	assert.NoError(t, err)

	// Parent flag should be set
	assert.True(t, *parentFlag)
	assert.True(t, parentCmd.Configured("verbose"))

	// Parent positional arg should NOT be set (subcommand takes precedence)
	assert.False(t, parentCmd.Configured("input"))
	assert.Equal(t, "", *parentArg)

	// Subcommand arg should be set
	assert.True(t, subCmd.Configured("output"))
	assert.Equal(t, "output.txt", *subArg)
}

func Test_ParentVariadicPositionalArgs_WithSubcommands(t *testing.T) {
	// Test variadic positional args in parent when subcommands are present
	parentCmd := NewCmd("parent")

	// Parent variadic positional args
	files, err := NewStringSlice("files").
		SetUsage("Input files").
		SetVariadic(true).
		SetOptional(true).
		Register(parentCmd)
	assert.NoError(t, err)

	// Subcommand
	subCmd := NewCmd("convert")
	subArg, err := NewString("format").
		SetUsage("Output format").
		Register(subCmd)
	assert.NoError(t, err)

	_, err = parentCmd.RegisterCmd(subCmd)
	assert.NoError(t, err)

	// Test: subcommand should take precedence over variadic collection
	err = parentCmd.ParseOrError([]string{"convert", "pdf"})
	assert.NoError(t, err)

	// Variadic parent args should not collect "convert"
	assert.False(t, parentCmd.Configured("files"))
	assert.Equal(t, []string{}, *files)

	// Subcommand should be invoked
	assert.True(t, subCmd.Configured("format"))
	assert.Equal(t, "pdf", *subArg)

	// Test: non-subcommand args should be collected as variadic
	err = parentCmd.ParseOrError([]string{"file1.txt", "file2.txt", "file3.txt"})
	assert.NoError(t, err)

	assert.True(t, parentCmd.Configured("files"))
	assert.Equal(t, []string{"file1.txt", "file2.txt", "file3.txt"}, *files)
}

func Test_SubcommandPositionalArgs_AfterDoubleDash(t *testing.T) {
	// Test that double dash works correctly with subcommand positional args
	parentCmd := NewCmd("parent")

	// Parent flag
	parentFlag, err := NewBool("debug").
		SetShort("d").
		SetOptional(true).
		Register(parentCmd)
	assert.NoError(t, err)

	// Subcommand with positional args
	subCmd := NewCmd("execute")

	// Required positional arg
	script, err := NewString("script").
		SetUsage("Script to execute").
		Register(subCmd)
	assert.NoError(t, err)

	// Variadic positional args for script arguments
	args, err := NewStringSlice("args").
		SetUsage("Script arguments").
		SetVariadic(true).
		SetOptional(true).
		Register(subCmd)
	assert.NoError(t, err)

	_, err = parentCmd.RegisterCmd(subCmd)
	assert.NoError(t, err)

	// Test: parent flag + subcommand + positional args including flag-like values after --
	err = parentCmd.ParseOrError([]string{"--debug", "execute", "myscript.sh", "--", "--verbose", "-x", "file.txt"})
	assert.NoError(t, err)

	// Parent flag should be set
	assert.True(t, *parentFlag)

	// Subcommand args should be parsed correctly
	assert.Equal(t, "myscript.sh", *script)
	assert.Equal(t, []string{"--verbose", "-x", "file.txt"}, *args)
}

func Test_SubcommandName_ConflictWithParentPositionalValue(t *testing.T) {
	// Test edge case where subcommand name could be valid value for parent positional arg
	parentCmd := NewCmd("parent")

	// Parent positional arg that could accept subcommand name as valid value
	mode, err := NewString("mode").
		SetUsage("Operation mode").
		SetEnumConstraint([]string{"create", "delete", "update", "sub"}).
		Register(parentCmd)
	assert.NoError(t, err)

	// Subcommand named "sub" - same as valid enum value
	subCmd := NewCmd("sub")
	subArg, err := NewString("target").
		SetUsage("Target for sub operation").
		Register(subCmd)
	assert.NoError(t, err)

	_, err = parentCmd.RegisterCmd(subCmd)
	assert.NoError(t, err)

	// Test: "sub" should be interpreted as subcommand, not as parent positional value
	err = parentCmd.ParseOrError([]string{"sub", "mytarget"})
	assert.NoError(t, err)

	// Parent mode should not be set
	assert.False(t, parentCmd.Configured("mode"))
	assert.Equal(t, "", *mode)

	// Subcommand should be invoked
	assert.True(t, subCmd.Configured("target"))
	assert.Equal(t, "mytarget", *subArg)

	// Test: explicit parent mode setting should work when not matching subcommand
	err = parentCmd.ParseOrError([]string{"create"})
	assert.NoError(t, err)

	assert.True(t, parentCmd.Configured("mode"))
	assert.Equal(t, "create", *mode)
}

func Test_MultipleFlagsBeforeSubcommand_WithPositionalArgs(t *testing.T) {
	// Test multiple parent flags before subcommand with positional args
	parentCmd := NewCmd("deploy")

	// Parent flags
	verbose, err := NewBool("verbose").
		SetShort("v").
		SetOptional(true).
		Register(parentCmd)
	assert.NoError(t, err)

	dryRun, err := NewBool("dry-run").
		SetShort("n").
		SetOptional(true).
		Register(parentCmd)
	assert.NoError(t, err)

	env, err := NewString("env").
		SetShort("e").
		SetOptional(true).
		Register(parentCmd)
	assert.NoError(t, err)

	// Subcommand
	appCmd := NewCmd("app")

	appName, err := NewString("name").
		SetUsage("Application name").
		Register(appCmd)
	assert.NoError(t, err)

	version, err := NewString("version").
		SetUsage("Version to deploy").
		SetOptional(true).
		Register(appCmd)
	assert.NoError(t, err)

	_, err = parentCmd.RegisterCmd(appCmd)
	assert.NoError(t, err)

	// Test: multiple parent flags + subcommand + subcommand positional args
	err = parentCmd.ParseOrError([]string{"--verbose", "--dry-run", "--env", "staging", "app", "myapp", "v1.2.3"})
	assert.NoError(t, err)

	// All parent flags should be set
	assert.True(t, *verbose)
	assert.True(t, *dryRun)
	assert.Equal(t, "staging", *env)

	// Subcommand positional args should be set
	assert.Equal(t, "myapp", *appName)
	assert.Equal(t, "v1.2.3", *version)
}

func Test_GlobalFlagsInheritedBySubcommand_WithPositionalArgs(t *testing.T) {
	// Test that global flags work correctly with subcommand positional args
	parentCmd := NewCmd("build")

	// Global flag
	outputDir, err := NewString("output-dir").
		SetShort("o").
		SetUsage("Output directory").
		SetOptional(true).
		Register(parentCmd, WithGlobal(true))
	assert.NoError(t, err)

	// Subcommand
	dockerCmd := NewCmd("docker")

	tag, err := NewString("tag").
		SetUsage("Docker image tag").
		Register(dockerCmd)
	assert.NoError(t, err)

	context, err := NewString("context").
		SetUsage("Build context").
		SetOptional(true).
		Register(dockerCmd)
	assert.NoError(t, err)

	_, err = parentCmd.RegisterCmd(dockerCmd)
	assert.NoError(t, err)

	// Test: global flag set during subcommand parsing with positional args
	err = parentCmd.ParseOrError([]string{"docker", "--output-dir", "/tmp/build", "v1.0", "."})
	assert.NoError(t, err)

	// Global flag should be accessible from both parent and subcommand
	assert.True(t, parentCmd.Configured("output-dir"))
	assert.True(t, dockerCmd.Configured("output-dir"))
	assert.Equal(t, "/tmp/build", *outputDir)

	// Subcommand positional args should be correctly parsed
	assert.Equal(t, "v1.0", *tag)
	assert.Equal(t, ".", *context)
}

func Test_ErrorHandling_SubcommandWithMissingRequiredPositional(t *testing.T) {
	// Test error handling when subcommand is missing required positional args
	parentCmd := NewCmd("main")

	// Parent optional flag
	debug, err := NewBool("debug").
		SetOptional(true).
		Register(parentCmd)
	assert.NoError(t, err)

	// Subcommand with required positional arg
	processCmd := NewCmd("process")

	inputFile, err := NewString("input").
		SetUsage("Input file path").
		Register(processCmd) // Required by default
	assert.NoError(t, err)

	_, err = parentCmd.RegisterCmd(processCmd)
	assert.NoError(t, err)

	// Test: subcommand invoked but missing required positional arg should error
	err = parentCmd.ParseOrError([]string{"--debug", "process"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Missing required arguments")
	assert.Contains(t, err.Error(), "input")

	// Parent flag should still be set despite subcommand error
	assert.True(t, *debug)

	_ = inputFile // Suppress unused variable warning
}
