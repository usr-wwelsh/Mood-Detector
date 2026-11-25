package mood

import (
	"math"
	"strings"
	"testing"
)

type moodCase struct {
	tempo, energy, centroid float64
	confidence, zcr         float64
	major                   bool
	want                    string
}

func chromaFor(major bool) []float64 {
	c := make([]float64, 12)
	c[0] = 1
	if major {
		c[4] = 0.5
	} else {
		c[3] = 0.5
	}
	return c
}

func (c moodCase) classify() Result {
	conf := c.confidence
	if conf == 0 {
		conf = 0.9
	}
	return Classify(Features{
		Tempo:            c.tempo,
		TempoConfidence:  conf,
		Energy:           c.energy,
		SpectralCentroid: c.centroid,
		ZeroCrossingRate: c.zcr,
		Chroma:           chromaFor(c.major),
	})
}

func TestClassifyThresholdBoundaries(t *testing.T) {
	cases := []moodCase{
		{tempo: 90, energy: 0.14, centroid: 2500, confidence: 0.19, want: "Ambient/Drone"},
		{tempo: 90, energy: 0.15, centroid: 2500, confidence: 0.19, want: "Atmospheric/Textural"},
		{tempo: 90, energy: 0.49, centroid: 2500, confidence: 0.19, want: "Atmospheric/Textural"},
		{tempo: 90, energy: 0.50, centroid: 1499, confidence: 0.19, want: "Atmospheric/Textural"},
		{tempo: 90, energy: 0.50, centroid: 1500, confidence: 0.19, want: "Noise/Experimental"},
		{tempo: 90, energy: 0.69, centroid: 1500, confidence: 0.19, want: "Noise/Experimental"},
		{tempo: 90, energy: 0.70, centroid: 1500, confidence: 0.19, want: "Harsh Noise/Experimental"},
		{tempo: 90, energy: 0.14, centroid: 2500, confidence: 0.2, want: "Midtempo Groove"},
		{tempo: 90, energy: 0.14, centroid: 2500, confidence: 0.19, want: "Ambient/Drone"},

		{tempo: 128, energy: 0.8, centroid: 2500, zcr: 0.25, want: "Techno/Industrial"},
		{tempo: 128, energy: 0.8, centroid: 2500, zcr: 0.26, want: "Harsh Noise/Experimental"},
		{tempo: 128, energy: 0.3, centroid: 2500, zcr: 0.26, want: "Glitch/Experimental"},
		{tempo: 128, energy: 0.31, centroid: 2500, zcr: 0.26, want: "Harsh Noise/Experimental"},

		{tempo: 120, energy: 0.7, centroid: 2500, want: "Techno/Industrial"},
		{tempo: 119.9, energy: 0.7, centroid: 3000, major: true, want: "House/Dance"},
		{tempo: 135, energy: 0.7, centroid: 2500, want: "Techno/Industrial"},
		{tempo: 135.1, energy: 0.7, centroid: 2500, want: "Driving Electronic"},
		{tempo: 128, energy: 0.69, centroid: 2750, major: true, want: "House/Dance"},
		{tempo: 128, energy: 0.7, centroid: 1999, want: "Techno/Dark"},
		{tempo: 128, energy: 0.7, centroid: 2000, want: "Techno/Industrial"},

		{tempo: 128, energy: 0.6, centroid: 2501, major: true, want: "House/Dance"},
		{tempo: 128, energy: 0.6, centroid: 2501, want: "Dark House"},
		{tempo: 128, energy: 0.6, centroid: 2500, major: true, want: "Club/Groovy"},
		{tempo: 135, energy: 0.6, centroid: 2500, want: "Club/Groovy"},
		{tempo: 135.1, energy: 0.6, centroid: 2500, want: "Driving Electronic"},
		{tempo: 120, energy: 0.6, centroid: 2500, want: "Club/Groovy"},
		{tempo: 119.9, energy: 0.6, centroid: 2500, want: "Disco/Funk"},
		{tempo: 128, energy: 0.5, centroid: 2500, want: "Club/Groovy"},
		{tempo: 128, energy: 0.49, centroid: 2500, want: "Disco/Funk"},

		{tempo: 128, energy: 0.2, centroid: 1000, want: "Techno/Minimal"},
		{tempo: 128, energy: 0.19, centroid: 1000, want: "Club/Groovy"},
		{tempo: 128, energy: 0.49, centroid: 1000, want: "Techno/Minimal"},
		{tempo: 128, energy: 0.5, centroid: 1000, want: "Club/Groovy"},
		{tempo: 128, energy: 0.3, centroid: 1249, want: "Techno/Minimal"},
		{tempo: 128, energy: 0.3, centroid: 1250, want: "Club/Groovy"},
		{tempo: 120, energy: 0.3, centroid: 1000, want: "Techno/Minimal"},
		{tempo: 119.9, energy: 0.3, centroid: 1000, want: "Club/Groovy"},
		{tempo: 135, energy: 0.3, centroid: 1000, want: "Techno/Minimal"},
		{tempo: 135.1, energy: 0.3, centroid: 1000, want: "Driving Electronic"},

		{tempo: 100, energy: 0.3, centroid: 2500, want: "Disco/Funk"},
		{tempo: 99.9, energy: 0.3, centroid: 2500, want: "Moderate Groove"},
		{tempo: 110, energy: 0.15, centroid: 2500, want: "Disco/Funk"},
		{tempo: 110, energy: 0.14, centroid: 2500, want: "Minimal/Sparse"},
		{tempo: 130, energy: 0.3, centroid: 2500, want: "Disco/Funk"},
		{tempo: 130.1, energy: 0.3, centroid: 2500, want: "Upbeat/Moderate"},
		{tempo: 115, energy: 0.71, centroid: 3000, want: "Techno/Industrial"},
		{tempo: 115, energy: 0.71, centroid: 2500, want: "Techno/Dark"},
		{tempo: 114.9, energy: 0.71, centroid: 2500, want: "Energetic/Rave"},
		{tempo: 115, energy: 0.7, centroid: 3000, major: true, want: "House/Dance"},
		{tempo: 115, energy: 0.7, centroid: 3000, want: "Dark House"},
		{tempo: 115, energy: 0.49, centroid: 3000, major: true, want: "Disco/Funk"},
		{tempo: 115.1, energy: 0.49, centroid: 3000, major: true, want: "House/Dance"},
		{tempo: 115, energy: 0.5, centroid: 3000, want: "Dark House"},
		{tempo: 110, energy: 0.6, centroid: 2000, want: "Club/Groovy"},
		{tempo: 110, energy: 0.6, centroid: 2001, want: "Disco/Funk"},
		{tempo: 110, energy: 0.6, centroid: 2500, major: true, want: "Disco/Funk"},
		{tempo: 110, energy: 0.6, centroid: 2501, major: true, want: "House/Dance"},
		{tempo: 110, energy: 0.6, centroid: 2501, want: "Dark House"},

		{tempo: 132, energy: 0.6, centroid: 3000, major: true, want: "House/Dance"},
		{tempo: 132, energy: 0.6, centroid: 3000, want: "Dark House"},
		{tempo: 147.5, energy: 0.24, centroid: 2500, want: "Relaxed/Moderate"},
		{tempo: 140, energy: 0.25, centroid: 2500, want: "Driving Electronic"},
		{tempo: 140, energy: 0.24, centroid: 2500, want: "Relaxed/Moderate"},
		{tempo: 147.9, energy: 0.3, centroid: 2500, want: "Driving Electronic"},
		{tempo: 148, energy: 0.3, centroid: 2500, want: "Breakbeat/Fast"},
		{tempo: 148, energy: 0.41, centroid: 2500, want: "Drum & Bass"},
		{tempo: 148, energy: 0.4, centroid: 2500, want: "Breakbeat/Fast"},
		{tempo: 148, energy: 0.26, centroid: 2500, want: "Breakbeat/Fast"},
		{tempo: 148, energy: 0.25, centroid: 2500, want: "Fast/Atmospheric"},
		{tempo: 180, energy: 0.5, centroid: 2500, want: "Drum & Bass"},
		{tempo: 135, energy: 0.3, centroid: 2500, want: "Upbeat/Moderate"},

		{tempo: 180.1, energy: 0.5, centroid: 2500, want: "Driving Electronic"},
		{tempo: 180.1, energy: 0.61, centroid: 2500, want: "Energetic/Rave"},
		{tempo: 180.1, energy: 0.6, centroid: 2500, want: "Driving Electronic"},
		{tempo: 180.1, energy: 0.81, centroid: 2500, want: "Hard/Aggressive"},
		{tempo: 180.1, energy: 0.8, centroid: 2500, want: "Energetic/Rave"},
		{tempo: 90, energy: 0.81, centroid: 2500, want: "Hard/Aggressive"},
		{tempo: 90, energy: 0.71, centroid: 2500, want: "Energetic/Rave"},
		{tempo: 90, energy: 0.7, centroid: 2500, want: "Moderate Groove"},

		{tempo: 60, energy: 0.09, centroid: 2500, want: "Ambient/Atmospheric"},
		{tempo: 60, energy: 0.1, centroid: 2500, want: "Melancholic/Sad"},
		{tempo: 60, energy: 0.1, centroid: 2500, major: true, want: "Downtempo/Relaxed"},
		{tempo: 69.9, energy: 0.09, centroid: 2500, want: "Ambient/Atmospheric"},
		{tempo: 70, energy: 0.09, centroid: 2500, want: "Ambient/Chill"},
		{tempo: 70, energy: 0.09, centroid: 1500, want: "Downtempo/Dark"},
		{tempo: 80, energy: 0.09, centroid: 1999, want: "Downtempo/Dark"},
		{tempo: 80, energy: 0.09, centroid: 2000, want: "Ambient/Chill"},
		{tempo: 80, energy: 0.1, centroid: 1500, want: "Ambient/Chill"},
		{tempo: 80, energy: 0.11, centroid: 2500, want: "Ambient/Chill"},
		{tempo: 80, energy: 0.12, centroid: 2500, want: "Midtempo Groove"},
		{tempo: 99.9, energy: 0.14, centroid: 2500, want: "Midtempo Groove"},
		{tempo: 100, energy: 0.14, centroid: 2500, want: "Minimal/Sparse"},
		{tempo: 50, energy: 0.19, centroid: 2500, want: "Melancholic/Sad"},
		{tempo: 50, energy: 0.2, centroid: 2500, want: "Relaxed/Moderate"},
		{tempo: 50, energy: 0.25, centroid: 2500, want: "Moderate Groove"},
		{tempo: 130.1, energy: 0.2, centroid: 2500, want: "Relaxed/Moderate"},
		{tempo: 130.1, energy: 0.25, centroid: 2500, want: "Upbeat/Moderate"},
	}
	for _, c := range cases {
		if got := c.classify().Mood; got != c.want {
			t.Errorf("tempo=%v energy=%v centroid=%v conf=%v zcr=%v major=%v: got %q want %q",
				c.tempo, c.energy, c.centroid, c.confidence, c.zcr, c.major, got, c.want)
		}
	}
}

