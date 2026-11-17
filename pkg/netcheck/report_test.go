package netcheck

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestReportMarshalling(t *testing.T) {
	in := Report{BundleID: "b", Description: "d", Started: time.Date(2026, 11, 17, 10, 0, 0, 0, time.UTC), Duration: 1500 * time.Millisecond,
		Results: []Result{{Check: Check{Address: "h:1", Protocol: TCP}, Status: Passed, Attempts: 1}}}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"bundle_id":"b"`, `"description":"d"`, `"duration":"1.5s"`, `"started":"2026-11-17T10:00:00Z"`, `"results":[`, `"status":"passed"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("missing %s in %s", want, data)
		}
	}
	var out Report
	if err := json.Unmarshal(data, &out); err != nil || out.BundleID != "b" || out.Duration != in.Duration || !out.Started.Equal(in.Started) || len(out.Results) != 1 {
		t.Fatalf("%+v %v", out, err)
	}
	y, err := yaml.Marshal(in)
	if err != nil || !strings.Contains(string(y), "duration: 1.5s") || !strings.Contains(string(y), "bundle_id: b") {
		t.Fatalf("%s %v", y, err)
	}
	out = Report{}
	if err := yaml.Unmarshal(y, &out); err != nil || out.Description != "d" || out.Duration != in.Duration {
		t.Fatalf("%+v %v", out, err)
	}
}

func TestRunFillsTheReportDescription(t *testing.T) {
	rep, err := Run(t.Context(), &Bundle{ID: "x", Description: "about x", Checks: []Check{{Address: listener(t), Protocol: TCP, Retries: 1}}})
	if err != nil || rep.Description != "about x" {
		t.Fatalf("%+v %v", rep, err)
	}
}
