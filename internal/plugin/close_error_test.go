package plugin

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type closeErrorBody struct {
	io.Reader
	closed bool
}

func (b *closeErrorBody) Close() error {
	b.closed = true
	return errors.New("close failed")
}

type closeErrorRoundTripper func(*http.Request) (*http.Response, error)

func (fn closeErrorRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestCreateTagIgnoresCloseErrorAfterAcceptedResponse(t *testing.T) {
	body := &closeErrorBody{Reader: strings.NewReader(`{"name":"v1.0.0"}`)}
	client := NewClient(Config{BaseURL: "https://bitbucket.example.test", Workspace: "workspace", RepoSlug: "repo"})
	client.http = &http.Client{Transport: closeErrorRoundTripper(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusCreated, Body: body, Header: make(http.Header)}, nil
	})}

	tag, err := client.CreateTag(context.Background(), "v1.0.0", "abc123", "")
	if err != nil {
		t.Fatalf("CreateTag() error = %v", err)
	}
	if tag.Name != "v1.0.0" || !body.closed {
		t.Fatalf("tag = %#v, body closed = %t", tag, body.closed)
	}
}
