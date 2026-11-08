package mood

import (
	"fmt"
	"math"
	"strings"
)

type Features struct {
	Tempo            float64   `json:"tempo"`
	TempoConfidence  float64   `json:"tempo_confidence"`
	Energy           float64   `json:"energy"`
	SpectralCentroid float64   `json:"spectral_centroid"`
	SpectralRolloff  float64   `json:"spectral_rolloff"`
	ZeroCrossingRate float64   `json:"zero_crossing_rate"`
	Chroma           []float64 `json:"chroma"`
	Duration         float64   `json:"duration"`
}

type Result struct {
	Mood             string             `json:"mood"`
	Energy           float64            `json:"energy"`
	Tempo            float64            `json:"tempo"`
	Key              string             `json:"key"`
	SimilarityScores map[string]float64 `json:"similarity_scores"`
	Explanation      string             `json:"explanation"`
	Duration         float64            `json:"-"`
}

var noteNames = [12]string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}

func Classify(f Features) Result {
	m, tempo := detectMood(f)
	key := determineKey(f.Chroma)
	return Result{
		Mood:             m,
		Energy:           f.Energy,
		Tempo:            tempo,
		Key:              key,
		SimilarityScores: similarityScores(f.Energy, tempo, f.SpectralCentroid),
		Explanation:      explain(f.Energy, tempo, f.SpectralCentroid, key),
		Duration:         f.Duration,
	}
}

func detectMood(f Features) (string, float64) {
	energy, tempo := f.Energy, f.Tempo
	brightness := math.Min(f.SpectralCentroid/5000.0, 1.0)
	major := isMajor(f.Chroma)

	if f.TempoConfidence < 0.2 {
		switch {
		case energy < 0.15:
			return "Ambient/Drone", tempo
		case energy < 0.50:
			return "Atmospheric/Textural", tempo
		case energy < 0.70:
			if brightness < 0.3 {
				return "Atmospheric/Textural", tempo
			}
			return "Noise/Experimental", tempo
		default:
			return "Harsh Noise/Experimental", tempo
		}
	}

	if f.ZeroCrossingRate > 0.25 {
		if energy > 0.3 {
			return "Harsh Noise/Experimental", tempo
		}
		return "Glitch/Experimental", tempo
	}

	if tempo >= 120 && tempo <= 145 && energy >= 0.50 {
		if brightness < 0.4 {
			return "Techno/Dark", tempo
		}
		return "Techno/Industrial", tempo
	}

	if tempo >= 100 && tempo <= 130 && energy >= 0.15 {
		switch {
		case energy > 0.7:
			if tempo >= 115 {
				if brightness > 0.5 {
					return "Techno/Industrial", tempo
				}
				return "Techno/Dark", tempo
			}
			return "Energetic/Rave", tempo
		case tempo <= 115 && energy < 0.5:
			return "Disco/Funk", tempo
		case brightness > 0.5:
			if major {
				return "House/Dance", tempo
			}
			return "Dark House", tempo
		case brightness > 0.4:
			return "Disco/Funk", tempo
		default:
			return "Club/Groovy", tempo
		}
	}

	if tempo >= 148 && tempo <= 180 {
		switch {
		case energy > 0.4:
			return "Drum & Bass", tempo
		case energy > 0.25:
			return "Breakbeat/Fast", tempo
		default:
			return "Fast/Atmospheric", tempo
		}
	}

	if tempo > 180 || energy > 0.7 {
		switch {
		case energy > 0.8:
			return "Hard/Aggressive", tempo
		case energy > 0.6:
			return "Energetic/Rave", tempo
		default:
			return "Driving Electronic", tempo
		}
	}

	if energy < 0.2 {
		switch {
		case tempo < 70:
			switch {
			case energy < 0.1:
				return "Ambient/Atmospheric", tempo
			case !major:
				return "Melancholic/Sad", tempo
			default:
				return "Downtempo/Relaxed", tempo
			}
		case tempo < 100:
			switch {
			case energy < 0.1 && brightness < 0.4:
				return "Downtempo/Dark", tempo
			case energy < 0.12:
				return "Ambient/Chill", tempo
			default:
				return "Midtempo Groove", tempo
			}
		default:
			return "Minimal/Sparse", tempo
		}
	}

	switch {
	case energy >= 0.25:
		if tempo > 100 {
			return "Upbeat/Moderate", tempo
		}
		return "Moderate Groove", tempo
	case energy >= 0.15:
		return "Relaxed/Moderate", tempo
	default:
		return "Low Energy", tempo
	}
}

func argmax(v []float64) int {
	best := 0
	for i, x := range v {
		if x > v[best] {
			best = i
		}
	}
	return best
}

func isMajor(chroma []float64) bool {
	root := argmax(chroma)
	return chroma[(root+4)%12] > chroma[(root+3)%12]
}

func determineKey(chroma []float64) string {
	mode := "minor"
	if isMajor(chroma) {
		mode = "major"
	}
	return noteNames[argmax(chroma)] + " " + mode
}

var referenceMoods = []struct {
	name                      string
	energy, tempo, brightness float64
}{
	{"House", 0.15, 125, 0.6},
	{"Techno", 0.16, 130, 0.4},
	{"Disco", 0.14, 115, 0.7},
	{"Ambient", 0.04, 70, 0.5},
	{"DnB", 0.18, 170, 0.5},
	{"Downtempo", 0.08, 90, 0.4},
}

func similarityScores(energy, tempo, centroid float64) map[string]float64 {
	brightness := math.Min(centroid/5000.0, 1.0)
	out := make(map[string]float64, len(referenceMoods))
	for _, r := range referenceMoods {
		e := math.Abs(energy-r.energy) * 2
		t := math.Abs(tempo-r.tempo) / 100 * 3
		b := math.Abs(brightness - r.brightness)
		d := math.Sqrt(e*e + t*t + b*b)
		s := math.Max(0, 1-d/3)
		out[r.name] = math.Round(s*100) / 100
	}
	return out
}

func explain(energy, tempo, centroid float64, key string) string {
	brightness := math.Min(centroid/5000.0, 1.0)

	var e string
	switch {
	case energy < 0.1:
		e = "very low"
	case energy < 0.2:
		e = "low"
	case energy < 0.35:
		e = "moderate"
	case energy < 0.5:
		e = "high"
	default:
		e = "very high"
	}

	var b string
	switch {
	case brightness < 0.3:
		b = "dark/mellow"
	case brightness < 0.6:
		b = "balanced"
	default:
		b = "bright/sharp"
	}

	var t string
	switch {
	case tempo < 80:
		t = "slow"
	case tempo < 110:
		t = "moderate"
	case tempo < 130:
		t = "dance"
	case tempo < 150:
		t = "fast"
	default:
		t = "very fast"
	}

	return fmt.Sprintf("%s energy (%.3f), %s tempo (%.1f BPM), %s timbre, key of %s",
		strings.ToUpper(e[:1])+e[1:], energy, t, tempo, b, key)
}
