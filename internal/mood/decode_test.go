package mood

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestAnalysisWindowMatchesPythonSelection(t *testing.T) {
	cases := []struct{ total, offset, duration float64 }{
		{200, 30, 30}, {61, 30, 30}, {60, 15, 30 - 0}, {40, 10, 30}, {31, 7.75, 23.25}, {10, 0, 10},
	}
	for _, c := range cases {
		o, d := analysisWindow(c.total)
		if o != c.offset || d != c.duration {
			t.Errorf("total %v: got (%v,%v) want (%v,%v)", c.total, o, d, c.offset, c.duration)
		}
	}
}

func TestAnalyzeFileRejectsMissingAndNonAudio(t *testing.T) {
	if _, err := AnalyzeFile(filepath.Join(t.TempDir(), "nope.mp3")); err == nil {
		t.Error("missing file: want error")
	}
	junk := filepath.Join(t.TempDir(), "junk.mp3")
	if err := os.WriteFile(junk, []byte("not audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := AnalyzeFile(junk); err == nil {
		t.Error("non-audio file: want error")
	}
}

func TestAnalyzeFileEndToEndOnGeneratedTone(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	wav := filepath.Join(t.TempDir(), "tone.wav")
	if err := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "sine=frequency=440:duration=5", wav).Run(); err != nil {
		t.Fatal(err)
	}
	r, err := AnalyzeFile(wav)
	if err != nil {
		t.Fatal(err)
	}
	if r.Key != "A major" && r.Key != "A minor" {
		t.Errorf("key: got %q want A", r.Key)
	}
	if r.Mood == "" {
		t.Error("empty mood")
	}
}

func TestStereoDownmixAveragesChannelsLikeLibrosa(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	dir := t.TempDir()
	mono, stereo := filepath.Join(dir, "m.wav"), filepath.Join(dir, "s.wav")
	if err := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "sine=frequency=440:duration=3", mono).Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("ffmpeg", "-v", "error", "-filter_complex",
		"sine=frequency=440:duration=3[a];sine=frequency=440:duration=3[b];[a][b]amerge=inputs=2", stereo).Run(); err != nil {
		t.Fatal(err)
	}
	m, err := AnalyzeFile(mono)
	if err != nil {
		t.Fatal(err)
	}
	s, err := AnalyzeFile(stereo)
	if err != nil {
		t.Fatal(err)
	}
	if d := m.Energy - s.Energy; d > 0.005 || d < -0.005 {
		t.Errorf("mono energy %.4f != stereo energy %.4f", m.Energy, s.Energy)
	}
}
