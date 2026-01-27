# Cenk Corapci

A minimal, fast Go blog with dark mode and full-text search.

## Features

- 🌙 **Sophisticated Minimalist Style** - Beautiful dark theme with Inter typography
- 📝 **Markdown support** - Write posts in Markdown, rendered with goldmark
- 🔍 **Full-text search** - Fast inverted index search with caching
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

### Adding Blog Posts

Create a new `.md` file in the `blog/` directory with frontmatter:

```markdown
---
title: Your Post Title
date: 2024-01-26
---

# Your Post Title

Your content here...
```

The blog automatically loads all posts from the `blog/` directory on startup.

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

