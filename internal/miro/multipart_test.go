package miro

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDoMultipartBodySendsRawContentTypeAndBody(t *testing.T) {
	var (
		gotMethod string
		gotCT     string
		gotBody   []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotCT = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"id":"new-1"}`))
	}))
	defer srv.Close()

	client := New(&Config{Token: "t", BaseURL: srv.URL})

	body := &MultipartBody{
		Body:        bytes.NewBuffer([]byte("--boundary--\r\nsomebytes\r\n--boundary----\r\n")),
		ContentType: "multipart/form-data; boundary=boundary",
	}
	var out map[string]any
	if err := client.Post(context.Background(), "/v2/boards/abc/images", body, &out); err != nil {
		t.Fatalf("Post: %v", err)
	}
	if gotMethod != "POST" {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotCT != "multipart/form-data; boundary=boundary" {
		t.Errorf("Content-Type = %q", gotCT)
	}
	if !strings.Contains(string(gotBody), "somebytes") {
		t.Errorf("server got body %q, missing somebytes", string(gotBody))
	}
	if out["id"] != "new-1" {
		t.Errorf("response decoded incorrectly: %+v", out)
	}
}

func TestDoMultipartBodyRejectsNilBuffer(t *testing.T) {
	t.Parallel()
	client := New(&Config{Token: "t", BaseURL: "http://example.invalid"})
	err := client.Post(context.Background(), "/v2/boards/abc/images", &MultipartBody{ContentType: "multipart/form-data"}, nil)
	if err == nil {
		t.Fatal("Post with nil MultipartBody.Body returned nil, want error")
	}
}
