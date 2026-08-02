package main

import "testing"

func TestRiskNodeUsesConfiguredRule(t *testing.T) {
	current := order{amount: 3000}

	normalNode := riskNode{rule: normalRiskRule}
	if err := normalNode.Check(current); err == nil {
		t.Fatal("normalNode.Check() error = nil, want risk error")
	}

	vipNode := riskNode{rule: vipRiskRule}
	if err := vipNode.Check(current); err != nil {
		t.Fatalf("vipNode.Check() error = %v, want nil", err)
	}
}

func TestRiskNodeRejectsMissingRule(t *testing.T) {
	err := (riskNode{}).Check(order{amount: 100})
	if err == nil {
		t.Fatal("riskNode.Check() error = nil, want missing rule error")
	}
}
