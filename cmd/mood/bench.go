package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/usr-wwelsh/mood-detector/internal/mood"
)

const benchUsage = "usage: mood bench extract [--dir bench] <audio-dir>\n       mood bench run [--dir bench] [--min-mood N] [--min-tempo N] [--min-key N]\n"

var audioExts = []string{".mp3", ".wav", ".flac", ".ogg", ".opus", ".m4a", ".aac"}

func bench(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, benchUsage)
		return 2
	}
	switch args[0] {
	case "extract":
		return benchExtract(args[1:], stdout, stderr)
	case "run":
		return benchRun(args[1:], stdout, stderr)
	default:
		fmt.Fprint(stderr, benchUsage)
		return 2
	}
}

func benchExtract(args []string, stdout, stderr io.Writer) int {
	fl := flag.NewFlagSet("bench extract", flag.ContinueOnError)
	fl.SetOutput(stderr)
	dir := fl.String("dir", "bench", "bench data directory")
	if err := fl.Parse(args); err != nil || fl.NArg() != 1 {
		fmt.Fprint(stderr, benchUsage)
		return 2
	}
	if err := os.MkdirAll(*dir, 0o750); err != nil {
		return fail(stderr, false, err)
	}
	featPath := filepath.Join(*dir, "features.json")
	labelPath := filepath.Join(*dir, "labels.json")
	namesPath := filepath.Join(*dir, "names.local.json")

	feats, err := mood.LoadFeatures(featPath)
	if errors.Is(err, fs.ErrNotExist) {
		feats, err = map[string]mood.Features{}, nil
	}
	if err != nil {
		return fail(stderr, false, err)
	}
	labels, err := mood.LoadLabels(labelPath)
	if err != nil {
		return fail(stderr, false, err)
	}
	names := map[string]string{}
	if raw, err := os.ReadFile(namesPath); err == nil { // #nosec G304 -- bench dir flag
		_ = json.Unmarshal(raw, &names)
	}

	added, failed := 0, 0
	walkErr := filepath.WalkDir(fl.Arg(0), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !slices.Contains(audioExts, strings.ToLower(filepath.Ext(path))) {
			return nil
		}
		id, err := mood.TrackID(path)
		if err != nil {
			return err
		}
		names[id] = d.Name()
		if _, done := feats[id]; done {
			return nil
		}
		f, err := mood.ExtractFile(path)
		if err != nil {
			fmt.Fprintf(stderr, "skip %s: %v\n", d.Name(), err)
			failed++
			return nil
		}
		feats[id] = f
		if _, ok := labels[id]; !ok {
			labels[id] = mood.Label{}
		}
		added++
		return nil
	})
	if walkErr != nil {
		return fail(stderr, false, walkErr)
	}
	if err := mood.SaveFeatures(featPath, feats); err != nil {
		return fail(stderr, false, err)
	}
	if err := mood.SaveLabels(labelPath, labels); err != nil {
		return fail(stderr, false, err)
	}
	b, err := json.MarshalIndent(names, "", "  ")
	if err != nil {
		return fail(stderr, false, err)
	}
	if err := os.WriteFile(namesPath, append(b, '\n'), 0o600); err != nil {
		return fail(stderr, false, err)
	}
	fmt.Fprintf(stdout, "extracted %d new, %d failed, %d total; label them in %s\n", added, failed, len(feats), labelPath)
	return 0
}

func benchRun(args []string, stdout, stderr io.Writer) int {
	fl := flag.NewFlagSet("bench run", flag.ContinueOnError)
	fl.SetOutput(stderr)
	dir := fl.String("dir", "bench", "bench data directory")
	minMood := fl.Float64("min-mood", 0, "fail below this mood accuracy percent")
	minTempo := fl.Float64("min-tempo", 0, "fail below this tempo accuracy percent")
	minKey := fl.Float64("min-key", 0, "fail below this key accuracy percent")
	if err := fl.Parse(args); err != nil || fl.NArg() != 0 {
		fmt.Fprint(stderr, benchUsage)
		return 2
	}
	feats, err := mood.LoadFeatures(filepath.Join(*dir, "features.json"))
	if errors.Is(err, fs.ErrNotExist) {
		return fail(stderr, false, fmt.Errorf("no features in %s; run mood bench extract <audio-dir> first", *dir))
	}
	if err != nil {
		return fail(stderr, false, err)
	}
	labels, err := mood.LoadLabels(filepath.Join(*dir, "labels.json"))
	if err != nil {
		return fail(stderr, false, err)
	}
	r := mood.Evaluate(feats, labels)
	r.Write(stdout)
	code := 0
	for _, c := range []struct {
		name string
		s    mood.Score
		min  float64
	}{{"mood", r.Mood, *minMood}, {"tempo", r.Tempo, *minTempo}, {"key", r.Key, *minKey}} {
		if c.min > 0 && (c.s.Total == 0 || 100*float64(c.s.Right)/float64(c.s.Total) < c.min) {
			fmt.Fprintf(stderr, "%s below %.1f%%\n", c.name, c.min)
			code = 1
		}
	}
	return code
}
