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
tags: test, go
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

	if len(post.Tags) != 2 || post.Tags[0] != "test" || post.Tags[1] != "go" {
		t.Errorf("Expected tags [test, go], got %v", post.Tags)
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
