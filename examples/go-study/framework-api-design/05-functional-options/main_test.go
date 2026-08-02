package main

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNewModelClientUsesDefaults(t *testing.T) {
	client, err := NewModelClient("https://model.example.com")
	if err != nil {
		t.Fatalf("NewModelClient() error = %v, want nil", err)
	}
	if client.config.timeout != 10*time.Second {
		t.Fatalf("timeout = %s, want %s", client.config.timeout, 10*time.Second)
	}
	if client.config.maxRetries != 2 {
		t.Fatalf("maxRetries = %d, want 2", client.config.maxRetries)
	}
}

func TestOptionsOverrideSelectedDefaults(t *testing.T) {
	client, err := NewModelClient(
		"https://model.example.com",
		WithTimeout(30*time.Second),
		WithMaxRetries(5),
	)
	if err != nil {
		t.Fatalf("NewModelClient() error = %v, want nil", err)
	}
	if client.config.timeout != 30*time.Second || client.config.maxRetries != 5 {
		t.Fatalf("config = %#v, want timeout=30s and maxRetries=5", client.config)
	}
}

func TestInvalidOptionPreservesErrorChain(t *testing.T) {
	errInvalid := errors.New("测试配置非法")
	_, err := NewModelClient("https://model.example.com", func(*clientConfig) error {
		return errInvalid
	})
	if !errors.Is(err, errInvalid) {
		t.Fatalf("NewModelClient() error = %v, want wrapped %v", err, errInvalid)
	}
}

func TestNewModelClientRejectsNilOption(t *testing.T) {
	_, err := NewModelClient("https://model.example.com", nil)
	if err == nil || !strings.Contains(err.Error(), "配置项不能为空") {
		t.Fatalf("NewModelClient() error = %v, want nil option error", err)
	}
}

func TestOptionsRejectInvalidValues(t *testing.T) {
	tests := []struct {
		name   string
		option Option
		want   string
	}{
		{name: "zero timeout", option: WithTimeout(0), want: "超时时间必须大于 0"},
		{name: "negative retries", option: WithMaxRetries(-1), want: "最大重试次数不能小于 0"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewModelClient("https://model.example.com", test.option)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("NewModelClient() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestNewModelClientRejectsBlankEndpoint(t *testing.T) {
	_, err := NewModelClient("   ")
	if err == nil {
		t.Fatal("NewModelClient() error = nil, want blank endpoint error")
	}
}
