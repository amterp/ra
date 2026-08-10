package ra

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Constraints on a slice flag apply to each element, not to the slice. These
// tests cover both entry paths - the repeated flag and the positional
// collection - because they reach the value through different code.

func Test_SliceEnumConstraint(t *testing.T) {
	fs := NewCmd("test")
	envs, err := NewStringSlice("envs").
		SetEnumConstraint([]string{"dev", "stg", "prd"}).
		Register(fs)
	assert.NoError(t, err)

	err = fs.ParseOrError([]string{"--envs", "dev", "--envs", "prd"})
	assert.Nil(t, err)
	assert.Equal(t, []string{"dev", "prd"}, *envs)
}

func Test_SliceEnumConstraintRejectsOneBadElement(t *testing.T) {
	fs := NewCmd("test")
	_, err := NewStringSlice("envs").
		SetEnumConstraint([]string{"dev", "stg", "prd"}).
		Register(fs)
	assert.NoError(t, err)

	err = fs.ParseOrError([]string{"--envs", "dev", "--envs", "nope"})
	assert.NotNil(t, err)
	assert.Equal(t, "Invalid 'envs' value: nope (valid values: dev, stg, prd)", err.Error())
}

func Test_SliceEnumConstraintChecksEachSeparatedPart(t *testing.T) {
	fs := NewCmd("test")
	_, err := NewStringSlice("envs").
		SetSeparator(",").
		SetEnumConstraint([]string{"dev", "stg", "prd"}).
		Register(fs)
	assert.NoError(t, err)

	err = fs.ParseOrError([]string{"--envs", "dev,nope"})
	assert.NotNil(t, err)
	assert.Equal(t, "Invalid 'envs' value: nope (valid values: dev, stg, prd)", err.Error())
}

func Test_SliceEnumConstraintAppliesToPositionals(t *testing.T) {
	fs := NewCmd("test")
	_, err := NewStringSlice("envs").
		SetEnumConstraint([]string{"dev", "stg", "prd"}).
		Register(fs)
	assert.NoError(t, err)

	err = fs.ParseOrError([]string{"dev", "nope"})
	assert.NotNil(t, err)
	assert.Equal(t, "Invalid 'envs' value: nope (valid values: dev, stg, prd)", err.Error())
}

func Test_SliceRegexConstraint(t *testing.T) {
	fs := NewCmd("test")
	tags, err := NewStringSlice("tags").
		SetRegexConstraint(regexp.MustCompile("^[a-z][a-z0-9-]*$")).
		Register(fs)
	assert.NoError(t, err)

	err = fs.ParseOrError([]string{"--tags", "web", "--tags", "db-1"})
	assert.Nil(t, err)
	assert.Equal(t, []string{"web", "db-1"}, *tags)
}

func Test_SliceRegexConstraintError(t *testing.T) {
	fs := NewCmd("test")
	_, err := NewStringSlice("tags").
		SetRegexConstraint(regexp.MustCompile("^[a-z][a-z0-9-]*$")).
		Register(fs)
	assert.NoError(t, err)

	err = fs.ParseOrError([]string{"--tags", "web", "--tags", "ABC"})
	assert.NotNil(t, err)
	assert.Equal(t, "Invalid 'tags' value: ABC (must match regex: ^[a-z][a-z0-9-]*$)", err.Error())
}

func Test_IntSliceRangeConstraint(t *testing.T) {
	fs := NewCmd("test")
	scores, err := NewIntSlice("scores").
		SetMin(0, true).
		SetMax(100, true).
		Register(fs)
	assert.NoError(t, err)

	err = fs.ParseOrError([]string{"--scores", "0", "--scores", "100"})
	assert.Nil(t, err)
	assert.Equal(t, []int{0, 100}, *scores)
}

func Test_IntSliceRangeConstraintAboveMax(t *testing.T) {
	fs := NewCmd("test")
	_, err := NewIntSlice("scores").
		SetMin(0, true).
		SetMax(100, true).
		Register(fs)
	assert.NoError(t, err)

	err = fs.ParseOrError([]string{"--scores", "5", "--scores", "500"})
	assert.NotNil(t, err)
	assert.Equal(t, "'scores' value 500 is > maximum 100", err.Error())
}

