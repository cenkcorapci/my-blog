package blog

import (
	"embed"
	"strings"
	"testing"
	"time"
)

func TestParsePost(t *testing.T) {
	blog, _ := NewBlog(embed.FS{}, embed.FS{}, embed.FS{})
	content := `---
title: Test Post
date: 2024-01-27
---
# Hello World
This is a test post.`

	post, err := blog.parsePost("test-post.md", content)
	if err != nil {
		t.Fatalf("Failed to parse post: %v", err)
	}

	if post.Title != "Test Post" {
		t.Errorf("Expected title 'Test Post', got '%s'", post.Title)
	}

	expectedDate, _ := time.Parse("2006-01-02", "2024-01-27")
	if !post.Date.Equal(expectedDate) {
		t.Errorf("Expected date %v, got %v", expectedDate, post.Date)
	}

	if !strings.Contains(string(post.HTMLContent), "Hello World</h1>") {
		t.Errorf("Expected HTML content to contain Hello World</h1>, got %s", post.HTMLContent)
	}

	if post.Slug != "test-post" {
		t.Errorf("Expected slug 'test-post', got '%s'", post.Slug)
	}
}

func TestSearch(t *testing.T) {
	blog, _ := NewBlog(embed.FS{}, embed.FS{}, embed.FS{})

	p1 := &Post{ID: "p1", Title: "Go Programming", Content: "Go is great", Date: time.Now()}
	p2 := &Post{ID: "p2", Title: "Python Guide", Content: "Python is also good", Date: time.Now().Add(-time.Hour)}

	blog.posts["p1"] = p1
	blog.posts["p2"] = p2
	blog.postList = []*Post{p1, p2}
	blog.buildInvertedIndex()

	tests := []struct {
		name     string
		query    string
		expected int
	}{
		{"Single word match", "Go", 1},
		{"Case insensitive", "python", 1},
		{"No match", "Rust", 0},
		{"Multiple words", "Go Programming", 1},
		{"Empty query", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := blog.search(tt.query)
			if len(results) != tt.expected {
				t.Errorf("For query '%s', expected %d results, got %d", tt.query, tt.expected, len(results))
			}
		})
	}
}

func TestTokenize(t *testing.T) {
	text := "Hello, World! 123"
	tokens := tokenize(text)
	expected := []string{"Hello", "World", "123"}

	if len(tokens) != len(expected) {
		t.Fatalf("Expected %d tokens, got %d", len(expected), len(tokens))
	}

	for i, v := range tokens {
		if v != expected[i] {
			t.Errorf("At index %d, expected %s, got %s", i, expected[i], v)
		}
	}
}

func TestIntersection(t *testing.T) {
	a := []string{"1", "2", "3"}
	b := []string{"2", "3", "4"}
	result := intersection(a, b)

	if len(result) != 2 {
		t.Errorf("Expected intersection size 2, got %d", len(result))
	}

	matches := 0
	for _, v := range result {
		if v == "2" || v == "3" {
			matches++
		}
	}
	if matches != 2 {
		t.Errorf("Intersection results incorrect: %v", result)
	}
}
