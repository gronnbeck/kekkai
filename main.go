package main

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
)

func main() {
	code, err := run(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "kekkai:", err)
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}

func run(argv []string) (int, error) {
	opts, claudeArgs, err := parseArgs(argv, defaultOptions())
	if err != nil {
		return 2, err
	}
	if opts.help {
		fmt.Print(usage)
		return 0, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return 1, err
	}
	if resolved, err := filepath.EvalSymlinks(cwd); err == nil {
		cwd = resolved
	}

	if opts.init {
		return runInit(opts, cwd, claudeArgs)
	}

	if opts.stateDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return 1, err
		}
		opts.stateDir = filepath.Join(home, ".kekkai")
	}
	configDir := filepath.Join(opts.stateDir, "config")
	runDir := filepath.Join(opts.stateDir, "run")
	for _, d := range []string{configDir, runDir} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return 1, err
		}
	}

	if !opts.noSeed {
		if err := seedConfig(configDir); err != nil {
			return 1, err
		}
	}
	if err := ensureGlobalConfig(configDir); err != nil {
		return 1, err
	}

	image := opts.image
	if image == "" {
		build := ensureImage
		if opts.dryRun {
			build = plannedImage
		}
		if image, err = build(opts, cwd); err != nil {
			return 1, err
		}
	}

	extraEnv := map[string]string{"CLAUDE_CONFIG_DIR": containerConfigDir}
	_, haveCreds := os.Stat(filepath.Join(configDir, ".credentials.json"))
	if !authFromEnv() && haveCreds != nil && !opts.dryRun {
		tok, err := keychainToken()
		if err != nil {
			fmt.Fprintf(os.Stderr, "kekkai: %v\n", err)
		} else {
			extraEnv["CLAUDE_CODE_OAUTH_TOKEN"] = tok
		}
	}
	env := containerEnv(opts, extraEnv)

	mounts := []mount{
		{src: cwd, dst: cwd},
		{src: configDir, dst: containerConfigDir},
	}
	if common := gitCommonDir(cwd); common != "" && !within(common, cwd) {
		mounts = append(mounts, mount{src: common, dst: common})
	}
	mounts = append(mounts, claudeArgMounts(claudeArgs, cwd)...)
	for _, spec := range opts.mounts {
		mounts = append(mounts, parseMountSpec(spec, cwd))
	}
	mounts = dedupeMounts(mounts)

	tty := isTerminal(os.Stdin) && isTerminal(os.Stdout)
	envFile := filepath.Join(runDir, fmt.Sprintf("%d.env", os.Getpid()))
	args := runArgs(runSpec{
		name:    containerName(cwd),
		image:   image,
		workdir: cwd,
		tty:     tty,
		mounts:  mounts,
		envFile: envFile,
		cpus:    opts.cpus,
		memory:  opts.memory,
	}, claudeArgs)

	if opts.dryRun {
		fmt.Println(shellJoin(append([]string{"container"}, args...)))
		for _, kv := range env {
			k, _, _ := strings.Cut(kv, "=")
			fmt.Println("# env", k)
		}
		return 0, nil
	}

	if err := os.WriteFile(envFile, []byte(strings.Join(env, "\n")+"\n"), 0o600); err != nil {
		return 1, err
	}
	defer os.Remove(envFile)

	return execContainer(args)
}

type runSpec struct {
	name, image, workdir, envFile, cpus, memory string
	tty                                         bool
	mounts                                      []mount
}

func runArgs(s runSpec, claudeArgs []string) []string {
	args := []string{"run", "--rm", "--interactive", "--name", s.name, "--workdir", s.workdir, "--env-file", s.envFile}
	if s.tty {
		args = append(args, "--tty")
	}
	if s.cpus != "" {
		args = append(args, "--cpus", s.cpus)
	}
	if s.memory != "" {
		args = append(args, "--memory", s.memory)
	}
	for _, m := range s.mounts {
		args = append(args, "--volume", m.volumeArg())
	}
	args = append(args, s.image, "claude")
	return append(args, claudeArgs...)
}

var unsafeName = regexp.MustCompile(`[^a-zA-Z0-9_.-]+`)

func containerName(cwd string) string {
	base := strings.Trim(unsafeName.ReplaceAllString(filepath.Base(cwd), "-"), "-.")
	if len(base) > 40 {
		base = base[:40]
	}
	return fmt.Sprintf("kekkai-%s-%06x", base, rand.IntN(1<<24))
}

func execContainer(args []string) (int, error) {
	code, err := execCommand(exec.Command("container", args...))
	if code == 127 {
		err = fmt.Errorf("is Apple container installed? `brew install container && container system start`: %w", err)
	}
	return code, err
}

func execCommand(cmd *exec.Cmd) (int, error) {
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	sigs := make(chan os.Signal, 4)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGWINCH)
	defer signal.Stop(sigs)

	if err := cmd.Start(); err != nil {
		return 127, err
	}
	go func() {
		for sig := range sigs {
			cmd.Process.Signal(sig)
		}
	}()

	err := cmd.Wait()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			return 128 + int(ws.Signal()), nil
		}
		return exitErr.ExitCode(), nil
	}
	return 0, err
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func shellJoin(args []string) string {
	quoted := make([]string, len(args))
	for i, a := range args {
		if a != "" && !strings.ContainsAny(a, " \t\n'\"\\$`!*?[]{}()<>|&;#~") {
			quoted[i] = a
			continue
		}
		quoted[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
	}
	return strings.Join(quoted, " ")
}
