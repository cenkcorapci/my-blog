package main

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS

type Post struct {
	ID       string
	Title    string
	Date     time.Time
	Content  string
	HTMLContent template.HTML
	Slug     string
}

type SearchCache struct {
	mu      sync.RWMutex
	results map[string][]string // map[query][]postIDs
}

type InvertedIndex struct {
	mu    sync.RWMutex
	index map[string][]string // map[word][]postIDs
}

type Blog struct {
	posts         map[string]*Post
	postList      []*Post
	templates     *template.Template
	markdown      goldmark.Markdown
	searchCache   *SearchCache
	invertedIndex *InvertedIndex
}

func NewBlog() *Blog {
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithXHTML(),
		),
	)

	templates, err := template.ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		log.Fatal("Error loading templates:", err)
	}

	return &Blog{
		posts:         make(map[string]*Post),
		postList:      make([]*Post, 0),
		templates:     templates,
		markdown:      md,
		searchCache:   &SearchCache{results: make(map[string][]string)},
		invertedIndex: &InvertedIndex{index: make(map[string][]string)},
	}
}

func (b *Blog) LoadPosts(dir string) error {
	files, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read blog directory: %w", err)
	}

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".md") {
			continue
		}

		path := filepath.Join(dir, file.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			log.Printf("Error reading file %s: %v", path, err)
			continue
		}

		post, err := b.parsePost(file.Name(), string(content))
		if err != nil {
			log.Printf("Error parsing post %s: %v", file.Name(), err)
			continue
		}

		b.posts[post.ID] = post
		b.postList = append(b.postList, post)
	}

	// Sort posts by date (newest first)
	sort.Slice(b.postList, func(i, j int) bool {
		return b.postList[i].Date.After(b.postList[j].Date)
	})

	// Build inverted index
	b.buildInvertedIndex()

	log.Printf("Loaded %d blog posts", len(b.posts))
	return nil
}

func (b *Blog) parsePost(filename, content string) (*Post, error) {
	// Extract frontmatter
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid frontmatter")
	}

	frontmatter := parts[1]
	markdownContent := strings.TrimSpace(parts[2])

	// Parse frontmatter
	var title string
	var date time.Time
	for _, line := range strings.Split(frontmatter, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "title:") {
			title = strings.TrimSpace(strings.TrimPrefix(line, "title:"))
		} else if strings.HasPrefix(line, "date:") {
			dateStr := strings.TrimSpace(strings.TrimPrefix(line, "date:"))
			var err error
			date, err = time.Parse("2006-01-02", dateStr)
			if err != nil {
				log.Printf("Error parsing date: %v", err)
				date = time.Now()
			}
		}
	}

	// Convert markdown to HTML
	var buf bytes.Buffer
	if err := b.markdown.Convert([]byte(markdownContent), &buf); err != nil {
		return nil, fmt.Errorf("failed to convert markdown: %w", err)
	}

	slug := strings.TrimSuffix(filename, ".md")

	return &Post{
		ID:          slug,
		Title:       title,
		Date:        date,
		Content:     markdownContent,
		HTMLContent: template.HTML(buf.String()),
		Slug:        slug,
	}, nil
}

func (b *Blog) buildInvertedIndex() {
	b.invertedIndex.mu.Lock()
	defer b.invertedIndex.mu.Unlock()

	b.invertedIndex.index = make(map[string][]string)

	for _, post := range b.posts {
		// Tokenize title and content
		words := tokenize(post.Title + " " + post.Content)

		for _, word := range words {
			word = strings.ToLower(word)
			// Add post ID to the word's posting list if not already present
			if !contains(b.invertedIndex.index[word], post.ID) {
				b.invertedIndex.index[word] = append(b.invertedIndex.index[word], post.ID)
			}
		}
	}
}

func tokenize(text string) []string {
	// Remove special characters and split by whitespace
	re := regexp.MustCompile(`[a-zA-Z0-9]+`)
	return re.FindAllString(text, -1)
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func (b *Blog) search(query string) []*Post {
	query = strings.TrimSpace(strings.ToLower(query))
	if query == "" {
		return nil
	}

	// Check cache first
	b.searchCache.mu.RLock()
	if cachedIDs, ok := b.searchCache.results[query]; ok {
		b.searchCache.mu.RUnlock()
		return b.getPostsByIDs(cachedIDs)
	}
	b.searchCache.mu.RUnlock()

	// Tokenize search query
	words := tokenize(query)

	// Find posts matching all words (AND search)
	b.invertedIndex.mu.RLock()
	var matchingPostIDs []string
	for i, word := range words {
		word = strings.ToLower(word)
		postIDs := b.invertedIndex.index[word]

		if i == 0 {
			matchingPostIDs = postIDs
		} else {
			matchingPostIDs = intersection(matchingPostIDs, postIDs)
		}

		if len(matchingPostIDs) == 0 {
			break
		}
	}
	b.invertedIndex.mu.RUnlock()

	// Cache the results
	b.searchCache.mu.Lock()
	b.searchCache.results[query] = matchingPostIDs
	b.searchCache.mu.Unlock()

	return b.getPostsByIDs(matchingPostIDs)
}

func intersection(a, b []string) []string {
	set := make(map[string]bool)
	for _, item := range a {
		set[item] = true
	}

	var result []string
	for _, item := range b {
		if set[item] {
			result = append(result, item)
		}
	}
	return result
}

func (b *Blog) getPostsByIDs(ids []string) []*Post {
	var posts []*Post
	for _, id := range ids {
		if post, ok := b.posts[id]; ok {
			posts = append(posts, post)
		}
	}
	// Sort by date (newest first)
	sort.Slice(posts, func(i, j int) bool {
		return posts[i].Date.After(posts[j].Date)
	})
	return posts
}

// HTTP Handlers
func (b *Blog) handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	data := map[string]interface{}{
		"Title": "Home",
		"Posts": b.postList,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := b.templates.ExecuteTemplate(w, "index.html", data); err != nil {
		log.Printf("Error rendering template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (b *Blog) handlePost(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimPrefix(r.URL.Path, "/post/")
	if slug == "" {
		http.NotFound(w, r)
		return
	}

	post, ok := b.posts[slug]
	if !ok {
		http.NotFound(w, r)
		return
	}

	data := map[string]interface{}{
		"Title": post.Title,
		"Post":  post,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := b.templates.ExecuteTemplate(w, "post.html", data); err != nil {
		log.Printf("Error rendering template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (b *Blog) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	posts := b.search(query)

	data := map[string]interface{}{
		"Title": "Search Results",
		"Query": query,
		"Posts": posts,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := b.templates.ExecuteTemplate(w, "search.html", data); err != nil {
		log.Printf("Error rendering template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func main() {
	blog := NewBlog()

	// Load blog posts from /blog directory
	if err := blog.LoadPosts("blog"); err != nil {
		log.Fatal("Error loading blog posts:", err)
	}

	// Setup routes
	http.HandleFunc("/", blog.handleHome)
	http.HandleFunc("/post/", blog.handlePost)
	http.HandleFunc("/search", blog.handleSearch)

	// Serve static files
	staticContent, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Fatal("Error loading static files:", err)
	}
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticContent))))

	// Get port from environment variable (Heroku sets PORT)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting server on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal("Error starting server:", err)
	}
}
