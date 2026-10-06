Write a Dockerfile for this repository at `{{TARGET}}`. {{EXISTING}}

kekkai runs a coding agent inside this image with Apple `container`. The agent must be able to build, test, lint and run this project inside the container without installing anything at runtime.

How kekkai runs the image:

- The repo is bind-mounted read-write at its host path, so every edit lands on the host filesystem. Never `COPY` source into the image and never move the working tree elsewhere.
- The command is `claude` with `CLAUDE_CONFIG_DIR=/claude-config` mounted from the host.
- The container runs as the image's default `USER`, whose uid differs from the host owner of the mounted files.

Requirements:

1. Start from this base and keep everything in it (Debian, the `claude` user with a writable `HOME`, `claude` on `PATH`, `DISABLE_AUTOUPDATER=1`, `git safe.directory '*'`):

```dockerfile
{{DEFAULT}}
```

2. Inspect the repo to find what it needs: language version files (`.tool-versions`, `.ruby-version`, `.nvmrc`, `.python-version`, `go.mod`, `rust-toolchain.toml`, `package.json` `engines`/`packageManager`), lockfiles, `Makefile`/`justfile`/`bin/` scripts, CI workflows, `docker-compose` files and the README. Install those toolchains at the versions the repo pins, plus the system libraries native extensions need and CLIs the scripts call (database clients, `gh`, linters).
3. Keep dependency caches and installs outside the repo where the tool allows (`GOPATH`, `CARGO_HOME`, `BUNDLE_PATH`, `PIP_CACHE_DIR`, npm cache in `HOME`), so Linux builds do not overwrite the host's platform-specific artifacts. Do not install project dependencies into the image, because the mounted repo shadows them.
4. Do not run databases or other services in the image. Install clients only.
5. Order layers from least to most frequently changed, clean package caches, and keep root-only steps before `USER claude`.
6. Add short comments only where a choice is not obvious.

Then verify it builds: `container build --file {{TARGET}} --tag kekkai-init-check {{ROOT}}`. Fix and rebuild until it succeeds. If `container` is not available, say so and stop after writing the file.

Only edit `{{TARGET}}`. Finish with a short list of what the image installs and why.
