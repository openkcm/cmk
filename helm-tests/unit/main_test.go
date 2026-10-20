//go:build helmtests

// Package unit_test renders the CMK Helm chart with `helm template` and checks the output.
// Run from the repository root: go test -tags=helmtests ./helm-tests/unit/...
package unit_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const (
	chartPath   = "../../charts/cmk"
	releaseName = "cmk"
)

// appVersion returns the chart's appVersion, the default image tag.
func appVersion(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(chartPath + "/Chart.yaml")
	if err != nil {
		t.Fatalf("reading Chart.yaml: %v", err)
	}
	var chart struct {
		AppVersion string `yaml:"appVersion"`
	}
	err = yaml.Unmarshal(data, &chart)
	if err != nil || chart.AppVersion == "" {
		t.Fatalf("no appVersion in Chart.yaml: %v", err)
	}
	return chart.AppVersion
}

// render runs `helm template` with the given --set/-f/-s arguments and returns the output.
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
	return runHelm(t, append([]string{"template", release, chartPath, "--namespace", "default"}, args...)...)
}

// runHelm runs helm with the given arguments and returns its combined output.
func runHelm(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "helm", args...)
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

// manifest holds the fields of a rendered resource that the tests read.
type manifest struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Data map[string]string `yaml:"data"`
	Spec struct {
		Template struct {
			Metadata struct {
				Annotations map[string]string `yaml:"annotations"`
			} `yaml:"metadata"`
		} `yaml:"template"`
	} `yaml:"spec"`
}

// manifests parses the output of `helm template`.
func manifests(t *testing.T, output string) []manifest {
	t.Helper()
	var result []manifest
	decoder := yaml.NewDecoder(strings.NewReader(output))
	for {
		var m manifest
		err := decoder.Decode(&m)
		if errors.Is(err, io.EOF) {
			return result
		}
		if err != nil {
			t.Fatalf("parsing the rendered manifests: %v", err)
		}
		if m.Kind != "" {
			result = append(result, m)
		}
	}
}

// checksums returns the checksum/config annotation of every Deployment and Job, by name.
func checksums(t *testing.T, output string) map[string]string {
	t.Helper()
	result := map[string]string{}
	for _, m := range manifests(t, output) {
		if m.Kind == "Deployment" || m.Kind == "Job" {
			result[m.Metadata.Name] = m.Spec.Template.Metadata.Annotations["checksum/config"]
		}
	}
	return result
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
