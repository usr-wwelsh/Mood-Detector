package main

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnalyzeJSONPrintsPythonWorkerShape(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	wav := filepath.Join(t.TempDir(), "tone.wav")
	if err := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "sine=frequency=440:duration=3", wav).Run(); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := run([]string{"analyze", "--json", wav}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	var m map[string]any
	if err := json.Unmarshal(out.Bytes(), &m); err != nil {
		t.Fatalf("stdout not JSON: %v\n%s", err, out.String())
	}
	for _, k := range []string{"mood", "energy", "tempo", "key", "similarity_scores", "explanation"} {
		if _, ok := m[k]; !ok {
			t.Errorf("missing %q", k)
		}
	}
}

func TestAnalyzeMissingFileFailsWithJSONErrorOnStderr(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"analyze", "--json", "/nonexistent/x.mp3"}, &out, &errOut)
	if code != 1 || out.Len() != 0 {
		t.Fatalf("code=%d stdout=%q", code, out.String())
	}
	var m map[string]string
	if err := json.Unmarshal(errOut.Bytes(), &m); err != nil || m["error"] == "" {
		t.Fatalf("stderr not {error}: %q", errOut.String())
	}
}

func TestUnknownCommandPrintsUsage(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"bogus"}, &out, &errOut); code != 2 || !strings.Contains(errOut.String(), "usage") {
		t.Fatalf("code=%d err=%q", code, errOut.String())
	}
}
