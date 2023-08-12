package main

import (
	"fmt"

	"golang.org/x/net/context"
)

// http_post_func makes a http post
func http_post_func(ctx context.Context, uri string) (context.Context, error) {
	fmt.Printf("http_post_func called with uri: %s\n", uri)

	return ctx, nil
}

func with_headers(ctx context.Context, headers map[string]string) (context.Context, error) {
	return ctx, nil
}

func with_body(ctx context.Context, body string) (context.Context, error) {
	return ctx, nil
}

func body_contains(ctx context.Context, contains string) (context.Context, error) {
	return ctx, nil
}

func return_status_code(ctx context.Context, code int) (context.Context, error) {
	return ctx, nil
}

func body(ctx context.Context) (context.Context, error) {
	return ctx, nil
}

func resources_are_created(ctx context.Context, r []string) (context.Context, error) {
	return ctx, nil
}

func script(ctx context.Context, path string) (context.Context, error) {
	return ctx, nil
}

func with_arguments(ctx context.Context, headers map[string]string) (context.Context, error) {
	return ctx, nil
}

func have_an_exit_code(ctx context.Context, code int) (context.Context, error) {
	return ctx, nil
}

func output(ctx context.Context, out string) (context.Context, error) {
	return ctx, nil
}

func and(ctx context.Context) (context.Context, error) {
	return ctx, nil
}
