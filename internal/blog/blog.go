package blog

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	mathjax "github.com/litao91/goldmark-mathjax"
	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/css"
	mhtml "github.com/tdewolff/minify/v2/html"
	"github.com/tdewolff/minify/v2/js"
	mjson "github.com/tdewolff/minify/v2/json"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	ghml "github.com/yuin/goldmark/renderer/html"
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

type InvertedIndex struct {
	mu    sync.RWMutex
	index map[string][]string // map[word][]postIDs
}

type Blog struct {
	posts         map[string]*Post
	postList      []*Post
	templates     *template.Template
	markdown      goldmark.Markdown
	invertedIndex *InvertedIndex
	Config        Config
	templatesFS   embed.FS
	staticFS      embed.FS
	blogFS        embed.FS
	minifier      *minify.M
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
			ghml.WithHardWraps(),
			ghml.WithXHTML(),
		),
	)

	templates, err := template.ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		log.Printf("Warning: Error loading templates: %v", err)
	}

	m := minify.New()
	m.AddFunc("text/css", css.Minify)
	m.AddFunc("text/html", mhtml.Minify)
	m.AddFunc("text/javascript", js.Minify)
	m.AddFunc("application/json", mjson.Minify)

	return &Blog{
		posts:         make(map[string]*Post),
		postList:      make([]*Post, 0),
		templates:     templates,
		markdown:      md,
		invertedIndex: &InvertedIndex{index: make(map[string][]string)},
		Config:        loadConfig(),
		templatesFS:   templatesFS,
		staticFS:      staticFS,
		blogFS:        blogFS,
		minifier:      m,
	}, nil
}

func loadConfig() Config {
	file, err := os.ReadFile("config.yaml")
	if err != nil {
		log.Printf("Warning: Could not read config.yaml, using defaults: %v", err)
		return Config{
			BlogName:     "Cenk Corapci",
			Introduction: "Hello 👋. I'm Cenk. A data engineer living in the Netherlands.",
		}
	}

	var config Config
	if err := yaml.Unmarshal(file, &config); err != nil {
		log.Printf("Warning: Could not parse config.yaml, using defaults: %v", err)
		return Config{
			BlogName:     "Cenk Corapci",
			Introduction: "Hello 👋. I'm Cenk. A data engineer living in the Netherlands.",
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

func (b *Blog) Export(distDir string) {
	os.RemoveAll(distDir)
	os.MkdirAll(distDir, 0755)

	exportHTML := func(filename string, templateName string, data interface{}) {
		var buf bytes.Buffer
		_ = b.templates.ExecuteTemplate(&buf, templateName, data)
		minified, _ := b.minifier.Bytes("text/html", buf.Bytes())
		_ = os.WriteFile(filepath.Join(distDir, filename), minified, 0644)
	}

	// Export Home
	data := map[string]interface{}{
		"Title":      "Home",
		"Posts":      b.postList,
		"Config":     b.Config,
		"StaticMode": true,
	}
	exportHTML("index.html", "index.html", data)

	// Export Search Page
	os.MkdirAll(filepath.Join(distDir, "search"), 0755)
	searchData := map[string]interface{}{
		"Title":      "Search Results",
		"Query":      "",
		"Posts":      nil,
		"Config":     b.Config,
		"StaticMode": true,
	}
	exportHTML("search/index.html", "search.html", searchData)

	// Export Posts
	os.MkdirAll(filepath.Join(distDir, "post"), 0755)
	for slug, post := range b.posts {
		os.MkdirAll(filepath.Join(distDir, "post", slug), 0755)
		postData := map[string]interface{}{
			"Title":      post.Title,
			"Post":       post,
			"Config":     b.Config,
			"StaticMode": true,
		}
		exportHTML("post/"+slug+"/index.html", "post.html", postData)
	}

	// Export Static Files
	os.MkdirAll(filepath.Join(distDir, "static"), 0755)
	entries, _ := b.staticFS.ReadDir("static")
	for _, entry := range entries {
		path := "static/" + entry.Name()
		data, _ := b.staticFS.ReadFile(path)

		var minified []byte
		ext := filepath.Ext(entry.Name())
		switch ext {
		case ".css":
			minified, _ = b.minifier.Bytes("text/css", data)
		case ".js":
			minified, _ = b.minifier.Bytes("text/javascript", data)
		default:
			minified = data
		}
		os.WriteFile(filepath.Join(distDir, "static", entry.Name()), minified, 0644)
	}

	// Export Search Index
	var indexPosts []struct {
		ID    string   `json:"id"`
		Title string   `json:"title"`
		Date  string   `json:"date"`
		Tags  []string `json:"tags"`
		Slug  string   `json:"slug"`
	}
	for _, post := range b.postList {
		tags := post.Tags
		if tags == nil {
			tags = []string{}
		}
		indexPosts = append(indexPosts, struct {
			ID    string   `json:"id"`
			Title string   `json:"title"`
			Date  string   `json:"date"`
			Tags  []string `json:"tags"`
			Slug  string   `json:"slug"`
		}{
			ID:    post.ID,
			Title: post.Title,
			Date:  post.Date.Format("2006-01-02"),
			Tags:  tags,
			Slug:  post.Slug,
		})
	}

	b.invertedIndex.mu.RLock()
	invertedIndex := make(map[string][]string)
	for word, ids := range b.invertedIndex.index {
		invertedIndex[word] = ids
	}
	b.invertedIndex.mu.RUnlock()

	searchIndex := map[string]interface{}{
		"posts":         indexPosts,
		"invertedIndex": invertedIndex,
	}

	jsonData, _ := json.Marshal(searchIndex) // Minified JSON
	os.WriteFile(filepath.Join(distDir, "search-index.json"), jsonData, 0644)

	fmt.Printf("Successfully generated optimized static site in ./%s\n", distDir)
}
