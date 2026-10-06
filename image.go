package main

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

const dockerfileName = "kekkai.Dockerfile"

//go:embed kekkai.Dockerfile
var defaultDockerfile []byte

func resolveDockerfile(opts options, cwd string) (path string, err error) {
	if opts.dockerfile != "" {
		p := opts.dockerfile
		if !filepath.IsAbs(p) {
			p = filepath.Join(cwd, p)
		}
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("dockerfile: %w", err)
		}
		return p, nil
	}

	for _, dir := range []string{gitTopLevel(cwd), cwd} {
		if dir == "" {
			continue
		}
		p := filepath.Join(dir, dockerfileName)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	dir := filepath.Join(opts.stateDir, "default-image")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	p := filepath.Join(dir, dockerfileName)
	return p, os.WriteFile(p, defaultDockerfile, 0o644)
}

func imageTag(dockerfile []byte) string {
	sum := sha256.Sum256(dockerfile)
	return "kekkai:" + hex.EncodeToString(sum[:])[:12]
}

func plannedImage(opts options, cwd string) (string, error) {
	tag, _, err := imageFor(opts, cwd)
	return tag, err
}

func imageFor(opts options, cwd string) (tag, dockerfile string, err error) {
	dockerfile, err = resolveDockerfile(opts, cwd)
	if err != nil {
		return "", "", err
	}
	content, err := os.ReadFile(dockerfile)
	if err != nil {
		return "", "", err
	}
	return imageTag(content), dockerfile, nil
}

func ensureImage(opts options, cwd string) (string, error) {
	tag, path, err := imageFor(opts, cwd)
	if err != nil {
		return "", err
	}

	unlock, err := lockFile(filepath.Join(opts.stateDir, "build.lock"))
	if err != nil {
		return "", err
	}
	defer unlock()

	if !opts.rebuild && exec.Command("container", "image", "inspect", tag).Run() == nil {
		return tag, nil
	}

	fmt.Fprintf(os.Stderr, "kekkai: building %s from %s\n", tag, path)
	build := exec.Command("container", "build", "--tag", tag, "--file", path, filepath.Dir(path))
	build.Stdout = os.Stderr
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		return "", fmt.Errorf("container build: %w", err)
	}
	return tag, nil
}

func lockFile(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return func() {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
	}, nil
}
