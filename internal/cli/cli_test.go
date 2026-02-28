package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionFlag(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := RunWithArgs("0.1.0-test", []string{"--version"})

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "chronicle 0.1.0-test")
}

func TestVersionOutputFormat(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	_ = RunWithArgs("1.2.3", []string{"--version"})

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := strings.TrimSpace(buf.String())

	assert.Equal(t, "chronicle 1.2.3", output)
}

func TestStatusSubcommandRecognized(t *testing.T) {
	parser, _, _ := buildParser("test")
	_, err := parser.ParseArgs([]string{"status"})
	assert.NoError(t, err)
}

func TestSearchSubcommandRecognized(t *testing.T) {
	parser, _, _ := buildParser("test")
	_, err := parser.ParseArgs([]string{"search", "test query"})
	assert.NoError(t, err)
}

func TestOpenSubcommandRecognized(t *testing.T) {
	parser, _, _ := buildParser("test")
	_, err := parser.ParseArgs([]string{"open", "--id", "CHR-abc123"})
	assert.NoError(t, err)
}

func TestAddSubcommandRecognized(t *testing.T) {
	parser, _, _ := buildParser("test")
	_, err := parser.ParseArgs([]string{"add", "--url", "https://example.com", "--title", "Test"})
	assert.NoError(t, err)
}

func TestIngestSubcommandRecognized(t *testing.T) {
	parser, _, _ := buildParser("test")
	_, err := parser.ParseArgs([]string{"ingest"})
	assert.NoError(t, err)
}

func TestPruneSubcommandRecognized(t *testing.T) {
	parser, _, _ := buildParser("test")
	_, err := parser.ParseArgs([]string{"prune"})
	assert.NoError(t, err)
}

func TestPurgeSubcommandRecognized(t *testing.T) {
	parser, _, _ := buildParser("test")
	_, err := parser.ParseArgs([]string{"purge", "--all"})
	assert.NoError(t, err)
}

