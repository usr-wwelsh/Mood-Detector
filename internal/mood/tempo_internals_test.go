package mood

import (
	"math"
	"testing"
)

func TestLagValueInterpolatesLinearlyAndRejectsOutOfRange(t *testing.T) {
	ac := []float64{9, 1, 2, 3, 9}
	cases := []struct{ x, want float64 }{
		{0.99, 0},
		{1, 1},
		{1.5, 1.5},
		{2.25, 2.25},
		{3.5, 6},
		{3.99, 8.94},
		{4, 0},
		{10, 0},
	}
	for _, c := range cases {
		if got := lagValue(ac, c.x); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("lagValue(%v) = %v want %v", c.x, got, c.want)
		}
	}
}

func TestVarianceIsPopulationVariance(t *testing.T) {
	if got := variance(nil); got != 0 {
		t.Errorf("empty: got %v", got)
	}
	if got := variance([]float64{5, 5, 5}); got != 0 {
		t.Errorf("constant: got %v", got)
	}
	if got := variance([]float64{1, 2, 3, 4}); math.Abs(got-1.25) > 1e-12 {
		t.Errorf("got %v want 1.25", got)
	}
}

func TestEstimateTempoOfEmptyEnvelopeIsStartBPM(t *testing.T) {
	if got := estimateTempo(nil); got != startBPM {
		t.Errorf("got %v want %v", got, startBPM)
	}
}

func TestEstimateTempoStaysWithinBPMRange(t *testing.T) {
	for _, bpm := range []float64{30, 45, 250, 300} {
		got := estimateTempo(pulseEnvelope(bpm, 1300))
		if got < minBPM || got > maxBPM {
			t.Errorf("pulse at %.0f bpm: got %.1f outside [%v,%v]", bpm, got, minBPM, maxBPM)
		}
	}
}

func TestEstimateTempoOfSilenceIsFiniteAndNearStartBPM(t *testing.T) {
	got := estimateTempo(make([]float64, 800))
	if math.IsNaN(got) || math.Abs(got-startBPM) > 0.03*startBPM {
		t.Errorf("got %v", got)
	}
}

func TestMeanAutocorrOfSilenceIsZeroNotNaN(t *testing.T) {
	for i, v := range meanAutocorr(make([]float64, 50)) {
		if v != 0 {
			t.Fatalf("lag %d: got %v", i, v)
		}
	}
}

func TestMeanAutocorrPeaksAtZeroLagAndAtPulsePeriod(t *testing.T) {
	period := 60.0 * SampleRate / (hopLength * 120)
	mean := meanAutocorr(pulseEnvelope(120, 1300))
	if len(mean) != tempoWindow {
		t.Fatalf("len %d want %d", len(mean), tempoWindow)
	}
	if mean[0] <= mean[1] {
		t.Errorf("lag 0 (%v) should exceed lag 1 (%v)", mean[0], mean[1])
	}
	p := int(math.Round(period))
	if mean[p] <= mean[p-3] || mean[p] <= mean[p+3] {
		t.Errorf("no peak at period %d: %v %v %v", p, mean[p-3], mean[p], mean[p+3])
	}
}

func TestSlaneyMelScaleMatchesKnownPointsAndRoundTrips(t *testing.T) {
	known := []struct{ hz, mel float64 }{{0, 0}, {200, 3}, {1000, 15}, {6400, 42}}
	for _, k := range known {
		if got := slaneyHzToMel(k.hz); math.Abs(got-k.mel) > 1e-9 {
			t.Errorf("hzToMel(%v) = %v want %v", k.hz, got, k.mel)
		}
		if got := slaneyMelToHz(k.mel); math.Abs(got-k.hz) > 1e-6 {
			t.Errorf("melToHz(%v) = %v want %v", k.mel, got, k.hz)
		}
	}
	for _, hz := range []float64{100, 999, 1000, 1001, 4000, 11025} {
		if got := slaneyMelToHz(slaneyHzToMel(hz)); math.Abs(got-hz) > 1e-6 {
			t.Errorf("round trip %v -> %v", hz, got)
		}
	}
}

func TestMelFilterBankBandsAreOrderedNonNegativeAndAreaNormalised(t *testing.T) {
	bank := melFilterBank()
	if len(bank) != nMels {
		t.Fatalf("got %d bands want %d", len(bank), nMels)
	}
	binHz := float64(SampleRate) / nFFT
	prevLo := -1
	for m, b := range bank {
		if b.lo >= b.hi || b.hi > nBins || len(b.w) != b.hi-b.lo {
			t.Fatalf("band %d: lo=%d hi=%d len=%d", m, b.lo, b.hi, len(b.w))
		}
		if b.lo < prevLo {
			t.Errorf("band %d starts before band %d", m, m-1)
		}
		prevLo = b.lo
		if b.w[0] == 0 || b.w[len(b.w)-1] == 0 {
			t.Errorf("band %d not trimmed to non-zero weights", m)
		}
		var area float64
		for _, w := range b.w {
			if w < 0 {
				t.Fatalf("band %d has negative weight", m)
			}
			area += w * binHz
		}
		if m > 20 && math.Abs(area-1) > 0.2 {
			t.Errorf("band %d area %v want ~1", m, area)
		}
	}
	if bank[0].lo != 1 {
		t.Errorf("first band should start at bin 1 (bin 0 has zero weight), got %d", bank[0].lo)
	}
	if bank[nMels-1].hi < nBins-5 {
		t.Errorf("last band should reach the top bins, ends at %d of %d", bank[nMels-1].hi, nBins)
	}
}

func flatSpec(frames int, level float64) [][]float64 {
	spec := make([][]float64, frames)
	for t := range spec {
		spec[t] = make([]float64, nBins)
		for k := range spec[t] {
			spec[t][k] = level
		}
	}
	return spec
}

func TestOnsetStrengthIsZeroForSteadySpectrum(t *testing.T) {
	env := onsetStrength(flatSpec(20, 0.5))
	if len(env) != 20 {
		t.Fatalf("len %d", len(env))
	}
	for i, v := range env {
		if v != 0 {
			t.Fatalf("frame %d: got %v", i, v)
		}
	}
}

func TestOnsetStrengthFiresOnlyAtTheRiseAndIsDelayedByPadding(t *testing.T) {
	const burst = 8
	spec := flatSpec(20, 0.001)
	spec[burst] = flatSpec(1, 1)[0]
	env := onsetStrength(spec)
	const pad = 1 + nFFT/(2*hopLength)
	at := burst - 1 + pad
	for i, v := range env {
		switch {
		case i == at && v <= 0:
			t.Errorf("no onset at frame %d", i)
		case i != at && v != 0:
			t.Errorf("unexpected onset %v at frame %d", v, i)
		}
	}
}

func TestOnsetStrengthScalesWithRiseInDecibels(t *testing.T) {
	rise := func(ratio float64) float64 {
		spec := flatSpec(20, 0.01)
		spec[8] = flatSpec(1, 0.01*ratio)[0]
		env := onsetStrength(spec)
		var peak float64
		for _, v := range env {
			peak = math.Max(peak, v)
		}
		return peak
	}
	a, b := rise(10), rise(100)
	if math.Abs(a-20) > 1 || math.Abs(b-40) > 1 {
		t.Errorf("10x amplitude rise gave %v dB, 100x gave %v dB; want ~20 and ~40", a, b)
	}
}
