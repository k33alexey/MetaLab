package metadata

import (
	"fmt"
	"unicode/utf8"
)

// Limits apply to decoded JSON as well as bounded YAML sources.
const MaxPolicyTemplates = 100
const MaxPolicyValues = 100

// PolicyOperator compares the restricted field with the rule operand.
type PolicyOperator string

const (
	PolicyEqual    PolicyOperator = "eq"
	PolicyNotEqual PolicyOperator = "ne"
	PolicyIn       PolicyOperator = "in"
	PolicyNotIn    PolicyOperator = "not-in"
)

// PolicyRule restricts rows by comparing one field of the object with literal
// values or with a session parameter. Rules are declarative on purpose: unlike
// hand-written condition text they are validated when the project is published,
// so a broken restriction cannot reach a working database.
//
// Exactly one operand is set: Values for literals, Parameter for a session
// parameter reference. PolicyEqual and PolicyNotEqual take a single value;
// PolicyIn and PolicyNotIn take a list, which is what expresses the common
// "this user may see these three warehouses" case.
type PolicyRule struct {
	Field     string         `yaml:"field" json:"field"`
	Operator  PolicyOperator `yaml:"operator" json:"operator"`
	Values    []Value        `yaml:"values,omitempty" json:"values,omitempty"`
	Parameter string         `yaml:"parameter,omitempty" json:"parameter,omitempty"`
}

// PolicyTemplate names a rule once inside a role so several restrictions of that
// same role can reuse it - the declarative replacement for 1C text templates.
// Templates are scoped to one role and are not shared between roles.
type PolicyTemplate struct {
	Name string     `yaml:"name" json:"name"`
	Rule PolicyRule `yaml:"rule" json:"rule"`
}

// AccessPolicy restricts which rows of one object the role may touch with the
// listed operations. Exactly one of Rule and Template is set.
//
// Policies of different roles combine with OR, matching how the operation grants
// themselves combine: holding more roles never takes access away.
type AccessPolicy struct {
	Operations []PermissionOperation `yaml:"operations" json:"operations"`
	Rule       *PolicyRule           `yaml:"rule,omitempty" json:"rule,omitempty"`
	Template   string                `yaml:"template,omitempty" json:"template,omitempty"`
}

func validatePolicyTemplates(templates []PolicyTemplate) []string {
	if len(templates) > MaxPolicyTemplates {
		return []string{fmt.Sprintf("policy_templates must not exceed %d entries", MaxPolicyTemplates)}
	}
	var issues []string
	names := make(map[string]bool, len(templates))
	for index, template := range templates {
		prefix := fmt.Sprintf("policy_templates[%d]", index)
		if !validIdentifier(template.Name) || utf8.RuneCountInString(template.Name) > 128 {
			issues = append(issues, prefix+".name must start with a letter, contain only letters or digits and not exceed 128 characters")
		}
		if names[template.Name] {
			issues = append(issues, prefix+".name must be unique within the role")
		}
		names[template.Name] = true
		issues = append(issues, validatePolicyRule(prefix+".rule", template.Rule)...)
	}
	return issues
}

// validatePolicies checks the restrictions of one object. templates carries the
// template names declared by the owning role, so a restriction cannot reference
// a template that does not exist.
func validatePolicies(path string, policies []AccessPolicy, templates map[string]bool) []string {
	var issues []string
	for index, policy := range policies {
		prefix := fmt.Sprintf("%s[%d]", path, index)
		if len(policy.Operations) == 0 {
			issues = append(issues, prefix+".operations must list at least one operation")
		}
		issues = append(issues, validatePermissionOperations(prefix+".operations", policy.Operations, false)...)
		switch {
		case policy.Rule != nil && policy.Template != "":
			issues = append(issues, prefix+" must set either rule or template, not both")
		case policy.Rule != nil:
			issues = append(issues, validatePolicyRule(prefix+".rule", *policy.Rule)...)
		case policy.Template != "":
			if !templates[policy.Template] {
				issues = append(issues, prefix+".template must name a policy template declared by this role")
			}
		default:
			issues = append(issues, prefix+" must set either rule or template")
		}
	}
	return issues
}

func validatePolicyRule(path string, rule PolicyRule) []string {
	var issues []string
	if !validPermissionField(rule.Field) {
		issues = append(issues, path+".field must be a canonical UUID or lowercase standard field key")
	}
	single := rule.Operator == PolicyEqual || rule.Operator == PolicyNotEqual
	list := rule.Operator == PolicyIn || rule.Operator == PolicyNotIn
	if !single && !list {
		issues = append(issues, path+".operator must be one of eq, ne, in, not-in")
	}
	switch {
	case len(rule.Values) > 0 && rule.Parameter != "":
		issues = append(issues, path+" must compare against either values or a session parameter, not both")
	case rule.Parameter != "":
		if !validIdentifier(rule.Parameter) || utf8.RuneCountInString(rule.Parameter) > 128 {
			issues = append(issues, path+".parameter must be a session parameter name")
		}
	case len(rule.Values) > 0:
		if len(rule.Values) > MaxPolicyValues {
			issues = append(issues, fmt.Sprintf("%s.values must not exceed %d entries", path, MaxPolicyValues))
		}
		if single && len(rule.Values) != 1 {
			issues = append(issues, path+".values must hold exactly one value for operator eq or ne")
		}
	default:
		issues = append(issues, path+" must compare against either values or a session parameter")
	}
	return issues
}

func clonePolicyRule(rule PolicyRule) PolicyRule {
	rule.Values = append([]Value(nil), rule.Values...)
	return rule
}

func clonePolicies(policies []AccessPolicy) []AccessPolicy {
	if policies == nil {
		return nil
	}
	cloned := make([]AccessPolicy, len(policies))
	for index, policy := range policies {
		policy.Operations = append([]PermissionOperation(nil), policy.Operations...)
		if policy.Rule != nil {
			rule := clonePolicyRule(*policy.Rule)
			policy.Rule = &rule
		}
		cloned[index] = policy
	}
	return cloned
}

func clonePolicyTemplates(templates []PolicyTemplate) []PolicyTemplate {
	if templates == nil {
		return nil
	}
	cloned := make([]PolicyTemplate, len(templates))
	for index, template := range templates {
		template.Rule = clonePolicyRule(template.Rule)
		cloned[index] = template
	}
	return cloned
}
