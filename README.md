# kekkai 結界

Claude run in box. Box keep Claude in. Your Mac stay safe.

Kekkai mean magic wall in Japanese.

> [!WARNING]
> Experimental. Kekkai young. Kekkai may break.
> Wall may have hole. No trust wall with big secret.

## Why

Claude strong. Claude do many things.
`--dangerously-skip-permissions` let Claude do all things.
In box, "all things" only mean your repo.

## Need

- Mac with Apple chip
- macOS 26 or newer

## Get

```sh
git clone https://github.com/gronnbeck/kekkai
cd kekkai
./install.sh
```

Script get [Apple container](https://github.com/apple/container), Go and git.
Script make `kekkai` and put in `~/.local/bin`.

## Use

Same as `claude`. Just say `kekkai`.

```sh
kekkai
kekkai -p "fix bug"
kekkai --dangerously-skip-permissions
```

All `claude` flags work. Kekkai pass them to Claude.

## Your box

Put `kekkai.Dockerfile` in repo root.
Each repo get own box. Add tools repo need.
No file? Kekkai use `~/.kekkai/default.Dockerfile`.
No that either? Kekkai use [default box](default.Dockerfile).

Change file, box build again.

Lazy? Let Claude make it:

```sh
kekkai --kekkai-init
```

Claude look at repo. Claude find tools and versions.
Claude write `kekkai.Dockerfile`. Claude build it to check.
Claude only touch that one file.

Want other agent? `--kekkai-init-agent codex`.

## What Claude see

- Your repo
- Dirs you give with `--add-dir`
- Own Claude home in `~/.kekkai`

Claude change file in repo? Change land on your Mac. You keep it.
Claude no see rest of Mac.
Claude no touch your real `~/.claude`.

Kekkai copy your `CLAUDE.md`, settings, skills and hooks into box.
No want? Use `--kekkai-no-seed`.

Want Claude only look, no change? Use `--kekkai-read-only`.
Repo and every mount go read-only. Only `~/.kekkai` stay writable.

## Login

Kekkai find login in this order:

1. `ANTHROPIC_API_KEY` or `CLAUDE_CODE_OAUTH_TOKEN`
2. Login made inside box (`kekkai auth login`)
3. Token from Mac keychain

Keychain token die fast. Run `claude setup-token` for long life.

## Conductor

Kekkai work in [Conductor](https://conductor.build).
Tell Conductor use `kekkai`, not `claude`.
No TTY? Kekkai no ask for one. Output stay clean.

## Kekkai flags

Kekkai flags start with `--kekkai-`. Rest go to Claude.

```sh
kekkai --kekkai-help
```

| Flag | Do |
|---|---|
| `--kekkai-init` | Make `kekkai.Dockerfile` |
| `--kekkai-dry-run` | Show command. No run. |
| `--kekkai-rebuild` | Build box again |
| `--kekkai-mount src[:dst][:ro]` | Give box more dir |
| `--kekkai-env NAME[=VAL]` | Give box env var |
| `--kekkai-cpus 4` | More brain |
| `--kekkai-memory 8G` | More memory |
| `--kekkai-file path` | Use other Dockerfile |
| `--kekkai-image ref` | Use ready image. No build. |

Most flags also work as env var, like `KEKKAI_CPUS=4`.
`--kekkai-help` show which.
