package mood

import "math"

const (
	nMels       = 128
	tempoWindow = 384
	startBPM    = 140.0
	minBPM      = 55.0
	maxBPM      = 220.0
	tempoSigma  = 1.2

	lagSearchMin  = 10.0
	lagSearchMax  = 150.0
	lagSearchStep = 0.1
	acSize        = 1024
)

func slaneyHzToMel(f float64) float64 {
	const fSp, minLogHz = 200.0 / 3, 1000.0
	if f < minLogHz {
		return f / fSp
	}
	return minLogHz/fSp + math.Log(f/minLogHz)/(math.Log(6.4)/27)
}

func slaneyMelToHz(m float64) float64 {
	const fSp, minLogHz = 200.0 / 3, 1000.0
	if m < minLogHz/fSp {
		return m * fSp
	}
	return minLogHz * math.Exp((math.Log(6.4)/27)*(m-minLogHz/fSp))
}

type melBand struct {
	lo, hi int
	w      []float64
}

func melFilterBank() []melBand {
	freqs := binFrequencies()
	lo, hi := slaneyHzToMel(0), slaneyHzToMel(SampleRate/2)
	pts := make([]float64, nMels+2)
	for i := range pts {
		pts[i] = slaneyMelToHz(lo + (hi-lo)*float64(i)/float64(nMels+1))
	}
	bank := make([]melBand, nMels)
	for m := range bank {
		w := pts[m : m+3 : m+3]
		left, mid, right := w[0], w[1], w[2]
		row := make([]float64, nBins)
		enorm := 2.0 / (right - left)
		for k, f := range freqs {
			lower := (f - left) / (mid - left)
			upper := (right - f) / (right - mid)
			row[k] = math.Max(0, math.Min(lower, upper)) * enorm
		}
		lo, hi := 0, nBins
		for lo < hi && row[lo] == 0 {
			lo++
		}
		for hi > lo && row[hi-1] == 0 {
			hi--
		}
		bank[m] = melBand{lo, hi, row[lo:hi]}
	}
	return bank
}

func onsetStrength(spec [][]float64) []float64 {
	frames := len(spec)
	bank := melFilterBank()
	db := make([][]float64, frames)
	peak := math.Inf(-1)
	for t, row := range spec {
		db[t] = make([]float64, nMels)
		for m, w := range bank {
			var s float64
			for k := w.lo; k < w.hi; k++ {
				s += w.w[k-w.lo] * row[k] * row[k]
			}
			d := 10 * math.Log10(math.Max(1e-10, s))
			db[t][m] = d
			peak = math.Max(peak, d)
		}
	}
	for t := range db {
		for m := range db[t] {
			db[t][m] = math.Max(db[t][m], peak-80)
		}
	}
	const pad = 1 + nFFT/(2*hopLength)
	env := make([]float64, frames)
	for t := 1; t < frames; t++ {
		var s float64
		for m := 0; m < nMels; m++ {
			s += math.Max(0, db[t][m]-db[t-1][m])
		}
		if i := t - 1 + pad; i < frames {
			env[i] = s / nMels
		}
	}
	return env
}

func variance(x []float64) float64 {
	if len(x) == 0 {
		return 0
	}
	var mean float64
	for _, v := range x {
		mean += v
	}
	mean /= float64(len(x))
	var s float64
	for _, v := range x {
		s += (v - mean) * (v - mean)
	}
	return s / float64(len(x))
}

func estimateTempo(env []float64) float64 {
	if len(env) == 0 {
		return startBPM
	}
	return pickTempo(meanAutocorr(env), len(env))
}

func meanAutocorr(env []float64) []float64 {
	n := len(env)
	const half = tempoWindow / 2
	padded := make([]float64, n+tempoWindow)
	copy(padded[half:], env)
	for i := 0; i < half; i++ {
		padded[i] = env[0] * float64(i) / half
		padded[half+n+i] = env[n-1] * float64(half-1-i) / half
	}
	win := periodicHann(tempoWindow)
	mean := make([]float64, tempoWindow)
	seg := make([]float64, tempoWindow)
	ac := make([]float64, tempoWindow)
	buf := make([]complex128, acSize)
	for w := 0; w < n; w++ {
		for i := range seg {
			seg[i] = padded[w+i] * win[i]
		}
		for i := range buf {
			buf[i] = 0
		}
		for i, v := range seg {
			buf[i] = complex(v, 0)
		}
		fft(buf)
		for i, v := range buf {
			buf[i] = complex(real(v)*real(v)+imag(v)*imag(v), 0)
		}
		fft(buf)
		var peak float64
		for lag := range ac {
			ac[lag] = real(buf[lag]) / acSize
			peak = math.Max(peak, math.Abs(ac[lag]))
		}
		if peak < 1e-300 {
			continue
		}
		for lag := range ac {
			mean[lag] += ac[lag] / peak
		}
	}
	return mean
}

var lagMultiples = []float64{1, 2, 3, 4, 0.5, 1.5}

func lagValue(ac []float64, x float64) float64 {
	if x < 1 || x >= float64(len(ac)-1) {
		return 0
	}
	i := int(x)
	f := x - float64(i)
	return ac[i]*(1-f) + ac[i+1]*f
}

func pickTempo(mean []float64, n int) float64 {
	best, bestScore := 0.0, math.Inf(-1)
	for lag := lagSearchMin; lag < lagSearchMax; lag += lagSearchStep {
		bpm := 60.0 * SampleRate / (hopLength * lag)
		if bpm < minBPM || bpm > maxBPM {
			continue
		}
		var v float64
		for _, m := range lagMultiples {
			v += lagValue(mean, lag*m)
		}
		v /= float64(len(lagMultiples))
		d := (math.Log2(bpm) - math.Log2(startBPM)) / tempoSigma
		score := math.Log1p(1e6*v/float64(n)) - 0.5*d*d
		if score > bestScore {
			best, bestScore = lag, score
		}
	}
	if best == 0 {
		return startBPM
	}
	return 60.0 * SampleRate / (hopLength * best)
}
