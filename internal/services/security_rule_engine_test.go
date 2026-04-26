package services

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestRuleEngine_Evaluate(t *testing.T) {
	logger := logrus.New()
	engine := NewRuleEngine(logger)

	// Test rules
	rules := []*Rule{
		{
			ID:         "rule-001",
			Name:       "Block salary keywords",
			Conditions: `prompt contains "salary"`,
			Action:     ActionBlock,
			Enabled:    true,
			Priority:   1,
		},
		{
			ID:         "rule-002",
			Name:       "Block specific user",
			Conditions: `user.id == "malicious-user-123"`,
			Action:     ActionBlock,
			Enabled:    true,
			Priority:   2,
		},
		{
			ID:         "rule-003",
			Name:       "Block department list",
			Conditions: `user.department_id in [999, 888]`,
			Action:     ActionBlock,
			Enabled:    true,
			Priority:   3,
		},
		{
			ID:         "rule-004",
			Name:       "Alert on long prompts",
			Conditions:  `len(prompt) > 1000`,
			Action:     ActionAlert,
			Enabled:    true,
			Priority:   4,
		},
	}
	engine.LoadRules(rules)

	tests := []struct {
		name          string
		evalCtx       EvaluationContext
		expectMatch   bool
		expectedRule  string
	}{
		{
			name: "prompt contains salary keyword",
			evalCtx: EvaluationContext{
				Prompt: "What is the salary for this position?",
				User:   UserContext{ID: "user-1", DepartmentID: 1},
			},
			expectMatch:  true,
			expectedRule: "rule-001",
		},
		{
			name: "malicious user blocked",
			evalCtx: EvaluationContext{
				Prompt: "Hello world",
				User:   UserContext{ID: "malicious-user-123", DepartmentID: 1},
			},
			expectMatch:  true,
			expectedRule: "rule-002",
		},
		{
			name: "department blocked",
			evalCtx: EvaluationContext{
				Prompt: "Normal prompt",
				User:   UserContext{ID: "user-2", DepartmentID: 999},
			},
			expectMatch:  true,
			expectedRule: "rule-003",
		},
		{
			name: "long prompt triggers alert",
			evalCtx: EvaluationContext{
				Prompt: string(make([]byte, 1100)),
				User:   UserContext{ID: "user-3", DepartmentID: 1},
			},
			expectMatch:  true,
			expectedRule: "rule-004",
		},
		{
			name: "no rules matched",
			evalCtx: EvaluationContext{
				Prompt: "Hello world",
				User:   UserContext{ID: "user-4", DepartmentID: 1},
			},
			expectMatch:  false,
			expectedRule: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule, err := engine.Evaluate(context.Background(), tt.evalCtx)
			if err != nil {
				t.Errorf("Evaluate failed: %v", err)
				return
			}
			if tt.expectMatch {
				if rule == nil {
					t.Errorf("Expected rule match, got none")
					return
				}
				if rule.ID != tt.expectedRule {
					t.Errorf("Expected rule %s, got %s", tt.expectedRule, rule.ID)
				}
			} else {
				if rule != nil {
					t.Errorf("Expected no match, got rule %s", rule.ID)
				}
			}
		})
	}
}

func TestRuleEngine_ExecuteAction_Block(t *testing.T) {
	logger := logrus.New()
	engine := NewRuleEngine(logger)

	rule := &Rule{
		ID:         "test-block",
		Name:       "Test Block",
		Conditions: `prompt contains "block"`,
		Action:     ActionBlock,
		Enabled:    true,
	}

	evalCtx := EvaluationContext{
		Prompt: "This should be blocked",
		User:   UserContext{ID: "user-1", DepartmentID: 1},
	}

	err := engine.ExecuteAction(context.Background(), rule, evalCtx)
	if err == nil {
		t.Errorf("Expected error for block action, got nil")
	}
}

func TestRuleEngine_ExecuteAction_Alert(t *testing.T) {
	logger := logrus.New()
	engine := NewRuleEngine(logger)

	rule := &Rule{
		ID:         "test-alert",
		Name:       "Test Alert",
		Conditions: `prompt contains "alert"`,
		Action:     ActionAlert,
		Enabled:    true,
	}

	evalCtx := EvaluationContext{
		Prompt: "This triggers an alert",
		User:   UserContext{ID: "user-1", DepartmentID: 1},
	}

	err := engine.ExecuteAction(context.Background(), rule, evalCtx)
	if err != nil {
		t.Errorf("Expected no error for alert action, got %v", err)
	}
}

func TestRuleEngine_TestRule(t *testing.T) {
	logger := logrus.New()
	engine := NewRuleEngine(logger)

	rule := &Rule{
		ID:         "test-rule",
		Name:       "Test Rule",
		Conditions: `prompt contains "test"`,
		Action:     ActionBlock,
		Enabled:    true,
	}

	tests := []struct {
		name        string
		prompt      string
		expectMatch bool
	}{
		{
			name:        "prompt contains test",
			prompt:      "This is a test prompt",
			expectMatch: true,
		},
		{
			name:        "prompt does not contain test",
			prompt:      "Hello world",
			expectMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evalCtx := EvaluationContext{
				Prompt: tt.prompt,
				User:   UserContext{ID: "user-1", DepartmentID: 1},
			}
			matched, err := engine.TestRule(rule, evalCtx)
			if err != nil {
				t.Errorf("TestRule failed: %v", err)
				return
			}
			if matched != tt.expectMatch {
				t.Errorf("Expected %v, got %v", tt.expectMatch, matched)
			}
		})
	}
}

func TestRuleEngine_MultipleRules_FirstMatchWins(t *testing.T) {
	logger := logrus.New()
	engine := NewRuleEngine(logger)

	// Add rules in specific order
	engine.LoadRules([]*Rule{
		{
			ID:         "rule-first",
			Name:       "First Rule",
			Conditions: `prompt contains "test"`,
			Action:     ActionAlert,
			Enabled:    true,
			Priority:   1,
		},
		{
			ID:         "rule-second",
			Name:       "Second Rule",
			Conditions: `prompt contains "test"`,
			Action:     ActionBlock,
			Enabled:    true,
			Priority:   2,
		},
	})

	evalCtx := EvaluationContext{
		Prompt: "This is a test prompt",
		User:   UserContext{ID: "user-1", DepartmentID: 1},
	}

	rule, err := engine.Evaluate(context.Background(), evalCtx)
	if err != nil {
		t.Errorf("Evaluate failed: %v", err)
		return
	}
	if rule == nil {
		t.Errorf("Expected rule match, got none")
		return
	}
	if rule.ID != "rule-first" {
		t.Errorf("Expected rule-first, got %s", rule.ID)
	}
}
