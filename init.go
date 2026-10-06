package main

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//go:embed init_prompt.md
var initPrompt string

func runInit(opts options, cwd string, agentArgs []string) (int, error) {
	root := gitTopLevel(cwd)
	if root == "" {
		root = cwd
	}
	target := filepath.Join(root, dockerfileName)
	_, statErr := os.Stat(target)

	agent := opts.initAgent
	if agent == "" {
		agent = "claude"
	}
	interactive := isTerminal(os.Stdin) && isTerminal(os.Stdout)
	args := initArgs(agent, buildInitPrompt(target, root, statErr == nil), target, interactive)
	args = append(args, agentArgs...)

	if opts.dryRun {
		fmt.Println(shellJoin(append([]string{agent}, args...)))
		return 0, nil
	}

	cmd := exec.Command(agent, args...)
	cmd.Dir = root
	code, err := execCommand(cmd)
	if err == nil && code == 0 {
		if _, statErr := os.Stat(target); statErr == nil {
			fmt.Fprintf(os.Stderr, "kekkai: wrote %s\n", target)
		}
	}
	return code, err
}

func buildInitPrompt(target, root string, exists bool) string {
	existing := "It does not exist yet."
	if exists {
		existing = "It already exists: read it first and improve it rather than starting over."
	}
	return strings.NewReplacer(
		"{{TARGET}}", target,
		"{{ROOT}}", root,
		"{{EXISTING}}", existing,
		"{{DEFAULT}}", strings.TrimSpace(string(defaultDockerfile)),
	).Replace(initPrompt)
}

func initArgs(agent, prompt, target string, interactive bool) []string {
	if filepath.Base(agent) != "claude" {
		return []string{prompt}
	}
	var args []string
	if !interactive {
		args = append(args, "-p")
	}
	return append(args, prompt,
		"--allowedTools",
		"Read", "Glob", "Grep",
		"Edit(/"+target+")",
		"Bash(container build *)",
	)
}
