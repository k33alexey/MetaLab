package studio

import (
	"encoding/binary"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"unicode/utf16"
)

// The faces the windows choosing a font list (ML-STUDIO.md, «Окно стиля»;
// the owner, 27.10.2026, question В6 of series 2.595): the fonts of the
// system Studio runs on, as the prototype lists them - by family, in the
// order of the alphabet. A browser gives no list of its own, so the
// server of Studio reads the font files of the system once and answers
// every window from it.

// systemFaces are the families of the fonts of the system, read once.
var systemFaces = sync.OnceValue(func() []string {
	return readFaces(fontFolders())
})

// fontFolders are where the system keeps its fonts and its user's.
func fontFolders() []string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		return []string{"/System/Library/Fonts", "/Library/Fonts", filepath.Join(home, "Library", "Fonts")}
	case "windows":
		folders := []string{filepath.Join(os.Getenv("WINDIR"), "Fonts")}
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			folders = append(folders, filepath.Join(local, "Microsoft", "Windows", "Fonts"))
		}
		return folders
	default:
		return []string{"/usr/share/fonts", "/usr/local/share/fonts", filepath.Join(home, ".local", "share", "fonts"), filepath.Join(home, ".fonts")}
	}
}

// readFaces walks the folders for TrueType and OpenType files and answers
// the families they name, each once, by the alphabet whatever the case. A
// family whose name starts with a dot is one the system keeps for itself
// (macOS «.SF NS») and lists nowhere.
func readFaces(folders []string) []string {
	seen := map[string]bool{}
	faces := []string{}
	for _, folder := range folders {
		filepath.WalkDir(folder, func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return nil
			}
			switch strings.ToLower(filepath.Ext(path)) {
			case ".ttf", ".otf", ".ttc", ".otc":
			default:
				return nil
			}
			file, err := os.Open(path)
			if err != nil {
				return nil
			}
			defer file.Close()
			info, err := file.Stat()
			if err != nil {
				return nil
			}
			data := io.NewSectionReader(file, 0, info.Size())
			for _, face := range fontFamilies(data) {
				if face == "" || strings.HasPrefix(face, ".") || seen[strings.ToLower(face)] {
					continue
				}
				seen[strings.ToLower(face)] = true
				faces = append(faces, face)
			}
			return nil
		})
	}
	slices.SortFunc(faces, func(one, other string) int {
		return strings.Compare(strings.ToLower(one), strings.ToLower(other))
	})
	return faces
}

// fontFamilies answers the family of each font of a TrueType or OpenType
// file - one, or several in a collection - from the table «name» of the
// format: the typographic family (16) where it is given, else the family
// (1), in English where the font names it so. Only the directories and the
// table «name» are read, not the whole file.
func fontFamilies(data *io.SectionReader) []string {
	head := readSpan(data, 0, 12)
	offsets := []uint32{0}
	if len(head) == 12 && string(head[:4]) == "ttcf" {
		count := min(binary.BigEndian.Uint32(head[8:12]), 1024)
		list := readSpan(data, 12, int64(4*count))
		offsets = offsets[:0]
		for index := 0; index+4 <= len(list); index += 4 {
			offsets = append(offsets, binary.BigEndian.Uint32(list[index:]))
		}
	}
	families := []string{}
	for _, offset := range offsets {
		if family := fontFamily(data, int64(offset)); family != "" {
			families = append(families, family)
		}
	}
	return families
}

// readSpan reads length bytes at offset, or answers nil where the file
// holds fewer.
func readSpan(data *io.SectionReader, offset, length int64) []byte {
	if offset < 0 || length < 0 || offset+length > data.Size() {
		return nil
	}
	span := make([]byte, length)
	if _, err := data.ReadAt(span, offset); err != nil {
		return nil
	}
	return span
}

func fontFamily(data *io.SectionReader, offset int64) string {
	head := readSpan(data, offset, 12)
	if head == nil {
		return ""
	}
	directory := readSpan(data, offset+12, 16*int64(binary.BigEndian.Uint16(head[4:])))
	for at := 0; at+16 <= len(directory); at += 16 {
		if string(directory[at:at+4]) == "name" {
			table := readSpan(data, int64(binary.BigEndian.Uint32(directory[at+8:])), int64(binary.BigEndian.Uint32(directory[at+12:])))
			return nameFamily(table, 0)
		}
	}
	return ""
}

// nameFamily reads the table «name» at start: the best of its records
// naming the typographic family or the family - Windows in US English
// first, then any Windows one, then Macintosh Roman.
func nameFamily(data []byte, start uint64) string {
	if start+6 > uint64(len(data)) {
		return ""
	}
	count := uint64(binary.BigEndian.Uint16(data[start+2:]))
	pool := start + uint64(binary.BigEndian.Uint16(data[start+4:]))
	best, bestRank := "", 0
	for index := uint64(0); index < count; index++ {
		at := start + 6 + index*12
		if at+12 > uint64(len(data)) {
			break
		}
		platform, encoding := binary.BigEndian.Uint16(data[at:]), binary.BigEndian.Uint16(data[at+2:])
		language, id := binary.BigEndian.Uint16(data[at+4:]), binary.BigEndian.Uint16(data[at+6:])
		length, place := uint64(binary.BigEndian.Uint16(data[at+8:])), uint64(binary.BigEndian.Uint16(data[at+10:]))
		if id != 1 && id != 16 {
			continue
		}
		rank := 0
		switch {
		case platform == 3 && (encoding == 1 || encoding == 10) && language == 0x409:
			rank = 6
		case platform == 3 && (encoding == 1 || encoding == 10):
			rank = 4
		case platform == 1 && encoding == 0 && language == 0:
			rank = 2
		default:
			continue
		}
		if id == 16 {
			rank++
		}
		if rank <= bestRank || pool+place+length > uint64(len(data)) {
			continue
		}
		raw := data[pool+place : pool+place+length]
		if platform == 3 {
			units := make([]uint16, len(raw)/2)
			for unit := range units {
				units[unit] = binary.BigEndian.Uint16(raw[2*unit:])
			}
			best = string(utf16.Decode(units))
		} else {
			best = string(raw)
		}
		bestRank = rank
	}
	return best
}

// fontsAddress answers the faces of the system.
const fontsAddress = "/api/fonts"

func registerFontRoutes(routes *http.ServeMux) {
	routes.HandleFunc("GET "+fontsAddress, answerQuery(writeSourceError, func(*http.Request) (any, error) {
		return systemFaces(), nil
	}))
}
