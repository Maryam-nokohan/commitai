package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// retryDelays is a var so tests can shorten it.
var retryDelays = []time.Duration{1 * time.Second, 3 * time.Second}

const maxResponseBytes = 2 << 20

func postJSON(ctx context.Context, endpoint string, headers map[string]string, payload any, timeout time.Duration) (int, []byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, nil, fmt.Errorf("encode request: %w", err)
	}

	client := &http.Client{Timeout: timeout}
	var (
		lastStatus int
		lastBody   []byte
		lastErr    error
	)

	for attempt := 0; attempt <= len(retryDelays); attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return 0, nil, ctx.Err()
			case <-time.After(retryDelays[attempt-1]):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return 0, nil, fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		resp, err := client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return 0, nil, ctx.Err()
			}
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				return 0, nil, err
			}
			lastErr, lastStatus, lastBody = err, 0, nil
			continue
		}

		data, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
		resp.Body.Close()
		if readErr != nil {
			lastErr, lastStatus, lastBody = fmt.Errorf("read response: %w", readErr), 0, nil
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr, lastStatus, lastBody = nil, resp.StatusCode, data
			continue
		}
		return resp.StatusCode, data, nil
	}

	if lastErr != nil {
		return 0, nil, lastErr
	}
	return lastStatus, lastBody, nil
}

func apiError(provider string, status int, body []byte) error {
	msg := strings.TrimSpace(string(body))
	if r := []rune(msg); len(r) > 500 {
		msg = string(r[:500]) + "..."
	}
	hint := ""
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		hint = " - check your API key"
	case http.StatusNotFound:
		hint = " - check the model name and api_base"
	case http.StatusTooManyRequests:
		hint = " - rate limited or out of quota"
	}
	return fmt.Errorf("%s error (HTTP %d)%s: %s", provider, status, hint, msg)
}