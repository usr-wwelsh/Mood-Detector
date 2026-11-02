package mood

func ExtractFeatures(y []float32) Features {
	samples := make([]float64, len(y))
	for i, v := range y {
		samples[i] = float64(v)
	}
	spec := magnitudeSpectrogram(samples)
	env := onsetStrength(spec)

	f := Features{
		Energy:           min(max(meanRMS(samples)*2.5, 0), 1),
		SpectralCentroid: meanSpectralCentroid(spec),
		SpectralRolloff:  meanSpectralRolloff(spec),
		ZeroCrossingRate: meanZeroCrossingRate(samples),
		Chroma:           meanChroma(spec),
	}
	f.TempoConfidence = variance(env)
	if f.TempoConfidence < 0.01 {
		f.Tempo = 60
	} else {
		f.Tempo = estimateTempo(env)
	}
	return f
}
