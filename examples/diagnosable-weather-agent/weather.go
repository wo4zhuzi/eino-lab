package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

const weatherToolName = "weather_lookup"

var (
	ErrUnsupportedCity    = errors.New("unsupported city")
	ErrWeatherUnavailable = errors.New("weather service unavailable")
)

type Weather struct {
	City            string `json:"city"`
	Condition       string `json:"condition"`
	TemperatureC    int    `json:"temperature_c"`
	HumidityPercent int    `json:"humidity_percent"`
}

type WeatherRequest struct {
	City string `json:"city" jsonschema:"required,description=City name such as Beijing Shanghai or Shenzhen"`
}

type WeatherProvider interface {
	Lookup(ctx context.Context, city string) (Weather, error)
}

type StaticWeatherProvider struct {
	weatherByCity map[string]Weather
	aliases       map[string]string
}

func NewStaticWeatherProvider() *StaticWeatherProvider {
	return &StaticWeatherProvider{
		weatherByCity: map[string]Weather{
			"beijing": {
				City:            "Beijing",
				Condition:       "Sunny",
				TemperatureC:    28,
				HumidityPercent: 35,
			},
			"shanghai": {
				City:            "Shanghai",
				Condition:       "Cloudy",
				TemperatureC:    26,
				HumidityPercent: 72,
			},
			"shenzhen": {
				City:            "Shenzhen",
				Condition:       "Rainy",
				TemperatureC:    30,
				HumidityPercent: 84,
			},
		},
		aliases: map[string]string{
			"北京": "beijing",
			"上海": "shanghai",
			"深圳": "shenzhen",
		},
	}
}

func (p *StaticWeatherProvider) Lookup(ctx context.Context, city string) (Weather, error) {
	if err := ctx.Err(); err != nil {
		return Weather{}, fmt.Errorf("weather provider context: %w", err)
	}

	key := strings.ToLower(strings.TrimSpace(city))
	if alias, ok := p.aliases[key]; ok {
		key = alias
	}
	weather, ok := p.weatherByCity[key]
	if !ok {
		return Weather{}, fmt.Errorf("%w: %s", ErrUnsupportedCity, strings.TrimSpace(city))
	}
	return weather, nil
}

func NewWeatherTool(provider WeatherProvider) (tool.InvokableTool, error) {
	if provider == nil {
		return nil, errors.New("weather provider is required")
	}

	return utils.InferTool(
		weatherToolName,
		"Look up current weather for Beijing, Shanghai, or Shenzhen.",
		func(ctx context.Context, input WeatherRequest) (Weather, error) {
			if strings.TrimSpace(input.City) == "" {
				return Weather{}, fmt.Errorf("weather_lookup input: %w", ErrUnsupportedCity)
			}
			weather, err := provider.Lookup(ctx, input.City)
			if err != nil {
				return Weather{}, fmt.Errorf("weather_lookup provider: %w", err)
			}
			return weather, nil
		},
	)
}
