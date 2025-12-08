# Integration Guide

This guide covers how to integrate Mood Detector into your projects using different methods.

## Python Package Integration

### Installation
```bash
pip install mood-detector
```

### Basic Integration
```python
from mood_detector import analyze_audio

def process_song(file_path):
    result = analyze_audio(file_path)
    return {
        'mood': result.mood,
        'energy': result.energy,
        'tempo': result.tempo
    }

# Usage
analysis = process_song("song.mp3")
print(f"Detected mood: {analysis['mood']}")
```

### Advanced Options
```python
from mood_detector import analyze_audio

# Detailed analysis with similarity scores
result = analyze_audio(
    "song.mp3",
    detailed=True,
    similarity_search=True
)

print(f"Mood: {result.mood}")
print(f"Similar moods: {result.similarity_scores}")
```

## REST API Integration

### With Docker
1. Start the API server:
```bash
docker run -p 8000:8000 usr-wwelsh/mood-detector
```

2. Make API requests to `http://localhost:8000`

### Example: Python requests
```python
import requests

def analyze_with_api(file_path):
    url = "http://localhost:8000/analyze"
    files = {"file": open(file_path, "rb")}
    response = requests.post(url, files=files)
    return response.json()

# Usage
result = analyze_with_api("song.mp3")
print(f"Mood: {result['mood']}")
```

### Example: JavaScript fetch
```javascript
async function analyzeWithAPI(file) {
  const formData = new FormData();
  formData.append('file', file);
  
  const response = await fetch('http://localhost:8000/analyze', {
    method: 'POST',
    body: formData
  });
  
  return await response.json();
}

// Usage
const fileInput = document.querySelector('input[type="file"]');
const result = await analyzeWithAPI(fileInput.files[0]);
console.log(`Mood: ${result.mood}`);
```

### Example: cURL
```bash
curl -X POST http://localhost:8000/analyze \
  -F "file=@song.mp3" \
  -F "detailed=true"
```

## CLI Tool Integration

### In Shell Scripts
```bash
# Analyze a song and capture output
MOOD=$(mood analyze song.mp3 | grep "Mood:" | cut -d' ' -f2-)
echo "Detected mood: $MOOD"
```

### Batch Processing
```bash
# Process multiple files
for file in *.mp3; do
  mood analyze "$file"
done
```

## Web Application Integration

### Frontend (JavaScript)
```javascript
// HTML
// <input type="file" id="audioFile" accept="audio/*">

document.getElementById('audioFile').addEventListener('change', async (event) => {
  const file = event.target.files[0];
  const formData = new FormData();
  formData.append('file', file);
  
  const response = await fetch('http://localhost:8000/analyze', {
    method: 'POST',
    body: formData
  });
  
  const result = await response.json();
  
  document.getElementById('mood-result').innerText = 
    `Mood: ${result.mood}, Energy: ${result.energy}`;
});
```

### Backend (Node.js/Express)
```javascript
const express = require('express');
const multer = require('multer');
const axios = require('axios');
const FormData = require('form-data');

const app = express();
const upload = multer({ dest: 'uploads/' });

app.post('/analyze', upload.single('audio'), async (req, res) => {
  const form = new FormData();
  form.append('file', req.file.buffer, req.file.originalname);
  
  try {
    const response = await axios.post('http://localhost:8000/analyze', form, {
      headers: form.getHeaders()
    });
    
    res.json(response.data);
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});
```

## Error Handling

### Python
```python
from mood_detector import analyze_audio

try:
    result = analyze_audio("song.mp3")
    print(f"Mood: {result.mood}")
except FileNotFoundError:
    print("Audio file not found")
except ValueError as e:
    print(f"Invalid file format: {e}")
except Exception as e:
    print(f"Analysis failed: {e}")
```

### API
```python
import requests

def safe_analyze(file_path):
    try:
        with open(file_path, 'rb') as f:
            files = {'file': f}
            response = requests.post('http://localhost:8000/analyze', files=files)
            
        if response.status_code == 200:
            return response.json()
        else:
            print(f"API error: {response.status_code} - {response.text}")
            return None
            
    except Exception as e:
        print(f"Request failed: {e}")
        return None
```

## Performance Considerations

- Audio analysis typically takes 1-5 seconds depending on file length
- Consider processing files in batches for better performance
- For real-time applications, consider caching results
- Large files (over 5 minutes) may take significantly longer to process

## Common Integration Patterns

### Music Playlist Curation
```python
from mood_detector import analyze_audio

def create_mood_playlist(songs, target_mood):
    matching_songs = []
    
    for song in songs:
        result = analyze_audio(song)
        if target_mood.lower() in result.mood.lower():
            matching_songs.append(song)
    
    return matching_songs
```

### Mood-Based Music Discovery
```python
def suggest_similar_mood(song_path, all_songs):
    original_result = analyze_audio(song_path, similarity_search=True)
    target_mood = original_result.mood
    
    suggestions = []
    for song in all_songs:
        if song != song_path:  # Don't include the original song
            result = analyze_audio(song)
            if result.mood == target_mood:
                suggestions.append(song)
    
    return suggestions
```