func TestClassifyReportsKeyFromChroma(t *testing.T) {
	cases := []struct {
		root  int
		major bool
		want  string
	}{
		{0, true, "C major"},
		{0, false, "C minor"},
		{9, true, "A major"},
		{11, false, "B minor"},
	}
	for _, c := range cases {
		chroma := make([]float64, 12)
		chroma[c.root] = 1
		if c.major {
			chroma[(c.root+4)%12] = 0.5
		} else {
			chroma[(c.root+3)%12] = 0.5
		}
		got := Classify(Features{Energy: 0.3, Tempo: 90, TempoConfidence: 0.9, SpectralCentroid: 2500, Chroma: chroma}).Key
		if got != c.want {
			t.Errorf("root %d major=%v: got %q want %q", c.root, c.major, got, c.want)
		}
	}
}

func TestClassifySimilarityScoresMatchHandComputedDistances(t *testing.T) {
	cases := []struct {
		energy, tempo, centroid float64
		want                    map[string]float64
	}{
		{0.3, 100, 2000, map[string]float64{"House": 0.72, "Techno": 0.69, "Disco": 0.79, "Ambient": 0.65, "DnB": 0.29, "Downtempo": 0.82}},
		{0.9, 150, 10000, map[string]float64{"House": 0.43, "Techno": 0.43, "Disco": 0.38, "Ambient": 0.0, "DnB": 0.45, "Downtempo": 0.16}},
		{0.15, 125, 3000, map[string]float64{"House": 1.0, "Techno": 0.92, "Disco": 0.89, "Ambient": 0.44, "DnB": 0.55, "Downtempo": 0.64}},
	}
	for _, c := range cases {
		got := Classify(Features{Energy: c.energy, Tempo: c.tempo, TempoConfidence: 0.9, SpectralCentroid: c.centroid, Chroma: make([]float64, 12)}).SimilarityScores
		if len(got) != len(c.want) {
			t.Errorf("input %+v: got %d scores want %d", c, len(got), len(c.want))
		}
		for k, w := range c.want {
			if math.Abs(got[k]-w) > 1e-9 {
				t.Errorf("energy=%v tempo=%v centroid=%v %s: got %v want %v", c.energy, c.tempo, c.centroid, k, got[k], w)
			}
		}
	}
}

