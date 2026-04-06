# Argo



**Argo** is a powerful, terminal-based AI coding agent written in Go, designed to bring the capabilities of large language models directly into your development workflow. Interact with an intelligent assistant that can read, write, and modify code within your projects, all from the comfort and speed of your terminal. Argo aims to be secure out of the box as "Security is the best policy".

## Getting Started

This section will guide you through setting up and running Argo for the first time.

### Prerequisites

Before you begin, ensure you have the following installed:

*   **Go** 1.23 or later ([Installation Guide](https://go.dev/doc/install))
*   An API key for your chosen Large Language Model (LLM) provider (e.g., Anthropic, OpenAI, Google Gemini). You will need to set this as an environment variable:
    *   `ANTHROPIC_API_KEY` for Anthropic models
    *   `OPENAI_API_KEY` for OpenAI models
    *   `GEMINI_API_KEY` for Google Gemini models

## Features

Argo provides a rich set of features to streamline your coding experience:

*   **🖥️ Beautiful Terminal UI:** Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) for interactive applications and [Lip Gloss](https://github.com/charmbracelet/lipgloss) for stunning terminal styling, offering an intuitive and visually appealing interface.
*   **🤖 Multi-provider LLM Support:** Seamlessly integrate with various Large Language Model providers, including Anthropic, OpenAI, and Google Gemini, allowing you to choose your preferred AI backend.
*   **🗄️ Local Conversation History:** Your interactions with Argo are securely stored locally using SQLite, enabling persistent conversation history across sessions.
*   **⚡ Fast and Lightweight:** Developed in Go, Argo compiles into a single, self-contained binary with no external runtime dependencies, ensuring high performance and minimal footprint.
*   **🎨 Markdown Rendering in the Terminal:** Leveraging [Glamour](https://github.com/charmbracelet/glamour), Argo renders Markdown content directly in your terminal, making AI responses and documentation easy to read and digest.

## Build & Run

- [Go](https://go.dev/) 1.23 or later
- An API key for your chosen LLM provider (e.g. `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `GEMINI_API_KEY`)

## Build & Run

Argo uses a `Makefile` for common tasks:

```shell
# Build the binary
make build

# Run directly without building
make run

# Run tests
make test

# Lint (requires golangci-lint)
make lint

# Tidy dependencies
make tidy

# Clean build artifacts
make clean
```

The compiled binary will be placed at `bin/argo`.

## Configuration

You can pass flags when running Argo:

| Flag         | Default                        | Description                  |
|--------------|--------------------------------|------------------------------|
| `--model`    | `claude-sonnet-4-20250514`     | The model to use             |
| `--provider` | `anthropic`                    | The LLM provider to use (`anthropic`, `openai`, `gemini`) |

Example:

```shell
./bin/argo --model claude-sonnet-4-20250514 --provider anthropic
```

```shell
./bin/argo --model gemini-2.5-flash --provider gemini
```

## License

MIT
