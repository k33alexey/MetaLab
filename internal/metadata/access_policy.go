package metadata

import (
	"fmt"
	"unicode/utf8"

	"github.com/k33alexey/MetaLab/internal/uuid"
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
	Field     string          `yaml:"field" json:"field"`
	Operator  PolicyOperator  `yaml:"operator" json:"operator"`
	Values    []Value         `yaml:"values,omitempty" json:"values,omitempty"`
	Parameter string          `yaml:"parameter,omitempty" json:"parameter,omitempty"`
	Subquery  *PolicySubquery `yaml:"subquery,omitempty" json:"subquery,omitempty"`
}

// PolicySubquery restricts rows by membership in a set read from another object:
// "the warehouse of this document is one of the warehouses this user is allowed",
// where the allowance itself lives in a register. Comparing against a literal or
// a session parameter cannot express that, and it is the shape real
// configurations use most.
//
// The subquery is evaluated with platform authority, NOT with the caller's own
// permissions. That is deliberate: the table holding the allowances is usually
// one the user may not read directly, and requiring a grant on it would make
// every such policy unusable. The developer authoring the policy decides what it
// may consult - exactly as they decide the rest of the rule.
type PolicySubquery struct {
	Object uuid.UUID `yaml:"object" json:"object"`
	// Field of Object supplying the values compared against the restricted field.
	Field string `yaml:"field" json:"field"`
	// Where narrows the set; its rules address fields of Object, and all of them
	// must hold (AND), which is what makes the set a single well-defined answer.
	Where []PolicyRule `yaml:"where,omitempty" json:"where,omitempty"`
}

// PolicyTemplate names a rule once inside a role so several restrictions of that
// same role can reuse it - the declarative replacement for 1C text templates.
// Templates are scoped to one role and are not shared between roles.
type PolicyTemplate struct {
	Name string `yaml:"name" json:"name"`
	// Parameters name the placeholders the rule may use as its field, written
	// "$Имя". Without them a template is one fixed rule and cannot be reused for
	// a different field, which is precisely what real configurations do with
	// theirs - one template applied to many objects, each naming its own column.
	Parameters []string   `yaml:"parameters,omitempty" json:"parameters,omitempty"`
	Rule       PolicyRule `yaml:"rule" json:"rule"`
}

// policyPlaceholder reports the parameter a field reference stands for, if any.
func policyPlaceholder(field string) (string, bool) {
	if len(field) > 1 && field[0] == '$' {
		return field[1:], true
	}
	return "", false
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
	// Arguments supply this object's field for each of the template's
	// parameters, in order.
	Arguments []string `yaml:"arguments,omitempty" json:"arguments,omitempty"`
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
		parameters := make(map[string]bool, len(template.Parameters))
		for parameterIndex, parameter := range template.Parameters {
			if !validIdentifier(parameter) || parameters[parameter] {
				issues = append(issues, fmt.Sprintf("%s.parameters[%d] must be a unique identifier", prefix, parameterIndex))
			}
			parameters[parameter] = true
		}
		issues = append(issues, validatePolicyRule(prefix+".rule", template.Rule, parameters)...)
	}
	return issues
}

// validatePolicies checks the restrictions of one object. templates carries the
// template names declared by the owning role, so a restriction cannot reference
// a template that does not exist.
func validatePolicies(path string, policies []AccessPolicy, templates map[string]int) []string {
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
			if len(policy.Arguments) != 0 {
				issues = append(issues, prefix+".arguments belong to a template, not to an inline rule")
			}
			issues = append(issues, validatePolicyRule(prefix+".rule", *policy.Rule, nil)...)
		case policy.Template != "":
			arity, declared := templates[policy.Template]
			if !declared {
				issues = append(issues, prefix+".template must name a policy template declared by this role")
				break
			}
			if len(policy.Arguments) != arity {
				issues = append(issues, fmt.Sprintf("%s.arguments must supply %d field(s) for template %q", prefix, arity, policy.Template))
			}
			for argumentIndex, argument := range policy.Arguments {
				if !validPermissionField(argument) {
					issues = append(issues, fmt.Sprintf("%s.arguments[%d] must be a canonical UUID or lowercase standard field key", prefix, argumentIndex))
				}
			}
		default:
			issues = append(issues, prefix+" must set either rule or template")
		}
	}
	return issues
}

