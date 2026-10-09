package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const containerConfigDir = "/claude-config"

var seedEntries = []string{"CLAUDE.md", "settings.json", "agents", "commands", "skills", "output-styles", "hooks"}

var authEnv = []string{"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "CLAUDE_CODE_OAUTH_TOKEN"}

func seedConfig(configDir string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	unlock, err := lockFile(filepath.Join(configDir, ".seed.lock"))
	if err != nil {
		return err
	}
	defer unlock()

	src := filepath.Join(home, ".claude")
	for _, name := range seedEntries {
		from := filepath.Join(src, name)
		if _, err := os.Stat(from); err != nil {
			continue
		}
		if err := replaceEntry(from, filepath.Join(configDir, name)); err != nil {
			return fmt.Errorf("seed %s: %w", name, err)
		}
	}
	return repointSettings(filepath.Join(configDir, "settings.json"), home)
}

func repointSettings(path, home string) error {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	s := strings.NewReplacer(
		"$HOME/.claude", "$CLAUDE_CONFIG_DIR",
		"${HOME}/.claude", "$CLAUDE_CONFIG_DIR",
		"~/.claude", "$CLAUDE_CONFIG_DIR",
		filepath.Join(home, ".claude"), "$CLAUDE_CONFIG_DIR",
	).Replace(string(b))
	return os.WriteFile(path, []byte(s), 0o644)
}

func replaceEntry(from, to string) error {
	tmp, err := os.MkdirTemp(filepath.Dir(to), ".seed-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	fresh := filepath.Join(tmp, "new")
	if err := copyTree(from, fresh); err != nil {
		return err
	}
	old := filepath.Join(tmp, "old")
	if err := os.Rename(to, old); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(fresh, to)
}

func copyTree(from, to string) error {
	from, err := filepath.EvalSymlinks(from)
	if err != nil {
		return err
	}
	return filepath.WalkDir(from, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, p)
		dst := filepath.Join(to, rel)
		info, err := os.Stat(p)
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if d.Type()&fs.ModeSymlink != 0 {
				return copyTree(p, dst)
			}
			return os.MkdirAll(dst, 0o755)
		}
		return copyFile(p, dst, info.Mode().Perm())
	})
}

func copyFile(from, to string, perm fs.FileMode) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(to, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func ensureGlobalConfig(configDir string) error {
	p := filepath.Join(configDir, ".claude.json")
	if _, err := os.Stat(p); err == nil {
		return nil
	}
	return os.WriteFile(p, []byte(`{"hasCompletedOnboarding":true}`+"\n"), 0o600)
}

func keychainToken() (string, error) {
	out, err := exec.Command("security", "find-generic-password", "-s", "Claude Code-credentials", "-w").Output()
	if err != nil {
		return "", fmt.Errorf("no Claude Code credentials in the keychain")
	}
	var creds struct {
		ClaudeAiOauth struct {
			AccessToken string `json:"accessToken"`
			ExpiresAt   int64  `json:"expiresAt"`
		} `json:"claudeAiOauth"`
	}
	if err := json.Unmarshal(out, &creds); err != nil {
		return "", fmt.Errorf("keychain credentials: %w", err)
	}
	tok := creds.ClaudeAiOauth
	if tok.AccessToken == "" {
		return "", fmt.Errorf("keychain credentials hold no access token")
	}
	if tok.ExpiresAt > 0 && time.UnixMilli(tok.ExpiresAt).Before(time.Now().Add(time.Minute)) {
		return "", fmt.Errorf("keychain token has expired (run claude on the host to refresh it, or use `claude setup-token`)")
	}
	return tok.AccessToken, nil
}

func authFromEnv() bool {
	for _, k := range authEnv {
		if os.Getenv(k) != "" {
			return true
		}
	}
	return false
}

var forwardPrefixes = []string{"ANTHROPIC_", "CLAUDE_CODE_", "DISABLE_", "MAX_", "MCP_", "BASH_", "OTEL_"}

var hostOnlyMarkers = []string{"SESSION", "EXECPATH", "MESSAGING", "CHILD", "SSE_PORT", "IDE"}

var forwardNames = []string{"TERM", "COLORTERM", "LANG", "LC_ALL", "TZ", "NO_COLOR", "FORCE_COLOR"}

func containerEnv(opts options, extra map[string]string) []string {
	env := map[string]string{}
	for _, kv := range os.Environ() {
		k, v, _ := strings.Cut(kv, "=")
		forward := false
		for _, p := range forwardPrefixes {
			forward = forward || strings.HasPrefix(k, p)
		}
		for _, n := range forwardNames {
			forward = forward || k == n
		}
		for _, m := range hostOnlyMarkers {
			forward = forward && !(strings.HasPrefix(k, "CLAUDE_CODE_") && strings.Contains(k, m))
		}
		if forward {
			env[k] = v
		}
	}
	for _, spec := range opts.env {
		k, v, ok := strings.Cut(spec, "=")
		if !ok {
			v, ok = os.LookupEnv(k)
		}
		if ok {
			env[k] = v
		}
	}
	for k, v := range extra {
		env[k] = v
	}

	var out []string
	for k, v := range env {
		if strings.ContainsAny(v, "\n\r") {
			fmt.Fprintf(os.Stderr, "kekkai: skipping env %s (multi-line value)\n", k)
			continue
		}
		out = append(out, k+"="+v)
	}
	sort.Strings(out)
	return out
}
