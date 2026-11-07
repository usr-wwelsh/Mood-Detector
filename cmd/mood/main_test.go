package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/usr-wwelsh/mood-detector/internal/mood"
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

func writeTone(t *testing.T, path string, hz int) {
	t.Helper()
	if err := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", fmt.Sprintf("sine=frequency=%d:duration=3", hz), path).Run(); err != nil {
		t.Fatal(err)
	}
}

func TestBenchExtractCachesFeaturesAndSeedsLabelsWithoutClobbering(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	tracks, bench := t.TempDir(), t.TempDir()
	writeTone(t, filepath.Join(tracks, "a.wav"), 440)
	var out, errOut bytes.Buffer
	if code := run([]string{"bench", "extract", "--dir", bench, tracks}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	feats, err := mood.LoadFeatures(filepath.Join(bench, "features.json"))
	if err != nil || len(feats) != 1 {
		t.Fatalf("feats=%v err=%v", feats, err)
	}
	var id string
	for id = range feats {
	}
	names, _ := os.ReadFile(filepath.Join(bench, "names.local.json"))
	if !strings.Contains(string(names), "a.wav") {
		t.Fatalf("names: %s", names)
	}
	labels := map[string]mood.Label{id: {Key: "A major"}}
	if err := mood.SaveLabels(filepath.Join(bench, "labels.json"), labels); err != nil {
		t.Fatal(err)
	}
	writeTone(t, filepath.Join(tracks, "b.wav"), 880)
	if code := run([]string{"bench", "extract", "--dir", bench, tracks}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	feats, _ = mood.LoadFeatures(filepath.Join(bench, "features.json"))
	got, _ := mood.LoadLabels(filepath.Join(bench, "labels.json"))
	if len(feats) != 2 || len(got) != 2 || got[id].Key != "A major" {
		t.Fatalf("feats=%d labels=%v", len(feats), got)
	}
}

func TestBenchRunPrintsScoresAndFailsBelowThreshold(t *testing.T) {
	bench := t.TempDir()
	f := mood.Features{Tempo: 128, TempoConfidence: 0.5, Energy: 0.6, SpectralCentroid: 1000, Chroma: make([]float64, 12)}
	if err := mood.SaveFeatures(filepath.Join(bench, "features.json"), map[string]mood.Features{"a": f}); err != nil {
		t.Fatal(err)
	}
	if err := mood.SaveLabels(filepath.Join(bench, "labels.json"), map[string]mood.Label{"a": {Tempo: 128}}); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := run([]string{"bench", "run", "--dir", bench}, &out, &errOut); code != 0 || !strings.Contains(out.String(), "tempo") {
		t.Fatalf("code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
	if err := mood.SaveLabels(filepath.Join(bench, "labels.json"), map[string]mood.Label{"a": {Tempo: 60}}); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if code := run([]string{"bench", "run", "--dir", bench, "--min-tempo", "50"}, &out, &errOut); code != 1 {
		t.Fatalf("want exit 1 below threshold, got %d", code)
	}
}

func TestBenchRunWithoutFeaturesSuggestsExtract(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"bench", "run", "--dir", t.TempDir()}, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "bench extract") {
		t.Fatalf("code=%d err=%q", code, errOut.String())
	}
}
