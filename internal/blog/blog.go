package blog

import (
	"bytes"
	"embed"
	"encoding/json"
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

	mathjax "github.com/litao91/goldmark-mathjax"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"gopkg.in/yaml.v3"
)

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
	staticMode    bool
	templatesFS   embed.FS
	staticFS      embed.FS
	blogFS        embed.FS
}

func NewBlog(templatesFS, staticFS, blogFS embed.FS) (*Blog, error) {
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
		// During tests, templates might be missing
		log.Printf("Warning: Error loading templates: %v", err)
	}

	return &Blog{
		posts:         make(map[string]*Post),
		postList:      make([]*Post, 0),
		templates:     templates,
		markdown:      md,
		searchCache:   &SearchCache{results: make(map[string][]string)},
		invertedIndex: &InvertedIndex{index: make(map[string][]string)},
		Config:        loadConfig(),
		templatesFS:   templatesFS,
		staticFS:      staticFS,
		blogFS:        blogFS,
	}, nil
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

func (b *Blog) LoadPosts() error {
	entries, err := b.blogFS.ReadDir("blog")
	if err != nil {
		return fmt.Errorf("failed to read blog directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		path := "blog/" + entry.Name()
		content, err := b.blogFS.ReadFile(path)
		if err != nil {
			log.Printf("Error reading file %s: %v", path, err)
			continue
		}

		post, err := b.parsePost(entry.Name(), string(content))
		if err != nil {
			log.Printf("Error parsing post %s: %v", entry.Name(), err)
			continue
		}

		b.posts[post.ID] = post
		b.postList = append(b.postList, post)
	}

	sort.Slice(b.postList, func(i, j int) bool {
		return b.postList[i].Date.After(b.postList[j].Date)
	})

	b.buildInvertedIndex()
	return nil
}

func (b *Blog) parsePost(filename, content string) (*Post, error) {
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid frontmatter")
	}

	frontmatter := parts[1]
	markdownContent := strings.TrimSpace(parts[2])

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
		HTMLContent: template.HTML(buf.String()),
		Slug:        slug,
	}, nil
}

func (b *Blog) buildInvertedIndex() {
	b.invertedIndex.mu.Lock()
	defer b.invertedIndex.mu.Unlock()

	b.invertedIndex.index = make(map[string][]string)

	for _, post := range b.posts {
		words := tokenize(post.Title + " " + post.Content)
		for _, word := range words {
			word = strings.ToLower(word)
			if !contains(b.invertedIndex.index[word], post.ID) {
				b.invertedIndex.index[word] = append(b.invertedIndex.index[word], post.ID)
			}
		}
	}
}

func tokenize(text string) []string {
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

	b.searchCache.mu.RLock()
	if cachedIDs, ok := b.searchCache.results[query]; ok {
		b.searchCache.mu.RUnlock()
		return b.getPostsByIDs(cachedIDs)
	}
	b.searchCache.mu.RUnlock()

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
		sort.Slice(tagMatches, func(i, j int) bool {
			return tagMatches[i].Date.After(tagMatches[j].Date)
		})
		return tagMatches
	}

	words := tokenize(query)
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
	sort.Slice(posts, func(i, j int) bool {
		return posts[i].Date.After(posts[j].Date)
	})
	return posts
}

func (b *Blog) handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "" {
		http.NotFound(w, r)
		return
	}

	data := map[string]interface{}{
		"Title":      "Home",
		"Posts":      b.postList,
		"Config":     b.Config,
		"StaticMode": b.staticMode,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	b.templates.ExecuteTemplate(w, "index.html", data)
}

func (b *Blog) handlePost(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimPrefix(r.URL.Path, "/post/")
	slug = strings.TrimSuffix(slug, "/")
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
		"Title":      post.Title,
		"Post":       post,
		"Config":     b.Config,
		"StaticMode": b.staticMode,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	b.templates.ExecuteTemplate(w, "post.html", data)
}

