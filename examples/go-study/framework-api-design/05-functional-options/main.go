package main

import (
	"fmt"
	"strings"
	"time"
)

type clientConfig struct {
	timeout    time.Duration
	maxRetries int
}

// Option 表示一项可选配置。返回错误可以在构造阶段拒绝非法参数。
type Option func(*clientConfig) error

func WithTimeout(timeout time.Duration) Option {
	return func(config *clientConfig) error {
		if timeout <= 0 {
			return fmt.Errorf("超时时间必须大于 0")
		}
		config.timeout = timeout
		return nil
	}
}

func WithMaxRetries(maxRetries int) Option {
	return func(config *clientConfig) error {
		if maxRetries < 0 {
			return fmt.Errorf("最大重试次数不能小于 0")
		}
		config.maxRetries = maxRetries
		return nil
	}
}

type modelClient struct {
	endpoint string
	config   clientConfig
}

func NewModelClient(endpoint string, options ...Option) (*modelClient, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("模型服务地址不能为空")
	}

	config := clientConfig{
		timeout:    10 * time.Second,
		maxRetries: 2,
	}
	for index, option := range options {
		if option == nil {
			return nil, fmt.Errorf("第 %d 个配置项不能为空", index+1)
		}
		if err := option(&config); err != nil {
			return nil, fmt.Errorf("应用第 %d 个配置项失败: %w", index+1, err)
		}
	}

	return &modelClient{endpoint: endpoint, config: config}, nil
}

func main() {
	defaultClient, err := NewModelClient("https://model.example.com")
	if err != nil {
		panic(err)
	}

	batchClient, err := NewModelClient(
		"https://model.example.com",
		WithTimeout(30*time.Second),
		WithMaxRetries(5),
	)
	if err != nil {
		panic(err)
	}

	fmt.Printf("default timeout=%s retries=%d\n", defaultClient.config.timeout, defaultClient.config.maxRetries)
	fmt.Printf("batch timeout=%s retries=%d\n", batchClient.config.timeout, batchClient.config.maxRetries)
}