func Test_IntSliceRangeConstraintBelowMin(t *testing.T) {
	fs := NewCmd("test")
	_, err := NewIntSlice("scores").SetMin(0, true).Register(fs)
	assert.NoError(t, err)

	err = fs.ParseOrError([]string{"--scores", "-1"})
	assert.NotNil(t, err)
	assert.Equal(t, "'scores' value -1 is < minimum 0", err.Error())
}

func Test_IntSliceRangeConstraintExclusive(t *testing.T) {
	fs := NewCmd("test")
	_, err := NewIntSlice("scores").SetMax(100, false).Register(fs)
	assert.NoError(t, err)

	err = fs.ParseOrError([]string{"--scores", "100"})
	assert.NotNil(t, err)
	assert.Equal(t, "'scores' value 100 is >= maximum (exclusive) 100", err.Error())
}

func Test_Int64SliceRangeConstraintError(t *testing.T) {
	fs := NewCmd("test")
	_, err := NewInt64Slice("ids").SetMax(10, true).Register(fs)
	assert.NoError(t, err)

	err = fs.ParseOrError([]string{"--ids", "11"})
	assert.NotNil(t, err)
	assert.Equal(t, "'ids' value 11 is > maximum 10", err.Error())
}

func Test_Float64SliceRangeConstraintError(t *testing.T) {
	fs := NewCmd("test")
	_, err := NewFloat64Slice("ratios").SetMin(0, true).SetMax(1, true).Register(fs)
	assert.NoError(t, err)

	err = fs.ParseOrError([]string{"--ratios", "0.5", "--ratios", "1.5"})
	assert.NotNil(t, err)
	assert.Equal(t, "'ratios' value 1.5 is > maximum 1", err.Error())
}

// A constraint that cannot apply to the element type is rejected at
// registration. Accepting and ignoring it is the bug this feature fixes.

func Test_SliceEnumConstraintRejectedOnNumbers(t *testing.T) {
	fs := NewCmd("test")
	_, err := NewIntSlice("scores").SetEnumConstraint([]string{"a"}).Register(fs)
	assert.NotNil(t, err)
	assert.Equal(t, `enum constraint on "scores" is not valid for number elements`, err.Error())
}

func Test_SliceRegexConstraintRejectedOnBools(t *testing.T) {
	fs := NewCmd("test")
	_, err := NewBoolSlice("flags").SetRegexConstraint(regexp.MustCompile("x")).Register(fs)
	assert.NotNil(t, err)
	assert.Equal(t, `regex constraint on "flags" is not valid for bool elements`, err.Error())
}

func Test_SliceRangeConstraintRejectedOnStrings(t *testing.T) {
	fs := NewCmd("test")
	_, err := NewStringSlice("names").SetMax(5, true).Register(fs)
	assert.NotNil(t, err)
	assert.Equal(t, `range constraint on "names" is not valid for string elements`, err.Error())
}

// Length bounds count the elements rather than checking them.

func Test_SliceLenConstraintExact(t *testing.T) {
	fs := NewCmd("test")
	pair, err := NewStringSlice("pair").SetMinLen(2, true).SetMaxLen(2, true).Register(fs)
	assert.NoError(t, err)

	err = fs.ParseOrError([]string{"--pair", "a", "--pair", "b"})
	assert.Nil(t, err)
	assert.Equal(t, []string{"a", "b"}, *pair)
}

func Test_SliceLenConstraintTooFew(t *testing.T) {
	fs := NewCmd("test")
	_, err := NewStringSlice("pair").SetMinLen(2, true).SetMaxLen(2, true).Register(fs)
	assert.NoError(t, err)

	err = fs.ParseOrError([]string{"--pair", "a"})
	assert.NotNil(t, err)
	assert.Equal(t, "'pair' takes at least 2 values, got 1", err.Error())
}

