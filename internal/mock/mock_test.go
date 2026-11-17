package mock

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/dihedron/netcheck/v2/pkg/netcheck"
)

// D-15: the mock follows the library. A field added to a library type without
// the matching method here makes this test fail.
func TestMockHasEveryLibraryField(t *testing.T) {
	for _, pair := range []struct{ library, mock reflect.Type }{
		{reflect.TypeOf(netcheck.Report{}), reflect.TypeOf(&Report{})},
		{reflect.TypeOf(netcheck.Result{}), reflect.TypeOf(&Result{})},
		{reflect.TypeOf(netcheck.Check{}), reflect.TypeOf(&Check{})},
		{reflect.TypeOf(netcheck.Detail{}), reflect.TypeOf(&Detail{})},
		{reflect.TypeOf(netcheck.TLSFacts{}), reflect.TypeOf(&TLSFacts{})},
	} {
		for i := range pair.library.NumField() {
			f := pair.library.Field(i)
			if !f.IsExported() || f.Tag.Get("json") == "-" { // API-only fields are not in the output
				continue
			}
			if _, ok := pair.mock.MethodByName(f.Name); !ok {
				t.Errorf("the mock %s has no %s: add it to internal/mock (and to Diagnostics)", pair.mock.Elem().Name(), f.Name)
			}
		}
	}
}

// Diagnostics shows every field of the mock, so that the mock and its
// printout cannot drift apart.
func TestDiagnosticsListsEveryMethod(t *testing.T) {
	var buf bytes.Buffer
	Diagnostics(&buf, Reports(), func(f string, a ...any) string { return "*" + fmt.Sprintf(f, a...) })
	out := buf.String()
	for _, typ := range []reflect.Type{reflect.TypeOf(&Report{}), reflect.TypeOf(&Result{}), reflect.TypeOf(&Check{}), reflect.TypeOf(&Detail{}), reflect.TypeOf(&TLSFacts{})} {
		for i := range typ.NumMethod() {
			if name := "." + typ.Method(i).Name; !strings.Contains(out, name) {
				t.Errorf("Diagnostics does not list %s of %s", name, typ.Elem().Name())
			}
		}
	}
	if strings.Contains(out, "*.") {
		t.Errorf("nothing was read yet:\n%s", out)
	}
}

func TestAccessIsTracked(t *testing.T) {
	reports := Reports()
	r := reports[1]
	_ = r.BundleID()
	res := r.Results()[0]
	_ = res.Check().Address()
	_ = res.Detail().TLS().Issuer()
	var buf bytes.Buffer
	Diagnostics(&buf, reports, func(f string, a ...any) string { return "*" + fmt.Sprintf(f, a...) })
	for _, want := range []string{"*.BundleID", "*.Results [", "*.Check {", "*.Address", "*.Detail {", "*.TLS {", "*.Issuer"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("%s not marked:\n%s", want, buf.String())
		}
	}
	if strings.Contains(buf.String(), "*.Description") || strings.Contains(buf.String(), "*.Subject") {
		t.Error("unread fields must not be marked")
	}
}

func TestReportsAreFresh(t *testing.T) {
	a := Reports()
	_ = a[0].BundleID()
	if Reports()[0].bundleID.Accessed() {
		t.Error("every call returns new, unread data")
	}
}