func TestClassifyExplanationBands(t *testing.T) {
	explain := func(energy, tempo, centroid float64) string {
		return Classify(Features{Energy: energy, Tempo: tempo, TempoConfidence: 0.9, SpectralCentroid: centroid, Chroma: make([]float64, 12)}).Explanation
	}
	cases := []struct {
		energy, tempo, centroid float64
		want                    string
	}{
		{0.09, 90, 2500, "Very low energy"},
		{0.1, 90, 2500, "Low energy"},
		{0.19, 90, 2500, "Low energy"},
		{0.2, 90, 2500, "Moderate energy"},
		{0.34, 90, 2500, "Moderate energy"},
		{0.35, 90, 2500, "High energy"},
		{0.49, 90, 2500, "High energy"},
		{0.5, 90, 2500, "Very high energy"},
		{0.3, 79.9, 2500, ", slow tempo"},
		{0.3, 80, 2500, ", moderate tempo"},
		{0.3, 109.9, 2500, ", moderate tempo"},
		{0.3, 110, 2500, ", dance tempo"},
		{0.3, 129.9, 2500, ", dance tempo"},
		{0.3, 130, 2500, ", fast tempo"},
		{0.3, 149.9, 2500, ", fast tempo"},
		{0.3, 149.5, 2500, ", fast tempo"},
		{0.3, 150, 2500, ", very fast tempo"},
		{0.3, 90, 1499, "dark/mellow timbre"},
		{0.3, 90, 1499.8, "dark/mellow timbre"},
		{0.3, 90, 1500, "balanced timbre"},
		{0.3, 90, 1500.2, "balanced timbre"},
		{0.3, 90, 2999, "balanced timbre"},
		{0.3, 90, 3000, "bright/sharp timbre"},
	}
	for _, c := range cases {
		if got := explain(c.energy, c.tempo, c.centroid); !strings.Contains(got, c.want) {
			t.Errorf("energy=%v tempo=%v centroid=%v: %q lacks %q", c.energy, c.tempo, c.centroid, got, c.want)
		}
	}
}

func TestClassifyExplanationFormat(t *testing.T) {
	got := Classify(Features{Energy: 0.09, Tempo: 79.9, TempoConfidence: 0.9, SpectralCentroid: 1000, Chroma: make([]float64, 12)}).Explanation
	want := "Very low energy (0.090), slow tempo (79.9 BPM), dark/mellow timbre, key of C minor"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}
