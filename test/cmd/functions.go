package main

import (
	"fmt"
	"strings"

	"golang.org/x/net/context"
)

// http_post_func makes a http post
func http_post_func(ctx context.Context, uri string) (context.Context, error) {
	body := ""
	headers := map[string]string{}

	if v := ctx.Value("headers"); v != nil {
		headers = v.(map[string]string)
	}

	if v := ctx.Value("body"); v != nil {
		body = v.(string)
	}

	fmt.Printf("     http_post_func called with uri: %s, headers: %#v, body: %s\n", uri, headers, body)

	// set the status code and the body for assertions
	ctx = context.WithValue(ctx, "response_body", "ok")
	ctx = context.WithValue(ctx, "response_code", 200)

	return ctx, nil
}

func with_headers(ctx context.Context, headers map[string]string) (context.Context, error) {
	// set the headers to the context
	ctx = context.WithValue(ctx, "headers", headers)
	return ctx, nil
}

func with_body(ctx context.Context, body string) (context.Context, error) {
	// set the headers to the context
	ctx = context.WithValue(ctx, "body", body)
	return ctx, nil
}

func body_contains(ctx context.Context, contains string) (context.Context, error) {
	body := ""

	if b := ctx.Value("response_body"); b != nil {
		body = b.(string)
	}

	if !strings.Contains(body, contains) {
		return ctx, fmt.Errorf("expected body '%s' to contain '%s'", body, contains)
	}

	return ctx, nil
}

func return_status_code(ctx context.Context, code int) (context.Context, error) {
	scode := 0

	if c := ctx.Value("response_code"); c != nil {
		scode = c.(int)
	}

	if code != scode {
		return ctx, fmt.Errorf("expected status code '%d' got '%d'", code, scode)
	}

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
