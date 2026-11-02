package mood

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	windowSeconds = 30.0
	execTimeout   = 120 * time.Second
)

func analysisWindow(total float64) (offset, duration float64) {
	switch {
	case total > 60:
		return 30, windowSeconds
	case total > 30:
		offset = total * 0.25
		return offset, math.Min(windowSeconds, total-offset)
	default:
		return 0, math.Min(windowSeconds, total)
	}
}

type probeInfo struct {
	duration float64
	channels int
}

func probe(ctx context.Context, path string) (probeInfo, error) {
	invalid := fmt.Errorf("invalid or unsupported audio file: %s", path)
	out, err := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-select_streams", "a:0",
		"-show_entries", "format=duration:stream=channels", "-of", "default=nw=1", "-i", path).Output()
	if err != nil {
		return probeInfo{}, invalid
	}
	var p probeInfo
	for _, line := range strings.Split(string(out), "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch k {
		case "duration":
			p.duration, _ = strconv.ParseFloat(v, 64)
		case "channels":
			p.channels, _ = strconv.Atoi(v)
		}
	}
	if p.duration <= 0 || math.IsInf(p.duration, 0) || math.IsNaN(p.duration) || p.channels < 1 || p.channels > 64 {
		return probeInfo{}, invalid
	}
	return p, nil
}

func downmixFilter(channels int) string {
	if channels == 1 {
		return "anull"
	}
	terms := make([]string, channels)
	for i := range terms {
		terms[i] = fmt.Sprintf("%.10f*c%d", 1/float64(channels), i)
	}
	return "pan=mono|c0=" + strings.Join(terms, "+")
}

func decode(ctx context.Context, path string, offset, duration float64, channels int) ([]float32, error) {
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "ffmpeg", "-v", "error", "-nostdin",
		"-ss", strconv.FormatFloat(offset, 'f', 3, 64),
		"-t", strconv.FormatFloat(duration, 'f', 3, 64),
		"-i", path, "-vn", "-af", downmixFilter(channels), "-ar", strconv.Itoa(SampleRate), "-ac", "1", "-f", "f32le", "pipe:1")
	cmd.Stderr = &stderr
	raw, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("decode failed: %s", strings.TrimSpace(stderr.String()))
	}
	y := make([]float32, len(raw)/4)
	for i := range y {
		y[i] = math.Float32frombits(binary.LittleEndian.Uint32(raw[i*4:]))
	}
	if len(y) == 0 {
		return nil, fmt.Errorf("invalid or unsupported audio file: %s", path)
	}
	return y, nil
}

func AnalyzeFile(path string) (Result, error) {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Result{}, fmt.Errorf("audio file not found: %s", path)
		}
		return Result{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), execTimeout)
	defer cancel()
	info, err := probe(ctx, path)
	if err != nil {
		return Result{}, err
	}
	total := info.duration
	offset, duration := analysisWindow(total)
	y, err := decode(ctx, path, offset, duration, info.channels)
	if err != nil {
		return Result{}, err
	}
	f := ExtractFeatures(y)
	f.Duration = total
	return Classify(f), nil
}
