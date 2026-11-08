package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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
	if !strings.Contains(errOut.String(), "extracting a.wav") {
		t.Fatalf("no progress: %q", errOut.String())
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

func jamendoServer(t *testing.T, hits *int) *httptest.Server {
	t.Helper()
	tsv := "H\n" +
		"t1\ta\tb\t10/1.mp3\t1\tmood/theme---sad\n" +
		"t2\ta\tb\t10/2.mp3\t1\tmood/theme---sad\n" +
		"t3\ta\tb\t11/3.mp3\t1\tmood/theme---happy\n"
	mux := http.NewServeMux()
	mux.HandleFunc("/meta.tsv", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, tsv) })
	mux.HandleFunc("/audio/", func(w http.ResponseWriter, r *http.Request) {
		*hits++
		fmt.Fprint(w, "bytes of "+r.URL.Path)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestBenchFetchDownloadsCappedSliceAndSeedsMoodLabels(t *testing.T) {
	hits := 0
	srv := jamendoServer(t, &hits)
	bench := t.TempDir()
	args := []string{"bench", "fetch", "--set", "jamendo", "--dir", bench, "--per-mood", "1", "--meta-url", srv.URL + "/meta.tsv", "--audio-url", srv.URL + "/audio"}
	var out, errOut bytes.Buffer
	if code := run(args, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if hits != 2 {
		t.Fatalf("downloaded %d tracks, want 1 per mood", hits)
	}
	for _, want := range []string{"[1/2]", "[2/2]", "happy", "sad"} {
		if !strings.Contains(errOut.String(), want) {
			t.Fatalf("progress missing %q: %s", want, errOut.String())
		}
	}
	labels, err := mood.LoadLabels(filepath.Join(bench, "labels.json"))
	if err != nil || len(labels) != 2 {
		t.Fatalf("%v %v", labels, err)
	}
	for _, l := range labels {
		if len(l.Mood) == 0 {
			t.Fatalf("unlabeled: %+v", labels)
		}
	}
	files, _ := filepath.Glob(filepath.Join(bench, "audio", "*", "*.low.mp3"))
	if len(files) != 2 {
		t.Fatalf("%v", files)
	}
	if code := run(args, &out, &errOut); code != 0 || hits != 2 {
		t.Fatalf("rerun exit %d, hits %d", code, hits)
	}
}

func TestBenchFetchFailsOnBadDownloadStatus(t *testing.T) {
	hits := 0
	srv := jamendoServer(t, &hits)
	var out, errOut bytes.Buffer
	args := []string{"bench", "fetch", "--set", "jamendo", "--dir", t.TempDir(), "--meta-url", srv.URL + "/meta.tsv", "--audio-url", srv.URL + "/missing"}
	if code := run(args, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "404") {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
}

type gsFixture struct{ id, bpm, genre, audio string }

func giantStepsServer(t *testing.T, tracks []gsFixture, corruptAudio bool) *httptest.Server {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	add := func(name, body string) {
		if err := tw.WriteHeader(&tar.Header{Name: "repo/" + name, Mode: 0o600, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		_, _ = tw.Write([]byte(body))
	}
	mux := http.NewServeMux()
	for _, g := range tracks {
		sum := md5.Sum([]byte(g.audio))
		add("md5/"+g.id+".md5", hex.EncodeToString(sum[:]))
		add("annotations_v2/tempo/"+g.id+".bpm", g.bpm)
		add("annotations/genre/"+g.id+".genre", g.genre)
		body := g.audio
		if corruptAudio {
			body = "tampered"
		}
		mux.HandleFunc("/audio/"+g.id+".mp3", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) })
	}
	_ = tw.Close()
	_ = gz.Close()
	mux.HandleFunc("/gs.tgz", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(buf.Bytes()) })
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

var gsTracks = []gsFixture{
	{"1.LOFI", "70.0", "chill-out", "slow track"},
	{"2.LOFI", "128.0", "house", "mid track"},
	{"3.LOFI", "174.0", "drum-and-bass", "fast track"},
	{"4.LOFI", "0.0", "techno", "no tempo"},
}

func gsArgs(srv *httptest.Server, dir string, extra ...string) []string {
	return append([]string{"bench", "fetch", "--set", "giantsteps", "--dir", dir, "--gs-meta-url", srv.URL + "/gs.tgz", "--gs-audio-url", srv.URL + "/audio"}, extra...)
}

func TestBenchFetchGiantStepsDownloadsOutliersWithTempoAndStyleLabels(t *testing.T) {
	srv := giantStepsServer(t, gsTracks, false)
	bench := t.TempDir()
	var out, errOut bytes.Buffer
	if code := run(gsArgs(srv, bench, "--count", "2", "--outliers", "1"), &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	labels, err := mood.LoadLabels(filepath.Join(bench, "labels.json"))
	if err != nil || len(labels) != 2 {
		t.Fatalf("%v %v", labels, err)
	}
	var tempos []float64
	for _, l := range labels {
		tempos = append(tempos, l.Tempo)
	}
	if !(tempos[0] == 70 && tempos[1] == 174 || tempos[0] == 174 && tempos[1] == 70) {
		t.Fatalf("expected slowest and fastest, got %v", tempos)
	}
	files, _ := filepath.Glob(filepath.Join(bench, "audio", "giantsteps", "*.mp3"))
	if len(files) != 2 {
		t.Fatalf("%v", files)
	}
	for _, want := range []string{"[1/2]", "[2/2]"} {
		if !strings.Contains(errOut.String(), want) {
			t.Fatalf("progress missing %q: %s", want, errOut.String())
		}
	}
}

func TestBenchFetchGiantStepsRejectsAudioFailingMD5(t *testing.T) {
	srv := giantStepsServer(t, gsTracks, true)
	var out, errOut bytes.Buffer
	if code := run(gsArgs(srv, t.TempDir(), "--count", "2", "--outliers", "1"), &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "md5") {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
}

func TestBenchFetchRejectsUnknownSet(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"bench", "fetch", "--set", "nope", "--dir", t.TempDir()}, &out, &errOut); code != 2 {
		t.Fatalf("exit %d", code)
	}
}
