package metadata

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/k33alexey/MetaLab/internal/uuid"
)

// RightRestriction is a restriction of access to data on one right of an
// object, as the prototype keeps it (help, «Ограничение доступа к данным»): a
// condition in the query language, with the preprocessor and the role's
// templates, and the fields it checks. A restriction with no fields is the
// one on «Прочие поля» - every field the others do not name.
//
// Only reading, adding, changing and deleting are restricted, on an object
// that keeps data; reading may have a restriction per set of fields, the other
// three one each, with no fields. The right restricted is the role's, granted
// or denied: the prototype writes the restriction inside the right and keeps
// it when the right is taken away (629 in the exports).
//
// The condition is carried as written and is not run: running it is block
// 10's. Until then a right a role restricts gives that role nothing - no rows
// to read, no data to add, change or delete - so a restriction never opens
// more than it would (ACCESS-RIGHTS.md, «Ограничения на уровне записей»).
type RightRestriction struct {
	Right PermissionOperation `yaml:"right" json:"right"`
	// Fields are keys of the object's fields (restrictionFieldsOf): a declared
	// field by its identifier, a standard one by its prototype name in lower
	// case, the line number of a table part after the part's identifier. A
	// field the object does not have is kept and noted
	// (restriction-absent-field) until Studio writes the role
	// (DropAbsentRestrictionFields).
	Fields    []string `yaml:"fields,omitempty" json:"fields,omitempty"`
	Condition string   `yaml:"condition,omitempty" json:"condition,omitempty"`
}

// RestrictionTemplate is a template of the role's restrictions (help,
// «Редактирование роли», «Шаблоны ограничений»): a name, with the names of its
// parameters in brackets where it takes any, and a text the condition of a
// restriction takes in place of #Имя("значение", ...). The name is kept as
// the prototype writes it, brackets and all.
type RestrictionTemplate struct {
	Name      string `yaml:"name" json:"name"`
	Condition string `yaml:"condition,omitempty" json:"condition,omitempty"`
}

// RestrictionRule is how one right is restricted, by the help («Ограничение
// доступа к данным»). The editor of roles gets the rules with its schema, so
// it offers what saving keeps (validateRestrictions) and knows them nowhere
// else.
type RestrictionRule struct {
	Right PermissionOperation `json:"right"`
	// ByFields: a restriction of the right names the fields it checks, one
	// per set of fields - a field in one of them at most - and one more, with
	// no fields, on «Прочие поля». A right not restricted by fields takes one
	// restriction, with no fields, on every field.
	ByFields bool `json:"byFields"`
}

// restrictionRules are the rights a restriction may be on, by the help: the
// rights to the data an object keeps in the infobase. Only reading is
// restricted by fields.
var restrictionRules = []RestrictionRule{
	{Right: PermissionRead, ByFields: true},
	{Right: PermissionInsert},
	{Right: PermissionUpdate},
	{Right: PermissionDelete},
}

// readingAlone is a kind whose reading alone is restricted, with no fields:
// a sequence, a recalculation (rol-posledovatelnost-*, rol-pereraschet-*).
var readingAlone = []RestrictionRule{{Right: PermissionRead}}

// restrictionRulesOf is how the window of a role restricts an object of the
// kind (kindUses.restricted).
func restrictionRulesOf(kind Kind) []RestrictionRule {
	if rules := usesOf(kind).restricted; rules != nil {
		return rules
	}
	return restrictionRules
}

func restrictionRuleOf(right PermissionOperation) (RestrictionRule, bool) {
	index := slices.IndexFunc(restrictionRules, func(rule RestrictionRule) bool { return rule.Right == right })
	if index < 0 {
		return RestrictionRule{}, false
	}
	return restrictionRules[index], true
}

