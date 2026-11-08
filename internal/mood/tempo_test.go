package mood

import (
	"math"
	"testing"
)

func pulseEnvelope(bpm float64, frames int) []float64 {
	period := 60.0 * SampleRate / (hopLength * bpm)
	env := make([]float64, frames)
	for beat := 0.0; beat*period < float64(frames); beat++ {
		env[int(math.Round(beat*period))%frames] = 1
	}
	return env
}

func TestEstimateTempoFindsPulseRate(t *testing.T) {
	for _, bpm := range []float64{80, 100, 128, 140} {
		got := estimateTempo(pulseEnvelope(bpm, 1300))
		if math.Abs(got-bpm) > 0.03*bpm {
			t.Errorf("pulse at %.0f bpm: got %.1f", bpm, got)
		}
	}
}

func TestEstimateTempoResolvesBetweenLagSteps(t *testing.T) {
	got := estimateTempo(pulseEnvelope(137, 1300))
	if math.Abs(got-137) > 1.5 {
		t.Errorf("got %.1f want ~137", got)
	}
}
