package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

var configFileNames = []string{".kekkai.yaml", ".kekkai.yml"}

type fileConfig struct {
	Dockerfile string              `yaml:"dockerfile"`
	Image      string              `yaml:"image"`
	CPUs       string              `yaml:"cpus"`
	Memory     string              `yaml:"memory"`
	Mounts     []string            `yaml:"mounts"`
	Env        map[string]envValue `yaml:"env"`
	NoSeed     bool                `yaml:"no_seed"`
	ReadOnly   bool                `yaml:"read_only"`
	InitAgent  string              `yaml:"init_agent"`

	dir string
}

type envValue struct {
	Value string
	From  string
}

func (v *envValue) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		v.Value = node.Value
		return nil
	}
	var ref struct {
		From string `yaml:"from"`
	}
	if err := node.Decode(&ref); err != nil || ref.From == "" {
		return fmt.Errorf("line %d: env value must be a string or {from: HOST_VAR}", node.Line)
	}
	v.From = ref.From
	return nil
}

type envEntry struct {
	name string
	envValue
}

func (e envEntry) resolve() (string, bool) {
	if e.From == "" {
		return e.Value, true
	}
	return os.LookupEnv(e.From)
}

func loadConfig(dir string) (*fileConfig, error) {
	for _, name := range configFileNames {
		path := filepath.Join(dir, name)
		b, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		cfg := &fileConfig{dir: dir}
		dec := yaml.NewDecoder(strings.NewReader(string(b)))
		dec.KnownFields(true)
		if err := dec.Decode(cfg); err != nil && !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		return cfg, nil
	}
	return nil, nil
}

func applyConfigs(opts options, configs ...*fileConfig) options {
	set := func(field *string, value string) {
		if *field == "" {
			*field = value
		}
	}
	for i := len(configs) - 1; i >= 0; i-- {
		cfg := configs[i]
		if cfg == nil {
			continue
		}
		set(&opts.dockerfile, resolvePath(cfg.Dockerfile, cfg.dir))
		set(&opts.image, cfg.Image)
		set(&opts.cpus, cfg.CPUs)
		set(&opts.memory, cfg.Memory)
		set(&opts.initAgent, cfg.InitAgent)
		opts.noSeed = opts.noSeed || cfg.NoSeed
		opts.readOnly = opts.readOnly || cfg.ReadOnly
	}

	var mounts []mount
	var env []envEntry
	for _, cfg := range configs {
		if cfg == nil {
			continue
		}
		for _, spec := range cfg.Mounts {
			mounts = append(mounts, parseMountSpec(spec, cfg.dir))
		}
		names := make([]string, 0, len(cfg.Env))
		for name := range cfg.Env {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			env = append(env, envEntry{name, cfg.Env[name]})
		}
	}
	opts.configMounts = mounts
	opts.configEnv = env

	set(&opts.cpus, defaultCPUs)
	set(&opts.memory, defaultMemory)
	return opts
}

func resolvePath(p, base string) string {
	if p == "" {
		return ""
	}
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(base, p)
}