// validatePolicyRule checks one rule. placeholders names the template parameters
// the rule may use in place of a field; an inline rule passes nil, so a
// placeholder outside a template is reported rather than silently accepted.
func validatePolicyRule(path string, rule PolicyRule, placeholders map[string]bool) []string {
	var issues []string
	if name, ok := policyPlaceholder(rule.Field); ok {
		if !placeholders[name] {
			issues = append(issues, path+".field references $"+name+", which this template does not declare as a parameter")
		}
	} else if !validPermissionField(rule.Field) {
		issues = append(issues, path+".field must be a canonical UUID or lowercase standard field key")
	}
	single := rule.Operator == PolicyEqual || rule.Operator == PolicyNotEqual
	list := rule.Operator == PolicyIn || rule.Operator == PolicyNotIn
	if !single && !list {
		issues = append(issues, path+".operator must be one of eq, ne, in, not-in")
	}
	operands := 0
	for _, present := range []bool{len(rule.Values) > 0, rule.Parameter != "", rule.Subquery != nil} {
		if present {
			operands++
		}
	}
	switch {
	case operands > 1:
		issues = append(issues, path+" must compare against exactly one of values, a session parameter or a subquery")
	case rule.Parameter != "":
		if !validIdentifier(rule.Parameter) || utf8.RuneCountInString(rule.Parameter) > 128 {
			issues = append(issues, path+".parameter must be a session parameter name")
		}
	case rule.Subquery != nil:
		if !list {
			issues = append(issues, path+".operator must be in or not-in when comparing against a subquery")
		}
		issues = append(issues, validatePolicySubquery(path+".subquery", *rule.Subquery, placeholders)...)
	case len(rule.Values) > 0:
		if len(rule.Values) > MaxPolicyValues {
			issues = append(issues, fmt.Sprintf("%s.values must not exceed %d entries", path, MaxPolicyValues))
		}
		if single && len(rule.Values) != 1 {
			issues = append(issues, path+".values must hold exactly one value for operator eq or ne")
		}
	default:
		issues = append(issues, path+" must compare against values, a session parameter or a subquery")
	}
	return issues
}

func validatePolicySubquery(path string, subquery PolicySubquery, placeholders map[string]bool) []string {
	var issues []string
	if subquery.Object.IsZero() {
		issues = append(issues, path+".object must be a non-zero UUID")
	}
	if !validPermissionField(subquery.Field) {
		issues = append(issues, path+".field must be a canonical UUID or lowercase standard field key")
	}
	if len(subquery.Where) > MaxPolicyValues {
		issues = append(issues, fmt.Sprintf("%s.where must not exceed %d conditions", path, MaxPolicyValues))
	}
	for index, condition := range subquery.Where {
		if condition.Subquery != nil {
			// One level is enough to express the real pattern, and refusing
			// nesting keeps a policy's cost predictable.
			issues = append(issues, fmt.Sprintf("%s.where[%d] must not nest another subquery", path, index))
			continue
		}
		issues = append(issues, validatePolicyRule(fmt.Sprintf("%s.where[%d]", path, index), condition, placeholders)...)
	}
	return issues
}

// substitutePolicyPlaceholders replaces every $Parameter reference with the field
// the restriction supplied for it, so nothing downstream - compilation, SQL
// rendering, the Studio screens - ever has to know templates had parameters.
func substitutePolicyPlaceholders(rule PolicyRule, parameters, arguments []string) (PolicyRule, error) {
	if len(parameters) != len(arguments) {
		return PolicyRule{}, fmt.Errorf("policy template expects %d field(s), got %d", len(parameters), len(arguments))
	}
	bound := make(map[string]string, len(parameters))
	for index, parameter := range parameters {
		bound[parameter] = arguments[index]
	}
	resolve := func(field string) (string, error) {
		name, ok := policyPlaceholder(field)
		if !ok {
			return field, nil
		}
		argument, ok := bound[name]
		if !ok {
			return "", fmt.Errorf("policy template does not declare parameter $%s", name)
		}
		return argument, nil
	}
	result := clonePolicyRule(rule)
	field, err := resolve(result.Field)
	if err != nil {
		return PolicyRule{}, err
	}
	result.Field = field
	if result.Subquery != nil {
		for index := range result.Subquery.Where {
			field, err := resolve(result.Subquery.Where[index].Field)
			if err != nil {
				return PolicyRule{}, err
			}
			result.Subquery.Where[index].Field = field
		}
	}
	return result, nil
}

func clonePolicyRule(rule PolicyRule) PolicyRule {
	rule.Values = append([]Value(nil), rule.Values...)
	if rule.Subquery != nil {
		subquery := *rule.Subquery
		subquery.Where = make([]PolicyRule, len(rule.Subquery.Where))
		for index, condition := range rule.Subquery.Where {
			subquery.Where[index] = clonePolicyRule(condition)
		}
		rule.Subquery = &subquery
	}
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
