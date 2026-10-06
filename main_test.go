package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseArgsStripsWrapperFlags(t *testing.T) {
	opts, rest, err := parseArgs([]string{
		"-p", "--kekkai-cpus", "4", "--output-format", "stream-json",
		"--kekkai-mount=/data:ro", "--kekkai-rebuild", "--verbose", "hello",
	}, options{})
	if err != nil {
		t.Fatal(err)
	}
	if opts.cpus != "4" || !opts.rebuild || !reflect.DeepEqual(opts.mounts, []string{"/data:ro"}) {
		t.Errorf("opts = %+v", opts)
	}
	want := []string{"-p", "--output-format", "stream-json", "--verbose", "hello"}
	if !reflect.DeepEqual(rest, want) {
		t.Errorf("rest = %q, want %q", rest, want)
	}
}

func TestParseArgsLeavesArgsAfterDoubleDash(t *testing.T) {
	_, rest, err := parseArgs([]string{"-p", "--", "--kekkai-help"}, options{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rest, []string{"-p", "--", "--kekkai-help"}) {
		t.Errorf("rest = %q", rest)
	}
}

func TestParseArgsRejectsUnknownWrapperFlag(t *testing.T) {
	if _, _, err := parseArgs([]string{"--kekkai-nope", "x"}, options{}); err == nil {
		t.Error("want error")
	}
}

func TestClaudeArgMounts(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a")
	b := filepath.Join(root, "b")
	cfgDir := filepath.Join(root, "cfg")
	for _, d := range []string{a, b, cfgDir} {
		os.Mkdir(d, 0o755)
	}
	mcp := filepath.Join(cfgDir, "mcp.json")
	os.WriteFile(mcp, []byte("{}"), 0o644)

	got := claudeArgMounts([]string{
		"--add-dir", a, b, "--mcp-config", mcp, `{"mcpServers":{}}`,
		"--settings", `{"x":1}`, "--debug-file=" + filepath.Join(root, "logs", "d.log"), "-p", "hi",
	}, root)
	want := []mount{
		{src: a, dst: a},
		{src: b, dst: b},
		{src: cfgDir, dst: cfgDir, readOnly: true},
		{src: filepath.Join(root, "logs"), dst: filepath.Join(root, "logs")},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}

func TestDedupeMountsDropsCoveredPaths(t *testing.T) {
	got := dedupeMounts([]mount{
		{src: "/repo/sub", dst: "/repo/sub", readOnly: true},
		{src: "/repo", dst: "/repo"},
		{src: "/repo", dst: "/repo"},
		{src: "/other", dst: "/other", readOnly: true},
		{src: "/other/w", dst: "/other/w"},
		{src: "/cfg", dst: "/claude-config"},
	})
	want := []mount{
		{src: "/repo", dst: "/repo"},
		{src: "/other", dst: "/other", readOnly: true},
		{src: "/other/w", dst: "/other/w"},
		{src: "/cfg", dst: "/claude-config"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}

func TestParseMountSpec(t *testing.T) {
	cases := map[string]mount{
		"/a":       {src: "/a", dst: "/a"},
		"/a:ro":    {src: "/a", dst: "/a", readOnly: true},
		"/a:/b":    {src: "/a", dst: "/b"},
		"/a:/b:ro": {src: "/a", dst: "/b", readOnly: true},
		"rel:/b":   {src: "/cwd/rel", dst: "/b"},
	}
	for spec, want := range cases {
		if got := parseMountSpec(spec, "/cwd"); got != want {
			t.Errorf("%s: got %+v, want %+v", spec, got, want)
		}
	}
}

func TestResolveDockerfilePrefersRepoFile(t *testing.T) {
	repo := t.TempDir()
	os.WriteFile(filepath.Join(repo, dockerfileName), []byte("FROM alpine\n"), 0o644)
	got, err := resolveDockerfile(options{stateDir: t.TempDir()}, repo)
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(repo, dockerfileName) {
		t.Errorf("got %s", got)
	}
}

func TestResolveDockerfileFallsBackToDefault(t *testing.T) {
	state := t.TempDir()
	got, err := resolveDockerfile(options{stateDir: state}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(got)
	if string(content) != string(defaultDockerfile) {
		t.Error("default dockerfile not written")
	}
}

func TestRunArgsAddsTTYOnlyWhenAsked(t *testing.T) {
	spec := runSpec{name: "n", image: "img", workdir: "/w", envFile: "/e"}
	got := runArgs(spec, []string{"-p", "hi"})
	want := []string{"run", "--rm", "--interactive", "--name", "n", "--workdir", "/w", "--env-file", "/e", "img", "claude", "-p", "hi"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q", got)
	}
	spec.tty = true
	if got := runArgs(spec, nil); got[9] != "--tty" {
		t.Errorf("got %q", got)
	}
}

func TestContainerEnvSkipsHostSessionVars(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_ID", "x")
	t.Setenv("CLAUDE_CODE_EXECPATH", "/x")
	t.Setenv("CLAUDE_CODE_USE_BEDROCK", "1")
	t.Setenv("ANTHROPIC_MODEL", "opus")
	t.Setenv("SECRET", "s")
	env := map[string]bool{}
	for _, kv := range containerEnv(options{env: []string{"GH_TOKEN=t"}}, nil) {
		env[kv] = true
	}
	for _, kv := range []string{"CLAUDE_CODE_USE_BEDROCK=1", "ANTHROPIC_MODEL=opus", "GH_TOKEN=t"} {
		if !env[kv] {
			t.Errorf("missing %s", kv)
		}
	}
	for _, kv := range []string{"CLAUDE_CODE_SESSION_ID=x", "CLAUDE_CODE_EXECPATH=/x", "SECRET=s"} {
		if env[kv] {
			t.Errorf("leaked %s", kv)
		}
	}
}

func TestParseArgsInit(t *testing.T) {
	opts, rest, err := parseArgs([]string{"--kekkai-init", "--kekkai-init-agent", "codex", "--model", "opus"}, options{})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.init || opts.initAgent != "codex" || !reflect.DeepEqual(rest, []string{"--model", "opus"}) {
		t.Errorf("opts = %+v, rest = %q", opts, rest)
	}
}

func TestInitArgsLimitClaudeToTheDockerfile(t *testing.T) {
	got := initArgs("/usr/bin/claude", "P", "/repo/kekkai.Dockerfile", false)
	want := []string{"-p", "P", "--allowedTools", "Read", "Glob", "Grep",
		"Edit(//repo/kekkai.Dockerfile)", "Bash(container build *)"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q", got)
	}
	if got := initArgs("claude", "P", "/t", true); got[0] != "P" {
		t.Errorf("interactive should not pass -p: %q", got)
	}
	if got := initArgs("codex", "P", "/t", false); !reflect.DeepEqual(got, []string{"P"}) {
		t.Errorf("other agents get the prompt only: %q", got)
	}
}

func TestBuildInitPrompt(t *testing.T) {
	p := buildInitPrompt("/repo/kekkai.Dockerfile", "/repo", true)
	for _, want := range []string{"/repo/kekkai.Dockerfile", "--tag kekkai-init-check /repo", "improve it", "FROM debian"} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
	if strings.Contains(p, "{{") {
		t.Error("unreplaced placeholder")
	}
}
