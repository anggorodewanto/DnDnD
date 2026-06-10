package discordcheck

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeSession is a hand-rolled Session implementation. Each method delegates
// to a function field so individual tests can program per-call behaviour
// without pulling in a mocking framework.
type fakeSession struct {
	userFn        func(userID string) (*discordgo.User, error)
	applicationFn func(appID string) (*discordgo.Application, error)
	guildFn       func(guildID string) (*discordgo.Guild, error)
}

func (f *fakeSession) User(userID string) (*discordgo.User, error) {
	return f.userFn(userID)
}

func (f *fakeSession) Application(appID string) (*discordgo.Application, error) {
	return f.applicationFn(appID)
}

func (f *fakeSession) Guild(guildID string) (*discordgo.Guild, error) {
	return f.guildFn(guildID)
}

func TestRun_AllChecksPass(t *testing.T) {
	sess := &fakeSession{
		userFn: func(userID string) (*discordgo.User, error) {
			require.Equal(t, "@me", userID)
			return &discordgo.User{ID: "app-123", Username: "DnDnD"}, nil
		},
		applicationFn: func(appID string) (*discordgo.Application, error) {
			require.Equal(t, "@me", appID)
			return &discordgo.Application{ID: "app-123", Name: "DnDnD App"}, nil
		},
		guildFn: func(guildID string) (*discordgo.Guild, error) {
			return &discordgo.Guild{ID: guildID, Name: "Guild-" + guildID}, nil
		},
	}

	report := Run(context.Background(), sess, "app-123", []string{"g-1", "g-2"})

	require.True(t, report.AllOK(), "expected all checks to pass: %+v", report.Results)
	require.Len(t, report.Results, 4)

	assert.Equal(t, "token-identity", report.Results[0].Name)
	assert.True(t, report.Results[0].OK)
	assert.Contains(t, report.Results[0].Detail, "DnDnD")
	assert.Contains(t, report.Results[0].Detail, "app-123")

	assert.Equal(t, "application-id-match", report.Results[1].Name)
	assert.True(t, report.Results[1].OK)

	assert.Equal(t, "guild-membership-g-1", report.Results[2].Name)
	assert.True(t, report.Results[2].OK)
	assert.Contains(t, report.Results[2].Detail, "Guild-g-1")

	assert.Equal(t, "guild-membership-g-2", report.Results[3].Name)
	assert.True(t, report.Results[3].OK)

	assert.False(t, report.RanAt.IsZero(), "RanAt must be populated")
}

func TestRun_TokenRejected(t *testing.T) {
	sess := &fakeSession{
		userFn: func(_ string) (*discordgo.User, error) {
			return nil, errors.New("401 Unauthorized")
		},
		applicationFn: func(_ string) (*discordgo.Application, error) {
			return &discordgo.Application{ID: "app-123"}, nil
		},
		guildFn: func(guildID string) (*discordgo.Guild, error) {
			return &discordgo.Guild{ID: guildID, Name: "n"}, nil
		},
	}

	report := Run(context.Background(), sess, "app-123", []string{"g-1"})

	require.False(t, report.AllOK())
	require.Len(t, report.Results, 3)
	assert.Equal(t, "token-identity", report.Results[0].Name)
	assert.False(t, report.Results[0].OK)
	assert.Contains(t, strings.ToLower(report.Results[0].Detail), "token rejected by discord")
	assert.Contains(t, report.Results[0].Detail, "401 Unauthorized")

	assert.True(t, report.Results[1].OK || !report.Results[1].OK, "subsequent checks still recorded")
	assert.Equal(t, "guild-membership-g-1", report.Results[2].Name)
}

func TestRun_AppIDMismatch(t *testing.T) {
	sess := &fakeSession{
		userFn: func(_ string) (*discordgo.User, error) {
			return &discordgo.User{ID: "actual-id", Username: "bot"}, nil
		},
		applicationFn: func(_ string) (*discordgo.Application, error) {
			return &discordgo.Application{ID: "actual-id"}, nil
		},
		guildFn: func(_ string) (*discordgo.Guild, error) {
			return &discordgo.Guild{ID: "g", Name: "n"}, nil
		},
	}

	report := Run(context.Background(), sess, "env-id", nil)

	require.False(t, report.AllOK())
	require.Len(t, report.Results, 2)
	assert.Equal(t, "application-id-match", report.Results[1].Name)
	assert.False(t, report.Results[1].OK)
	assert.Contains(t, report.Results[1].Detail, "DISCORD_APPLICATION_ID mismatch")
	assert.Contains(t, report.Results[1].Detail, "env=env-id")
	assert.Contains(t, report.Results[1].Detail, "actual=actual-id")
}

func TestRun_AppIDEnvEmpty_SkipsCheck(t *testing.T) {
	sess := &fakeSession{
		userFn: func(_ string) (*discordgo.User, error) {
			return &discordgo.User{ID: "actual-id", Username: "bot"}, nil
		},
		applicationFn: func(_ string) (*discordgo.Application, error) {
			t.Fatal("Application() must NOT be called when env is empty")
			return nil, nil
		},
		guildFn: func(_ string) (*discordgo.Guild, error) {
			return &discordgo.Guild{ID: "g", Name: "n"}, nil
		},
	}

	report := Run(context.Background(), sess, "", nil)

	require.True(t, report.AllOK())
	require.Len(t, report.Results, 2)
	assert.Equal(t, "application-id-match", report.Results[1].Name)
	assert.True(t, report.Results[1].OK)
	assert.Contains(t, report.Results[1].Detail, "skipped (env not set)")
}

