# commitai

**AI-powered Git commit messages, right from your terminal.**

`commitai` analyzes your staged Git changes and generates meaningful commit messages. Review suggestions, edit them, regenerate when needed, and commit without leaving your terminal.


https://github.com/user-attachments/assets/e73d5f1e-6385-4f65-8360-a745a03a002d


- **Multiple AI providers:** OpenAI, Groq, Gemini, DeepSeek, OpenRouter, Together, and Anthropic.
- **Local-first:** Generate commit messages offline without an API key.
- **Interactive TUI:** Select, edit, regenerate, and commit messages in one workflow.
- **Security-focused:** Excludes sensitive files and redacts detected secrets before sending diffs to remote providers.
- **Lightweight:** Distributed as a single binary.

## Features

- Analyzes staged Git diffs rather than your entire working directory.
- Generates multiple commit message suggestions.
- Supports Conventional Commits and other formatting styles.
- Works with cloud-based and local language models.
- Allows custom prompt rules and examples.
- Filters sensitive files and redacts secret-looking values.
- Supports Linux, macOS, and Windows.

## Installation

### Option 1: Download a release

The recommended installation method is to download a prebuilt binary.

1. Visit the [GitHub Releases](https://github.com/maryam-nokohan/commitai/releases) page.
2. Download the archive for your operating system and architecture.
3. Extract and install the binary.

Example for Linux (AMD64):

```bash
tar -xzf commitai_*_linux_amd64.tar.gz

mkdir -p ~/.local/bin
install -m 0755 commitai ~/.local/bin/commitai
```

Ensure `~/.local/bin` is in your `PATH`.

### Option 2: Install with Go

Requires Go 1.24 or later.

```bash
go install github.com/maryam-nokohan/commitai/cmd/commitai@latest
```

The binary is installed to `$(go env GOPATH)/bin`.

Make sure that directory is in your `PATH`.

### Option 3: Build from source

```bash
git clone https://github.com/maryam-nokohan/commitai.git
cd commitai

./install.sh
```

Alternatively:

```bash
make install
```

By default, the installer uses `~/.local/bin`.

To change the installation directory:

```bash
PREFIX=/usr/local ./install.sh
```

Uninstall:

```bash
./install.sh uninstall
```

Or:

```bash
make uninstall
```

### Add the binary to PATH

If `commitai` is not found, add the installation directory to your shell's `PATH`.

**Bash / Zsh:**

```bash
export PATH="$HOME/.local/bin:$PATH"
```

**Fish:**

```fish
fish_add_path ~/.local/bin
```

Verify the installation:

```bash
commitai -version
```

> Windows users can use the prebuilt releases or `go install`. The installation script requires a Bash-compatible environment such as Git Bash or WSL.

## Quick Start

Stage your changes:

```bash
git add .
```

Run `commitai`:

```bash
commitai
```

Review the generated suggestions, select one, and commit.

### Keyboard shortcuts

| Key | Action |
|---|---|
| `↑` / `↓` or `j` / `k` | Navigate suggestions |
| `Enter` | Select a suggestion |
| `e` | Edit the selected message |
| `r` | Regenerate suggestions |
| `q` / `Ctrl+C` | Quit |

By default, `commitai` uses its local backend. No API key or network connection is required.

## Configuration

Configuration is optional. You can use the default settings or customize the backend, model, commit style, and number of suggestions.

### Configuration file

The default configuration location is:

```text
~/.config/commitai/config.yml
```

Create it using the example configuration:

```bash
mkdir -p ~/.config/commitai
cp config.example.yml ~/.config/commitai/config.yml
```

To display the configuration path used by the application:

```bash
commitai -config-path
```

To use a different configuration file:

```bash
export COMMITAI_CONFIG=/path/to/config.yml
```

`commitai` deliberately does not load configuration or API keys from the repository being committed. This prevents an untrusted repository from redirecting your credentials.

### Configuration precedence

Settings are resolved in the following order, from highest to lowest priority:

1. Command-line flags
2. Environment variables
3. Configuration file
4. Built-in defaults

### API keys

API keys can be provided through environment variables or stored in:

```text
~/.config/commitai/.env
```

Use the provided `.env.example` as a reference.

For example:

```env
OPENAI_API_KEY=your_openai_key
GROQ_API_KEY=your_groq_key
GEMINI_API_KEY=your_gemini_key
OPENROUTER_API_KEY=your_openrouter_key
ANTHROPIC_API_KEY=your_anthropic_key
```

Protect the file:

```bash
chmod 600 ~/.config/commitai/.env
```

**Supported provider credentials:**

| Provider | Environment variable |
|---|---|
| OpenAI | `OPENAI_API_KEY` |
| Groq | `GROQ_API_KEY` |
| Gemini | `GEMINI_API_KEY` |
| OpenRouter | `OPENROUTER_API_KEY` |
| DeepSeek | `DEEPSEEK_API_KEY` |
| Together | `TOGETHER_API_KEY` |
| Anthropic | `ANTHROPIC_API_KEY` |

### Configuration options

| YAML key | Environment variable | CLI flag | Default |
|---|---|---|---|
| `backend` | `COMMITAI_BACKEND` | `-backend` | `local` |
| `model` | `COMMITAI_MODEL` | `-model` | Backend-specific |
| `style` | `COMMITAI_STYLE` | `-style` | `conventional` |
| `api_base` | `COMMITAI_API_BASE` | `-api-base` | Provider-specific |
| `api_key` | `COMMITAI_API_KEY` | `-api-key` | Provider environment variable |
| `suggestions` | `COMMITAI_SUGGESTIONS` | `-suggestions` | `3` (maximum `10`) |
| `ollama_url` | `OLLAMA_HOST` | — | `http://localhost:11434` |

Additional flags:

| Flag | Description |
|---|---|
| `-choose-backend` | Always display the backend picker |
| `-config-path` | Display the active configuration path |
| `-version` | Display the application version |

Avoid passing API keys through command-line flags because they may be visible in process listings.

## AI Backends

### Local

The default backend. Works offline without an API key.

```yaml
backend: "local"
```

### OpenAI

```yaml
backend: "openai"
model: "gpt-4o-mini"
api_base: "https://api.openai.com/v1"
```

Set `OPENAI_API_KEY` in your environment or `.env` file.

### Groq

```yaml
backend: "groq"
```

Groq automatically configures its API endpoint and default model.

Set `GROQ_API_KEY` to authenticate.

### Gemini

```yaml
backend: "gemini"
```

Set `GEMINI_API_KEY` to authenticate. Choose a model currently available to your Google AI API account.

### OpenRouter

OpenRouter provides access to multiple models through an OpenAI-compatible API.

```yaml
backend: "openai"
model: "openrouter/free"
api_base: "https://openrouter.ai/api/v1"
```

Set `OPENROUTER_API_KEY` in your environment or `.env` file.

### DeepSeek and Together

Both providers can be configured through the OpenAI-compatible backend by specifying their API base URL and model.

### Anthropic

Uses the native Anthropic Messages API.

```yaml
backend: "anthropic"
model: "claude-sonnet-4-6"
```

Set `ANTHROPIC_API_KEY` to authenticate.

### Ollama

Run local language models using [Ollama](https://ollama.com/).

```yaml
backend: "ollama"
model: "llama3.2"
```

Pull and start a model:

```bash
ollama pull llama3.2
ollama serve
```

No cloud API key is required.

## Commit Message Styles

Choose a built-in style or define your own template.

| Style | Example |
|---|---|
| `conventional` | `feat(auth): add login` |
| `colon` | `feat: add login` |
| `bracket` | `[feat] add login` |
| `plain` | `add login` |
| Custom template | `{type}({scope}): {message}` |

Custom templates must contain `{message}`.

Set the style in your configuration:

```yaml
style: "conventional"
```

## Customizing Generation

`commitai` includes built-in prompt rules and examples.

You can override the following knowledge files:

- `rules.txt`
- `examples.txt`
- `bad_examples.txt`

Place customized files in either location:

**Project-specific:**

```text
<repository>/.commitai/
```

**Global:**

```text
~/.config/commitai/knowledge/
```

Project-specific files allow different repositories to use their own commit conventions and generation rules.

## Security

Security is a core design consideration.

Before sending a diff to a remote AI provider, `commitai` applies several safeguards:

- **Sensitive file filtering:** Files such as `.env*`, `*.pem`, `*.key`, `id_rsa*`, and files with names associated with secrets, credentials, tokens, or passwords are excluded.
- **Secret redaction:** Detects and redacts API keys, bearer tokens, private keys, cloud credentials, and URL credentials.
- **Diff size management:** Large diffs are trimmed fairly across files.
- **Lock file handling:** Lock files are reduced to their headers.
- **Secure endpoints:** HTTPS is required for remote API endpoints. Plain HTTP is permitted for localhost services such as Ollama.
- **Local execution:** The local backend does not transmit data to external services.

Only the filtered and processed diff is sent to a remote provider.

> No automated secret filter can guarantee detection of every sensitive value. Review your staged changes before committing or sending them to a remote provider.

If an API key is exposed, revoke it and generate a replacement immediately.

## Development

### Requirements

- Go 1.24+
- Git
- Make (optional)

### Build and test

```bash
make check
make build
make run
```

`make check` runs formatting, static analysis, and tests.

### Project Structure

```text
cmd/commitai/       CLI entry point and flags
internal/ai/        AI providers and generation logic
internal/config/    Configuration and environment handling
internal/git/       Git repository integration
internal/knowledge/ Prompt rules and examples
internal/safety/    Sensitive-file filtering and redaction
internal/style/     Commit message formatting
internal/ui/        Interactive terminal interface
```

See [GitHub Releases](https://github.com/maryam-nokohan/commitai/releases).

## License

Add a `LICENSE` file to the repository before publishing a license declaration. MIT is a common choice for open-source developer tools.
