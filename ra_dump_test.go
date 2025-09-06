package ra

import (
	"bytes"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDumpParseOrError(t *testing.T) {
	t.Setenv("RA_COLOR", "auto")

	cmd := NewCmd("testapp")
	cmd.SetDescription("Test application")

	inputFlag, err := NewString("input").
		SetShort("i").
		SetUsage("Input file path").
		Register(cmd)
	assert.NoError(t, err)

	err = cmd.ParseOrError([]string{"test.txt"}, WithDump(true))

	assert.Error(t, err)
	assert.True(t, errors.Is(err, DumpInvokedErr))
	assert.Equal(t, "", *inputFlag)
}

func TestDumpParseOrExit(t *testing.T) {
	t.Setenv("RA_COLOR", "auto")

	cmd := NewCmd("testapp")
	cmd.SetDescription("Test application")

	_, err := NewString("input").
		SetShort("i").
		SetUsage("Input file path").
		Register(cmd)
	assert.NoError(t, err)

	var stdout bytes.Buffer
	SetStdoutWriter(&stdout)
	defer SetStdoutWriter(os.Stdout)

	var exitCalled bool
	var exitCode int
	SetExitFunc(func(code int) {
		exitCalled = true
		exitCode = code
	})
	defer SetExitFunc(os.Exit)

	args := []string{"--input", "test.txt"}
	cmd.ParseOrExit(args, WithDump(true))

	assert.True(t, exitCalled)
	assert.Equal(t, 0, exitCode)

	expected := `Ra Command Dump
==================================================

Parse Configuration:
  Ignore Unknown: false
  Dump Enabled: true

Command Information:
  Name: testapp
  Description: Test application
  Help Enabled: true
  Auto Help on No Args: false
  Exclude Name from Usage: false
  Custom Usage Function: not set
  PostParse Hook: not set
  Subcommands: none

Arguments to Parse:
  [0]: "--input"
  [1]: "test.txt"

Flags Structure:
  Total Flags: 2
  Positional Flags: 1
  Non-Positional Flags: 1
  Global Flags: 1

  Positional Flags (in order):
    [0] input (-i) type:string required usage:"Input file path"

  Non-Positional Flags:
    help (-h) type:bool optional flags:[flag-only] usage:"Print usage string."

  Global Flags:
    help (-h) type:bool optional flags:[flag-only] usage:"Print usage string."

Environment:
  RA_COLOR: auto
`

	assert.Equal(t, expected, stdout.String())
}

func TestDumpWithSubcommands(t *testing.T) {
	t.Setenv("RA_COLOR", "auto")

	parent := NewCmd("parent")
	parent.SetDescription("Parent command")

	_, err := NewBool("global").
		SetShort("g").
		SetUsage("Global flag").
		Register(parent, WithGlobal(true))
	assert.NoError(t, err)

	// First subcommand with a grandchild
	sub := NewCmd("sub")
	sub.SetDescription("Subcommand")

	_, err = NewString("local").
		SetShort("l").
		SetUsage("Local flag").
		Register(sub)
	assert.NoError(t, err)

	// Grandchild under first subcommand
	grandchild := NewCmd("grandchild")
	grandchild.SetDescription("Grandchild command")

	_, err = NewInt("count").
		SetShort("c").
		SetUsage("Count value").
		SetDefault(42).
		Register(grandchild)
	assert.NoError(t, err)

	_, err = sub.RegisterCmd(grandchild)
	assert.NoError(t, err)

	// Second subcommand
	sub2 := NewCmd("sub2")
	sub2.SetDescription("Second subcommand")

	_, err = NewBool("enabled").
		SetShort("e").
		SetUsage("Enable feature").
		SetDefault(true).
		Register(sub2)
	assert.NoError(t, err)

	subUsed, err := parent.RegisterCmd(sub)
	assert.NoError(t, err)

	_, err = parent.RegisterCmd(sub2)
	assert.NoError(t, err)

	var stdout bytes.Buffer
	SetStdoutWriter(&stdout)
	defer SetStdoutWriter(os.Stdout)

	var exitCalled bool
	SetExitFunc(func(code int) { exitCalled = true })
	defer SetExitFunc(os.Exit)

	parent.ParseOrExit([]string{}, WithDump(true))

	assert.True(t, exitCalled)
	assert.False(t, *subUsed)

	expected := `Ra Command Dump
==================================================

Parse Configuration:
  Ignore Unknown: false
  Dump Enabled: true

Command Information:
  Name: parent
  Description: Parent command
  Help Enabled: true
  Auto Help on No Args: false
  Exclude Name from Usage: false
  Custom Usage Function: not set
  PostParse Hook: not set
  Subcommands (2): sub, sub2

Arguments to Parse:
  <no arguments>

Flags Structure:
  Total Flags: 2
  Positional Flags: 0
  Non-Positional Flags: 2
  Global Flags: 2

  Non-Positional Flags:
    global (-g) type:bool optional (default:false) flags:[flag-only] usage:"Global flag"
    help (-h) type:bool optional flags:[flag-only] usage:"Print usage string."

  Global Flags:
    global (-g) type:bool optional (default:false) flags:[flag-only] usage:"Global flag"
    help (-h) type:bool optional flags:[flag-only] usage:"Print usage string."

Environment:
  RA_COLOR: auto

Subcommand Details:
==================================================

  Subcommand Dump (sub)
  ------------------------------

  Parse Configuration:
    Ignore Unknown: false
    Dump Enabled: true

  Command Information:
    Name: sub
    Description: Subcommand
    Help Enabled: true
    Auto Help on No Args: false
    Exclude Name from Usage: false
    Custom Usage Function: not set
    PostParse Hook: not set
    Subcommands (1): grandchild

  Flags Structure:
    Total Flags: 2
    Positional Flags: 1
    Non-Positional Flags: 1
    Global Flags: 1

    Positional Flags (in order):
      [0] local (-l) type:string required usage:"Local flag"

    Non-Positional Flags:
      global (-g) type:bool optional (default:false) flags:[flag-only] usage:"Global flag"

    Global Flags:
      global (-g) type:bool optional (default:false) flags:[flag-only] usage:"Global flag"

    Subcommand Dump (grandchild)
    ------------------------------

    Parse Configuration:
      Ignore Unknown: false
      Dump Enabled: true

    Command Information:
      Name: grandchild
      Description: Grandchild command
      Help Enabled: true
      Auto Help on No Args: false
      Exclude Name from Usage: false
      Custom Usage Function: not set
      PostParse Hook: not set
      Subcommands: none

    Flags Structure:
      Total Flags: 1
      Positional Flags: 1
      Non-Positional Flags: 0
      Global Flags: 0

      Positional Flags (in order):
        [0] count (-c) type:int optional (default:42) usage:"Count value"


  Subcommand Dump (sub2)
  ------------------------------

  Parse Configuration:
    Ignore Unknown: false
    Dump Enabled: true

  Command Information:
    Name: sub2
    Description: Second subcommand
    Help Enabled: true
    Auto Help on No Args: false
    Exclude Name from Usage: false
    Custom Usage Function: not set
    PostParse Hook: not set
    Subcommands: none

  Flags Structure:
    Total Flags: 2
    Positional Flags: 0
    Non-Positional Flags: 2
    Global Flags: 1

    Non-Positional Flags:
      enabled (-e) type:bool optional (default:true) current:false usage:"Enable feature"
      global (-g) type:bool optional (default:false) flags:[flag-only] usage:"Global flag"

    Global Flags:
      global (-g) type:bool optional (default:false) flags:[flag-only] usage:"Global flag"

`

	assert.Equal(t, expected, stdout.String())
}

func TestDumpWithComplexFlags(t *testing.T) {
	t.Setenv("RA_COLOR", "auto")

	cmd := NewCmd("complex")

	_, err := NewString("input").
		SetShort("i").
		SetUsage("Input file").
		SetEnumConstraint([]string{"json", "xml"}).
		Register(cmd)
	assert.NoError(t, err)

	_, err = NewInt("count").
		SetShort("c").
		SetUsage("Count").
		SetDefault(10).
		SetMin(1, false).
		SetMax(100, true).
		Register(cmd)
	assert.NoError(t, err)

	_, err = NewStringSlice("tags").
		SetShort("t").
		SetUsage("Tags").
		SetVariadic(true).
		Register(cmd)
	assert.NoError(t, err)

	var stdout bytes.Buffer
	SetStdoutWriter(&stdout)
	defer SetStdoutWriter(os.Stdout)

	var exitCalled bool
	SetExitFunc(func(code int) { exitCalled = true })
	defer SetExitFunc(os.Exit)

	cmd.ParseOrExit([]string{"--input", "json", "--count", "5", "tag1"}, WithDump(true))

	assert.True(t, exitCalled)

	expected := `Ra Command Dump
==================================================

Parse Configuration:
  Ignore Unknown: false
  Dump Enabled: true

Command Information:
  Name: complex
  Description: <not set>
  Help Enabled: true
  Auto Help on No Args: false
  Exclude Name from Usage: false
  Custom Usage Function: not set
  PostParse Hook: not set
  Subcommands: none

Arguments to Parse:
  [0]: "--input"
  [1]: "json"
  [2]: "--count"
  [3]: "5"
  [4]: "tag1"

Flags Structure:
  Total Flags: 4
  Positional Flags: 3
  Non-Positional Flags: 1
  Global Flags: 1

  Positional Flags (in order):
    [0] input (-i) type:string{json,xml} required usage:"Input file"
    [1] count (-c) type:int(1,100] optional (default:10) current:10 usage:"Count"
    [2] tags (-t) type:[]string(variadic) required usage:"Tags"

  Non-Positional Flags:
    help (-h) type:bool optional flags:[flag-only] usage:"Print usage string."

  Global Flags:
    help (-h) type:bool optional flags:[flag-only] usage:"Print usage string."

Environment:
  RA_COLOR: auto
`

	assert.Equal(t, expected, stdout.String())
}
