package mood

import (
	"encoding/json"
	"math"
	"os"
	"testing"
)

type goldenFile struct {
	Chromas [][]float64 `json:"chromas"`
	Cases   []struct {
		Tempo            float64 `json:"tempo"`
		TempoConfidence  float64 `json:"tempo_confidence"`
		Energy           float64 `json:"energy"`
		SpectralCentroid float64 `json:"spectral_centroid"`
		ZeroCrossingRate float64 `json:"zero_crossing_rate"`
		Chroma           int     `json:"chroma"`
		Want             Result  `json:"want"`
	} `json:"cases"`
}

func TestClassifyMatchesPythonReference(t *testing.T) {
	raw, err := os.ReadFile("testdata/classify_golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var g goldenFile
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatal(err)
	}
	for _, c := range g.Cases {
		got := Classify(Features{
			Tempo:            c.Tempo,
			TempoConfidence:  c.TempoConfidence,
			Energy:           c.Energy,
			SpectralCentroid: c.SpectralCentroid,
			ZeroCrossingRate: c.ZeroCrossingRate,
			Chroma:           g.Chromas[c.Chroma],
		})
		if got.Mood != c.Want.Mood || got.Key != c.Want.Key || got.Explanation != c.Want.Explanation ||
			math.Abs(got.Tempo-c.Want.Tempo) > 1e-9 || math.Abs(got.Energy-c.Want.Energy) > 1e-9 {
			t.Fatalf("input %+v\n got %+v\nwant %+v", c, got, c.Want)
		}
		for k, w := range c.Want.SimilarityScores {
			if math.Abs(got.SimilarityScores[k]-w) > 0.0101 {
				t.Fatalf("input %+v similarity %s got %v want %v", c, k, got.SimilarityScores[k], w)
			}
		}
	}
}

func TestResultJSONKeysMatchPythonWorkerOutput(t *testing.T) {
	out, err := json.Marshal(Result{SimilarityScores: map[string]float64{}})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"mood", "energy", "tempo", "key", "similarity_scores", "explanation"} {
		if _, ok := m[k]; !ok {
			t.Errorf("missing key %q", k)
		}
	}
}
