package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/usr-wwelsh/mood-detector/internal/mood"
)

const benchUsage = "usage: mood bench fetch [--set giantsteps|jamendo|all] [--dir bench] [--count N] [--outliers N] [--per-mood N]\n       mood bench extract [--dir bench] <audio-dir>\n       mood bench run [--dir bench] [--min-mood N] [--min-tempo N] [--min-key N]\n"

var audioExts = []string{".mp3", ".wav", ".flac", ".ogg", ".opus", ".m4a", ".aac"}

func bench(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, benchUsage)
		return 2
	}
	switch args[0] {
	case "fetch":
		return benchFetch(args[1:], stdout, stderr)
	case "extract":
		return benchExtract(args[1:], stdout, stderr)
	case "run":
		return benchRun(args[1:], stdout, stderr)
	default:
		fmt.Fprint(stderr, benchUsage)
		return 2
	}
}

const (
	jamendoMetaURL  = "https://raw.githubusercontent.com/MTG/mtg-jamendo-dataset/b43b942a99ce79902bc409728b9df81715b4cf5d/data/autotagging_moodtheme.tsv"
	jamendoAudioURL = "https://cdn.freesound.org/mtg-jamendo/raw_30s/audio-low"
	gsMetaURL       = "https://codeload.github.com/GiantSteps/giantsteps-tempo-dataset/tar.gz/d51ab2422e76abacfaa86616a57054bc222ec9fd"
	gsAudioURL      = "https://www.cp.jku.at/datasets/giantsteps/backup"
)

type fetchOpts struct {
	dir, jamendoMeta, jamendoAudio, gsMeta, gsAudio string
	perMood, count, outliers                        int
}

func benchFetch(args []string, stdout, stderr io.Writer) int {
	fl := flag.NewFlagSet("bench fetch", flag.ContinueOnError)
	fl.SetOutput(stderr)
	var o fetchOpts
	set := fl.String("set", "all", "dataset to fetch: giantsteps, jamendo or all")
	fl.StringVar(&o.dir, "dir", "bench", "bench data directory")
	fl.IntVar(&o.perMood, "per-mood", 10, "jamendo: tracks to download per mood")
	fl.IntVar(&o.count, "count", 60, "giantsteps: tracks to download")
	fl.IntVar(&o.outliers, "outliers", 4, "giantsteps: slowest and fastest tracks to always include")
	fl.StringVar(&o.jamendoMeta, "meta-url", jamendoMetaURL, "MTG-Jamendo mood/theme metadata TSV")
	fl.StringVar(&o.jamendoAudio, "audio-url", jamendoAudioURL, "MTG-Jamendo low-bitrate audio base URL")
	fl.StringVar(&o.gsMeta, "gs-meta-url", gsMetaURL, "GiantSteps tempo dataset tarball")
	fl.StringVar(&o.gsAudio, "gs-audio-url", gsAudioURL, "GiantSteps audio base URL")
	if err := fl.Parse(args); err != nil || fl.NArg() != 0 || o.perMood < 1 || o.count < 1 || o.outliers < 0 ||
		(*set != "all" && *set != "giantsteps" && *set != "jamendo") {
		fmt.Fprint(stderr, benchUsage)
		return 2
	}
	labelPath := filepath.Join(o.dir, "labels.json")
	labels, err := mood.LoadLabels(labelPath)
	if err != nil {
		return fail(stderr, false, err)
	}
	client := &http.Client{Timeout: 2 * time.Minute}
	audioDir := filepath.Join(o.dir, "audio")
	total := 0
	if *set == "all" || *set == "giantsteps" {
		n, err := fetchGiantSteps(client, o, audioDir, labels, stderr)
		if err != nil {
			return fail(stderr, false, err)
		}
		total += n
	}
	if *set == "all" || *set == "jamendo" {
		n, err := fetchJamendo(client, o, audioDir, labels, stderr)
		if err != nil {
			return fail(stderr, false, err)
		}
		total += n
	}
	if err := os.MkdirAll(o.dir, 0o750); err != nil {
		return fail(stderr, false, err)
	}
	if err := mood.SaveLabels(labelPath, labels); err != nil {
		return fail(stderr, false, err)
	}
	fmt.Fprintf(stdout, "fetched %d tracks into %s; now run: mood bench extract %s\n", total, audioDir, audioDir)
	fmt.Fprintln(stdout, "GiantSteps (CC BY-SA annotations, Beatport previews) and MTG-Jamendo (Creative Commons) are for research use; see github.com/GiantSteps/giantsteps-tempo-dataset and github.com/MTG/mtg-jamendo-dataset")
	return 0
}

func getBody(c *http.Client, url string) (io.ReadCloser, error) {
	resp, err := c.Get(url) // #nosec G107 -- user-supplied flag
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	return resp.Body, nil
}

func fetchGiantSteps(c *http.Client, o fetchOpts, audioDir string, labels map[string]mood.Label, stderr io.Writer) (int, error) {
	fmt.Fprintln(stderr, "giantsteps: fetching annotations...")
	body, err := getBody(c, o.gsMeta)
	if err != nil {
		return 0, err
	}
	defer body.Close()
	tracks, err := mood.ParseGiantSteps(body)
	if err != nil {
		return 0, err
	}
	picked := mood.PickTempoSlice(tracks, o.count, o.outliers)
	for i, t := range picked {
		fmt.Fprintf(stderr, "[%d/%d] giantsteps %-22s %5.1f bpm\n", i+1, len(picked), t.Genre, t.BPM)
		path, err := mood.FetchGiantStepsTrack(c, o.gsAudio, t, audioDir)
		if err != nil {
			return 0, err
		}
		id, err := mood.TrackID(path)
		if err != nil {
			return 0, err
		}
		l := labels[id]
		g := mood.GSLabel(t)
		l.Tempo, l.Style = g.Tempo, g.Style
		labels[id] = l
	}
	return len(picked), nil
}

func fetchJamendo(c *http.Client, o fetchOpts, audioDir string, labels map[string]mood.Label, stderr io.Writer) (int, error) {
	fmt.Fprintln(stderr, "jamendo: fetching track list...")
	body, err := getBody(c, o.jamendoMeta)
	if err != nil {
		return 0, err
	}
	defer body.Close()
	tracks, err := mood.ParseJamendo(body)
	if err != nil {
		return 0, err
	}
	picked := mood.PickSlice(tracks, o.perMood)
	for i, t := range picked {
		fmt.Fprintf(stderr, "[%d/%d] jamendo %-9s %s\n", i+1, len(picked), t.Tag, t.Path)
		path, err := mood.FetchJamendoTrack(c, o.jamendoAudio, t, audioDir)
		if err != nil {
			return 0, err
		}
		id, err := mood.TrackID(path)
		if err != nil {
			return 0, err
		}
		l := labels[id]
		l.Mood = mood.JamendoLabel(t.Tag)
		labels[id] = l
	}
	return len(picked), nil
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
		fmt.Fprintf(stderr, "[%d] extracting %s\n", added+failed+1, d.Name())
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
