package discord

import (
	"errors"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitMessage_Short(t *testing.T) {
	content := "Hello, world!"
	parts := SplitMessage(content)
	require.Len(t, parts, 1)
	assert.Equal(t, "Hello, world!", parts[0])
}

func TestSplitMessage_Exactly2000(t *testing.T) {
	content := makeString(2000)
	parts := SplitMessage(content)
	require.Len(t, parts, 1)
	assert.Len(t, parts[0], 2000)
}

func TestSplitMessage_MidRange_SplitsAtNewlines(t *testing.T) {
	// Build content with newline-separated sections totaling ~3000 chars
	section1 := makeString(1500) + "\n"
	section2 := makeString(1400)
	content := section1 + section2
	parts := SplitMessage(content)
	require.Len(t, parts, 2)
	assert.Equal(t, section1[:len(section1)-1], parts[0]) // newline stripped
	assert.Equal(t, section2, parts[1])
}

func TestSplitMessage_Over6000_ReturnsNil(t *testing.T) {
	content := makeString(6001)
	parts := SplitMessage(content)
	assert.Nil(t, parts)
}

func TestSplitMessage_ThreeWaySplit(t *testing.T) {
	// 3 sections of ~1800 chars each = ~5400 total
	s1 := makeString(1800)
	s2 := makeString(1800)
	s3 := makeString(1800)
	content := s1 + "\n" + s2 + "\n" + s3
	parts := SplitMessage(content)
	require.Len(t, parts, 3)
	assert.Equal(t, s1, parts[0])
	assert.Equal(t, s2, parts[1])
	assert.Equal(t, s3, parts[2])
}

func TestSplitMessage_NoNewlines_HardCut(t *testing.T) {
	content := makeString(4000)
	parts := SplitMessage(content)
	require.Len(t, parts, 2)
	assert.Len(t, parts[0], 2000)
	assert.Len(t, parts[1], 2000)
}

func TestSplitMessage_Empty(t *testing.T) {
	parts := SplitMessage("")
	require.Len(t, parts, 1)
	assert.Equal(t, "", parts[0])
}

func TestSplitMessage_Exactly6000(t *testing.T) {
	// 3 sections of 1998 chars + 2 newlines = 5998 chars; use exactly 6000
	s1 := makeString(1999)
	s2 := makeString(1999)
	s3 := makeString(2000)
	content := s1 + "\n" + s2 + "\n" + s3
	assert.Len(t, content, 6000)
	parts := SplitMessage(content)
	require.NotNil(t, parts)
	require.Len(t, parts, 3)
}

func TestSendContent_Short_SingleMessage(t *testing.T) {
	var sent []string
	mock := newTestMock()
	mock.ChannelMessageSendFunc = func(channelID, content string) (*discordgo.Message, error) {
		sent = append(sent, content)
		return &discordgo.Message{}, nil
	}

	err := SendContent(mock, "ch-1", "Hello!")
	require.NoError(t, err)
	require.Len(t, sent, 1)
	assert.Equal(t, "Hello!", sent[0])
}

func TestSendContent_MidRange_SplitsMessages(t *testing.T) {
	var sent []string
	mock := newTestMock()
	mock.ChannelMessageSendFunc = func(channelID, content string) (*discordgo.Message, error) {
		sent = append(sent, content)
		return &discordgo.Message{}, nil
	}

	s1 := makeString(1500)
	s2 := makeString(1500)
	content := s1 + "\n" + s2
	err := SendContent(mock, "ch-1", content)
	require.NoError(t, err)
	require.Len(t, sent, 2)
}

func TestSendContent_Large_SendsFileAttachment(t *testing.T) {
	var complexCalled bool
	var sentData *discordgo.MessageSend
	mock := newTestMock()
	mock.ChannelMessageSendComplexFunc = func(channelID string, data *discordgo.MessageSend) (*discordgo.Message, error) {
		complexCalled = true
		sentData = data
		return &discordgo.Message{}, nil
	}

	content := makeString(6001)
	err := SendContent(mock, "ch-1", content)
	require.NoError(t, err)
	assert.True(t, complexCalled)
	require.NotNil(t, sentData)
	assert.NotEmpty(t, sentData.Content) // summary line
	require.Len(t, sentData.Files, 1)
	assert.Equal(t, "details.txt", sentData.Files[0].Name)
}

func TestSendContent_SendError(t *testing.T) {
	mock := newTestMock()
	mock.ChannelMessageSendFunc = func(channelID, content string) (*discordgo.Message, error) {
		return nil, errors.New("send failed")
	}

	err := SendContent(mock, "ch-1", "Hello!")
	assert.Error(t, err)
}

func TestSendContent_FileAttachmentError(t *testing.T) {
	mock := newTestMock()
	mock.ChannelMessageSendComplexFunc = func(channelID string, data *discordgo.MessageSend) (*discordgo.Message, error) {
		return nil, errors.New("upload failed")
	}

	err := SendContent(mock, "ch-1", makeString(6001))
	assert.Error(t, err)
}

func TestSplitMessage_Exactly2001_SplitsIntoTwo(t *testing.T) {
	// Boundary test: 2001 chars should trigger splitting, not single message.
	content := makeString(1000) + "\n" + makeString(1000)
	require.Len(t, content, 2001)
	parts := SplitMessage(content)
	require.Len(t, parts, 2)
	assert.Equal(t, makeString(1000), parts[0])
	assert.Equal(t, makeString(1000), parts[1])
}

func TestSplitMessage_EarlyNewlines_NoDataLoss(t *testing.T) {
	// Build content with newlines every 100 chars (many small lines).
	// This triggers fallback to hard-cut when newline-splitting
	// would produce more than 3 parts.
	line := makeString(100) + "\n"
	content := strings.Repeat(line, 59) + makeString(37) // 59*101 + 37 = 5996
	require.Len(t, content, 5996)

	parts := SplitMessage(content)
	require.NotNil(t, parts, "should not return nil for content <= 6000")

	assertTotalLength(t, parts, len(content))
}

func TestSplitMessage_EarlyNewlines_FallsBackToHardCut(t *testing.T) {
	// When newline splitting would produce too many small parts,
	// the algorithm should use hard-cuts to stay within 3 parts.
	line := makeString(100) + "\n"
	content := strings.Repeat(line, 44) + makeString(5996-44*101) // 44*101=4444, pad to 5996
	require.Len(t, content, 5996)

	parts := SplitMessage(content)
	require.NotNil(t, parts)

	assertTotalLength(t, parts, len(content))
}

func TestNeedsFileAttachment(t *testing.T) {
	assert.False(t, NeedsFileAttachment("short"))
	assert.False(t, NeedsFileAttachment(makeString(2000)))
	assert.False(t, NeedsFileAttachment(makeString(6000)))
	assert.True(t, NeedsFileAttachment(makeString(6001)))
}

func assertTotalLength(t *testing.T, parts []string, expected int) {
	t.Helper()
	total := 0
	for _, p := range parts {
		total += len(p)
	}
	assert.Equal(t, expected, total,
		"all content must be preserved; got %d chars across %d parts", total, len(parts))
}

func makeString(n int) string {
	return strings.Repeat("a", n)
}
