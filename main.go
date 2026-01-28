//go:build !serverless

package main

import (
	"embed"
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/cenkcorapci/my-blog/internal/blog"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS

//go:embed blog/*
var blogFS embed.FS

func main() {
	staticExport := flag.Bool("static", false, "Export the blog as a static site")
	flag.Parse()

	b, err := blog.NewBlog(templatesFS, staticFS, blogFS)
	if err != nil {
		log.Fatalf("Error initializing blog: %v", err)
	}

	// Load blog posts
	if err := b.LoadPosts(); err != nil {
		log.Fatal(err)
	}

	if *staticExport {
		b.ExportStatic("dist")
		return
	}

	mux := b.Router()

	// Get port from environment variable (default to 8080)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting server on port %s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal("Error starting server:", err)
	}
}
