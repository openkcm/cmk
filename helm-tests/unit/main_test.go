//go:build helmtests

// Package unit_test renders the CMK Helm chart with `helm template` and checks the output.
// Run from the repository root: go test -tags=helmtests ./helm-tests/unit/...
package unit_test

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

const (
	chartPath   = "../../charts/cmk"
	releaseName = "cmk"
)

// render runs `helm template` with the given --set/-f/-s arguments and returns the output.
// The image tag is set, because the chart requires it.
func render(t *testing.T, args ...string) string {
	t.Helper()
	out, err := renderErr(t, args...)
	if err != nil {
		t.Fatalf("helm template failed: %v\nOutput: %s", err, out)
	}
	return out
}

// renderErr is render without failing the test, for checks of template errors.
func renderErr(t *testing.T, args ...string) (string, error) {
	t.Helper()
	return renderRelease(t, releaseName, args...)
}

// renderRelease is renderErr with another release name.
func renderRelease(t *testing.T, release string, args ...string) (string, error) {
	t.Helper()
	cmdArgs := append([]string{"template", release, chartPath, "--namespace", "default", "--set", "image.tag=v0.11.0"}, args...)
	cmd := exec.CommandContext(t.Context(), "helm", cmdArgs...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}

// document returns the rendered document of the given kind and metadata.name, or "".
func document(output, kind, name string) string {
	for _, doc := range strings.Split(output, "\n---\n") {
		if strings.Contains(doc, "\nkind: "+kind+"\n") && strings.Contains(doc, "\n  name: "+name+"\n") {
			return doc
		}
	}
	return ""
}

// split turns "--set a=b --set c=d" into arguments.
func split(values string) []string {
	if values == "" {
		return nil
	}
	return strings.Split(values, " ")
}

// checkContains reports every expected string missing from output, and every unexpected one present.
func checkContains(t *testing.T, output string, expected, unexpected []string) {
	t.Helper()
	for _, e := range expected {
		if !strings.Contains(output, e) {
			t.Errorf("expected output to contain %q, but it didn't.\nOutput: %s", e, output)
		}
	}
	for _, u := range unexpected {
		if strings.Contains(output, u) {
			t.Errorf("expected output NOT to contain %q, but it did.\nOutput: %s", u, output)
		}
	}
}
