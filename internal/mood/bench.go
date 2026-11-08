package mood

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"slices"
	"sort"
	"strings"
)

const FeatureVersion = 1

const (
	tempoTolerance = 0.04
	idLen          = 12
)

type Label struct {
	Mood  []string `json:"mood,omitempty"`
	Style []string `json:"style,omitempty"`
	Tempo float64  `json:"tempo,omitempty"`
	Key   string   `json:"key,omitempty"`
}

type Score struct{ Right, Total int }

func (s Score) pct() float64 {
	if s.Total == 0 {
		return 0
	}
	return 100 * float64(s.Right) / float64(s.Total)
}

type Miss struct{ ID, Field, Got, Want string }

type Report struct {
	Mood, Style, Tempo, TempoOctave, Key, KeyRelaxed Score
	Misses                                           []Miss
	Unlabeled, MissingFeatures                       int
}

func Evaluate(feats map[string]Features, labels map[string]Label) Report {
	var r Report
	for id := range feats {
		if _, ok := labels[id]; !ok {
			r.Unlabeled++
		}
	}
	ids := make([]string, 0, len(labels))
	for id := range labels {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		f, ok := feats[id]
		if !ok {
			r.MissingFeatures++
			continue
		}
		l := labels[id]
		got := Classify(f)
		if len(l.Mood) > 0 {
			r.Mood.Total++
			if slices.Contains(l.Mood, got.Mood) {
				r.Mood.Right++
			} else {
				r.Misses = append(r.Misses, Miss{id, "mood", got.Mood, strings.Join(l.Mood, " | ")})
			}
		}
		if len(l.Style) > 0 {
			r.Style.Total++
			if slices.Contains(l.Style, got.Mood) {
				r.Style.Right++
			} else {
				r.Misses = append(r.Misses, Miss{id, "style", got.Mood, strings.Join(l.Style, " | ")})
			}
		}
		if l.Tempo > 0 {
			r.Tempo.Total++
			r.TempoOctave.Total++
			if within(got.Tempo, l.Tempo) {
				r.Tempo.Right++
			} else {
				r.Misses = append(r.Misses, Miss{id, "tempo", fmt.Sprintf("%.1f", got.Tempo), fmt.Sprintf("%.1f", l.Tempo)})
			}
			if within(got.Tempo, l.Tempo) || within(got.Tempo, l.Tempo*2) || within(got.Tempo, l.Tempo/2) {
				r.TempoOctave.Right++
			}
		}
		if l.Key != "" {
			r.Key.Total++
			r.KeyRelaxed.Total++
			if got.Key == l.Key {
				r.Key.Right++
				r.KeyRelaxed.Right++
			} else {
				r.Misses = append(r.Misses, Miss{id, "key", got.Key, l.Key})
				if relativeKey(got.Key) == relativeKey(l.Key) {
					r.KeyRelaxed.Right++
				}
			}
		}
	}
	sort.SliceStable(r.Misses, func(i, j int) bool { return r.Misses[i].ID < r.Misses[j].ID })
	return r
}

func within(got, want float64) bool {
	return math.Abs(got-want) <= tempoTolerance*want
}

func relativeKey(k string) string {
	name, mode, ok := strings.Cut(k, " ")
	if !ok {
		return k
	}
	i := slices.Index(noteNames[:], name)
	if i < 0 {
		return k
	}
	if mode == "minor" {
		i = (i + 3) % 12
	}
	return noteNames[i]
}

func (r Report) Write(w io.Writer) {
	row := func(name string, s Score) {
		if s.Total > 0 {
			fmt.Fprintf(w, "%-14s %3d/%-3d %5.1f%%\n", name, s.Right, s.Total, s.pct())
		}
	}
	row("mood", r.Mood)
	row("style", r.Style)
	row("tempo", r.Tempo)
	row("tempo (octave)", r.TempoOctave)
	row("key", r.Key)
	row("key (relative)", r.KeyRelaxed)
	if r.Unlabeled > 0 || r.MissingFeatures > 0 {
		fmt.Fprintf(w, "unlabeled tracks: %d, labels without features: %d\n", r.Unlabeled, r.MissingFeatures)
	}
	if len(r.Misses) > 0 {
		fmt.Fprintln(w, "\nmisses:")
		for _, m := range r.Misses {
			fmt.Fprintf(w, "  %s %-5s got %-26s want %s\n", m.ID, m.Field, m.Got, m.Want)
		}
	}
}

type featuresFile struct {
	Version int                 `json:"version"`
	Tracks  map[string]Features `json:"tracks"`
}

func SaveFeatures(path string, feats map[string]Features) error {
	return saveJSON(path, featuresFile{FeatureVersion, feats})
}

func LoadFeatures(path string) (map[string]Features, error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- user-supplied bench path
	if err != nil {
		return nil, err
	}
	var f featuresFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if f.Version != FeatureVersion {
		return nil, fmt.Errorf("%s: feature version %d, extractor is %d; re-run bench extract", path, f.Version, FeatureVersion)
	}
	if f.Tracks == nil {
		f.Tracks = map[string]Features{}
	}
	return f.Tracks, nil
}

func SaveLabels(path string, labels map[string]Label) error {
	return saveJSON(path, labels)
}

func LoadLabels(path string) (map[string]Label, error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- user-supplied bench path
	if errors.Is(err, os.ErrNotExist) {
		return map[string]Label{}, nil
	}
	if err != nil {
		return nil, err
	}
	labels := map[string]Label{}
	if err := json.Unmarshal(raw, &labels); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return labels, nil
}

func TrackID(path string) (string, error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- user-supplied track path
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])[:idLen], nil
}

func saveJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(path, append(b, '\n'))
}

func writeFile(path string, b []byte) error {
	return os.WriteFile(path, b, 0o600)
}
