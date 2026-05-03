package playtest_test

import (
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ab/dndnd/internal/playtest"
)

func TestParse_Positional(t *testing.T) {
	got, err := playtest.Parse("/move A1")
	require.NoError(t, err)
	assert.Equal(t, "move", got.Name)
	assert.Equal(t, []string{"A1"}, got.Args)
	assert.Empty(t, got.NamedArgs)
}

func TestParse_Named(t *testing.T) {
	got, err := playtest.Parse("/attack target:G2 weapon:handaxe")
	require.NoError(t, err)
	assert.Equal(t, "attack", got.Name)
	assert.Empty(t, got.Args)
	assert.Equal(t, map[string]string{"target": "G2", "weapon": "handaxe"}, got.NamedArgs)
}

func TestParse_Mixed(t *testing.T) {
	got, err := playtest.Parse("  /cast fireball target:G4  ")
	require.NoError(t, err)
	assert.Equal(t, "cast", got.Name)
	assert.Equal(t, []string{"fireball"}, got.Args)
	assert.Equal(t, map[string]string{"target": "G4"}, got.NamedArgs)
}

func TestParse_Errors(t *testing.T) {
	cases := []string{"", "   ", "move A1", "/", "/   "}
	for _, in := range cases {
		_, err := playtest.Parse(in)
		assert.Error(t, err, "input %q", in)
	}
}

func sampleTable() *playtest.CommandTable {
	return playtest.NewCommandTable([]*discordgo.ApplicationCommand{
		{Name: "move", Options: []*discordgo.ApplicationCommandOption{
			{Name: "coordinate", Required: true},
		}},
		{Name: "attack", Options: []*discordgo.ApplicationCommandOption{
			{Name: "target", Required: true},
			{Name: "weapon"},
		}},
		{Name: "status"},
		{Name: "setup"}, // present in bot but not in player allow-list
		nil,
		{Name: ""},
	})
}

func TestNewCommandTable_DropsNilAndEmpty(t *testing.T) {
	tbl := sampleTable()
	assert.ElementsMatch(t, []string{"attack", "move", "setup", "status"}, tbl.Names())
}

func TestValidate_OKPositional(t *testing.T) {
	tbl := sampleTable()
	cmd, _ := playtest.Parse("/move A1")
	res := playtest.Validate(cmd, tbl)
	assert.True(t, res.OK, res.Reason)
}

func TestValidate_OKNamed(t *testing.T) {
	tbl := sampleTable()
	cmd, _ := playtest.Parse("/attack target:G2 weapon:handaxe")
	res := playtest.Validate(cmd, tbl)
	assert.True(t, res.OK, res.Reason)
}

func TestValidate_MissingRequired(t *testing.T) {
	tbl := sampleTable()
	cmd, _ := playtest.Parse("/move")
	res := playtest.Validate(cmd, tbl)
	assert.False(t, res.OK)
	assert.Equal(t, []string{"coordinate"}, res.Required)
}

func TestValidate_RejectsAdminCommand(t *testing.T) {
	tbl := sampleTable()
	cmd, _ := playtest.Parse("/setup")
	res := playtest.Validate(cmd, tbl)
	assert.False(t, res.OK)
	assert.Contains(t, res.Reason, "not a player command")
}

func TestValidate_UnknownToBot(t *testing.T) {
	tbl := sampleTable()
	cmd, _ := playtest.Parse("/recap")
	res := playtest.Validate(cmd, tbl)
	assert.False(t, res.OK)
	assert.Contains(t, res.Reason, "not registered")
}

func TestValidate_NamedSatisfiesPositionalGap(t *testing.T) {
	tbl := sampleTable()
	cmd, _ := playtest.Parse("/move coordinate:A1")
	res := playtest.Validate(cmd, tbl)
	assert.True(t, res.OK, res.Reason)
}

func TestFormat_RoundTrips(t *testing.T) {
	in := "/attack target:G2 weapon:handaxe"
	cmd, err := playtest.Parse(in)
	require.NoError(t, err)
	assert.Equal(t, in, playtest.Format(cmd))
}

func TestFormat_PositionalAndNamed(t *testing.T) {
	cmd, err := playtest.Parse("/cast fireball target:G4")
	require.NoError(t, err)
	assert.Equal(t, "/cast fireball target:G4", playtest.Format(cmd))
}
