package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type mount struct {
	src      string
	dst      string
	readOnly bool
}

func (m mount) volumeArg() string {
	s := m.src + ":" + m.dst
	if m.readOnly {
		s += ":ro"
	}
	return s
}

type pathKind int

const (
	dirRW pathKind = iota
	fileRO
	fileRW
)

type pathFlag struct {
	kind     pathKind
	variadic bool
}

var pathFlags = map[string]pathFlag{
	"--add-dir":                   {dirRW, true},
	"--mcp-config":                {fileRO, true},
	"--plugin-dir":                {fileRO, false},
	"--settings":                  {fileRO, false},
	"--agents":                    {fileRO, false},
	"--system-prompt-file":        {fileRO, false},
	"--append-system-prompt-file": {fileRO, false},
	"--debug-file":                {fileRW, false},
}

func claudeArgMounts(args []string, cwd string) []mount {
	var mounts []mount
	add := func(kind pathKind, value string) {
		if m, ok := mountFor(kind, value, cwd); ok {
			mounts = append(mounts, m)
		}
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		name, value, hasValue := strings.Cut(arg, "=")
		pf, ok := pathFlags[name]
		if !ok {
			continue
		}
		if hasValue {
			add(pf.kind, value)
			continue
		}
		for i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			i++
			add(pf.kind, args[i])
			if !pf.variadic {
				break
			}
		}
	}
	return mounts
}

func mountFor(kind pathKind, value, cwd string) (mount, bool) {
	p := value
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			p = filepath.Join(home, p[2:])
		}
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(cwd, p)
	}
	p = filepath.Clean(p)

	info, err := os.Stat(p)
	switch {
	case kind == fileRW && os.IsNotExist(err):
		dir := filepath.Dir(p)
		return mount{src: dir, dst: dir}, true
	case err != nil:
		return mount{}, false
	case info.IsDir():
		return mount{src: p, dst: p, readOnly: kind == fileRO}, true
	default:
		dir := filepath.Dir(p)
		return mount{src: dir, dst: dir, readOnly: kind == fileRO}, true
	}
}

func readOnlyExcept(mounts []mount, writableDst string) []mount {
	out := make([]mount, len(mounts))
	for i, m := range mounts {
		m.readOnly = m.dst != writableDst
		out[i] = m
	}
	return out
}

func gitCommonDir(dir string) string {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--path-format=absolute", "--git-common-dir").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func gitTopLevel(dir string) string {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func parseMountSpec(spec string, cwd string) mount {
	parts := strings.Split(spec, ":")
	m := mount{src: parts[0]}
	if len(parts) > 1 && parts[len(parts)-1] == "ro" {
		m.readOnly = true
		parts = parts[:len(parts)-1]
	}
	if len(parts) > 1 {
		m.dst = parts[1]
	}
	if !filepath.IsAbs(m.src) {
		m.src = filepath.Join(cwd, m.src)
	}
	if m.dst == "" {
		m.dst = m.src
	}
	return m
}

func within(p, root string) bool {
	rel, err := filepath.Rel(root, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, "../")
}

func dedupeMounts(mounts []mount) []mount {
	sort.SliceStable(mounts, func(i, j int) bool {
		if len(mounts[i].dst) != len(mounts[j].dst) {
			return len(mounts[i].dst) < len(mounts[j].dst)
		}
		return !mounts[i].readOnly && mounts[j].readOnly
	})

	var out []mount
	for _, m := range mounts {
		covered := false
		for _, kept := range out {
			if m.src == kept.src && m.dst == kept.dst ||
				m.src == m.dst && kept.src == kept.dst && within(m.dst, kept.dst) && (m.readOnly || !kept.readOnly) {
				covered = true
				break
			}
		}
		if !covered {
			out = append(out, m)
		}
	}
	return out
}
