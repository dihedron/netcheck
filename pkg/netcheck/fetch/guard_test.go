package fetch

import (
	"os/exec"
	"strings"
	"testing"
)

// T-11: the heavy sources are plug-ins; fetch itself does not pull in their libraries.
func TestFetchDoesNotImportTheHeavySources(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", ".").Output()
	if err != nil {
		t.Skipf("cannot run go list: %v", err)
	}
	for _, pkg := range strings.Fields(string(out)) {
		for _, banned := range []string{"github.com/redis/", "github.com/hashicorp/consul", "github.com/pion/", "github.com/dpotapov/"} {
			if strings.HasPrefix(pkg, banned) {
				t.Errorf("fetch imports %s", pkg)
			}
		}
	}
}
