# lokal

**Step-through debugger for CI pipelines.** Run your GitHub Actions, GitLab CI, or CircleCI pipeline locally in Docker — pausing before each step so you can inspect, skip, retry, or drop into a live shell.

No more commit → push → wait → fail → repeat.

---

## Quickstart

**Requirements:** [Go 1.22+](https://go.dev/dl/) · [Docker Desktop](https://www.docker.com/products/docker-desktop/)

```bash
git clone https://github.com/KevinLanahan/Lokal.git
cd Lokal
go build -o lokal .
./lokal run
```

That's it. lokal will auto-discover your workflow file and start stepping through it.

> **Don't have a workflow file yet?** Run `./lokal init` to scaffold one for GitHub Actions, GitLab CI, or CircleCI.

---

## Controls

When lokal pauses before a step:

| Key | Action |
|-----|--------|
| `c` | Run the step |
| `s` | Skip the step |
| `sh` | Drop into a live shell inside the container |
| `a` | Abort the run |
| `r` | Retry (shown after a failure) |

---

## Secrets

Store secrets for your pipeline runs — they get injected into the container automatically:

```bash
./lokal secrets set AWS_ACCESS_KEY_ID=abc123
./lokal secrets set NPM_TOKEN=xyz789
./lokal secrets list        # view stored secrets (masked)
./lokal secrets remove KEY  # remove a secret
./lokal secrets import      # import matching vars from your shell env
```

Secrets are stored in `.env` (already in `.gitignore` — never committed).

---

## Live Share

Share a live link before your run starts and let teammates watch every step update in real time:

1. Set up Supabase (free): create a project, run the schema below, add keys to `.env`
2. Run `./lokal run` — lokal will ask if you want a live share link
3. Send the link — anyone can watch the pipeline run from their browser

**Supabase schema:**
```sql
create table sessions (
  id uuid default gen_random_uuid() primary key,
  slug text unique not null,
  workflow_name text not null,
  platform text not null,
  steps jsonb not null,
  session_status text not null default 'completed',
  created_at timestamptz default now()
);
alter table sessions enable row level security;
create policy "sessions are publicly readable" on sessions for select using (true);
create policy "sessions can be inserted by anyone" on sessions for insert with check (true);
create policy "sessions can be updated by anyone" on sessions for update using (true) with check (true);
```

**`.env` keys:**
```
SUPABASE_URL=https://your-project.supabase.co
SUPABASE_ANON_KEY=your-anon-key
```

To delete a shared session:
```bash
./lokal delete <slug>
```

---

## AI Failure Analysis

When a step fails, lokal automatically explains why and suggests a fix — powered by Claude.

Add your API key to `.env`:
```
ANTHROPIC_API_KEY=your-key
```

Get a key at [console.anthropic.com](https://console.anthropic.com).

---

## Supported Platforms

| Platform | File |
|----------|------|
| GitHub Actions | `.github/workflows/*.yml` |
| GitLab CI | `.gitlab-ci.yml` |
| CircleCI | `.circleci/config.yml` |

---

## All Features

- Step-through debugging — pause before every step
- Live shell inside the container at any point
- `if:` conditionals at job and step level
- `needs:` job dependency ordering
- `${{ secrets.* }}`, `${{ steps.*.outputs.* }}` expression evaluation
- `continue-on-error:` and `timeout-minutes:` support
- AI-powered failure analysis (via Claude)
- Live share viewer — watch pipelines run in real time from any device
- Secrets management with `lokal secrets`
- Session sharing and deletion with `lokal delete`
