package mood

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"testing"
)

func buildTarball(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		if err := tw.WriteHeader(&tar.Header{Name: "repo-abc/" + name, Mode: 0o600, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func gsFiles(id, md5, bpm, genre string) map[string]string {
	return map[string]string{
		"md5/" + id + ".md5":                  md5,
		"annotations_v2/tempo/" + id + ".bpm": bpm,
		"annotations/genre/" + id + ".genre":  genre,
	}
}

const md5a = "b74960065c4493b89b56c3e49671f90d"

func TestParseGiantStepsJoinsTempoGenreAndMD5ById(t *testing.T) {
	got, err := ParseGiantSteps(bytes.NewReader(buildTarball(t, gsFiles("1030011.LOFI", md5a, "127.0", "electronica"))))
	if err != nil {
		t.Fatal(err)
	}
	want := GSTrack{ID: "1030011.LOFI", BPM: 127, Genre: "electronica", MD5: md5a}
	if len(got) != 1 || got[0] != want {
		t.Fatalf("%+v", got)
	}
}

func TestParseGiantStepsDropsTracksWithoutUsableTempo(t *testing.T) {
	got, err := ParseGiantSteps(bytes.NewReader(buildTarball(t, gsFiles("1.LOFI", md5a, "0.0", "techno"))))
	if err != nil || len(got) != 0 {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestParseGiantStepsRejectsIdsOutsideAllowlist(t *testing.T) {
	for _, id := range []string{"../evil.LOFI", "1.LOFI/../../x", "a b.LOFI", "1.mp3"} {
		if _, err := ParseGiantSteps(bytes.NewReader(buildTarball(t, gsFiles(id, md5a, "120", "techno")))); err == nil {
			t.Fatalf("accepted %q", id)
		}
	}
}

func TestParseGiantStepsRejectsMalformedMD5(t *testing.T) {
	if _, err := ParseGiantSteps(bytes.NewReader(buildTarball(t, gsFiles("1.LOFI", "nothex", "120", "techno")))); err == nil {
		t.Fatal("accepted bad md5")
	}
}

func tempoRamp(n int) []GSTrack {
	var out []GSTrack
	for i := 0; i < n; i++ {
		out = append(out, GSTrack{ID: fmt.Sprintf("%03d.LOFI", i), BPM: float64(60 + i)})
	}
	return out
}

func TestPickTempoSliceAlwaysIncludesSlowestAndFastestOutliers(t *testing.T) {
	got := PickTempoSlice(tempoRamp(100), 10, 2)
	if len(got) != 10 {
		t.Fatalf("%d", len(got))
	}
	have := map[string]bool{}
	for _, tr := range got {
		if have[tr.ID] {
			t.Fatalf("duplicate %s", tr.ID)
		}
		have[tr.ID] = true
	}
	for _, id := range []string{"000.LOFI", "001.LOFI", "098.LOFI", "099.LOFI"} {
		if !have[id] {
			t.Fatalf("missing outlier %s in %+v", id, got)
		}
	}
}

func TestPickTempoSliceIsDeterministicAndReturnsAllWhenSmall(t *testing.T) {
	a, b := PickTempoSlice(tempoRamp(50), 7, 1), PickTempoSlice(tempoRamp(50), 7, 1)
	for i := range a {
		if a[i] != b[i] {
			t.Fatal("not deterministic")
		}
	}
	if got := PickTempoSlice(tempoRamp(5), 10, 2); len(got) != 5 {
		t.Fatalf("%d", len(got))
	}
}

func TestGSLabelMapsGenreToClassifierStylesAndKeepsTempo(t *testing.T) {
	l := GSLabel(GSTrack{BPM: 174, Genre: "drum-and-bass"})
	if l.Tempo != 174 || len(l.Style) != 1 || l.Style[0] != "Drum & Bass" {
		t.Fatalf("%+v", l)
	}
	if l := GSLabel(GSTrack{BPM: 140, Genre: "dubstep"}); l.Tempo != 140 || len(l.Style) != 0 {
		t.Fatalf("unmapped genre got styles: %+v", l)
	}
}
