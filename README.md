# Commit

A CLI tool that uses Claude AI to generate conventional commit messages from your staged changes.

## Overview

Commit is designed to streamline the Git commit process by:
1. Analyzing your staged changes
2. Using Claude AI to generate a conventional commit message
3. Allowing you to accept, edit, or reject the suggestion
4. Automatically committing the changes

## Installation

### Using Go Install (Recommended)

```bash
# Clone the repository
git clone https://github.com/asteroidai/devtools.git

# Navigate to the commit directory
cd devtools/commit

# Install globally
go install
```

Make sure `$GOPATH/bin` is in your PATH:

```bash
# Add to ~/.bashrc or ~/.zshrc
export GOPATH=$HOME/go
export PATH=$PATH:$GOPATH/bin
```

## Usage

Before running `git add`, simply run:

```bash
commit
```

The tool will:
1. Analyze your changes
2. Generate a conventional commit message
3. Present options to accept, edit, or reject the message
4. Create the commit if accepted

### Flags

- `--debug`: Enable debug output

### Environment Variables

- `ANTHROPIC_API_KEY`: Required. Your Claude API key
- `EDITOR`: Optional. Your preferred editor for message editing (defaults to vim)

## Requirements

- Go 1.22 or higher
- Git
- Anthropic API key
- Write access to the repository
