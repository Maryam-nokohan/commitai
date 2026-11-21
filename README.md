# commitai

A fast terminal assistant that reads your **staged** Git diff, suggests commit
messages, and lets you pick, edit, regenerate, and commit, all in one small
binary.

- **Local mode** (default): free, offline, no API key
- **OpenAI-compatible** APIs: OpenAI, Groq, Gemini, DeepSeek, OpenRouter, Together...
- **Anthropic (Claude)** via the native Messages API
- **Ollama** for local LLMs
- Secrets never leave your machine: sensitive files are excluded and secret-looking strings are redacted before anything is sent

## Install

### Option 1: prebuilt binary (recommended)

Download the archive for your OS/arch from the
[Releases page](https://github.com/maryam-nokohan/commitai/releases), extract it, and put `commitai` on your `PATH`:

```bash
tar -xzf commitai_*_linux_amd64.tar.gz
install -m 0755 commitai ~/.local/bin/commitai
```

### Option 2: `go install`

Requires Go 1.24+ (Go 1.21+ downloads the right toolchain automatically).

```bash
go install github.com/maryam-nokohan/commitai/cmd/commitai@latest
```

The binary lands in `$(go env GOPATH)/bin`. Make sure that directory is on your `PATH`.

### Option 3: from source

```bash
git clone https://github.com/maryam-nokohan/commitai.git
cd commitai
./install.sh        # or: make install
```

Installs to `~/.local/bin` (override with `PREFIX=/usr/local ./install.sh`).
Uninstall with `./install.sh uninstall` or `make uninstall`.

If `~/.local/bin` isn't on your `PATH`:

```bash
# bash / zsh
export PATH="$HOME/.local/bin:$PATH"
# fish
fish_add_path ~/.local/bin
```

Verify:

```bash
commitai -version
```

Windows: use Option 1 or 2 (`install.sh` needs a bash shell such as Git Bash/WSL).

## Quick start

```bash
git add .
commitai
```

With no configuration it uses the **local** backend (no network). On first run
without a configured backend you'll get a picker; set `backend` in your config
to skip it.

| Key | Action |
|---|---|
| `↑`/`↓` or `j`/`k` | navigate |
| `enter` | select |
| `e` | edit the message |
| `r` | regenerate |
| `q` / `ctrl+c` | quit |

## Configuration

Settings are resolved in this order (highest first):

1. command-line flags
2. environment variables (`COMMITAI_*`)
3. config file
4. built-in defaults

### Config file location

```bash
commitai -config-path       # prints the path in use
```

Default: `~/.config/commitai/config.yml` (on macOS/Windows the OS config dir is also checked).
Override with `COMMITAI_CONFIG=/path/to/config.yml`.

> commitai deliberately does **not** read a `config.yml` or `.env` from the
> repository you are committing in. A cloned repo must not be able to redirect your API key.

```bash
mkdir -p ~/.config/commitai
cp config.example.yml ~/.config/commitai/config.yml
```

### API keys

Put keys in your shell environment or in `~/.config/commitai/.env`
(see `.env.example`). Prefer this over `api_key` in the YAML file or `-api-key`,
which shows up in process listings.

| Backend / host | Variable |
|---|---|
| OpenAI (default for `openai`) | `OPENAI_API_KEY` |
| `api.groq.com` / `backend: groq` | `GROQ_API_KEY` |
| `*.googleapis.com` / `backend: gemini` | `GEMINI_API_KEY` |
| `openrouter.ai` | `OPENROUTER_API_KEY` |
| `deepseek.com` | `DEEPSEEK_API_KEY` |
| Anthropic | `ANTHROPIC_API_KEY` |

### Config keys and env vars

| YAML key | Env var | Flag | Default |
|---|---|---|---|
| `backend` | `COMMITAI_BACKEND` | `-backend` | `local` |
| `model` | `COMMITAI_MODEL` | `-model` | per backend |
| `style` | `COMMITAI_STYLE` | `-style` | `conventional` |
| `api_base` | `COMMITAI_API_BASE` | `-api-base` | provider default |
| `api_key` | `COMMITAI_API_KEY` | `-api-key` | from provider env var |
| `suggestions` | `COMMITAI_SUGGESTIONS` | `-suggestions` | `3` (max 10) |
| `ollama_url` | `OLLAMA_HOST` | - | `http://localhost:11434` |

Other flags: `-choose-backend` (always show the picker), `-config-path`, `-version`.

### Backend examples

**Local (free, offline):**
```yaml
backend: "local"
```

**Groq (free tier):**
```yaml
backend: "groq"                       # sets api_base and default model for you
# model: "llama-3.3-70b-versatile"
```
```bash
export GROQ_API_KEY=gsk_...
```

**Gemini (OpenAI-compatible endpoint):**
```yaml
backend: "gemini"
# model: "gemini-2.5-flash"
```
```bash
export GEMINI_API_KEY=...
```

**OpenAI or any compatible API:**
```yaml
backend: "openai"
model: "gpt-4o-mini"
api_base: "https://api.openai.com/v1"
```

**Claude:**
```yaml
backend: "anthropic"
model: "claude-sonnet-4-6"
```

**Ollama:**
```yaml
backend: "ollama"
model: "llama3.2"
```
```bash
ollama pull llama3.2 && ollama serve
```

### Commit styles

| `style` | Result |
|---|---|
| `conventional` | `feat(auth): add login` |
| `colon` | `feat: add login` |
| `bracket` | `[feat] add login` |
| `plain` | `add login` |
| custom template | e.g. `"{type}({scope}): {message}"` (must contain `{message}`) |

### Customizing the prompt

Default rules and examples are built into the binary. Override any of
`rules.txt`, `examples.txt`, `bad_examples.txt` by placing them in:

1. `<repo>/.commitai/` (per project), or
2. `~/.config/commitai/knowledge/` (global)

## Security

- Files such as `.env*`, `*.pem`, `*.key`, `id_rsa*`, and anything named like `secret`, `token`, `credential`, `password` are **never** sent.
- Remaining diff text is scanned for API keys, bearer tokens, private keys, cloud/provider token formats, and URL credentials, which are replaced with `[REDACTED]`.
- Lock files are reduced to their header. Large diffs are trimmed fairly per file.
- Only `https` API endpoints are accepted (plain `http` is allowed for localhost / Ollama).
- The **local** backend sends nothing anywhere.
- If a key is ever exposed (commit, log, paste), **revoke and rotate it** at the provider.

## Troubleshooting

| Problem | Fix |
|---|---|
| `no staged changes found` | Run `git add <files>` first. |
| `no API key set` | Export the right `*_API_KEY` or add it to `~/.config/commitai/.env`. |
| `HTTP 401/403` | Wrong or revoked key. |
| `HTTP 404` | Wrong `model` or `api_base`. |
| `HTTP 429` | Rate limit or quota; commitai retries briefly, then reports it. |
| `could not reach Ollama` | Start it with `ollama serve`; check `OLLAMA_HOST`/`ollama_url`. |
| `command not found: commitai` | Add the install directory to `PATH`. |
| Wrong backend/model used | `commitai -config-path` to confirm which file is read. |

## Development

```bash
make check        # gofmt, go vet, tests (race)
make build
make run
```

Project layout:

```
cmd/commitai       entry point and flags
internal/ai        backends (local, OpenAI-compatible, Anthropic, Ollama)
internal/config    config file, env, defaults, validation
internal/git       git plumbing
internal/knowledge embedded prompt rules/examples + overrides
internal/safety    sensitive-file filter, redaction, diff budgeting
internal/style     commit message formatting
internal/ui        Bubble Tea TUI
```

### Releasing

```bash
git tag v1.0.0 && git push origin v1.0.0
```

The Release workflow builds Linux/macOS/Windows (amd64/arm64) archives with GoReleaser.

## License

Add a `LICENSE` file (MIT is a common choice) before publishing.