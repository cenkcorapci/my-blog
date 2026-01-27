# Cenk Corapci

![CI Status](https://github.com/cenkcorapci/my-blog/actions/workflows/ci.yml/badge.svg)
![Deployment Status](https://github.com/cenkcorapci/my-blog/actions/workflows/deploy.yml/badge.svg)
[![codecov](https://codecov.io/gh/cenkcorapci/my-blog/branch/main/graph/badge.svg?token=G9P8UXYR8O)](https://codecov.io/gh/cenkcorapci/my-blog)

A minimal, fast Go blog with dark mode and full-text search.

## Features

- 🌙 **Sophisticated Minimalist Style** - Beautiful dark theme with Inter typography
- 🏷️ **Tags Support** - Display tags on post listings for better categorization
- 📝 **Markdown support** - Write posts in Markdown, rendered with goldmark
- 🔍 **Full-text & Tag search** - Intelligent search that filters by exact tag match or full-text keywords
- 💡 **Search Suggestions** - Real-time suggestions as you type, based on titles and tags
- 📐 **Math Support** - Render complex mathematical formulas using KaTeX
- 🌓 **Theme Switching** - Toggle between sophisticated dark and clean light modes
- ⚡ **Server-side rendering** - Minimal JavaScript, fast page loads
- 🚀 **Heroku-ready** - Easy deployment with included Procfile
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
go run main.go
```

3. Open http://localhost:8080 in your browser

The blog automatically loads all posts from the `blog/` directory on startup.

### Blog Post Structure

Each post must be a `.md` file with a **YAML Frontmatter** section at the top. This section is used by the Go backend to index and sort your posts. 

The following metadata fields are supported:

- `title`: The display name of your blog post (appears in search results and post list).
- `date`: The publication date in `YYYY-MM-DD` format (used for reverse-chronological sorting).
- `tags`: A comma-separated list of tags (e.g., `go, backend, tutorial`).

Example:

```markdown
---
title: Building Minimal APIs in Go
date: 2024-01-26
tags: go, web-dev
---

# Your Content Starts Here
```

> **Note**: These fields are strictly required for the post to be rendered and sorted correctly.

## Configuration

You can customize the blog name and introduction by editing `config.yaml`:

```yaml
blog_name: "Cenk Corapci"
introduction: "I'm Cenk, a data engineer based in Netherlands."
```

- `blog_name`: Changes the title across all pages and the navigation brand.
- `introduction`: Updates the "About Me" section on the home page.

### Supported Markdown Tags

Your blog supports **GitHub Flavored Markdown (GFM)** and **Monokai Syntax Highlighting**. You can use:

- **Headings**: `# h1`, `## h2`, `### h3`
- **Text Styling**: `**bold**`, `*italic*`, `~~strikethrough~~`
- **Lists**: Bulleted `- item` and numbered `1. item`
- **Links & Images**: `[link text](url)` and `![alt text](image-url)`
- **Code Blocks**: With syntax highlighting using triple backticks:
    ```go
    func hello() {
        fmt.Println("Hello, World!")
    }
    ```
- **Quotes**: `> This is a blockquote`
- **Tables**: Standard Markdown tables are supported.
- **Task Lists**: `- [x] Done` and `- [ ] Todo`
- **Math Formulas**: LaTeX style math using `$` for inline ($E=mc^2$) and `$$` for block:
    $$
    x = \frac{-b \pm \sqrt{b^2 - 4ac}}{2a}
    $$

## Building and Testing

The project includes a `Makefile` to automate common developer tasks:

- `make all`: Runs tests, builds the binary, and exports the static site.
- `make build`: Compiles the Go binary.
- `make test`: Executes unit tests.
- `make static`: Generates the static version of the blog in the `dist/` folder.
- `make run`: Starts the blog server locally.
- `make clean`: Removes the compiled binary and the `dist/` directory.

## Deployment to Heroku

### Setup

1. Create a Heroku app:
```bash
heroku create your-app-name
```

2. Add the required secrets to your GitHub repository:
   - `HEROKU_API_KEY` - Your Heroku API key
   - `HEROKU_APP_NAME` - Your Heroku app name
   - `HEROKU_EMAIL` - Your Heroku account email

3. Push to the `main` branch to trigger automatic deployment

### Manual Deployment

```bash
git push heroku main
```

## Deployment to Netlify

This blog supports high-performance static deployment to Netlify.

### Automated Setup (via GitHub Actions)

1. Create a new site on Netlify.
2. Add the following secrets to your GitHub repository:
   - `NETLIFY_AUTH_TOKEN`: Your Personal Access Token.
   - `NETLIFY_SITE_ID`: The API ID of your Netlify site.
3. Every push to `main` will trigger a custom static export and deploy to Netlify.

### How it Works (Static Export)

Since Netlify is a static hosting platform, the blog includes a custom engine to generate HTML files:
```bash
go run main.go -static
```
This renders every post and the homepage into a `dist/` folder which is then served globally.

> **Note**: In the Netlify version, the dynamic backend search API and suggestions are replaced by static search pages. For the full dynamic experience with real-time suggestions, Heroku is recommended.

## Architecture

### Search System

- **Inverted Index**: Maps each word to a list of post IDs containing that word
- **Search Cache**: Caches search results with RWMutex for thread-safe concurrent access
- **AND Search**: Returns posts that match all search terms

### Technology Stack

- **Backend**: Go (standard library)
- **Markdown**: goldmark with GFM extensions
- **Styling**: Sophisticated Minimalist CSS (no dependencies)
- **Deployment**: Heroku with GitHub Actions

## Project Structure

```
.
├── blog/                    # Markdown blog posts
├── templates/               # HTML templates
│   ├── index.html          # Homepage
│   ├── post.html           # Post detail page
│   └── search.html         # Search results page
├── static/                  # Static assets
│   └── style.css           # Custom CSS
├── .github/workflows/       # GitHub Actions
│   └── deploy.yml          # Heroku deployment workflow
├── main.go                  # Main application
├── Procfile                 # Heroku configuration
└── go.mod                   # Go module definition
```

## License

MIT License - see LICENSE file for details

