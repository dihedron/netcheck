package extensions

import (
	"strings"
	"testing"
	"text/template"
)

func TestFuncMapColoursAreUsableInTemplates(t *testing.T) {
	funcs := FuncMap()
	for _, name := range []string{"blue", "cyan", "green", "magenta", "purple", "red", "yellow", "white",
		"hiblue", "hicyan", "higreen", "himagenta", "hipurple", "hired", "hiyellow", "hiwhite"} {
		if _, ok := funcs[name]; !ok {
			t.Errorf("function %s missing", name)
			continue
		}
		tpl, err := template.New(name).Funcs(funcs).Parse("{{ \"x\" | " + name + " }}")
		if err != nil {
			t.Fatal(err)
		}
		var out strings.Builder
		if err := tpl.Execute(&out, nil); err != nil || !strings.Contains(out.String(), "x") {
			t.Errorf("%s: %q, %v", name, out.String(), err)
		}
	}
}
