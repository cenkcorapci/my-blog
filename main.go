//go:build !serverless

package main

import (
	"embed"
	"flag"
	"log"
	"net/http"

	"github.com/cenkcorapci/my-blog/internal/blog"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS

//go:embed blog/*
var blogFS embed.FS

func main() {
	serve := flag.Bool("serve", false, "Serve the generated site locally")
	distDir := flag.String("dist", "dist", "Directory to output the static site")
	port := flag.String("port", "8080", "Port to serve on (only used with -serve)")
	flag.Parse()

	b, err := blog.NewBlog(templatesFS, staticFS, blogFS)
	if err != nil {
		log.Fatalf("Error initializing blog: %v", err)
	}

	// Always load posts and generate the site
	if err := b.LoadPosts(); err != nil {
		log.Fatal(err)
	}

	b.Export(*distDir)

	if *serve {
		log.Printf("Serving %s on http://localhost:%s", *distDir, *port)
		err := http.ListenAndServe(":"+*port, http.FileServer(http.Dir(*distDir)))
		if err != nil {
			log.Fatal(err)
		}
	}
}
