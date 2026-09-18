package schemadiff

import "testing"

// The distinction the whole sub-point exists for: PostgreSQL refuses some
// changes loudly and performs others silently, and only the silent ones can
// destroy data without anyone noticing.
func TestColumnImpactSeparatesSilentRewritesFromLoudFailures(t *testing.T) {
	t.Parallel()
	column := func(sqlType string, nullable bool) Column {
		return Column{Name: "c_1", Type: sqlType, Nullable: nullable}
	}
	for name, test := range map[string]struct {
		before, after Column
		want          ChangeImpact
	}{
		"fewer decimals rewrites every value":     {column("numeric(10,2)", true), column("numeric(10,0)", true), ImpactValueRewrite},
		"unbounded numeric narrowed":              {column("numeric", true), column("numeric(10,2)", true), ImpactValueRewrite},
		"smaller integer part is rejected loudly": {column("numeric(10,2)", true), column("numeric(8,2)", true), ImpactMayFail},
		"more decimals is safe":                   {column("numeric(10,2)", true), column("numeric(12,4)", true), ImpactNone},
		"shorter string is rejected loudly":       {column("character varying(50)", true), column("character varying(10)", true), ImpactMayFail},
		"text narrowed to varchar":                {column("text", true), column("character varying(10)", true), ImpactMayFail},
		"longer string is safe":                   {column("character varying(10)", true), column("character varying(50)", true), ImpactNone},
		"varchar widened to text":                 {column("character varying(10)", true), column("text", true), ImpactNone},
		"another family re-encodes every value":   {column("character varying(50)", true), column("numeric(10,2)", true), ImpactValueRewrite},
		"not null on existing rows":               {column("text", true), column("text", false), ImpactMayFail},
		"nullable again is safe":                  {column("text", false), column("text", true), ImpactNone},
		"wider integer is safe":                   {column("integer", true), column("bigint", true), ImpactNone},
		"narrower integer is rejected loudly":     {column("bigint", true), column("integer", true), ImpactMayFail},
		"numeric into an integer type rounds":     {column("numeric(10,2)", true), column("bigint", true), ImpactValueRewrite},
		"integer into wide numeric is safe":       {column("integer", true), column("numeric(20,2)", true), ImpactNone},
		"integer into narrow numeric may fail":    {column("bigint", true), column("numeric(6,2)", true), ImpactMayFail},
		"default only is safe":                    {Column{Name: "c_1", Type: "text", Nullable: true}, Column{Name: "c_1", Type: "text", Nullable: true, Default: "''"}, ImpactNone},
	} {
		if got := columnImpact(test.before, test.after); got != test.want {
			t.Errorf("%s: impact = %q, want %q", name, got, test.want)
		}
	}
}

// Rounding is classified even when the same statement also narrows the integer
// part: of the two, the silent one is what the operator must be told about.
func TestColumnImpactReportsTheSilentChangeWhenBothApply(t *testing.T) {
	t.Parallel()
	before := Column{Name: "c_1", Type: "numeric(12,4)", Nullable: true}
	after := Column{Name: "c_1", Type: "numeric(6,0)", Nullable: false}
	if got := columnImpact(before, after); got != ImpactValueRewrite {
		t.Fatalf("impact = %q, want %q", got, ImpactValueRewrite)
	}
}

func TestPlanCountsEachImpactSeparately(t *testing.T) {
	t.Parallel()
	actual := Schema{Name: ApplicationSchema, Exists: true, Tables: []Table{{
		Name: "t_1", Columns: []Column{
			{Name: "price", Type: "numeric(10,2)", Nullable: true},
			{Name: "note", Type: "text", Nullable: true},
			{Name: "old", Type: "text", Nullable: true},
		},
	}}}
	desired := Schema{Name: ApplicationSchema, Exists: true, Tables: []Table{{
		Name: "t_1", Columns: []Column{
			{Name: "price", Type: "numeric(10,0)", Nullable: true},
			{Name: "note", Type: "text", Nullable: false},
		},
	}}}
	plan, err := Compare(desired, actual)
	if err != nil {
		t.Fatal(err)
	}
	if plan.ValueRewriteCount != 1 || plan.MayFailCount != 1 || plan.ObjectLossCount != 1 {
		t.Fatalf("counts: rewrite=%d mayFail=%d objectLoss=%d", plan.ValueRewriteCount, plan.MayFailCount, plan.ObjectLossCount)
	}
	if plan.DestructiveCount != 3 {
		t.Fatalf("destructive total = %d", plan.DestructiveCount)
	}
	for _, change := range plan.Changes {
		if change.Destructive != (change.Impact != ImpactNone) {
			t.Fatalf("destructive flag disagrees with impact: %+v", change)
		}
	}
}

func TestParseColumnTypeReadsTheClosedTypeSet(t *testing.T) {
	t.Parallel()
	for value, want := range map[string]columnType{
		"numeric(10,2)":            {family: "numeric", precision: 10, scale: 2, bounded: true},
		"numeric":                  {family: "numeric"},
		"character varying(150)":   {family: "character varying", precision: 150, bounded: true},
		"text":                     {family: "text"},
		"timestamp with time zone": {family: "timestamp with time zone"},
		"uuid":                     {family: "uuid"},
		"jsonb":                    {family: "jsonb"},
		"boolean":                  {family: "boolean"},
		"numeric(oops)":            {family: "numeric(oops)"},
	} {
		if got := parseColumnType(value); got != want {
			t.Errorf("%q parsed as %+v, want %+v", value, got, want)
		}
	}
}
