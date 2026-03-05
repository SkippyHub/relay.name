package main

import (
	"io"
	"net/http"
	"net/url"
	"strings"
)

type httpProxy struct {
	target string
}

func (p *httpProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	targetURL, err := url.Parse(p.target)
	if err != nil {
		http.Error(w, "Invalid target URL", http.StatusBadRequest)
		return
	}

	// Create new request to target
	targetReq, err := http.NewRequest(r.Method, p.target, r.Body)
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	// Copy headers
	for key, values := range r.Header {
		for _, value := range values {
			targetReq.Header.Add(key, value)
		}
	}

	// Update URL components
	targetReq.URL.Scheme = targetURL.Scheme
	targetReq.URL.Host = targetURL.Host
	if r.URL.Path != "/" {
		// Append path if target doesn't end with /
		if strings.HasSuffix(targetURL.Path, "/") {
			targetReq.URL.Path = targetURL.Path + strings.TrimPrefix(r.URL.Path, "/")
		} else {
			targetReq.URL.Path = targetURL.Path + r.URL.Path
		}
	} else {
		targetReq.URL.Path = targetURL.Path
	}
	targetReq.URL.RawQuery = r.URL.RawQuery
	targetReq.Host = targetURL.Host

	// Remove hop-by-hop headers
	targetReq.Header.Del("Connection")
	targetReq.Header.Del("Keep-Alive")
	targetReq.Header.Del("Proxy-Authenticate")
	targetReq.Header.Del("Proxy-Authorization")
	targetReq.Header.Del("Te")
	targetReq.Header.Del("Trailers")
	targetReq.Header.Del("Transfer-Encoding")
	targetReq.Header.Del("Upgrade")

	// Create HTTP client
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Forward request
	resp, err := client.Do(targetReq)
	if err != nil {
		http.Error(w, "Failed to connect to target", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Set status code
	w.WriteHeader(resp.StatusCode)

	// Copy response body
	io.Copy(w, resp.Body)
}

