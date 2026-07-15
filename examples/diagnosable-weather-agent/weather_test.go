package main

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type weatherProviderFunc func(ctx context.Context, city string) (Weather, error)

func (f weatherProviderFunc) Lookup(ctx context.Context, city string) (Weather, error) {
	return f(ctx, city)
}

func TestStaticWeatherProviderLookup(t *testing.T) {
	provider := NewStaticWeatherProvider()

	weather, err := provider.Lookup(context.Background(), " 北京 ")
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if weather.City != "Beijing" || weather.TemperatureC != 28 {
		t.Fatalf("Lookup() weather = %#v", weather)
	}
}

func TestStaticWeatherProviderUnsupportedCity(t *testing.T) {
	_, err := NewStaticWeatherProvider().Lookup(context.Background(), "Guangzhou")
	if !errors.Is(err, ErrUnsupportedCity) {
		t.Fatalf("Lookup() error = %v, want ErrUnsupportedCity", err)
	}
}

func TestStaticWeatherProviderHonorsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := NewStaticWeatherProvider().Lookup(ctx, "Beijing")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Lookup() error = %v, want context.Canceled", err)
	}
}

func TestWeatherToolSuccess(t *testing.T) {
	weatherTool, err := NewWeatherTool(NewStaticWeatherProvider())
	if err != nil {
		t.Fatalf("NewWeatherTool() error = %v", err)
	}

	output, err := weatherTool.InvokableRun(context.Background(), `{"city":"Shanghai"}`)
	if err != nil {
		t.Fatalf("InvokableRun() error = %v", err)
	}
	var weather Weather
	if err := json.Unmarshal([]byte(output), &weather); err != nil {
		t.Fatalf("decode tool output: %v", err)
	}
	if weather.City != "Shanghai" || weather.Condition != "Cloudy" {
		t.Fatalf("InvokableRun() weather = %#v", weather)
	}
}

func TestWeatherToolErrors(t *testing.T) {
	tests := []struct {
		name     string
		provider WeatherProvider
		input    string
		want     error
	}{
		{
			name:     "empty city",
			provider: NewStaticWeatherProvider(),
			input:    `{"city":""}`,
			want:     ErrUnsupportedCity,
		},
		{
			name: "provider unavailable",
			provider: weatherProviderFunc(func(context.Context, string) (Weather, error) {
				return Weather{}, ErrWeatherUnavailable
			}),
			input: `{"city":"Beijing"}`,
			want:  ErrWeatherUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			weatherTool, err := NewWeatherTool(tt.provider)
			if err != nil {
				t.Fatalf("NewWeatherTool() error = %v", err)
			}
			_, err = weatherTool.InvokableRun(context.Background(), tt.input)
			if !errors.Is(err, tt.want) {
				t.Fatalf("InvokableRun() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestNewWeatherToolRequiresProvider(t *testing.T) {
	_, err := NewWeatherTool(nil)
	if err == nil {
		t.Fatal("NewWeatherTool() error = nil, want non-nil")
	}
}
