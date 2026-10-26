package metadata

import (
	"slices"
	"testing"

	"github.com/k33alexey/MetaLab/internal/kinds"
)

// TestTheMethodsOfThePaletteAreTheModels catches the palette of a method of
// an HTTP service offering a method the model refuses, or leaving out one it
// takes: the seventeen verbs and «Любой» of the prototype's list
// (http-servis.md, «Палитра»: «совпадает с перечнем модели»).
func TestTheMethodsOfThePaletteAreTheModels(t *testing.T) {
	t.Parallel()
	var offered []string
	for _, tab := range kinds.PartPalette("httpMethod") {
		for _, row := range tab.Rows {
			if row.Key == "method" {
				for _, choice := range row.Choices {
					offered = append(offered, choice[0])
				}
			}
		}
	}
	if len(offered) != 18 {
		t.Fatalf("the palette offers %d methods, want 18: %v", len(offered), offered)
	}
	for _, method := range offered {
		if !slices.Contains(httpMethods, method) {
			t.Errorf("the palette offers %s, which the model refuses", method)
		}
	}
}
