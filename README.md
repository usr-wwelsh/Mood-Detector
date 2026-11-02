# 🎵 Mood Detector

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Read about the commits](https://img.shields.io/badge/commits-code%20blog-1a1a1a?style=flat-square)](https://wwel.sh/digest.html?repo=Mood-Detector)

Offline music mood, tempo, energy and key analysis. One static ~3 MB Go binary, no Python, no cloud.

Needs `ffmpeg` and `ffprobe` on `PATH` for decoding.

## Usage

```bash
go build -trimpath -ldflags="-s -w" -o mood ./cmd/mood

mood analyze track.mp3
mood analyze --json track.mp3
mood analyze --similarity --detailed track.mp3
mood batch *.mp3
```

`--json` prints `mood`, `energy`, `tempo`, `key`, `similarity_scores`, `explanation`. On failure it exits 1 with `{"error": "..."}` on stderr.

## How it works

Analyzes 30 s from the middle of the track (skips intros), then:

1. **Features:** tempo (onset-strength autocorrelation, half-tempo correction), RMS energy, spectral centroid and rolloff, zero-crossing rate, chroma for key.
2. **Rules:** tempo + energy + brightness + timbre + major/minor, with tempo confidence to catch drones and ambient. No ML.

The DSP follows librosa 0.11 and is tested against it on synthetic signals; the classifier is tested against golden output from the original Python implementation (see `main`).

## Moods

**Dance/Electronic:** House/Dance, Dark House, Disco/Funk, Club/Groovy, Techno/Dark, Techno/Industrial, Drum & Bass, Breakbeat/Fast, Energetic/Rave, Driving Electronic

**Chill/Ambient:** Ambient/Drone, Ambient/Atmospheric, Ambient/Chill, Atmospheric/Textural, Downtempo/Dark, Downtempo/Relaxed, Melancholic/Sad

**Experimental:** Glitch/Experimental, Noise/Experimental, Harsh Noise/Experimental

**High energy:** Hard/Aggressive

## Known issues

- Tracks above 0.8 energy at 80–145 BPM get their tempo doubled.
- Key detection works best with clear harmonic content.
- ffmpeg's resampler differs slightly from librosa's, so labels near thresholds can differ from the Python version.

MIT