func TestOpenRequiresID(t *testing.T) {
	err := RunWithArgs("test", []string{"open"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--id is required")
}

func TestAddRequiresURL(t *testing.T) {
	err := RunWithArgs("test", []string{"add", "--title", "Test"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--url is required")
}

func TestAddRequiresTitle(t *testing.T) {
	err := RunWithArgs("test", []string{"add", "--url", "https://example.com"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--title is required")
}

func TestPurgeRequiresAll(t *testing.T) {
	err := RunWithArgs("test", []string{"purge"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--all flag is required")
}

func TestSearchFlagsDefaults(t *testing.T) {
	_, _, cmds := buildParser("test")
	parser, _, _ := buildParser("test")
	// Need fresh parser+cmds together
	_ = parser
	_ = cmds

	p, _, c := buildParser("test")
	_, err := p.ParseArgs([]string{"search", "my query"})
	require.NoError(t, err)

	assert.Equal(t, "30d", c.Search.Since)
	assert.Equal(t, 10, c.Search.Limit)
	assert.Equal(t, 0, c.Search.Offset)
}

func TestGlobalFlagsJSON(t *testing.T) {
	parser, globals, _ := buildParser("test")
	_, err := parser.ParseArgs([]string{"--json", "status"})
	require.NoError(t, err)
	assert.True(t, globals.JSON)
}

func TestGlobalFlagsVerbose(t *testing.T) {
	parser, globals, _ := buildParser("test")
	_, err := parser.ParseArgs([]string{"--verbose", "status"})
	require.NoError(t, err)
	assert.True(t, globals.Verbose)
}

func TestGlobalFlagsConfig(t *testing.T) {
	parser, globals, _ := buildParser("test")
	_, err := parser.ParseArgs([]string{"--config", "/tmp/test.yaml", "status"})
	require.NoError(t, err)
	assert.Equal(t, "/tmp/test.yaml", globals.Config)
}

func TestUnknownSubcommandFails(t *testing.T) {
	parser, _, _ := buildParser("test")
	_, err := parser.ParseArgs([]string{"nonexistent"})
	require.Error(t, err)
}

func TestSearchDomainFlag(t *testing.T) {
	p, _, c := buildParser("test")
	_, err := p.ParseArgs([]string{"search", "--domain", "github.com", "query"})
	require.NoError(t, err)
	assert.Equal(t, []string{"github.com"}, c.Search.Domain)
}

func TestPruneDryRunFlag(t *testing.T) {
	p, _, c := buildParser("test")
	_, err := p.ParseArgs([]string{"prune", "--dry-run"})
	require.NoError(t, err)
	assert.True(t, c.Prune.DryRun)
}

func TestAllSubcommandsExist(t *testing.T) {
	expected := []string{"status", "search", "open", "add", "ingest", "prune", "purge"}
	parser, _, _ := buildParser("test")

	for _, name := range expected {
		cmd := parser.Find(name)
		assert.NotNil(t, cmd, "subcommand %q should exist", name)
	}
}

func TestHelpFlagDoesNotError(t *testing.T) {
	err := RunWithArgs("test", []string{"--help"})
	assert.NoError(t, err)
}

func TestIngestForegroundFlag(t *testing.T) {
	p, _, c := buildParser("test")
	_, err := p.ParseArgs([]string{"ingest", "--foreground"})
	require.NoError(t, err)
	assert.True(t, c.Ingest.Foreground)
}

func TestOpenFormatFlag(t *testing.T) {
	p, _, c := buildParser("test")
	_, err := p.ParseArgs([]string{"open", "--id", "CHR-test", "--format", "json"})
	require.NoError(t, err)
	assert.Equal(t, "json", c.Open.Format)
	assert.Equal(t, "CHR-test", c.Open.ID)
}

func TestSearchSemanticFlag(t *testing.T) {
	p, _, c := buildParser("test")
	_, err := p.ParseArgs([]string{"search", "--semantic", "query"})
	require.NoError(t, err)
	assert.True(t, c.Search.Semantic)
}

func TestSearchBrowserFlag(t *testing.T) {
	p, _, c := buildParser("test")
	_, err := p.ParseArgs([]string{"search", "--browser", "chrome", "query"})
	require.NoError(t, err)
	assert.Equal(t, []string{"chrome"}, c.Search.Browser)
}

func TestAddBrowserDefault(t *testing.T) {
	p, _, c := buildParser("test")
	_, err := p.ParseArgs([]string{"add", "--url", "https://example.com", "--title", "Test"})
	require.NoError(t, err)
	assert.Equal(t, "manual", c.Add.BrowserName)
}

func TestPruneOlderThanFlag(t *testing.T) {
	p, _, c := buildParser("test")
	_, err := p.ParseArgs([]string{"prune", "--older-than", "7d"})
	require.NoError(t, err)
	assert.Equal(t, "7d", c.Prune.OlderThan)
}

func TestPurgeForceFlag(t *testing.T) {
	p, _, c := buildParser("test")
	_, err := p.ParseArgs([]string{"purge", "--all", "--force"})
	require.NoError(t, err)
	assert.True(t, c.Purge.All)
	assert.True(t, c.Purge.Force)
}

func TestSearchHasBodyFlag(t *testing.T) {
	p, _, c := buildParser("test")
	_, err := p.ParseArgs([]string{"search", "--has-body", "query"})
	require.NoError(t, err)
	assert.True(t, c.Search.HasBody)
}

func TestSearchHasEmbeddingFlag(t *testing.T) {
	p, _, c := buildParser("test")
	_, err := p.ParseArgs([]string{"search", "--has-embedding", "query"})
	require.NoError(t, err)
	assert.True(t, c.Search.HasEmbedding)
}

func TestIngestPortFlag(t *testing.T) {
	p, _, c := buildParser("test")
	_, err := p.ParseArgs([]string{"ingest", "--port", "9999"})
	require.NoError(t, err)
	assert.Equal(t, 9999, c.Ingest.Port)
}

func TestAddEmbedFlag(t *testing.T) {
	p, _, c := buildParser("test")
	_, err := p.ParseArgs([]string{"add", "--url", "https://x.com", "--title", "T", "--embed"})
	require.NoError(t, err)
	assert.True(t, c.Add.Embed)
}