func (b *Blog) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	posts := b.search(query)

	data := map[string]interface{}{
		"Title":      "Search Results",
		"Query":      query,
		"Posts":      posts,
		"Config":     b.Config,
		"StaticMode": b.staticMode,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	b.templates.ExecuteTemplate(w, "search.html", data)
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

	for _, post := range b.posts {
		if strings.Contains(strings.ToLower(post.Title), query) {
			if !suggestionsMap[post.Title] {
				suggestionsMap[post.Title] = true
				suggestions = append(suggestions, post.Title)
			}
		}
	}

	if len(suggestions) > 10 {
		suggestions = suggestions[:10]
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(suggestions)
}

type SearchIndexPost struct {
	ID    string   `json:"id"`
	Title string   `json:"title"`
	Date  string   `json:"date"`
	Tags  []string `json:"tags"`
	Slug  string   `json:"slug"`
}

type SearchIndex struct {
	Posts         []SearchIndexPost   `json:"posts"`
	InvertedIndex map[string][]string `json:"invertedIndex"`
}

func (b *Blog) NewSearchIndex() SearchIndex {
	var posts []SearchIndexPost
	for _, post := range b.postList {
		posts = append(posts, SearchIndexPost{
			ID:    post.ID,
			Title: post.Title,
			Date:  post.Date.Format("2006-01-02"),
			Tags:  post.Tags,
			Slug:  post.Slug,
		})
	}

	b.invertedIndex.mu.RLock()
	invertedIndex := make(map[string][]string)
	for word, ids := range b.invertedIndex.index {
		invertedIndex[word] = ids
	}
	b.invertedIndex.mu.RUnlock()

	return SearchIndex{
		Posts:         posts,
		InvertedIndex: invertedIndex,
	}
}

func (b *Blog) exportSearchIndex(distDir string) {
	searchIndex := b.NewSearchIndex()
	jsonData, _ := json.MarshalIndent(searchIndex, "", "  ")
	os.WriteFile(filepath.Join(distDir, "search-index.json"), jsonData, 0644)
}

func (b *Blog) handleSearchIndex(w http.ResponseWriter, r *http.Request) {
	searchIndex := b.NewSearchIndex()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(searchIndex)
}

func (b *Blog) ExportStatic(distDir string) {
	b.staticMode = true
	os.RemoveAll(distDir)
	os.MkdirAll(distDir, 0755)

	homeFile, _ := os.Create(filepath.Join(distDir, "index.html"))
	b.handleHome(StaticResponseWriter{homeFile}, &http.Request{URL: &url.URL{Path: "/"}})
	homeFile.Close()

	os.MkdirAll(filepath.Join(distDir, "search"), 0755)
	searchFile, _ := os.Create(filepath.Join(distDir, "search", "index.html"))
	b.handleSearch(StaticResponseWriter{searchFile}, &http.Request{URL: &url.URL{Path: "/search"}})
	searchFile.Close()

	os.MkdirAll(filepath.Join(distDir, "post"), 0755)
	for slug := range b.posts {
		os.MkdirAll(filepath.Join(distDir, "post", slug), 0755)
		postFile, _ := os.Create(filepath.Join(distDir, "post", slug, "index.html"))
		b.handlePost(StaticResponseWriter{postFile}, &http.Request{URL: &url.URL{Path: "/post/" + slug}})
		postFile.Close()
	}

	os.MkdirAll(filepath.Join(distDir, "static"), 0755)
	entries, _ := b.staticFS.ReadDir("static")
	for _, entry := range entries {
		data, _ := b.staticFS.ReadFile("static/" + entry.Name())
		os.WriteFile(filepath.Join(distDir, "static", entry.Name()), data, 0644)
	}

	b.exportSearchIndex(distDir)
}

type StaticResponseWriter struct {
	File *os.File
}

func (s StaticResponseWriter) Header() http.Header         { return make(http.Header) }
func (s StaticResponseWriter) Write(b []byte) (int, error) { return s.File.Write(b) }
func (s StaticResponseWriter) WriteHeader(statusCode int)  {}

func (b *Blog) Router() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", b.handleHome)
	mux.HandleFunc("/post/", b.handlePost)
	mux.HandleFunc("/search", b.handleSearch)
	mux.HandleFunc("/api/suggestions", b.handleSuggestions)
	mux.HandleFunc("/search-index.json", b.handleSearchIndex)

	staticContent, err := fs.Sub(b.staticFS, "static")
	if err == nil {
		mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticContent))))
	}

	return mux
}
