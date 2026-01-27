package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"net/url"
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
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	mathjax "github.com/litao91/goldmark-mathjax"
	"gopkg.in/yaml.v3"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS

type Post struct {
	ID          string
	Title       string
	Date        time.Time
	Tags        []string
	Content     string
	HTMLContent template.HTML
	Slug        string
}

type Config struct {
	BlogName     string `yaml:"blog_name"`
	Introduction string `yaml:"introduction"`
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
	Config        Config
}

func NewBlog() *Blog {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			highlighting.NewHighlighting(
				highlighting.WithStyle("monokai"),
			),
			mathjax.MathJax,
		),
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
		Config:        loadConfig(),
	}
}

func loadConfig() Config {
	file, err := os.ReadFile("config.yaml")
	if err != nil {
		log.Printf("Warning: Could not read config.yaml, using defaults: %v", err)
		return Config{
			BlogName:     "Cenk Corapci",
			Introduction: "I'm Cenk, a data engineer based in Netherlands.",
		}
	}

	var config Config
	if err := yaml.Unmarshal(file, &config); err != nil {
		log.Printf("Warning: Could not parse config.yaml, using defaults: %v", err)
		return Config{
			BlogName:     "Cenk Corapci",
			Introduction: "I'm Cenk, a data engineer based in Netherlands.",
		}
	}
	return config
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
	var tags []string
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
		} else if strings.HasPrefix(line, "tags:") {
			tagsStr := strings.TrimSpace(strings.TrimPrefix(line, "tags:"))
			tagList := strings.Split(tagsStr, ",")
			for _, t := range tagList {
				tags = append(tags, strings.TrimSpace(t))
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
		Tags:        tags,
		Content:     markdownContent,
		// Note: Converting to template.HTML assumes trusted markdown sources (blog owner controls content)
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

	// Check if the query is an exact tag match
	trimmedQuery := strings.TrimSpace(query)
	var tagMatches []*Post
	for _, post := range b.posts {
		for _, tag := range post.Tags {
			if strings.EqualFold(tag, trimmedQuery) {
				tagMatches = append(tagMatches, post)
				break
			}
		}
	}

	if len(tagMatches) > 0 {
		// Sort by date (newest first)
		sort.Slice(tagMatches, func(i, j int) bool {
			return tagMatches[i].Date.After(tagMatches[j].Date)
		})
		return tagMatches
	}

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
		"Title":  "Home",
		"Posts":  b.postList,
		"Config": b.Config,
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
		"Title":  post.Title,
		"Post":   post,
		"Config": b.Config,
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
		"Title":  "Search Results",
		"Query":  query,
		"Posts":  posts,
		"Config": b.Config,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := b.templates.ExecuteTemplate(w, "search.html", data); err != nil {
		log.Printf("Error rendering template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (b *Blog) handleSuggestions(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("q")))
	if query == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]string{})
		return
	}

	suggestionsMap := make(map[string]bool)
	var suggestions []string

	// Suggest from tags
	for _, post := range b.posts {
		for _, tag := range post.Tags {
			if strings.HasPrefix(strings.ToLower(tag), query) {
				if !suggestionsMap[tag] {
					suggestionsMap[tag] = true
					suggestions = append(suggestions, tag)
				}
			}
		}
	}

	// Suggest from titles
	for _, post := range b.posts {
		if strings.Contains(strings.ToLower(post.Title), query) {
			if !suggestionsMap[post.Title] {
				suggestionsMap[post.Title] = true
				suggestions = append(suggestions, post.Title)
			}
		}
	}

	// Limit suggestions
	if len(suggestions) > 10 {
		suggestions = suggestions[:10]
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(suggestions)
}

func (b *Blog) exportStatic(distDir string) {
	os.RemoveAll(distDir)
	os.MkdirAll(distDir, 0755)

	// Export Home
	homeFile, _ := os.Create(filepath.Join(distDir, "index.html"))
	b.handleHome(StaticResponseWriter{homeFile}, &http.Request{URL: &url.URL{Path: "/"}})
	homeFile.Close()

	// Export Search Page (Initial empty state)
	os.MkdirAll(filepath.Join(distDir, "search"), 0755)
	searchFile, _ := os.Create(filepath.Join(distDir, "search", "index.html"))
	b.handleSearch(StaticResponseWriter{searchFile}, &http.Request{URL: &url.URL{Path: "/search"}})
	searchFile.Close()

	// Export Posts
	os.MkdirAll(filepath.Join(distDir, "post"), 0755)
	for slug, post := range b.posts {
		os.MkdirAll(filepath.Join(distDir, "post", slug), 0755)
		postFile, _ := os.Create(filepath.Join(distDir, "post", slug, "index.html"))
		b.handlePost(StaticResponseWriter{postFile}, &http.Request{URL: &url.URL{Path: "/post/" + slug}})
		postFile.Close()
	}

	// Export Static Files
	os.MkdirAll(filepath.Join(distDir, "static"), 0755)
	entries, _ := staticFS.ReadDir("static")
	for _, entry := range entries {
		data, _ := staticFS.ReadFile("static/" + entry.Name())
		os.WriteFile(filepath.Join(distDir, "static", entry.Name()), data, 0644)
	}

	fmt.Printf("Successfully exported static site to ./%s\n", distDir)
}

type StaticResponseWriter struct {
	file *os.File
}

func (s StaticResponseWriter) Header() http.Header         { return make(http.Header) }
func (s StaticResponseWriter) Write(b []byte) (int, error) { return s.file.Write(b) }
func (s StaticResponseWriter) WriteHeader(statusCode int)  {}

func main() {
	// Add flag import at the top of the file if not present
	// import "flag"

	staticExport := flag.Bool("static", false, "Export the blog as a static site")
	flag.Parse()

	blog := NewBlog()

	// Load blog posts from /blog directory
	if err := blog.LoadPosts("blog"); err != nil {
		log.Fatal(err)
	}

	if *staticExport {
		blog.exportStatic("dist")
		return
	}

	// Setup routes
	http.HandleFunc("/", blog.handleHome)
	http.HandleFunc("/post/", blog.handlePost)
	http.HandleFunc("/search", blog.handleSearch)
	http.HandleFunc("/api/suggestions", blog.handleSuggestions)

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
