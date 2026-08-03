package reviewworkflow

import (
	"context"
	"fmt"
	"reflect"
)

// Inspection 是 Inspector 返回的领域判断。
type Inspection struct {
	Score  int
	Reason string
}

// Inspector 定义审核节点需要的外部能力。
// 生产应用可以在这里接入规则引擎、模型或远程审核服务。
type Inspector interface {
	Inspect(ctx context.Context, content string) (Inspection, error)
}

// Reviser 定义修订节点需要的外部能力。
type Reviser interface {
	Revise(ctx context.Context, content string) (string, error)
}

// Dependencies 集中声明构建工作流所需的外部依赖。
type Dependencies struct {
	Inspector Inspector
	Reviser   Reviser
}

func (d Dependencies) validate() error {
	if isNilDependency(d.Inspector) {
		return fmt.Errorf("%w: Inspector 不能为空", ErrInvalidDependencies)
	}
	if isNilDependency(d.Reviser) {
		return fmt.Errorf("%w: Reviser 不能为空", ErrInvalidDependencies)
	}
	return nil
}

func isNilDependency(value any) bool {
	if value == nil {
		return true
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}
