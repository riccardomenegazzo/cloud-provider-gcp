/*
Copyright 2026 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package credentialconfig

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestReadDockerConfigFileFromURL(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr bool
	}{
		{name: "valid config", status: http.StatusOK, body: `{"registry.example.com":{"username":"user","password":"password"}}`},
		{name: "invalid JSON", status: http.StatusOK, body: `{`, wantErr: true},
		{name: "not found", status: http.StatusNotFound, body: "not found", wantErr: true},
		{name: "server error", status: http.StatusInternalServerError, body: "server error", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := r.Header.Get("Metadata-Flavor"); got != "Google" {
					t.Errorf("Metadata-Flavor header = %q, want Google", got)
				}
				w.WriteHeader(tt.status)
				_, _ = io.WriteString(w, tt.body)
			}))
			defer server.Close()
			header := http.Header{"Metadata-Flavor": []string{"Google"}}
			cfg, err := ReadDockerConfigFileFromURL(server.URL, server.Client(), &header)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error, got nil")
				}
				if cfg != nil {
					t.Errorf("expected nil config on error, got %#v", cfg)
				}
				if tt.status != http.StatusOK {
					var httpErr *HTTPError
					if !errors.As(err, &httpErr) {
						t.Fatalf("expected HTTPError, got %T: %v", err, err)
					}
					if httpErr.StatusCode != tt.status || httpErr.URL != server.URL {
						t.Errorf("HTTPError = %#v, want status %d and URL %q", httpErr, tt.status, server.URL)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			want := DockerConfig{"registry.example.com": {Username: "user", Password: "password"}}
			if !reflect.DeepEqual(cfg, want) {
				t.Errorf("config = %#v, want %#v", cfg, want)
			}
		})
	}
}

type failingConfigTransport struct {
	err error
}

func (f failingConfigTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, f.err
}

func TestReadDockerConfigFileFromURLTransportError(t *testing.T) {
	wantErr := errors.New("connection failed")
	client := &http.Client{Transport: failingConfigTransport{err: wantErr}}
	cfg, err := ReadDockerConfigFileFromURL("http://registry.example.com/config", client, nil)
	if !errors.Is(err, wantErr) {
		t.Errorf("error = %v, want %v", err, wantErr)
	}
	if cfg != nil {
		t.Errorf("expected nil config on error, got %#v", cfg)
	}
}
