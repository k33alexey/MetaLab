package testsuite

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestWriteReportFormats(t *testing.T) {
	report := Report{
		Results: []Result{{Case: Case{Module: "Проверки", Routine: "Ошибка", ID: "tests/x.bsl#Ошибка"}, Status: Failed, Error: "boom", Duration: time.Second}},
		Failed:  1, Duration: time.Second, Coverage: Coverage{Covered: 2, Total: 4, Percent: 50},
	}
	for _, test := range []struct{ format, contains string }{
		{FormatText, "coverage: 50.0%"},
		{FormatJSON, `"failed": 1`},
		{FormatJUnit, `<failure message="boom">boom</failure>`},
	} {
		var output bytes.Buffer
		if err := WriteReport(&output, test.format, report); err != nil {
			t.Fatalf("format %s: %v", test.format, err)
		}
		if !strings.Contains(output.String(), test.contains) {
			t.Fatalf("format %s output=%q", test.format, output.String())
		}
	}
	if err := WriteReport(&bytes.Buffer{}, "xml", report); err == nil {
		t.Fatal("unsupported format was accepted")
	}
}
