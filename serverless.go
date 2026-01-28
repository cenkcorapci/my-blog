//go:build serverless

package main

import (
	"context"
	"embed"
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"
	"github.com/cenkcorapci/my-blog/internal/blog"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS

//go:embed blog/*
var blogFS embed.FS

var adapter *httpadapter.HandlerAdapter

func init() {
	b, err := blog.NewBlog(templatesFS, staticFS, blogFS)
	if err != nil {
		log.Fatalf("Error loading blog posts: %v", err)
	}

	mux := b.Router()
	adapter = httpadapter.New(mux)
}

func Handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	return adapter.ProxyWithContext(ctx, req)
}

func main() {
	lambda.Start(Handler)
}
