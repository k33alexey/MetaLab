package schemadiff

import "fmt"

// conversionExpression is the USING clause an ALTER COLUMN ... TYPE needs, or ""
// when PostgreSQL converts the column on its own.
//
// It is built per pair of types rather than as a blanket "USING col::newtype",
// and that distinction matters: a blanket cast to character varying(n) would
// TRUNCATE values silently, turning a loud refusal into the same kind of quiet
// data loss this iteration exists to remove. Verified against PostgreSQL:
// varchar -> varchar(10) USING col::text still refuses values that do not fit,
// numeric -> varchar needs no clause at all, and text -> numeric is refused
// outright without one.
func conversionExpression(before, after Column) (string, error) {
	column := quoteName(before.Name)
	current, wanted := parseColumnType(before.Type), parseColumnType(after.Type)
	switch {
	case textual(current.family) && textual(wanted.family):
		// One family: PostgreSQL shortens or widens it by itself, refusing any
		// value that no longer fits.
		return "", nil
	case current.family == wanted.family:
		return "", nil
	case numericLike(current.family) && numericLike(wanted.family):
		// Number types have assignment casts in every direction; what they do to
		// the value is the impact classification's business, not this one's.
		return "", nil
	case textual(wanted.family):
		// Every scalar has a text representation and an assignment cast to it.
		return "", nil
	case current.family == "jsonb":
		// A composite value is {"kind":…,"data":…}; going back to one type means
		// taking the data and letting the cast refuse anything that is not of
		// that type, rather than quietly dropping those rows.
		return "(" + column + " ->> 'data')::" + after.Type, nil
	case wanted.family == "jsonb":
		kind, ok := compositeKind(current.family)
		if !ok {
			return "", fmt.Errorf("column %s: cannot convert %s to a composite value automatically - a uuid column may hold a reference, an enumeration value or a plain identifier, and the platform cannot tell which; add the second type on a new attribute instead", before.Name, before.Type)
		}
		if current.family == "timestamp with time zone" {
			// The stored form of a date is RFC 3339, not PostgreSQL's default
			// rendering, so the text has to be produced explicitly.
			return "jsonb_build_object('kind', '" + kind + "', 'data', to_char(" + column + " AT TIME ZONE 'UTC', 'YYYY-MM-DD\"T\"HH24:MI:SS.US\"Z\"'))", nil
		}
		return "jsonb_build_object('kind', '" + kind + "', 'data', " + column + "::text)", nil
	case textual(current.family):
		// Text into a stricter type: the cast refuses whatever does not parse.
		return column + "::" + after.Type, nil
	default:
		return "", fmt.Errorf("column %s: no safe conversion from %s to %s", before.Name, before.Type, after.Type)
	}
}

// compositeKind maps a PostgreSQL type onto the ML value kind stored inside a
// composite value. uuid is deliberately absent: four different ML types are
// stored as uuid, and guessing which one a value was would put a wrong kind into
// every row.
func compositeKind(family string) (string, bool) {
	switch family {
	case "text", "character varying", "character":
		return "string", true
	case "numeric":
		return "number", true
	case "boolean":
		return "boolean", true
	case "timestamp with time zone":
		return "date", true
	default:
		return "", false
	}
}
