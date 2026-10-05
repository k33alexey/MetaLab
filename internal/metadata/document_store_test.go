package metadata

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestDocumentNumberAndPeriodNormalization(t *testing.T) {
	t.Parallel()
	if value, err := normalizeDocumentNumber(DocumentNumber{Type: NumberType, Length: 4}, "0012"); err != nil || value != "12" {
		t.Fatalf("numeric number=%q error=%v", value, err)
	}
	for _, value := range []string{"-1", "1.2", "12345"} {
		if _, err := normalizeDocumentNumber(DocumentNumber{Type: NumberType, Length: 4}, value); err == nil {
			t.Fatalf("numeric number %q was accepted", value)
		}
	}
	invalidUTF8 := string([]byte{0xff})
	if _, err := normalizeDocumentNumber(DocumentNumber{Type: StringType, Length: 4}, invalidUTF8); err == nil {
		t.Fatal("invalid UTF-8 document number was accepted")
	}
	date := time.Date(2026, 9, 6, 12, 30, 0, 0, time.FixedZone("test", 3*60*60))
	if period := documentNumberPeriod(NumberPeriodYear, date); period != 2026 {
		t.Fatalf("year period=%d", period)
	}
	if period := documentNumberPeriod(NumberPeriodQuarter, date); period != 20263 {
		t.Fatalf("quarter period=%d", period)
	}
	if period := documentNumberPeriod(NumberPeriodMonth, date); period != 202609 {
		t.Fatalf("month period=%d", period)
	}
	if period := documentNumberPeriod(NumberPeriodDay, date); period != 20260906 {
		t.Fatalf("day period=%d", period)
	}
}

// A numeric number or code takes as many digits as its length, and one more is
// too many.
//
// Defect caught: a number or code of exactly its length refused.
func TestNumericNumberAndCodeFillTheirWholeLength(t *testing.T) {
	t.Parallel()
	if value, err := normalizeDocumentNumber(DocumentNumber{Type: NumberType, Length: 4}, "9999"); err != nil || value != "9999" {
		t.Errorf("number of four digits in four: %q, %v", value, err)
	}
	if _, err := normalizeDocumentNumber(DocumentNumber{Type: NumberType, Length: 4}, "10000"); err == nil || !strings.Contains(err.Error(), "at most 4 digits") {
		t.Errorf("number of five digits in four: %v", err)
	}
	if value, err := normalizeCatalogCode(CatalogCode{Type: NumberType, Length: 4}, "9999"); err != nil || value != "9999" {
		t.Errorf("code of four digits in four: %q, %v", value, err)
	}
	if _, err := normalizeCatalogCode(CatalogCode{Type: NumberType, Length: 4}, "10000"); err == nil || !strings.Contains(err.Error(), "at most 4 digits") {
		t.Errorf("code of five digits in four: %v", err)
	}
}

func TestDocumentWriteErrorIsSpecific(t *testing.T) {
	t.Parallel()
	err := documentWriteError("Продажа", errors.New("storage"))
	if !strings.Contains(err.Error(), "write document Продажа") {
		t.Fatalf("error=%v", err)
	}
}