// templateName splits a template's name into the name proper and its
// parameters: Шаблон(Поле, Пользователь) is the template Шаблон of two.
func templateName(written string) (name string, parameters []string, ok bool) {
	name, rest, bracketed := strings.Cut(written, "(")
	name = strings.TrimSpace(name)
	if !validIdentifier(name) || utf8.RuneCountInString(name) > maxNameLength {
		return "", nil, false
	}
	if !bracketed {
		return name, nil, true
	}
	inner, closed := strings.CutSuffix(strings.TrimSpace(rest), ")")
	if !closed {
		return "", nil, false
	}
	for _, parameter := range strings.Split(inner, ",") {
		parameter = strings.TrimSpace(parameter)
		if !validIdentifier(parameter) {
			return "", nil, false
		}
		parameters = append(parameters, parameter)
	}
	return name, parameters, true
}

// validateRestrictionTemplates checks the templates of a role. There is no
// ceiling on their number or on the length of a text: the prototype names
// none, and erp keeps a template of 431 thousand characters.
func validateRestrictionTemplates(templates []RestrictionTemplate) []string {
	var issues []string
	names := namesWithin(" within the role")
	for index, template := range templates {
		prefix := fmt.Sprintf("restriction_templates[%d]", index)
		name, _, ok := templateName(template.Name)
		if !ok {
			issues = append(issues, prefix+".name must be a name, with the names of its parameters in brackets where it takes any")
			continue
		}
		// The name, read out of what is written, is one of a template's
		// siblings like any other.
		issues = append(issues, names.take(prefix, name)...)
		if !utf8.ValidString(template.Condition) {
			issues = append(issues, prefix+".condition must be valid UTF-8")
		}
	}
	return issues
}

// validateRestrictions checks the restrictions of one object against the
// rights the role writes on it. Whether the object keeps data and has the
// fields named is the catalog's to say (validateRoleRestrictions).
func validateRestrictions(path string, object ObjectPermission) []string {
	var issues []string
	written := slices.Concat(object.Operations, object.Denied)
	others := map[PermissionOperation]bool{}
	fields := map[PermissionOperation]map[string]bool{}
	for index, restriction := range object.Restrictions {
		prefix := fmt.Sprintf("%s[%d]", path, index)
		rule, restricted := restrictionRuleOf(restriction.Right)
		if !restricted {
			issues = append(issues, fmt.Sprintf("%s.right names %q; only read, insert, update and delete are restricted", prefix, restriction.Right))
			continue
		}
		// The prototype writes a restriction inside the right it restricts,
		// and a right is written granted or denied.
		if !slices.Contains(written, restriction.Right) {
			issues = append(issues, fmt.Sprintf("%s restricts %q, which the role neither grants nor denies on the object", prefix, restriction.Right))
		}
		// A right not restricted by fields has its one restriction on the
		// other fields, so a second one is a second there.
		if !rule.ByFields && len(restriction.Fields) != 0 {
			issues = append(issues, fmt.Sprintf("%s.fields: %q is not restricted by fields; only reading is", prefix, restriction.Right))
		}
		if len(restriction.Fields) == 0 {
			if others[restriction.Right] {
				issues = append(issues, fmt.Sprintf("%s: %q has a restriction on the other fields already", prefix, restriction.Right))
			}
			others[restriction.Right] = true
		}
		if fields[restriction.Right] == nil {
			fields[restriction.Right] = map[string]bool{}
		}
		for fieldIndex, field := range restriction.Fields {
			if field == "" {
				issues = append(issues, fmt.Sprintf("%s.fields[%d] must name a field", prefix, fieldIndex))
			} else if !validRestrictionField(field) {
				issues = append(issues, fmt.Sprintf("%s.fields[%d] must be a canonical UUID, a lowercase standard field key "+
					"or a table part's UUID with a lowercase standard field key after a dot", prefix, fieldIndex))
			}
			if fields[restriction.Right][field] {
				issues = append(issues, fmt.Sprintf("%s.fields[%d] is restricted already", prefix, fieldIndex))
			}
			fields[restriction.Right][field] = true
		}
		if !utf8.ValidString(restriction.Condition) {
			issues = append(issues, prefix+".condition must be valid UTF-8")
		}
	}
	return issues
}

