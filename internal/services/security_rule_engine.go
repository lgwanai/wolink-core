package services

import (
	"context"
	"fmt"
	"sync"

	"github.com/expr-lang/expr"
	"github.com/sirupsen/logrus"
)

// RuleAction defines the action to take when a rule matches
type RuleAction string

const (
	ActionBlock RuleAction = "block"
	ActionAlert RuleAction = "alert"
	ActionMask  RuleAction = "mask"
)

// Rule represents a security rule
type Rule struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Conditions string     `json:"conditions"` // expr expression
	Action     RuleAction `json:"action"`
	Enabled    bool       `json:"enabled"`
	Priority   int        `json:"priority"` // Lower = higher priority
}

// UserContext provides user information for rule evaluation
type UserContext struct {
	ID           string `json:"id"`
	DepartmentID int    `json:"department_id"`
	Role         string `json:"role"`
}

// EvaluationContext provides context for rule evaluation
type EvaluationContext struct {
	Prompt string      `json:"prompt"`
	User   UserContext `json:"user"`
}

// RuleEngine evaluates security rules against prompts
type RuleEngine struct {
	rules  []*Rule
	mu     sync.RWMutex
	logger *logrus.Logger
}

var (
	engineOnce sync.Once
	engine     *RuleEngine
)

// NewRuleEngine creates a rule engine with default rules
func NewRuleEngine(logger *logrus.Logger) *RuleEngine {
	engineOnce.Do(func() {
		engine = &RuleEngine{
			rules:  make([]*Rule, 0),
			logger: logger,
		}
	})
	return engine
}

// LoadRules replaces all rules with the provided set
func (e *RuleEngine) LoadRules(rules []*Rule) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rules = rules
}

// AddRule adds a new rule to the engine
func (e *RuleEngine) AddRule(rule *Rule) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rules = append(e.rules, rule)
}

// Evaluate checks all rules against the context, returns first matching rule
func (e *RuleEngine) Evaluate(ctx context.Context, evalCtx EvaluationContext) (*Rule, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, rule := range e.rules {
		if !rule.Enabled {
			continue
		}

		// Build environment for expr evaluation
		env := map[string]interface{}{
			"prompt": evalCtx.Prompt,
			"user": map[string]interface{}{
				"id":             evalCtx.User.ID,
				"department_id":  evalCtx.User.DepartmentID,
				"role":           evalCtx.User.Role,
			},
			"len": func(s string) int { return len(s) },
		}

		// Compile and evaluate the expression
		program, err := expr.Compile(rule.Conditions)
		if err != nil {
			e.logger.WithError(err).WithField("rule", rule.ID).Error("Failed to compile rule conditions")
			continue
		}

		output, err := expr.Run(program, env)
		if err != nil {
			e.logger.WithError(err).WithField("rule", rule.ID).Error("Failed to evaluate rule")
			continue
		}

		// Check if the rule matched
		if matched, ok := output.(bool); ok && matched {
			e.logger.WithFields(logrus.Fields{
				"rule_id":     rule.ID,
				"rule_name":   rule.Name,
				"action":      rule.Action,
				"user_id":     evalCtx.User.ID,
				"prompt_len":  len(evalCtx.Prompt),
			}).Info("Security rule matched")
			return rule, nil
		}
	}

	return nil, nil
}

// ExecuteAction executes the rule's action
func (e *RuleEngine) ExecuteAction(ctx context.Context, rule *Rule, evalCtx EvaluationContext) error {
	switch rule.Action {
	case ActionBlock:
		e.logger.WithFields(logrus.Fields{
			"rule_id": rule.ID,
			"user_id": evalCtx.User.ID,
		}).Warn("Blocking request due to security rule")
		return fmt.Errorf("request blocked by security rule: %s", rule.Name)

	case ActionAlert:
		e.logger.WithFields(logrus.Fields{
			"rule_id":    rule.ID,
			"rule_name":  rule.Name,
			"user_id":    evalCtx.User.ID,
			"prompt_len": len(evalCtx.Prompt),
		}).Warn("Security alert triggered")
		return nil // Alert doesn't block, just logs

	case ActionMask:
		// Caller should handle masking (they have the detector)
		return nil

	default:
		return fmt.Errorf("unknown action: %s", rule.Action)
	}
}

// TestRule evaluates a rule against sample input (for testing)
func (e *RuleEngine) TestRule(rule *Rule, evalCtx EvaluationContext) (bool, error) {
	env := map[string]interface{}{
		"prompt": evalCtx.Prompt,
		"user": map[string]interface{}{
			"id":            evalCtx.User.ID,
			"department_id": evalCtx.User.DepartmentID,
			"role":          evalCtx.User.Role,
		},
		"len": func(s string) int { return len(s) },
	}

	program, err := expr.Compile(rule.Conditions)
	if err != nil {
		return false, fmt.Errorf("failed to compile rule: %w", err)
	}

	output, err := expr.Run(program, env)
	if err != nil {
		return false, fmt.Errorf("failed to evaluate rule: %w", err)
	}

	matched, ok := output.(bool)
	if !ok {
		return false, fmt.Errorf("rule did not return boolean")
	}

	return matched, nil
}

// GetAllRules returns all rules (for listing)
func (e *RuleEngine) GetAllRules() []*Rule {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]*Rule, len(e.rules))
	copy(result, e.rules)
	return result
}
