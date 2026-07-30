package main

import (
	"errors"
	"testing"
)

func TestCheckOrderUsesProvidedRule(t *testing.T) {
	current := order{id: "order-001", amount: 3000}

	if err := checkOrder(current, normalRiskRule); err == nil {
		t.Fatal("checkOrder(normalRiskRule) error = nil, want risk error")
	}
	if err := checkOrder(current, vipRiskRule); err != nil {
		t.Fatalf("checkOrder(vipRiskRule) error = %v, want nil", err)
	}
}

func TestCheckOrderRunsCommonValidationBeforeRule(t *testing.T) {
	ruleCalled := false
	rule := RiskRule(func(order) error {
		ruleCalled = true
		return nil
	})

	err := checkOrder(order{amount: 100}, rule)
	if err == nil {
		t.Fatal("checkOrder() error = nil, want missing ID error")
	}
	if ruleCalled {
		t.Fatal("rule was called before common validation passed")
	}
}

func TestCheckOrderPreservesRuleError(t *testing.T) {
	errRiskRejected := errors.New("risk rejected")
	rule := RiskRule(func(order) error {
		return errRiskRejected
	})

	err := checkOrder(order{id: "order-001", amount: 100}, rule)
	if !errors.Is(err, errRiskRejected) {
		t.Fatalf("checkOrder() error = %v, want wrapped %v", err, errRiskRejected)
	}
}