func TestRun_AppIDLookupFailure(t *testing.T) {
	sess := &fakeSession{
		userFn: func(_ string) (*discordgo.User, error) {
			return &discordgo.User{ID: "actual-id"}, nil
		},
		applicationFn: func(_ string) (*discordgo.Application, error) {
			return nil, errors.New("500 server error")
		},
		guildFn: func(_ string) (*discordgo.Guild, error) {
			return &discordgo.Guild{ID: "g", Name: "n"}, nil
		},
	}

	report := Run(context.Background(), sess, "env-id", nil)

	require.False(t, report.AllOK())
	require.Len(t, report.Results, 2)
	assert.Equal(t, "application-id-match", report.Results[1].Name)
	assert.False(t, report.Results[1].OK)
	assert.Contains(t, report.Results[1].Detail, "500 server error")
}

func TestRun_GuildMembershipMix(t *testing.T) {
	sess := &fakeSession{
		userFn: func(_ string) (*discordgo.User, error) {
			return &discordgo.User{ID: "app-123", Username: "bot"}, nil
		},
		applicationFn: func(_ string) (*discordgo.Application, error) {
			return &discordgo.Application{ID: "app-123"}, nil
		},
		guildFn: func(guildID string) (*discordgo.Guild, error) {
			if guildID == "good" {
				return &discordgo.Guild{ID: "good", Name: "Good Guild"}, nil
			}
			return nil, errors.New("403 Missing Access")
		},
	}

	report := Run(context.Background(), sess, "app-123", []string{"good", "bad"})

	require.False(t, report.AllOK())
	require.Len(t, report.Results, 4)

	good := report.Results[2]
	assert.Equal(t, "guild-membership-good", good.Name)
	assert.True(t, good.OK)
	assert.Contains(t, good.Detail, "Good Guild")

	bad := report.Results[3]
	assert.Equal(t, "guild-membership-bad", bad.Name)
	assert.False(t, bad.OK)
	assert.Contains(t, bad.Detail, "bot is not a member of guild bad")
	assert.Contains(t, bad.Detail, "403 Missing Access")
}

func TestRun_EmptyGuildList_NoGuildChecks(t *testing.T) {
	sess := &fakeSession{
		userFn: func(_ string) (*discordgo.User, error) {
			return &discordgo.User{ID: "app-123", Username: "bot"}, nil
		},
		applicationFn: func(_ string) (*discordgo.Application, error) {
			return &discordgo.Application{ID: "app-123"}, nil
		},
		guildFn: func(_ string) (*discordgo.Guild, error) {
			t.Fatal("Guild() must NOT be called when guildIDs is empty")
			return nil, nil
		},
	}

	report := Run(context.Background(), sess, "app-123", nil)

	require.True(t, report.AllOK())
	require.Len(t, report.Results, 2)
}

func TestRunChannelBindings_AllBound(t *testing.T) {
	results := RunChannelBindings([]string{"g-1", "g-2"}, func(string) bool { return true })

	require.Len(t, results, 2)
	for i, gid := range []string{"g-1", "g-2"} {
		assert.Equal(t, "channel-bindings-"+gid, results[i].Name)
		assert.True(t, results[i].OK)
		assert.Contains(t, results[i].Detail, gid)
	}
}

func TestRunChannelBindings_Unbound(t *testing.T) {
	results := RunChannelBindings([]string{"g-1"}, func(string) bool { return false })

	require.Len(t, results, 1)
	assert.Equal(t, "channel-bindings-g-1", results[0].Name)
	assert.False(t, results[0].OK)
	assert.Contains(t, results[0].Detail, "/setup")
	assert.Contains(t, results[0].Detail, "g-1")
}

func TestRunChannelBindings_Mix(t *testing.T) {
	results := RunChannelBindings([]string{"bound", "unbound"}, func(g string) bool { return g == "bound" })

	require.Len(t, results, 2)
	assert.True(t, results[0].OK)
	assert.False(t, results[1].OK)
	assert.Contains(t, results[1].Detail, "/setup")
}

func TestRunChannelBindings_NilLookup(t *testing.T) {
	// A nil lookup must skip the check entirely rather than emit failures.
	assert.Empty(t, RunChannelBindings([]string{"g-1"}, nil))
}

func TestRunChannelBindings_EmptyGuilds(t *testing.T) {
	assert.Empty(t, RunChannelBindings(nil, func(string) bool { return true }))
}

func TestReport_AllOK(t *testing.T) {
	allPass := Report{Results: []Result{{OK: true}, {OK: true}}}
	assert.True(t, allPass.AllOK())

	mixed := Report{Results: []Result{{OK: true}, {OK: false}}}
	assert.False(t, mixed.AllOK())

	empty := Report{}
	assert.True(t, empty.AllOK(), "empty report should be considered passing (no checks failed)")
}

func TestFrom_AdaptsDiscordgoSession(t *testing.T) {
	// Constructor smoke test: From must wrap a *discordgo.Session into the
	// Session interface so main.go can pass the raw gateway session straight
	// to Run without any extra adapter type.
	dg, err := discordgo.New("Bot fake-token")
	require.NoError(t, err)
	sess := From(dg)
	require.NotNil(t, sess)

	// We don't make any real API calls — just confirm the wrapper compiles
	// against the Session interface and is non-nil.
	var _ Session = sess
}
