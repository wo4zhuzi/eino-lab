package main

import "testing"

func TestMaxAmountRuleRemembersLimit(t *testing.T) {
	normalRiskRule := maxAmountRule(1000)
	vipRiskRule := maxAmountRule(5000)

	current := order{id: "order-001", amount: 3000}
	if err := normalRiskRule(current); err == nil {
		t.Fatal("normalRiskRule() error = nil, want limit error")
	}
	if err := vipRiskRule(current); err != nil {
		t.Fatalf("vipRiskRule() error = %v, want nil", err)
	}
}

func TestMaxAmountRuleAcceptsBoundaryValue(t *testing.T) {
	rule := maxAmountRule(1000)

	err := rule(order{id: "order-001", amount: 1000})
	if err != nil {
		t.Fatalf("rule() error = %v, want nil", err)
	}
}
