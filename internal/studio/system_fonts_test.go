package studio

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"unicode/utf16"
)

// section reads a file held in memory.
func section(data []byte) *io.SectionReader {
	return io.NewSectionReader(bytes.NewReader(data), 0, int64(len(data)))
}

// fontFile builds a font of the format with one table «name» holding the
// records given: {platform, encoding, language, id, text}.
func fontFile(records ...[5]any) []byte {
	var pool []byte
	table := make([]byte, 6+12*len(records))
	binary.BigEndian.PutUint16(table[2:], uint16(len(records)))
	binary.BigEndian.PutUint16(table[4:], uint16(len(table)))
	for index, record := range records {
		text := record[4].(string)
		raw := []byte(text)
		if record[0].(int) == 3 {
			raw = nil
			for _, unit := range utf16.Encode([]rune(text)) {
				raw = binary.BigEndian.AppendUint16(raw, unit)
			}
		}
		at := 6 + 12*index
		for field, value := range []int{record[0].(int), record[1].(int), record[2].(int), record[3].(int), len(raw), len(pool)} {
			binary.BigEndian.PutUint16(table[at+2*field:], uint16(value))
		}
		pool = append(pool, raw...)
	}
	table = append(table, pool...)
	font := make([]byte, 12+16)
	binary.BigEndian.PutUint32(font, 0x00010000)
	binary.BigEndian.PutUint16(font[4:], 1)
	copy(font[12:], "name")
	binary.BigEndian.PutUint32(font[12+8:], uint32(len(font)))
	binary.BigEndian.PutUint32(font[12+12:], uint32(len(table)))
	return append(font, table...)
}

// The faces Studio's server reads from the fonts of the system (ML-STUDIO.md,
// «Окно стиля»; the owner, 27.10.2026, В6 of series 2.595): a font is named
// by its typographic family where it has one, else by its family, in
// Windows US English before another Windows language before Macintosh
// Roman; a collection names each of its fonts; the list is by the alphabet
// whatever the case, each family once, without the families the system
// keeps for itself (a dot first), and files of no font format are passed
// by.
// Defects caught: a name of another kind (a style, a full name) taken for
// the family; the order of preference of the records reversed; a
// collection read as one font; a family twice; the order of the list
// minding the case; a hidden family listed; a broken file stopping the
// walk.
func TestTheFacesOfTheSystemAreTheFamiliesOfItsFonts(t *testing.T) {
	plain := fontFile([5]any{3, 1, 0x409, 2, "Bold"}, [5]any{3, 1, 0x409, 4, "Win Name Bold"}, [5]any{1, 0, 0, 1, "Mac Name"}, [5]any{3, 1, 0x419, 1, "Русское"}, [5]any{3, 1, 0x409, 1, "Win Name"})
	if got := fontFamilies(section(plain)); !slices.Equal(got, []string{"Win Name"}) {
		t.Errorf("the family of a font: %q", got)
	}
	if got := fontFamilies(section(fontFile([5]any{1, 0, 0, 1, "Mac Name"}, [5]any{3, 1, 0x419, 1, "Русское"}))); !slices.Equal(got, []string{"Русское"}) {
		t.Errorf("a Windows name before a Macintosh one: %q", got)
	}
	if got := fontFamilies(section(fontFile([5]any{1, 0, 0, 1, "Mac Name"}))); !slices.Equal(got, []string{"Mac Name"}) {
		t.Errorf("a Macintosh name: %q", got)
	}
	typographic := fontFile([5]any{3, 1, 0x409, 1, "Helvetica Neue Light"}, [5]any{3, 1, 0x409, 16, "Helvetica Neue"})
	if got := fontFamilies(section(typographic)); !slices.Equal(got, []string{"Helvetica Neue"}) {
		t.Errorf("the typographic family: %q", got)
	}
	// A collection: «ttcf», a version, the count and the offset of each font.
	first, second := fontFile([5]any{3, 1, 0x409, 1, "Alpha"}), fontFile([5]any{3, 1, 0x409, 1, "beta"})
	collection := []byte("ttcf\x00\x01\x00\x00")
	collection = binary.BigEndian.AppendUint32(collection, 2)
	start := uint32(len(collection) + 8)
	collection = binary.BigEndian.AppendUint32(collection, start)
	collection = binary.BigEndian.AppendUint32(collection, start+uint32(len(first)))
	relocate := func(font []byte, at uint32) []byte {
		moved := slices.Clone(font)
		binary.BigEndian.PutUint32(moved[12+8:], binary.BigEndian.Uint32(font[12+8:])+at)
		return moved
	}
	collection = append(collection, relocate(first, start)...)
	collection = append(collection, relocate(second, start+uint32(len(first)))...)
	if got := fontFamilies(section(collection)); !slices.Equal(got, []string{"Alpha", "beta"}) {
		t.Errorf("a collection: %q", got)
	}
	folder := t.TempDir()
	for name, data := range map[string][]byte{"a.ttc": collection, "b.ttf": plain, "c.otf": fontFile([5]any{3, 1, 0x409, 1, "ALPHA"}),
		"d.ttf": fontFile([5]any{3, 1, 0x409, 1, ".SF NS"}), "e.ttf": []byte("broken"), "f.txt": plain, "sub/g.otf": fontFile([5]any{3, 1, 0x409, 1, "Gamma"})} {
		path := filepath.Join(folder, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if got := readFaces([]string{folder, filepath.Join(folder, "none")}); !slices.Equal(got, []string{"Alpha", "beta", "Gamma", "Win Name"}) {
		t.Errorf("the faces: %q", got)
	}
}
