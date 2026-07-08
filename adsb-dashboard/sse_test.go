package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBrokerSubscribeBroadcast(t *testing.T) {
	b := NewBroker()
	ch := b.Subscribe()
	defer b.Unsubscribe(ch)

	msg := []byte(`{"test": true}`)
	b.Broadcast(msg)

	select {
	case received := <-ch:
		if string(received) != string(msg) {
			t.Errorf("expected %s, got %s", msg, received)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for message")
	}
}

func TestBrokerUnsubscribe(t *testing.T) {
	b := NewBroker()
	ch := b.Subscribe()
	b.Unsubscribe(ch)

	// Should not panic or hang
	b.Broadcast([]byte(`{"test": true}`))
}

func TestBrokerServeHTTP(t *testing.T) {
	b := NewBroker()

	go func() {
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			b.Broadcast([]byte(`{"test": true}`))
		}
	}()

	server := httptest.NewServer(http.HandlerFunc(b.ServeHTTP))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("SSE request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)
	if !strings.Contains(bodyStr, "data:") {
		t.Errorf("expected SSE data:, got: %s", bodyStr)
	}
	if !strings.Contains(bodyStr, `{"test": true}`) {
		t.Errorf("expected JSON in SSE, got: %s", bodyStr)
	}
}
