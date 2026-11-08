package mood

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

var techno = Features{Tempo: 128, TempoConfidence: 0.5, Energy: 0.6, SpectralCentroid: 1000, Chroma: cMajorChroma()}

func cMajorChroma() []float64 {
	c := make([]float64, 12)
	c[0], c[4] = 1, 0.6
	return c
}

func evalOne(f Features, l Label) Report {
	return Evaluate(map[string]Features{"a": f}, map[string]Label{"a": l})
}

func TestMoodMatchesAnyAcceptedLabel(t *testing.T) {
	r := evalOne(techno, Label{Mood: []string{"House/Dance", "Techno/Dark"}})
	if r.Mood.Right != 1 || r.Mood.Total != 1 {
		t.Fatalf("%+v", r.Mood)
	}
}

func TestMoodMissIsReportedWithPredictedAndExpected(t *testing.T) {
	r := evalOne(techno, Label{Mood: []string{"Ambient/Drone"}})
	if r.Mood.Right != 0 || len(r.Misses) != 1 {
		t.Fatalf("%+v", r)
	}
	m := r.Misses[0]
	if m.ID != "a" || m.Field != "mood" || m.Got != "Techno/Dark" || m.Want != "Ambient/Drone" {
		t.Fatalf("%+v", m)
	}
}

func TestTempoWithinFourPercentIsExact(t *testing.T) {
	if r := evalOne(techno, Label{Tempo: 124}); r.Tempo.Right != 1 {
		t.Fatalf("%+v", r.Tempo)
	}
	if r := evalOne(techno, Label{Tempo: 110}); r.Tempo.Right != 0 {
		t.Fatalf("%+v", r.Tempo)
	}
}

func TestTempoOctaveErrorCountsOnlyInOctaveTolerantScore(t *testing.T) {
	r := evalOne(techno, Label{Tempo: 64})
	if r.Tempo.Right != 0 || r.TempoOctave.Right != 1 {
		t.Fatalf("exact %+v octave %+v", r.Tempo, r.TempoOctave)
	}
	r = evalOne(techno, Label{Tempo: 256})
	if r.TempoOctave.Right != 1 {
		t.Fatalf("%+v", r.TempoOctave)
	}
}

func TestKeyExactAndRelativeScoredSeparately(t *testing.T) {
	if r := evalOne(techno, Label{Key: "C major"}); r.Key.Right != 1 || r.KeyRelaxed.Right != 1 {
		t.Fatalf("%+v %+v", r.Key, r.KeyRelaxed)
	}
	r := evalOne(techno, Label{Key: "A minor"})
	if r.Key.Right != 0 || r.KeyRelaxed.Right != 1 {
		t.Fatalf("exact %+v relaxed %+v", r.Key, r.KeyRelaxed)
	}
}

func TestUnlabeledFieldsAreNotScored(t *testing.T) {
	r := evalOne(techno, Label{Mood: []string{"Techno/Dark"}})
	if r.Tempo.Total != 0 || r.Key.Total != 0 || r.Mood.Total != 1 {
		t.Fatalf("%+v", r)
	}
}

func TestTracksWithoutFeaturesOrLabelsAreCounted(t *testing.T) {
	r := Evaluate(
		map[string]Features{"a": techno, "orphan": techno},
		map[string]Label{"a": {Tempo: 128}, "gone": {Tempo: 90}},
	)
	if r.Unlabeled != 1 || r.MissingFeatures != 1 || r.Tempo.Total != 1 {
		t.Fatalf("%+v", r)
	}
}

func TestMisslistIsSortedByID(t *testing.T) {
	feats := map[string]Features{"b": techno, "a": techno}
	labels := map[string]Label{"b": {Tempo: 60}, "a": {Tempo: 60}}
	r := Evaluate(feats, labels)
	if len(r.Misses) != 2 || r.Misses[0].ID != "a" {
		t.Fatalf("%+v", r.Misses)
	}
}

func TestReportPrintsEachScoreAndMiss(t *testing.T) {
	r := evalOne(techno, Label{Mood: []string{"Ambient/Drone"}, Tempo: 128})
	var b bytes.Buffer
	r.Write(&b)
	out := b.String()
	for _, want := range []string{"mood", "0/1", "tempo", "1/1", "Techno/Dark", "Ambient/Drone"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
}

func TestFeaturesFileRoundTrips(t *testing.T) {
	p := filepath.Join(t.TempDir(), "features.json")
	in := map[string]Features{"a": techno}
	if err := SaveFeatures(p, in); err != nil {
		t.Fatal(err)
	}
	out, err := LoadFeatures(p)
	if err != nil {
		t.Fatal(err)
	}
	if out["a"].Tempo != 128 || len(out["a"].Chroma) != 12 {
		t.Fatalf("%+v", out)
	}
}

func TestFeaturesFromOtherExtractorVersionAreRejected(t *testing.T) {
	p := filepath.Join(t.TempDir(), "features.json")
	if err := writeFile(p, []byte(`{"version":-1,"tracks":{}}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFeatures(p); err == nil || !strings.Contains(err.Error(), "version") {
		t.Fatalf("err=%v", err)
	}
}

func TestLabelsFileRoundTrips(t *testing.T) {
	p := filepath.Join(t.TempDir(), "labels.json")
	in := map[string]Label{"a": {Mood: []string{"Techno/Dark"}, Tempo: 128, Key: "C major"}}
	if err := SaveLabels(p, in); err != nil {
		t.Fatal(err)
	}
	out, err := LoadLabels(p)
	if err != nil {
		t.Fatal(err)
	}
	if out["a"].Key != "C major" || out["a"].Mood[0] != "Techno/Dark" {
		t.Fatalf("%+v", out)
	}
}

func TestMissingLabelsFileLoadsEmpty(t *testing.T) {
	out, err := LoadLabels(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil || len(out) != 0 {
		t.Fatalf("%v %v", out, err)
	}
}

func TestTrackIDIsStableContentHash(t *testing.T) {
	d := t.TempDir()
	a, b := filepath.Join(d, "a.mp3"), filepath.Join(d, "renamed.mp3")
	for _, p := range []string{a, b} {
		if err := writeFile(p, []byte("same bytes")); err != nil {
			t.Fatal(err)
		}
	}
	ia, err := TrackID(a)
	if err != nil {
		t.Fatal(err)
	}
	ib, _ := TrackID(b)
	if ia != ib || len(ia) != 12 {
		t.Fatalf("%q %q", ia, ib)
	}
}

func TestStyleScoresClassifierLabelIndependentlyOfMood(t *testing.T) {
	r := evalOne(techno, Label{Style: []string{"Techno/Dark"}, Mood: []string{"Ambient/Drone"}})
	if r.Style.Right != 1 || r.Style.Total != 1 || r.Mood.Right != 0 || r.Mood.Total != 1 {
		t.Fatalf("style %+v mood %+v", r.Style, r.Mood)
	}
	r = evalOne(techno, Label{Style: []string{"Drum & Bass"}})
	if r.Style.Right != 0 || len(r.Misses) != 1 || r.Misses[0].Field != "style" {
		t.Fatalf("%+v", r)
	}
}