func Test_SliceLenConstraintTooMany(t *testing.T) {
	fs := NewCmd("test")
	_, err := NewStringSlice("pair").SetMinLen(2, true).SetMaxLen(2, true).Register(fs)
	assert.NoError(t, err)

	err = fs.ParseOrError([]string{"--pair", "a", "--pair", "b", "--pair", "c"})
	assert.NotNil(t, err)
	assert.Equal(t, "'pair' takes at most 2 values, got 3", err.Error())
}

func Test_SliceLenConstraintExclusiveMax(t *testing.T) {
	fs := NewCmd("test")
	_, err := NewIntSlice("sample").SetMinLen(3, true).SetMaxLen(5, false).Register(fs)
	assert.NoError(t, err)

	err = fs.ParseOrError([]string{"--sample", "1", "--sample", "2", "--sample", "3", "--sample", "4", "--sample", "5"})
	assert.NotNil(t, err)
	assert.Equal(t, "'sample' takes fewer than 5 values, got 5", err.Error())
}

func Test_SliceLenConstraintAllowsBoolElements(t *testing.T) {
	fs := NewCmd("test")
	_, err := NewBoolSlice("flags").SetMaxLen(1, true).Register(fs)
	assert.NoError(t, err)

	err = fs.ParseOrError([]string{"--flags", "true", "--flags", "false"})
	assert.NotNil(t, err)
	assert.Equal(t, "'flags' takes at most 1 values, got 2", err.Error())
}

// Constraints have to reach the help output, or an author has no way to tell
// what a script will accept without reading its source.

func Test_Usage_SliceConstraintsAreShown(t *testing.T) {
	cmd := NewCmd("test")

	_, err := NewIntSlice("scores").SetMin(0, true).SetMax(100, true).Register(cmd)
	assert.NoError(t, err)
	_, err = NewStringSlice("envs").SetEnumConstraint([]string{"dev", "prd"}).Register(cmd)
	assert.NoError(t, err)
	_, err = NewStringSlice("tags").SetRegexConstraint(regexp.MustCompile("^[a-z]+$")).Register(cmd)
	assert.NoError(t, err)
	_, err = NewStringSlice("pair").SetMinLen(2, true).SetMaxLen(2, true).Register(cmd)
	assert.NoError(t, err)

	expected := `Usage:
  test <scores...> [OPTIONS]

Arguments:
      --scores ints   Range: [0, 100]
      --envs strs     Valid values: [dev, prd]
      --tags strs     Regex: ^[a-z]+$
      --pair strs     Length: [2, 2]
`
	assert.Equal(t, expected, cmd.GenerateUsage(false))
}

func Test_Dump_SliceConstraintsAreEncoded(t *testing.T) {
	cmd := NewCmd("test")

	_, err := NewStringSlice("envs").
		SetEnumConstraint([]string{"dev", "prd"}).
		SetMinLen(1, true).
		Register(cmd)
	assert.NoError(t, err)
	_, err = NewIntSlice("scores").SetMin(0, true).SetMax(100, true).Register(cmd)
	assert.NoError(t, err)

	dump := cmd.GenerateDump([]string{})
	assert.Contains(t, dump, "[]string{dev,prd} len:[1, )")
	assert.Contains(t, dump, "[]int[0,100]")
}

// An element constraint and a length constraint coexist on one flag.
func Test_SliceElementAndLenConstraintsTogether(t *testing.T) {
	fs := NewCmd("test")
	_, err := NewIntSlice("scores").
		SetMin(0, true).
		SetMax(100, true).
		SetMinLen(2, true).
		Register(fs)
	assert.NoError(t, err)

	// The element check fires during parsing, before the count is even known.
	err = fs.ParseOrError([]string{"--scores", "500"})
	assert.NotNil(t, err)
	assert.Equal(t, "'scores' value 500 is > maximum 100", err.Error())

	fs2 := NewCmd("test")
	_, err = NewIntSlice("scores").
		SetMin(0, true).
		SetMax(100, true).
		SetMinLen(2, true).
		Register(fs2)
	assert.NoError(t, err)

	err = fs2.ParseOrError([]string{"--scores", "50"})
	assert.NotNil(t, err)
	assert.Equal(t, "'scores' takes at least 2 values, got 1", err.Error())
}
