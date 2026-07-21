package main

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestCustomerReplyGeneratorFromEnvDefaultsToSimulated(t *testing.T) {
	generator, err := customerReplyGeneratorFromEnv(context.Background(), func(string) string { return "" })
	if err != nil {
		t.Fatalf("customerReplyGeneratorFromEnv() error = %v", err)
	}
	if _, ok := generator.(simulatedCustomerReplyGenerator); !ok {
		t.Fatalf("customerReplyGeneratorFromEnv() type = %T, want simulatedCustomerReplyGenerator", generator)
	}
}

func TestCustomerReplyGeneratorFromEnvBuildsModelGenerator(t *testing.T) {
	tests := []struct {
		mode string
		want any
	}{
		{mode: "model", want: (*chatModelCustomerReplyGenerator)(nil)},
		{mode: "model_graph", want: (*chatModelGraphCustomerReplyGenerator)(nil)},
	}

	for _, test := range tests {
		t.Run(test.mode, func(t *testing.T) {
			values := map[string]string{
				"CUSTOMER_REPLY_MODE":    test.mode,
				"CUSTOMER_REPLY_TIMEOUT": "3s",
				"OPENAI_API_KEY":         "test-key",
				"OPENAI_MODEL":           "test-model",
				"OPENAI_BASE_URL":        "http://127.0.0.1:1/v1",
			}

			generator, err := customerReplyGeneratorFromEnv(context.Background(), func(key string) string {
				return values[key]
			})
			if err != nil {
				t.Fatalf("customerReplyGeneratorFromEnv() error = %v", err)
			}
			switch test.want.(type) {
			case *chatModelCustomerReplyGenerator:
				if _, ok := generator.(*chatModelCustomerReplyGenerator); !ok {
					t.Fatalf("customerReplyGeneratorFromEnv() type = %T, want *chatModelCustomerReplyGenerator", generator)
				}
			case *chatModelGraphCustomerReplyGenerator:
				if _, ok := generator.(*chatModelGraphCustomerReplyGenerator); !ok {
					t.Fatalf("customerReplyGeneratorFromEnv() type = %T, want *chatModelGraphCustomerReplyGenerator", generator)
				}
			}
		})
	}
}

func TestCustomerReplyGeneratorFromEnvRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		values map[string]string
		want   string
	}{
		{
			name:   "unsupported mode",
			values: map[string]string{"CUSTOMER_REPLY_MODE": "automatic"},
			want:   "unsupported CUSTOMER_REPLY_MODE",
		},
		{
			name:   "missing model settings",
			values: map[string]string{"CUSTOMER_REPLY_MODE": "model"},
			want:   "OPENAI_API_KEY and OPENAI_MODEL",
		},
		{
			name: "invalid timeout",
			values: map[string]string{
				"CUSTOMER_REPLY_MODE":    "model",
				"OPENAI_API_KEY":         "test-key",
				"OPENAI_MODEL":           "test-model",
				"CUSTOMER_REPLY_TIMEOUT": "0s",
			},
			want: "CUSTOMER_REPLY_TIMEOUT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := customerReplyGeneratorFromEnv(context.Background(), func(key string) string {
				return tt.values[key]
			})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("customerReplyGeneratorFromEnv() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestCustomerReplyTimeout(t *testing.T) {
	if timeout, err := customerReplyTimeout(""); err != nil || timeout != defaultCustomerReplyTimeout {
		t.Fatalf("customerReplyTimeout(empty) = %v, %v", timeout, err)
	}
	if timeout, err := customerReplyTimeout("2s"); err != nil || timeout != 2*time.Second {
		t.Fatalf("customerReplyTimeout(2s) = %v, %v", timeout, err)
	}
}
