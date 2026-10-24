package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

type client struct {
	base string
	http *http.Client
}

// result é uma resposta da API (status + corpo) ou um erro de rede/timeout/cliente.
type result struct {
	status int
	body   []byte
	dur    time.Duration
	err    error
}

func newClient(base string, timeout time.Duration, conns int) *client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// Uma conexão ociosa por worker: sem isso o cliente abre e fecha conexões o tempo todo e o
	// teste passa a medir handshake TCP em vez da API.
	transport.MaxIdleConns = conns
	transport.MaxIdleConnsPerHost = conns
	return &client{base: base, http: &http.Client{Timeout: timeout, Transport: transport}}
}

func (c *client) do(ctx context.Context, method, path, bearer string, body any) result {
	var reader io.Reader
	if body != nil {
		switch b := body.(type) {
		case []byte:
			reader = bytes.NewReader(b)
		default:
			encoded, err := json.Marshal(b)
			if err != nil {
				return result{err: err}
			}
			reader = bytes.NewReader(encoded)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, c.base+path, reader)
	if err != nil {
		return result{err: err}
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}

	start := time.Now()
	res, err := c.http.Do(req)
	if err != nil {
		return result{err: err, dur: time.Since(start)}
	}
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	return result{status: res.StatusCode, body: data, dur: time.Since(start), err: err}
}

func (c *client) post(ctx context.Context, path, bearer string, body any) result {
	return c.do(ctx, http.MethodPost, path, bearer, body)
}

func (c *client) login(ctx context.Context, email, password string) result {
	return c.post(ctx, "/v1/auth/login", "", map[string]string{"email": email, "password": password})
}

// errorCode extrai o código de erro do corpo padrão da API: {"error":{"code":"..."}}.
func errorCode(body []byte) string {
	var e struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &e) != nil {
		return ""
	}
	return e.Error.Code
}