// restrictsData says the role's own restrictions keep it from a right on the
// object: a condition is there, and conditions are not run yet. A restriction
// without one restricts nothing.
func (object ObjectPermission) restrictsData(right PermissionOperation) bool {
	return slices.ContainsFunc(object.Restrictions, func(restriction RightRestriction) bool {
		return restriction.Right == right && strings.TrimSpace(restriction.Condition) != ""
	})
}

// DropFields takes fields the object no longer has out of the role, as the
// prototype does when an attribute or a table part is deleted (pass of
// 22.10.2026, the fourth; ACCESS-RIGHTS.md, «Удаление реквизита, по которому
// стоит ограничение роли»): the rights given or denied on them go, and every
// restriction of the object loses them (dropRestrictionFields). A right on a
// field the object has not would leave a role that does not load. Reports
// whether the role changed.
func (role *RoleDefinition) DropFields(object uuid.UUID, keys []string) bool {
	changed := false
	for index := range role.Objects {
		permission := &role.Objects[index]
		if permission.Object != object {
			continue
		}
		before := len(permission.Fields)
		permission.Fields = slices.DeleteFunc(permission.Fields, func(field FieldPermission) bool { return slices.Contains(keys, field.Field) })
		changed = changed || len(permission.Fields) != before
		changed = permission.dropRestrictionFields(func(field string) bool { return slices.Contains(keys, field) }) || changed
	}
	return changed
}

// DropAbsentRestrictionFields takes out of the restrictions of the roles the
// fields their objects have not now - Родитель of a catalog whose hierarchy
// was taken away -, as the prototype does whenever it writes a role (passes
// of 23.10.2026, the ninth and the tenth, absent-20…27, v2-*; ACCESS-RIGHTS.md,
// «Ограничение по полю, которого у объекта нет»): any change of the role, on
// this object or another, writes its restrictions without them
// (dropRestrictionFields); a role not written keeps them. An object the
// project does not hold is left as it is.
func DropAbsentRestrictionFields(root string, roles ...*RoleDefinition) error {
	if !slices.ContainsFunc(roles, namesRestrictionFields) {
		return nil
	}
	catalog, err := load(root, false)
	if err != nil {
		return err
	}
	for _, role := range roles {
		for index := range role.Objects {
			permission := &role.Objects[index]
			if kind, ok := catalog.kindOfObject(permission.Object); ok && len(permission.Restrictions) > 0 {
				present := catalog.restrictionFieldKeys(kind, permission.Object)
				permission.dropRestrictionFields(func(field string) bool { return !present[field] })
			}
		}
	}
	return nil
}

// namesRestrictionFields says a restriction of the role names a field.
func namesRestrictionFields(role *RoleDefinition) bool {
	return slices.ContainsFunc(role.Objects, func(permission ObjectPermission) bool {
		return slices.ContainsFunc(permission.Restrictions, func(restriction RightRestriction) bool { return len(restriction.Fields) > 0 })
	})
}

// dropRestrictionFields takes the fields drop names out of every
// restriction of the object: a restriction keeps the rest of its fields, and
// one that named only those goes away, it does not become the restriction of
// «Прочие поля», as the prototype does (ACCESS-RIGHTS.md, «Удаление
// реквизита, по которому стоит ограничение роли», «Ограничение по полю,
// которого у объекта нет»). Reports whether a restriction changed.
func (permission *ObjectPermission) dropRestrictionFields(drop func(string) bool) bool {
	changed := false
	kept := permission.Restrictions[:0]
	for _, restriction := range permission.Restrictions {
		fields := slices.DeleteFunc(slices.Clone(restriction.Fields), drop)
		if len(fields) == len(restriction.Fields) {
			kept = append(kept, restriction)
			continue
		}
		changed = true
		if len(fields) > 0 {
			restriction.Fields = fields
			kept = append(kept, restriction)
		}
	}
	permission.Restrictions = kept
	return changed
}

func cloneRestrictions(restrictions []RightRestriction) []RightRestriction {
	if restrictions == nil {
		return nil
	}
	cloned := slices.Clone(restrictions)
	for index := range cloned {
		cloned[index].Fields = slices.Clone(cloned[index].Fields)
	}
	return cloned
}
