package mood

import (
	"math"
	"math/cmplx"
	"sort"
	"sync"
)

const (
	SampleRate = 22050
	nFFT       = 2048
	hopLength  = 512
	nBins      = nFFT/2 + 1
)

var hannWindow = periodicHann(nFFT)

func periodicHann(n int) []float64 {
	w := make([]float64, n)
	for i := range w {
		w[i] = 0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/float64(n))
	}
	return w
}

var twiddleCache sync.Map

func twiddles(n int) []complex128 {
	if v, ok := twiddleCache.Load(n); ok {
		return v.([]complex128)
	}
	t := make([]complex128, n/2)
	for k := range t {
		a := -2 * math.Pi * float64(k) / float64(n)
		t[k] = complex(math.Cos(a), math.Sin(a))
	}
	twiddleCache.Store(n, t)
	return t
}

func fft(x []complex128) {
	n := len(x)
	for i, j := 1, 0; i < n; i++ {
		bit := n >> 1
		for ; j&bit != 0; bit >>= 1 {
			j ^= bit
		}
		j ^= bit
		if i < j {
			x[i], x[j] = x[j], x[i]
		}
	}
	tw := twiddles(n)
	for size := 2; size <= n; size <<= 1 {
		stride := n / size
		for start := 0; start < n; start += size {
			for k := 0; k < size/2; k++ {
				a, b := x[start+k], x[start+k+size/2]*tw[k*stride]
				x[start+k], x[start+k+size/2] = a+b, a-b
			}
		}
	}
}

func frameCount(samples int) int {
	return 1 + samples/hopLength
}

func magnitudeSpectrogram(y []float64) [][]float64 {
	frames := frameCount(len(y))
	out := make([][]float64, frames)
	buf := make([]complex128, nFFT)
	for t := range out {
		start := t*hopLength - nFFT/2
		for i := range buf {
			var v float64
			if idx := start + i; idx >= 0 && idx < len(y) {
				v = y[idx]
			}
			buf[i] = complex(v*hannWindow[i], 0)
		}
		fft(buf)
		row := make([]float64, nBins)
		for k := range row {
			row[k] = cmplx.Abs(buf[k])
		}
		out[t] = row
	}
	return out
}

func binFrequencies() []float64 {
	f := make([]float64, nBins)
	for k := range f {
		f[k] = float64(k) * SampleRate / nFFT
	}
	return f
}

func meanRMS(y []float64) float64 {
	frames := frameCount(len(y))
	var sum float64
	for t := 0; t < frames; t++ {
		start := t*hopLength - nFFT/2
		var sq float64
		for i := 0; i < nFFT; i++ {
			if idx := start + i; idx >= 0 && idx < len(y) {
				sq += y[idx] * y[idx]
			}
		}
		sum += math.Sqrt(sq / nFFT)
	}
	return sum / float64(frames)
}

func meanZeroCrossingRate(y []float64) float64 {
	if len(y) == 0 {
		return 0
	}
	frames := frameCount(len(y))
	at := func(i int) float64 {
		switch {
		case i < 0:
			return y[0]
		case i >= len(y):
			return y[len(y)-1]
		}
		return y[i]
	}
	neg := func(v float64) bool {
		if math.Abs(v) <= 1e-10 {
			return false
		}
		return v < 0
	}
	var sum float64
	for t := 0; t < frames; t++ {
		start := t*hopLength - nFFT/2
		count := 0
		prev := neg(at(start))
		for i := 1; i < nFFT; i++ {
			cur := neg(at(start + i))
			if cur != prev {
				count++
			}
			prev = cur
		}
		sum += float64(count) / float64(nFFT-1)
	}
	return sum / float64(frames)
}

func meanSpectralCentroid(spec [][]float64) float64 {
	freqs := binFrequencies()
	var sum float64
	for _, row := range spec {
		var num, den float64
		for k, v := range row {
			num += freqs[k] * v
			den += v
		}
		if den > 0 {
			sum += num / den
		}
	}
	return sum / float64(len(spec))
}

func meanSpectralRolloff(spec [][]float64) float64 {
	freqs := binFrequencies()
	var sum float64
	for _, row := range spec {
		var total float64
		for _, v := range row {
			total += v
		}
		threshold := 0.85 * total
		var cum float64
		for k, v := range row {
			cum += v
			if cum >= threshold {
				sum += freqs[k]
				break
			}
		}
	}
	return sum / float64(len(spec))
}

