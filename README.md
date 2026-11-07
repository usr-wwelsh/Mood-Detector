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

## Benchmark

Tune detection against ~50 tracks without committing audio. Only extracted features (a dozen numbers per track, keyed by file hash) and your labels go in git.

```bash
mood bench extract ~/Music/bench-set
$EDITOR bench/labels.json
mood bench run
mood bench run --min-mood 70 --min-tempo 80
```

`labels.json` entries: `{"mood": ["Techno/Dark", "Techno/Industrial"], "tempo": 128, "key": "A minor"}`. Any field can be omitted. `mood` accepts several answers. Track filenames land in `bench/names.local.json` (gitignored) so you can map ids back to files.

`run` scores mood, tempo (±4%, plus an octave-tolerant score), and key (exact plus relative major/minor), then lists every miss. Classifier edits re-score instantly from the cached features. DSP edits need `bench extract` again, so bump `FeatureVersion`.

## Known issues

- Tracks above 0.8 energy at 80–145 BPM get their tempo doubled.
- Key detection works best with clear harmonic content.
- ffmpeg's resampler differs slightly from librosa's, so labels near thresholds can differ from the Python version.

MIT
