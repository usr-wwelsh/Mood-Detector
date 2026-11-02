package mood

import (
	"encoding/json"
	"math"
	"os"
	"testing"
)

type signalSpec struct {
	Name             string    `json:"name"`
	Kind             string    `json:"kind"`
	Freqs            []float64 `json:"freqs"`
	Amps             []float64 `json:"amps"`
	BPM              float64   `json:"bpm"`
	Dur              float64   `json:"dur"`
	Tempo            float64   `json:"tempo"`
	TempoConfidence  float64   `json:"tempo_confidence"`
	Energy           float64   `json:"energy"`
	SpectralCentroid float64   `json:"spectral_centroid"`
	SpectralRolloff  float64   `json:"spectral_rolloff"`
	ZeroCrossingRate float64   `json:"zero_crossing_rate"`
	Chroma           []float64 `json:"chroma"`
}

func (s signalSpec) build() []float32 {
	n := int(s.Dur * SampleRate)
	y := make([]float64, n)
	if s.Kind == "sines" {
		for i := range y {
			t := float64(i) / SampleRate
			for j, f := range s.Freqs {
				y[i] += s.Amps[j] * math.Sin(2*math.Pi*f*t)
			}
		}
	} else {
		period := 60.0 / s.BPM
		for b := 0; ; b++ {
			start := int(math.RoundToEven(float64(b) * period * SampleRate))
			if start >= n {
				break
			}
			for k := 0; k < 2000 && start+k < n; k++ {
				y[start+k] += math.Exp(-float64(k)/200) * math.Sin(2*math.Pi*1000*float64(k)/SampleRate)
			}
		}
	}
	out := make([]float32, n)
	for i, v := range y {
		out[i] = float32(v)
	}
	return out
}

func near(t *testing.T, name, field string, got, want, relTol float64) {
	t.Helper()
	if math.Abs(got-want) > relTol*math.Max(math.Abs(want), 1e-9) {
		t.Errorf("%s %s: got %v want %v", name, field, got, want)
	}
}

func TestFeaturesMatchLibrosaOnSyntheticSignals(t *testing.T) {
	raw, err := os.ReadFile("testdata/features_golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var specs []signalSpec
	if err := json.Unmarshal(raw, &specs); err != nil {
		t.Fatal(err)
	}
	for _, s := range specs {
		f := ExtractFeatures(s.build())
		near(t, s.Name, "energy", f.Energy, s.Energy, 0.01)
		near(t, s.Name, "centroid", f.SpectralCentroid, s.SpectralCentroid, 0.02)
		near(t, s.Name, "rolloff", f.SpectralRolloff, s.SpectralRolloff, 0.02)
		near(t, s.Name, "zcr", f.ZeroCrossingRate, s.ZeroCrossingRate, 0.02)
		near(t, s.Name, "tempo", f.Tempo, s.Tempo, 0.01)
		if s.TempoConfidence >= 0.01 {
			near(t, s.Name, "tempo_confidence", f.TempoConfidence, s.TempoConfidence, 0.05)
		}
		if argmax(f.Chroma) != argmax(s.Chroma) {
			t.Errorf("%s chroma peak: got %d want %d", s.Name, argmax(f.Chroma), argmax(s.Chroma))
		}
		for c := range s.Chroma {
			if math.Abs(f.Chroma[c]-s.Chroma[c]) > 0.05 {
				t.Errorf("%s chroma[%d]: got %.3f want %.3f", s.Name, c, f.Chroma[c], s.Chroma[c])
			}
		}
	}
}

func TestFFTMatchesNaiveDFT(t *testing.T) {
	const n = 64
	x := make([]complex128, n)
	ref := make([]complex128, n)
	for i := range x {
		x[i] = complex(math.Sin(float64(i)*0.7)+0.3*float64(i%5), 0)
	}
	for k := 0; k < n; k++ {
		for i := 0; i < n; i++ {
			a := -2 * math.Pi * float64(k*i) / n
			ref[k] += x[i] * complex(math.Cos(a), math.Sin(a))
		}
	}
	fft(x)
	for k := range x {
		if d := x[k] - ref[k]; math.Hypot(real(d), imag(d)) > 1e-9 {
			t.Fatalf("bin %d: got %v want %v", k, x[k], ref[k])
		}
	}
}
