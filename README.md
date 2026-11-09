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

The DSP follows librosa 0.11 and is tested against it on synthetic signals; the classifier started as a port of the Python implementation (see `main`) and has since diverged where the bench showed it was wrong.

## Moods

**Dance/Electronic:** House/Dance, Dark House, Disco/Funk, Club/Groovy, Techno/Dark, Techno/Industrial, Drum & Bass, Breakbeat/Fast, Energetic/Rave, Driving Electronic

**Chill/Ambient:** Ambient/Drone, Ambient/Atmospheric, Ambient/Chill, Atmospheric/Textural, Downtempo/Dark, Downtempo/Relaxed, Melancholic/Sad

**Experimental:** Glitch/Experimental, Noise/Experimental, Harsh Noise/Experimental

**High energy:** Hard/Aggressive

## Benchmark

Tune detection against ~50 tracks without committing audio. Only extracted features (a dozen numbers per track, keyed by file hash) and your labels go in git.

Or fetch small labeled slices of public datasets. Audio is downloaded straight from the dataset hosts into the gitignored `bench/audio/` and never committed:

```bash
mood bench fetch
mood bench extract bench/audio
mood bench run
```

- **GiantSteps tempo** (`--set giantsteps`, default 60 tracks): EDM previews with crowdsourced tempo and Beatport genre. Each file is verified against the dataset's md5. The slice always includes the `--outliers` slowest and fastest tracks (default 4 each), then spreads the rest across the tempo range. Genre maps to the classifier's style labels where one exists; tempo is scored for every track.
- **MTG-Jamendo** (`--set jamendo`, default 10 per mood): Creative Commons tracks tagged sad, calm, energetic, dark, happy or party. Only tracks with exactly one of those tags are used, and mood labels are seeded in `labels.json`.

Slices are deterministic, so scores are comparable across machines. Re-running skips files already downloaded.

Or bring your own tracks:

```bash
mood bench extract ~/Music/bench-set
$EDITOR bench/labels.json
mood bench run
mood bench run --min-mood 70 --min-tempo 80
```

`labels.json` entries: `{"mood": ["Techno/Dark", "Techno/Industrial"], "tempo": 128, "key": "A minor"}`. Any field can be omitted. `mood` accepts several answers. Track filenames land in `bench/names.local.json` (gitignored) so you can map ids back to files.

`run` scores mood, style (classifier label vs. genre), tempo (±4%, plus an octave-tolerant score), and key (exact plus relative major/minor), then lists every miss. Classifier edits re-score instantly from the cached features. DSP edits need `bench extract` again, so bump `FeatureVersion`.

## Known issues

- Half-time tracks (dubstep, hip-hop feel) often read at double tempo.
- Key detection works best with clear harmonic content.
- ffmpeg's resampler differs slightly from librosa's, so labels near thresholds can differ from the Python version.

MIT
