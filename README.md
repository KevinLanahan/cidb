# lokal

A step-through debugger for CI pipelines (GitHub Actions, GitLab CI, and CircleCI).

Instead of commit → push → wait → fail → repeat, `lokal` runs your workflow locally in Docker and pauses before each step — so you can inspect the environment, skip steps, drop into a live shell, and debug in real time.

## Requirements

- [Go 1.22+](https://go.dev/dl/)
- [Docker Desktop](https://www.docker.com/products/docker-desktop/)

## Install

```bash
git clone https://github.com/KevinLanahan/lokal.git
cd lokal
go build -o lokal .
```

## Usage

Run a specific workflow file:
```bash
./lokal run .github/workflows/ci.yml
```

Or let lokal auto-discover a workflow in the current directory:
```bash
./lokal run
```

## Controls

At each step pause prompt:

| Key | Action |
|-----|--------|
| `c` | Run the step |
| `s` | Skip the step |
| `sh` | Drop into a shell inside the container |
| `a` | Abort the run |

If a step fails, lokal pauses again and lets you drop into a shell to inspect the environment before deciding what to do next.

## Supported CI Platforms

- **GitHub Actions** (`.github/workflows/*.yml`)
- **GitLab CI** (`.gitlab-ci.yml`)
- **CircleCI** (`.circleci/config.yml`)

## Features

- Step-through debugging with pause/continue/skip/retry
- Live shell inside the running container at any step
- `if:` conditionals at job and step level
- `needs:` job dependency ordering
- `${{ }}` expression evaluation
- `continue-on-error:` and `timeout-minutes:` support
- AI-powered failure analysis (via Claude)
- `actions/cache`, `actions/upload-artifact`, `actions/download-artifact` support
