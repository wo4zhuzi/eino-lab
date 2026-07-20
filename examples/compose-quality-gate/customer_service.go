package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var ErrEmptyCustomerQuestion = errors.New("customer question is empty")

const requiredRefundNotice = "退款到账时间以支付平台实际处理结果为准。"

// CustomerReplyGenerator 表示生成客服回复草稿的上游服务。
type CustomerReplyGenerator interface {
	Generate(ctx context.Context, question string) (string, error)
}

// simulatedCustomerReplyGenerator 使用确定性逻辑，保证示例无需外部服务即可运行。
type simulatedCustomerReplyGenerator struct{}

func (simulatedCustomerReplyGenerator) Generate(ctx context.Context, question string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	question = strings.TrimSpace(question)
	if question == "" {
		return "", ErrEmptyCustomerQuestion
	}

	return fmt.Sprintf("您好，关于“%s”，我们已收到您的问题，正在为您核实处理。", question), nil
}
