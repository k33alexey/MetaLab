package metadata

import (
	"strings"
	"testing"
)

// mobileCommandBarForm is a common form whose command bar on a mobile device
// holds bar, beside a group and the bar of the form it could name.
func mobileCommandBarForm(bar string) string {
	return "format: 1\nid: " + commonFormID + "\nname: Мобильная\ntitle: {ru: Мобильная}\nkind: common\n" +
		"auto_command_bar:\n  id: c0de0000-0000-4000-8000-000000990102\n  name: ФормаКоманднаяПанель\n" +
		"items:\n  - {id: c0de0000-0000-4000-8000-000000990104, name: Шапка, kind: usual-group}\n" +
		"mobile_command_bar: " + bar + "\n"
}

// What the command bar of a form holds on a mobile device is not carried:
// ML has no mobile client (METADATA-OBJECTS, «Мобильное приложение»; the
// owner's decision of 14.10.2026), and a description naming it, by an
// element of the form, empty or by the code of a deleted one, is refused by
// the strict reading - of a common form when the project loads, of any form
// when its description is read. Defect caught: the list still taken and
// carried, a property of a client ML does not have.
func TestTheMobileCommandBarOfAFormIsNotCarried(t *testing.T) {
	t.Parallel()
	for name, bar := range map[string]string{
		"элементы формы": "[ФормаКоманднаяПанель, Шапка]",
		"пустой пункт":   `[Шапка, ""]`,
		"код элемента":   `["5:0202"]`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := metadataProject(t)
			writeCommonForm(t, root, "Мобильная", mobileCommandBarForm(bar))
			if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "field mobile_command_bar not found") {
				t.Fatalf("the project loads: %v", err)
			}
			if _, err := DecodeManagedForm("form.yaml", strings.NewReader(mobileCommandBarForm(bar)), metadataConfiguration()); err == nil ||
				!strings.Contains(err.Error(), "field mobile_command_bar not found") {
				t.Fatalf("the description reads: %v", err)
			}
		})
	}
	// Without it the same form loads: the refusal is of the list alone.
	root := metadataProject(t)
	writeCommonForm(t, root, "Мобильная", strings.TrimSuffix(mobileCommandBarForm(""), "mobile_command_bar: \n"))
	if _, err := Load(root); err != nil {
		t.Fatalf("the form without the list: %v", err)
	}
}
