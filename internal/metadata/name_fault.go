package metadata

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/k33alexey/MetaLab/internal/project"
)

// NameFault is what keeps a typed name from its place: the prototype's three
// refusals at the field of a name (docs/block-00/obshchee.md, «Имя и
// синоним»; METADATA-OBJECTS.md, «Где имя уникально») and a name longer than
// any the model keeps.
type NameFault int

const (
	NameFits NameFault = iota
	NameEmpty
	NameNotIdentifier
	NameTooLong
	NameTaken
)

// EditorLimits are the model's ceilings on what the window of an object types
// in, one description Studio sets every such field by (studio-common.js,
// limited): the name of anything named by an identifier, the length of a
// code or a number, the length of a description. A field the model sets no
// ceiling on - a synonym, a comment - has none here and none in Studio. An
// exchange plan takes a longer description (maxExchangePlanDescriptionLength),
// which comes with a window of it.
type EditorLimits struct {
	Name        int `json:"name"`
	Code        int `json:"code"`
	Description int `json:"description"`
	// Levels is the most levels a hierarchy may be limited to.
	Levels int `json:"levels"`
}

// Limits is the description of the ceilings.
func Limits() EditorLimits {
	return EditorLimits{Name: maxNameLength, Code: maxCodeLength, Description: maxDescriptionLength, Levels: maxHierarchyLevelCount}
}

// NameRuleFault is what the rule of a name alone says of it, whatever stands
// beside it.
func NameRuleFault(name string) NameFault {
	switch {
	case name == "":
		return NameEmpty
	case !project.IsIdentifier(name):
		return NameNotIdentifier
	case utf8.RuneCountInString(name) > maxNameLength:
		return NameTooLong
	}
	return NameFits
}

// PartNameFault says whether the name of a part of a description - an
// attribute, a table part, an attribute of one - fits its branch, as reading
// the description answers it: which names are its siblings, which the
// platform keeps for the kind and which names of another branch the part may
// not take are the reading's, and are not said twice. relative is where the
// description lies, which names its kind; description is the object as an
// editor holds it, in JSON; at is the way down to the part by the JSON names
// of the lists and the positions in them: tableParts, 0, attributes, 2.
//
// The name is taken when the description refuses more names with the part
// than without it: a refusal may fall on the part - a sibling has its name -
// or on another - a table part has the name of the attribute typed.
func PartNameFault(relative string, description []byte, at []string, configuration project.Project) (NameFault, error) {
	item, _, ok := sourceAt(relative)
	if !ok {
		return NameFits, fmt.Errorf("%q is no description of an object", relative)
	}
	value := reflect.New(item.described)
	if err := json.Unmarshal(description, value.Interface()); err != nil {
		return NameFits, err
	}
	refused := func() (int, error) {
		var encoded bytes.Buffer
		if err := Encode(&encoded, value.Elem().Interface()); err != nil {
			return 0, err
		}
		_, err := item.decode(relative, &encoded, nil, configuration)
		return refusedNames(err), nil
	}
	return partNameFault(value.Elem(), at, refused)
}

// LanguageNameFault is PartNameFault for a language of the configuration: the
// configuration as its editor holds it, in JSON, and at - languages, 1.
func LanguageNameFault(configuration []byte, at []string) (NameFault, error) {
	var value project.Project
	if err := json.Unmarshal(configuration, &value); err != nil {
		return NameFits, err
	}
	return partNameFault(reflect.ValueOf(&value).Elem(), at, func() (int, error) {
		count := 0
		var validation *project.ValidationError
		if errors.As(value.Validate(), &validation) {
			for _, issue := range validation.Issues {
				if strings.HasSuffix(issue.Path, ".name") || issue.Path == "name" {
					count++
				}
			}
		}
		return count, nil
	})
}

// partNameFault weighs the part at the end of the way: the rule first, then
// how many names the description refuses with it, taken last in its list, and
// without it.
func partNameFault(value reflect.Value, at []string, refused func() (int, error)) (NameFault, error) {
	list, name, err := takeLast(value, at)
	if err != nil {
		return NameFits, err
	}
	if fault := NameRuleFault(name); fault != NameFits {
		return fault, nil
	}
	with, err := refused()
	if err != nil {
		return NameFits, err
	}
	list.Set(list.Slice(0, list.Len()-1))
	without, err := refused()
	if err != nil || with <= without {
		return NameFits, err
	}
	return NameTaken, nil
}

// refusedNames counts the names a reading refused: the issues of its list,
// which it starts with «validate <source>: » and parts with «; », that are
// about a name.
func refusedNames(err error) int {
	if err == nil {
		return 0
	}
	_, list, _ := strings.Cut(err.Error(), ": ")
	count := 0
	for _, issue := range strings.Split(list, "; ") {
		if path, _, _ := strings.Cut(issue, " "); path == "name" || strings.HasSuffix(path, ".name") {
			count++
		}
	}
	return count
}

// takeLast moves the part at the end of the way to the end of its list and
// answers the list and the part's name. The reading weighs every name of a
// list against those before it, so the last is weighed against all its
// siblings, and taking it away takes away only what it brought.
func takeLast(value reflect.Value, at []string) (reflect.Value, string, error) {
	current, list := value, reflect.Value{}
	for step, word := range at {
		if current.Kind() != reflect.Slice {
			if current = fieldByJSONName(current, word); !current.IsValid() {
				return list, "", fmt.Errorf("no list %q", word)
			}
			continue
		}
		position, err := strconv.Atoi(word)
		if err != nil || position < 0 || position >= current.Len() {
			return list, "", fmt.Errorf("no part %q", word)
		}
		if step == len(at)-1 {
			last := current.Len() - 1
			part := reflect.New(current.Type().Elem()).Elem()
			part.Set(current.Index(position))
			reflect.Copy(current.Slice(position, last), current.Slice(position+1, last+1))
			current.Index(last).Set(part)
			position, list = last, current
		}
		current = current.Index(position)
	}
	name := reflect.Value{}
	if current.Kind() == reflect.Struct {
		name = current.FieldByName("Name")
	}
	if !list.IsValid() || !name.IsValid() || name.Kind() != reflect.String {
		return list, "", errors.New("the way leads to no part with a name")
	}
	return list, name.String(), nil
}

// fieldByJSONName finds the field of a struct the JSON of an editor names, in
// the struct or in one it takes in; an invalid value where there is none.
func fieldByJSONName(value reflect.Value, name string) reflect.Value {
	if value.Kind() != reflect.Struct {
		return reflect.Value{}
	}
	for index := range value.NumField() {
		field := value.Type().Field(index)
		written, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if field.Anonymous && written == "" {
			if found := fieldByJSONName(value.Field(index), name); found.IsValid() {
				return found
			}
			continue
		}
		if written == name {
			return value.Field(index)
		}
	}
	return reflect.Value{}
}
