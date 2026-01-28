# Cenk Corapci

![CI Status](https://github.com/cenkcorapci/my-blog/actions/workflows/ci.yml/badge.svg)
[![Netlify Status](https://api.netlify.com/api/v1/badges/e078470a-6aac-40d8-8464-808888ae3d22/deploy-status)](https://app.netlify.com/projects/cenkcorapci/deploys)
[![codecov](https://codecov.io/gh/cenkcorapci/my-blog/branch/main/graph/badge.svg?token=G9P8UXYR8O)](https://codecov.io/gh/cenkcorapci/my-blog)

A minimal, fast Go blog with dark mode and hybrid search (Static + Netlify Functions).

## Features

- 🌙 **Sophisticated Minimalist Style** - Beautiful dark theme with Inter typography
- 🏷️ **Tags Support** - Display tags on post listings for better categorization
- 📝 **Markdown support** - Write posts in Markdown, rendered with goldmark
- 🔍 **Hybrid Search System** - Intelligence that adapts to your deployment
- 💡 **Search Suggestions** - Real-time suggestions powered by Netlify Functions
- 📐 **Math Support** - Render complex mathematical formulas using KaTeX
- 🌓 **Theme Switching** - Toggle between sophisticated dark and clean light modes
- ⚡ **Go-powered Backend** - Fast performance with standard library
- 🌐 **Static Export** - Optimized for Netlify/JAMstack with zero-latency loads
- 🔄 **Auto-deploy** - GitHub Actions workflow for CI/CD

## Quick Start

### Local Development

1. Clone the repository:
```bash
git clone https://github.com/cenkcorapci/my-blog.git
cd my-blog
```

2. Run the blog:
```bash
make run
```

3. Open http://localhost:8080 in your browser

The blog automatically loads all posts from the `blog/` directory on startup.

### Blog Post Structure

Each post must be a `.md` file with a **YAML Frontmatter** section at the top.

Example:

```markdown
---
title: Building Minimal APIs in Go
date: 2024-01-26
tags: go, web-dev
---

# Your Content Starts Here
```

## Hybrid Search Modes

This blog supports two search modes depending on how you build it:

### 1. Static Mode (Frontend Search)
Run: `make static`
- **When to use**: Minimal deployments on Netlify without any active server.
- **How it works**: Generates a `search-index.json` during build. `search.js` performs full-text search directly in the browser.
- **Benefit**: Zero cost, zero latency, works on any CDN.

### 2. Dynamic Mode (Backend Search)
Run: `make build` (Deployment uses Netlify Functions)
- **When to use**: Larger sites or when you want server-side heavy lifting.
- **How it works**: Search and suggestions are handled by a Netlify Function written in Go.
- **Benefit**: Extremely fast real-time suggestions, search results don't require downloading an index.

## Building and Testing

- `make all`: Runs tests, builds binary/functions, and generates static site.
- `make build`: Compiles the binary and Netlify functions.
- `make static`: Generates the static site in the `dist/` folder (Frontend Search).
- `make run`: Starts the blog server locally (Backend Search).
- `make test`: Executes unit tests.
- `make clean`: Removes build artifacts.

## Deployment to Netlify

This blog is optimized for Netlify.

### 1. Simple Static Deploy
Set build command to: `go run main.go -static`
Set publish directory to: `dist`

### 2. Full Hybrid Deploy (Recommended)
The project includes a `netlify.toml` which is configured to build the Go functions.
- Every push to `main` will trigger a build of the static pages AND the Netlify Functions.
- Your search suggestions will be powered by the Go backend in the cloud.

## Architecture

### Search System
- **Inverted Index**: Maps words to post IDs for fast lookup.
- **Search Cache**: Caches results for repeated queries.
- **AND Search**: Returns posts matching all search terms.

### Technology Stack
- **Backend**: Go (internal package optimized for speed)
- **Serverless**: Netlify Functions (Go runtime)
- **Frontend**: Vanilla JS + CSS (minimal footprints)

## Project Structure

```
.
├── internal/blog/           # Core blog engine (Shared Logic)
├── netlify/functions/       # Netlify serverless entry points
├── blog/                    # Markdown blog posts
├── templates/               # HTML templates
├── static/                  # Static assets
├── main.go                  # CLI entry point
├── netlify.toml             # Netlify configuration
└── Makefile                 # Build automation
```

## License

MIT License - see LICENSE file for details
