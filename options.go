package main

import (
	"fmt"
	"os"
	"strings"
)

const flagPrefix = "--kekkai-"

type options struct {
	dockerfile string
	image      string
	stateDir   string
	cpus       string
	memory     string
	mounts     []string
	env        []string
	rebuild    bool
	dryRun     bool
	noSeed     bool
	readOnly   bool
	help       bool
	init       bool
	initAgent  string
}

var boolFlags = map[string]bool{
	"rebuild":   true,
	"dry-run":   true,
	"no-seed":   true,
	"read-only": true,
	"help":      true,
	"init":      true,
}

func defaultOptions() options {
	return options{
		dockerfile: os.Getenv("KEKKAI_DOCKERFILE"),
		image:      os.Getenv("KEKKAI_IMAGE"),
		stateDir:   os.Getenv("KEKKAI_STATE_DIR"),
		cpus:       os.Getenv("KEKKAI_CPUS"),
		memory:     os.Getenv("KEKKAI_MEMORY"),
		mounts:     splitList(os.Getenv("KEKKAI_MOUNTS")),
		env:        splitList(os.Getenv("KEKKAI_ENV")),
		noSeed:     os.Getenv("KEKKAI_NO_SEED") != "",
		readOnly:   os.Getenv("KEKKAI_READ_ONLY") != "",
		initAgent:  os.Getenv("KEKKAI_INIT_AGENT"),
	}
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseArgs(args []string, opts options) (options, []string, error) {
	var rest []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			rest = append(rest, args[i:]...)
			break
		}
		if !strings.HasPrefix(arg, flagPrefix) {
			rest = append(rest, arg)
			continue
		}

		name, value, hasValue := strings.Cut(strings.TrimPrefix(arg, flagPrefix), "=")
		if boolFlags[name] {
			if hasValue {
				return opts, nil, fmt.Errorf("%s%s takes no value", flagPrefix, name)
			}
			switch name {
			case "rebuild":
				opts.rebuild = true
			case "dry-run":
				opts.dryRun = true
			case "no-seed":
				opts.noSeed = true
			case "read-only":
				opts.readOnly = true
			case "help":
				opts.help = true
			case "init":
				opts.init = true
			}
			continue
		}

		if !hasValue {
			if i+1 >= len(args) {
				return opts, nil, fmt.Errorf("%s%s needs a value", flagPrefix, name)
			}
			i++
			value = args[i]
		}
		switch name {
		case "file":
			opts.dockerfile = value
		case "image":
			opts.image = value
		case "state-dir":
			opts.stateDir = value
		case "cpus":
			opts.cpus = value
		case "memory":
			opts.memory = value
		case "mount":
			opts.mounts = append(opts.mounts, value)
		case "env":
			opts.env = append(opts.env, value)
		case "init-agent":
			opts.initAgent = value
		default:
			return opts, nil, fmt.Errorf("unknown flag %s%s", flagPrefix, name)
		}
	}
	return opts, rest, nil
}

const usage = `kekkai runs Claude Code inside an Apple container.

Every argument without the --kekkai- prefix goes to claude unchanged.

Wrapper flags (env var in brackets):
  --kekkai-init              have an agent write kekkai.Dockerfile for this repo
                             on the host, then exit; other args go to the agent
  --kekkai-init-agent <cmd>  agent for --kekkai-init, default claude [KEKKAI_INIT_AGENT]
  --kekkai-file <path>       Dockerfile to build [KEKKAI_DOCKERFILE]
                             default: <repo root>/kekkai.Dockerfile,
                             else <state dir>/default.Dockerfile, else built-in
  --kekkai-image <ref>       use this image and skip the build [KEKKAI_IMAGE]
  --kekkai-rebuild           rebuild the image even if it exists
  --kekkai-mount <src[:dst][:ro]>
                             extra mount, repeatable [KEKKAI_MOUNTS, comma list]
  --kekkai-env <NAME[=VAL]>  extra env var, repeatable [KEKKAI_ENV, comma list]
  --kekkai-cpus <n>          [KEKKAI_CPUS]
  --kekkai-memory <size>     e.g. 8G [KEKKAI_MEMORY]
  --kekkai-state-dir <path>  claude config/session dir [KEKKAI_STATE_DIR]
                             default: ~/.kekkai
  --kekkai-no-seed           skip copying CLAUDE.md, settings, skills etc. from ~/.claude
                             [KEKKAI_NO_SEED]
  --kekkai-read-only         mount everything read-only except the claude state dir
                             [KEKKAI_READ_ONLY]
  --kekkai-dry-run           print the container command and exit
  --kekkai-help              show this help
`
