// backend/internal/tests/unit/hrm/learning/helpers_test.go
// Shared builders and the reflective shape assertion used by the Phase 6A
// tests.
package learning_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/mridha/businesssaas/internal/platform/forms"
)

// ── Response builders ────────────────────────────────────────────────────────

func withBool(r *forms.Response, v bool) *forms.Response {
	r.AnswerBoolean = &v
	return r
}

func withNumber(r *forms.Response, s string) *forms.Response {
	d := dec(s)
	r.AnswerNumber = &d
	return r
}

func withText(r *forms.Response, s string) *forms.Response {
	r.AnswerText = &s
	return r
}

func withOptions(r *forms.Response, opts ...string) *forms.Response {
	r.AnswerOptions = opts
	return r
}

// ── Shape assertions ─────────────────────────────────────────────────────────

func reflectTypeOf(v any) reflect.Type { return reflect.TypeOf(v) }

// assertNoForbiddenFields walks a struct type — following embedded structs and
// pointers-to-struct, since an embedded type is exactly how a forbidden field
// sneaks back in — and fails if any field name contains a forbidden substring.
//
// This is the structural guard that keeps a correct answer out of a
// learner-facing DTO. The equivalent in internal/tests/unit/hrm/feedback
// keeps a respondent's identity out of an anonymised response.
func assertNoForbiddenFields(t *testing.T, typ reflect.Type, forbidden []string) {
	t.Helper()
	seen := map[reflect.Type]bool{}
	walkFields(t, typ, forbidden, seen, "")
}

func walkFields(t *testing.T, typ reflect.Type, forbidden []string, seen map[reflect.Type]bool, path string) {
	t.Helper()
	for typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct || seen[typ] {
		return
	}
	seen[typ] = true

	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		name := strings.ToLower(f.Name)
		for _, bad := range forbidden {
			if strings.Contains(name, bad) {
				t.Errorf("%s%s.%s must not exist — this type is served to a learner",
					path, typ.Name(), f.Name)
			}
		}
		if f.Anonymous {
			walkFields(t, f.Type, forbidden, seen, typ.Name()+".")
		}
	}
}

// unused keeps decimal imported for builders that take numeric literals.
var _ = decimal.Zero

// mustJSON serialises a value so a test can assert a secret appears nowhere in
// the payload — the check that catches a leak through a field nobody thought
// to look at. The equivalent in the feedback tests does this for identity.
func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

func containsFold(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}
