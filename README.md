# My Blog

![CI Status](https://github.com/cenkcorapci/my-blog/actions/workflows/ci.yml/badge.svg)
[![Netlify Status](https://api.netlify.com/api/v1/badges/e078470a-6aac-40d8-8464-808888ae3d22/deploy-status)](https://app.netlify.com/projects/cenkcorapci/deploys)
[![codecov](https://codecov.io/gh/cenkcorapci/my-blog/branch/main/graph/badge.svg?token=G9P8UXYR8O)](https://codecov.io/gh/cenkcorapci/my-blog)

A minimal, high-performance static blog generator in Go with zero-latency client-side search.

## Features

- 🌙 **Sophisticated Minimalist Style** - Beautiful dark theme with Inter typography
- 🔍 **Full-Text Frontend Search** - Instant search & suggestions with keyboard navigation
- 🏷️ **Tag Filtering** - Clickable tags to explore related content
- 📝 **Markdown support** - Write posts in Markdown, rendered with goldmark
- 📐 **Math Support** - Render complex mathematical formulas using KaTeX (MathJax)
- 🌓 **Theme Switching** - Toggle between dark and light modes with zero-flicker transitions
- 🚀 **Instant Navigation** - Hover-based prefetching for near-zero latency between pages
- 📦 **Automated Minification** - Built-in Go minifier for HTML, CSS, JS, and JSON
- ⚡ **Zero Backend** - Purely static, deployable anywhere (Netlify, GitHub Pages, etc.)
- 🌐 **Netlify Ready** - Optimized for high-performance JAMstack deployment with clean URLs

## Quick Start

### Local Development

1. Clone the repository:
```bash
git clone https://github.com/cenkcorapci/my-blog.git
cd my-blog
```

2. Run the generator and preview server:
```bash
make run
```

3. Open http://localhost:8080 in your browser.

The blog generates all content from the `blog/` directory. Any changes to markdown files will be reflected after a re-run/refresh.

## Search & Tags

The search system is powered by a pre-generated `search-index.json`. 
- **Full-Text Search**: Indexed titles and content.
- **Tag Search**: Priority matches for specific tags.
- **Instant Suggestions**: Real-time results as you type.

Everything happens on the client side for maximum speed and offline support.

## Building and Testing

### Build Targets

- `make all`: Runs all tests and generates the static site.
- `make build`: Compiles the site generator binary (`blog-gen`).
- `make static`: Generates the static site in the `dist/` folder.
- `make run`: Starts a local preview server for the generated site.
- `make clean`: Removes build artifacts.

### Testing

This project includes tests for both the Go generator and the JavaScript search engine.

**Go Tests:**
```bash
make test-go
```

**JavaScript Tests:** (Requires Node.js)
```bash
npm install
make test-js
```

## Deployment

Since the site is purely static, you can host it on any provider.

### Netlify (Recommended)
The project includes a `netlify.toml` which is ready for deployment.
- **Build Command**: `go run main.go -dist dist`
- **Publish Directory**: `dist`
- **Clean URLs**: Automatically handles `/post/slug/` redirects to `/post/slug/index.html`.

## Architecture

- **Generator**: Go (Loads posts, renders goldmark, minifies assets for production)
- **Search**: Vanilla JS (Consumes a minified JSON inverted index; supports keyboard navigation)
- **Style**: Vanilla CSS (Tailored dark/light themes; minified during build)

## Project Structure

```
.
├── internal/blog/           # Static generator logic
├── blog/                    # Markdown blog posts
├── templates/               # HTML templates
├── static/                  # Static assets (JS/CSS)
├── main.go                  # CLI entry point
├── netlify.toml             # Netlify configuration
├── package.json             # JS Dependencies (Jest)
└── Makefile                 # Build automation
```

## License

MIT License - see LICENSE file for details