func chromaFilterBank(tuning float64) [12][]float64 {
	const nChroma = 12
	const ctroct, octwidth = 5.0, 2.0
	freqs := binFrequencies()
	frq := make([]float64, nBins)
	for k := 1; k < nBins; k++ {
		frq[k] = nChroma * math.Log2(freqs[k]/(440.0*math.Pow(2, tuning/nChroma)/16))
	}
	frq[0] = frq[1] - 1.5*nChroma
	width := make([]float64, nBins)
	for k := 0; k < nBins-1; k++ {
		width[k] = math.Max(frq[k+1]-frq[k], 1)
	}
	width[nBins-1] = 1

	var wts [12][]float64
	for c := range wts {
		wts[c] = make([]float64, nBins)
	}
	for k := 0; k < nBins; k++ {
		var norm float64
		for c := 0; c < nChroma; c++ {
			d := frq[k] - float64(c)
			d = math.Mod(d+nChroma/2+10*nChroma, nChroma) - nChroma/2
			w := math.Exp(-0.5 * math.Pow(2*d/width[k], 2))
			wts[c][k] = w
			norm += w * w
		}
		norm = math.Sqrt(norm)
		oct := math.Exp(-0.5 * math.Pow((frq[k]/nChroma-ctroct)/octwidth, 2))
		for c := 0; c < nChroma; c++ {
			if norm > 0 {
				wts[c][k] /= norm
			}
			wts[c][k] *= oct
		}
	}
	var rolled [12][]float64
	for c := range rolled {
		rolled[c] = wts[(c+3)%12]
	}
	return rolled
}

func estimateTuning(spec [][]float64) float64 {
	freqs := binFrequencies()
	type peak struct{ freq, mag float64 }
	var peaks []peak
	for _, row := range spec {
		power := make([]float64, nBins)
		var top float64
		for k, v := range row {
			power[k] = v * v
			top = math.Max(top, power[k])
		}
		ref := 0.1 * top
		for k := 1; k < nBins-1; k++ {
			if freqs[k] < 150 || freqs[k] >= 4000 {
				continue
			}
			if !(power[k] > power[k-1] && power[k] >= power[k+1]) || power[k] <= ref {
				continue
			}
			avg := 0.5 * (power[k+1] - power[k-1])
			shift := 2*power[k] - power[k+1] - power[k-1]
			if math.Abs(shift) < 1e-300 {
				shift = 1
			}
			shift = avg / shift
			peaks = append(peaks, peak{(float64(k) + shift) * SampleRate / nFFT, power[k] + 0.5*avg*shift})
		}
	}
	valid := peaks[:0:0]
	for _, p := range peaks {
		if p.freq > 0 {
			valid = append(valid, p)
		}
	}
	if len(valid) == 0 {
		return 0
	}
	mags := make([]float64, len(valid))
	for i, p := range valid {
		mags[i] = p.mag
	}
	sort.Float64s(mags)
	var median float64
	if n := len(mags); n%2 == 1 {
		median = mags[n/2]
	} else {
		median = (mags[n/2-1] + mags[n/2]) / 2
	}
	const buckets = 100
	counts := make([]int, buckets)
	found := false
	for _, p := range valid {
		if p.mag < median {
			continue
		}
		r := 12 * math.Log2(p.freq/440)
		r -= math.Floor(r)
		if r >= 0.5 {
			r--
		}
		i := int((r + 0.5) * buckets)
		if i >= buckets {
			i = buckets - 1
		}
		counts[i]++
		found = true
	}
	if !found {
		return 0
	}
	best := 0
	for i, c := range counts {
		if c > counts[best] {
			best = i
		}
	}
	return -0.5 + float64(best)/buckets
}

func meanChroma(spec [][]float64) []float64 {
	bank := chromaFilterBank(estimateTuning(spec))
	out := make([]float64, 12)
	for _, row := range spec {
		var frame [12]float64
		var peak float64
		for c := range frame {
			var s float64
			for k, v := range row {
				s += bank[c][k] * v * v
			}
			frame[c] = s
			peak = math.Max(peak, s)
		}
		if peak < 1e-300 {
			continue
		}
		for c := range out {
			out[c] += frame[c] / peak
		}
	}
	for c := range out {
		out[c] /= float64(len(spec))
	}
	return out
}
