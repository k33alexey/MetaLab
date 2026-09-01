// Package uuid provides stable identifiers for MetaLab entities.
package uuid

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
)

const stringLength = 36

// UUID is a 128-bit universally unique identifier.
type UUID [16]byte

// New creates a random RFC 9562 UUID version 4.
func New() (UUID, error) {
	var id UUID
	if _, err := rand.Read(id[:]); err != nil {
		return UUID{}, fmt.Errorf("generate UUID: %w", err)
	}

	id[6] = id[6]&0x0f | 0x40
	id[8] = id[8]&0x3f | 0x80

	return id, nil
}

// MustNew creates a random UUID and panics if the operating system random
// source is unavailable.
func MustNew() UUID {
	id, err := New()
	if err != nil {
		panic(err)
	}

	return id
}

// Parse validates and parses the canonical UUID representation.
func Parse(value string) (UUID, error) {
	if len(value) != stringLength {
		return UUID{}, fmt.Errorf("invalid UUID length %d", len(value))
	}

	for _, position := range [...]int{8, 13, 18, 23} {
		if value[position] != '-' {
			return UUID{}, fmt.Errorf("invalid UUID separator at position %d", position)
		}
	}

	compact := make([]byte, 0, 32)
	for index := range value {
		if value[index] != '-' {
			compact = append(compact, value[index])
		}
	}

	var id UUID
	if _, err := hex.Decode(id[:], compact); err != nil {
		return UUID{}, fmt.Errorf("invalid UUID: %w", err)
	}

	return id, nil
}

// IsZero reports whether the UUID is empty.
func (id UUID) IsZero() bool {
	return id == UUID{}
}

// String returns the canonical lowercase UUID representation.
func (id UUID) String() string {
	var buffer [stringLength]byte
	hex.Encode(buffer[0:8], id[0:4])
	buffer[8] = '-'
	hex.Encode(buffer[9:13], id[4:6])
	buffer[13] = '-'
	hex.Encode(buffer[14:18], id[6:8])
	buffer[18] = '-'
	hex.Encode(buffer[19:23], id[8:10])
	buffer[23] = '-'
	hex.Encode(buffer[24:36], id[10:16])

	return string(buffer[:])
}

// MarshalText implements encoding.TextMarshaler.
func (id UUID) MarshalText() ([]byte, error) {
	if id.IsZero() {
		return nil, errors.New("cannot marshal zero UUID")
	}

	return []byte(id.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (id *UUID) UnmarshalText(data []byte) error {
	if id == nil {
		return errors.New("cannot unmarshal UUID into nil receiver")
	}

	parsed, err := Parse(string(data))
	if err != nil {
		return err
	}

	*id = parsed
	return nil
}
