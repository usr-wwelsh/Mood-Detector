package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/usr-wwelsh/mood-detector/internal/mood"
)

const usage = "usage: mood analyze [--json] [--detailed] [--similarity] <file>\n       mood batch [--detailed] [--similarity] <file>...\n       mood bench extract|run (see mood bench)\n"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "analyze":
		return analyze(args[1:], stdout, stderr)
	case "batch":
		return batch(args[1:], stdout, stderr)
	case "bench":
		return bench(args[1:], stdout, stderr)
	default:
		fmt.Fprint(stderr, usage)
		return 2
	}
}

type options struct {
	json, detailed, similarity bool
	files                      []string
}

func parse(name string, args []string, stderr io.Writer) (options, bool) {
	var o options
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.BoolVar(&o.json, "json", false, "print JSON")
	fs.BoolVar(&o.detailed, "detailed", false, "include duration in explanation")
	fs.BoolVar(&o.similarity, "similarity", false, "show similarity scores")
	if err := fs.Parse(args); err != nil {
		return o, false
	}
	o.files = fs.Args()
	return o, true
}

func analyzeOne(path string, o options) (mood.Result, error) {
	r, err := mood.AnalyzeFile(path)
	if err != nil {
		return r, err
	}
	if o.detailed {
		r.Explanation += fmt.Sprintf(". Duration: %.2f seconds", r.Duration)
	}
	return r, nil
}

func fail(stderr io.Writer, jsonOut bool, err error) int {
	if jsonOut {
		b, _ := json.Marshal(map[string]string{"error": err.Error()})
		fmt.Fprintln(stderr, string(b))
	} else {
		fmt.Fprintf(stderr, "Error analyzing file: %v\n", err)
	}
	return 1
}

func analyze(args []string, stdout, stderr io.Writer) int {
	o, ok := parse("analyze", args, stderr)
	if !ok || len(o.files) != 1 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	r, err := analyzeOne(o.files[0], o)
	if err != nil {
		return fail(stderr, o.json, err)
	}
	if o.json {
		return emit(stdout, stderr, r)
	}
	fmt.Fprintln(stdout, "🎵 Analysis Complete")
	fmt.Fprintf(stdout, "Mood: %s\n", r.Mood)
	fmt.Fprintf(stdout, "Energy: %s %.1f\n", bar(r.Energy), r.Energy)
	fmt.Fprintf(stdout, "Tempo: %.0f BPM\n", r.Tempo)
	fmt.Fprintf(stdout, "Key: %s\n", r.Key)
	fmt.Fprintf(stdout, "Explanation: %s\n", r.Explanation)
	if o.similarity {
		fmt.Fprintln(stdout, "\nSimilar moods:")
		names := make([]string, 0, len(r.SimilarityScores))
		for k := range r.SimilarityScores {
			names = append(names, k)
		}
		sort.Slice(names, func(i, j int) bool { return r.SimilarityScores[names[i]] > r.SimilarityScores[names[j]] })
		for _, k := range names[:min(5, len(names))] {
			fmt.Fprintf(stdout, "  %s: %.2f\n", k, r.SimilarityScores[k])
		}
	}
	return 0
}

func emit(stdout, stderr io.Writer, v any) int {
	b, err := json.Marshal(v)
	if err != nil {
		return fail(stderr, true, err)
	}
	fmt.Fprintln(stdout, string(b))
	return 0
}

func batch(args []string, stdout, stderr io.Writer) int {
	o, ok := parse("batch", args, stderr)
	if !ok || len(o.files) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	fmt.Fprintf(stdout, "🎵 Analyzing %d files...\n", len(o.files))
	for _, path := range o.files {
		r, err := analyzeOne(path, o)
		if err != nil {
			fmt.Fprintf(stdout, "  ⚠️  Error analyzing '%s': %v\n", path, err)
			continue
		}
		fmt.Fprintf(stdout, "\n%s:\n  Mood: %s\n  Energy: %.2f\n  Tempo: %.0f BPM\n", filepath.Base(path), r.Mood, r.Energy, r.Tempo)
	}
	return 0
}

func bar(v float64) string {
	n := min(max(int(v*10), 0), 10)
	b := ""
	for i := 0; i < 10; i++ {
		if i < n {
			b += "█"
		} else {
			b += "░"
		}
	}
	return b
}
