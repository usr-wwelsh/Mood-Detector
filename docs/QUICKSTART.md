# Quick Start Guide

This guide will help you get started with Mood Detector in just a few minutes.

## Installation

### Option 1: Python Package
```bash
pip install mood-detector
```

### Option 2: Docker (Recommended for non-Python developers)
```bash
docker run -p 8000:8000 usr-wwelsh/mood-detector
```

### Option 3: Local Development
```bash
git clone https://github.com/usr-wwelsh/mood-detector
cd mood-detector
pip install -e .
```

## Quick Usage Examples

### Python Usage
```python
from mood_detector import analyze_audio

# Analyze an audio file
result = analyze_audio("song.mp3")
print(f"Mood: {result.mood}")
print(f"Energy: {result.energy}")
print(f"Tempo: {result.tempo} BPM")
```

### API Usage
```bash
curl -X POST http://localhost:8000/analyze \
  -F "file=@song.mp3"
```

### CLI Usage
```bash
mood analyze song.mp3
```

## First Analysis

### If using Python:
1. Install the package: `pip install mood-detector`
2. Run Python:
   ```python
   from mood_detector import analyze_audio
   
   result = analyze_audio("your-song.mp3")
   print(f"Mood: {result.mood}")
   ```

### If using Docker API:
1. Start the server: `docker run -p 8000:8000 usr-wwelsh/mood-detector`
2. In another terminal, run: `curl -X POST http://localhost:8000/analyze -F "file=@song.mp3"`

### If using CLI:
1. Install the package: `pip install mood-detector`
2. Run: `mood analyze song.mp3`

## Supported Audio Formats

- MP3
- WAV
- FLAC
- M4A
- AAC
- OGG

## Troubleshooting

### Common Issues

**"File not found" error:**
- Make sure the file path is correct and the file exists

**"Unsupported file format" error:**
- Check that your file is in one of the supported formats

**"Docker container won't start":**
- Ensure Docker is running on your machine
- Check that port 8000 is not already in use

### Performance Notes

- Large audio files may take longer to process
- The first analysis may be slower as the system loads necessary components
- For best results, use audio files up to 5 minutes in length