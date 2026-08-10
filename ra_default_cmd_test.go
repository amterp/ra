package ra

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// defaultCmdFixture builds the shape these tests keep needing: a root with two
// sub-commands, `add` marked default and taking one positional, `list` taking
// none. It mirrors what a CLI with a primary action plus a few sub-commands
// looks like.
func defaultCmdFixture(t *testing.T) (root *Cmd, addName *string, addUsed, listUsed *bool) {
	t.Helper()

	root = NewCmd("tool")

	add := NewCmd("add")
	var err error
	addName, err = NewString("name").Register(add)
	assert.NoError(t, err)
	addUsed, err = root.RegisterCmd(add)
	assert.NoError(t, err)

	list := NewCmd("list")
	listUsed, err = root.RegisterCmd(list)
	assert.NoError(t, err)

	assert.NoError(t, root.SetDefaultCmd("add"))
	return root, addName, addUsed, listUsed
}

func Test_DefaultCmd_BarePositionalRoutesToDefault(t *testing.T) {
	root, addName, addUsed, listUsed := defaultCmdFixture(t)

	assert.NoError(t, root.ParseOrError([]string{"origin"}))

	assert.True(t, *addUsed)
	assert.False(t, *listUsed)
	assert.Equal(t, "origin", *addName)
}

func Test_DefaultCmd_StillInvokableByName(t *testing.T) {
	root, addName, addUsed, _ := defaultCmdFixture(t)

	assert.NoError(t, root.ParseOrError([]string{"add", "origin"}))

	assert.True(t, *addUsed)
	assert.Equal(t, "origin", *addName)
}

func Test_DefaultCmd_SiblingNameWinsOverDefault(t *testing.T) {
	root, _, addUsed, listUsed := defaultCmdFixture(t)

	assert.NoError(t, root.ParseOrError([]string{"list"}))

	assert.True(t, *listUsed)
	assert.False(t, *addUsed)
}

// The escape hatch when a value collides with a sibling's name: the explicit
// form. Without it there is no way to say "add a thing called list".
func Test_DefaultCmd_ExplicitNameEscapesCollision(t *testing.T) {
	root, addName, addUsed, listUsed := defaultCmdFixture(t)

	assert.NoError(t, root.ParseOrError([]string{"add", "list"}))

	assert.True(t, *addUsed)
	assert.False(t, *listUsed)
	assert.Equal(t, "list", *addName)
}

// The other escape hatch, and the conventional one. `--` suppresses sub-command
// lookup but must NOT suppress default routing, or it would turn a documented
// escape into an excess-positional error.
func Test_DefaultCmd_DashDashEscapesCollision(t *testing.T) {
	root, addName, addUsed, listUsed := defaultCmdFixture(t)

	assert.NoError(t, root.ParseOrError([]string{"--", "list"}))

	assert.True(t, *addUsed)
	assert.False(t, *listUsed)
	assert.Equal(t, "list", *addName)
}

// A default's own flags are registered on it, not on the parent, so without the
// unknown-flag route only the positional spellings would be reachable.
func Test_DefaultCmd_UnknownFlagRoutesToDefault(t *testing.T) {
	root, addName, addUsed, _ := defaultCmdFixture(t)

	assert.NoError(t, root.ParseOrError([]string{"--name", "origin"}))

	assert.True(t, *addUsed)
	assert.Equal(t, "origin", *addName)
}

func Test_DefaultCmd_BareInvocationRunsAllOptionalDefault(t *testing.T) {
	root := NewCmd("tool")

	sub := NewCmd("show")
	page, err := NewString("page").SetOptional(true).Register(sub)
	assert.NoError(t, err)
	subUsed, err := root.RegisterCmd(sub)
	assert.NoError(t, err)
	assert.NoError(t, root.SetDefaultCmd("show"))

	assert.NoError(t, root.ParseOrError([]string{}))

	assert.True(t, *subUsed)
	assert.Equal(t, "", *page)
}

// A bare invocation whose default needs an argument should show this level's
// help, which lists every command. Routing instead would answer with "missing
// required argument" and never mention that `list` exists.
func Test_DefaultCmd_BareInvocationWithRequiredDefaultShowsHelp(t *testing.T) {
	root, _, _, _ := defaultCmdFixture(t)
	root.SetAutoHelpOnNoArgs(true)

	err := root.ParseOrError([]string{})

	assert.ErrorIs(t, err, HelpInvokedErr)
}

func Test_DefaultCmd_NestedDefaultRoutesTwoLevels(t *testing.T) {
	root := NewCmd("tool")

	remote := NewCmd("remote")
	remoteUsed, err := root.RegisterCmd(remote)
	assert.NoError(t, err)

	add := NewCmd("add")
	addName, err := NewString("name").Register(add)
	assert.NoError(t, err)
	addUsed, err := remote.RegisterCmd(add)
	assert.NoError(t, err)
	assert.NoError(t, remote.SetDefaultCmd("add"))

	assert.NoError(t, root.ParseOrError([]string{"remote", "origin"}))

	assert.True(t, *remoteUsed)
	assert.True(t, *addUsed)
	assert.Equal(t, "origin", *addName)
}

func Test_DefaultCmd_SetDefaultCmdRejectsUnregisteredName(t *testing.T) {
	root := NewCmd("tool")
	_, err := root.RegisterCmd(NewCmd("add"))
	assert.NoError(t, err)

	err = root.SetDefaultCmd("nope")

	assert.EqualError(t, err, `cannot set default command "nope": not registered`)
}

func Test_DefaultCmd_SetDefaultCmdRejectsSecondDefault(t *testing.T) {
	root, _, _, _ := defaultCmdFixture(t)

	err := root.SetDefaultCmd("list")

	assert.EqualError(t, err, `default command already set to "add"`)
}

// A list positional swallows every bare value, so nothing is ever left over to
// route. Legal but almost never meant, so it is refused at registration rather
// than debugged from a default that mysteriously never fires.
func Test_DefaultCmd_SetDefaultCmdRejectsListPositionalHost(t *testing.T) {
	root := NewCmd("tool")
	_, err := NewStringSlice("files").Register(root)
	assert.NoError(t, err)
	_, err = root.RegisterCmd(NewCmd("add"))
	assert.NoError(t, err)

	err = root.SetDefaultCmd("add")

	assert.EqualError(t, err,
		`cannot set default command "add": "files" is a list positional and would absorb the values meant for it`)
}

// The regression guarantee: with no default registered, every one of these
// paths must behave exactly as it did before default routing existed.
func Test_DefaultCmd_AbsentDefaultLeavesExcessPositionalsErroring(t *testing.T) {
	root := NewCmd("tool")
	_, err := root.RegisterCmd(NewCmd("add"))
	assert.NoError(t, err)

	err = root.ParseOrError([]string{"stray"})

	var tooMany *TooManyPositionalArgsError
	assert.ErrorAs(t, err, &tooMany)
	assert.Equal(t, []string{"stray"}, tooMany.Unused)
}

func Test_DefaultCmd_AbsentDefaultLeavesUnknownFlagErroring(t *testing.T) {
	root := NewCmd("tool")
	_, err := root.RegisterCmd(NewCmd("add"))
	assert.NoError(t, err)

	err = root.ParseOrError([]string{"--nope"})

	assert.ErrorContains(t, err, "unknown flag")
}
