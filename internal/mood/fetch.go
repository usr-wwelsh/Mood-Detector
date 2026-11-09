package mood

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
)

const (
	jamendoTagPrefix = "mood/theme---"
	maxTrackBytes    = 32 << 20
)

var jamendoPath = regexp.MustCompile(`^[0-9]{1,3}/[0-9]{1,9}\.mp3$`)

var jamendoMoods = map[string][]string{
	"sad":       {"Melancholic/Sad", "Downtempo/Dark", "Minimal/Sparse"},
	"calm":      {"Ambient/Chill", "Ambient/Atmospheric", "Downtempo/Relaxed", "Relaxed/Moderate"},
	"energetic": {"Energetic/Rave", "Driving Electronic", "Upbeat/Moderate", "Hard/Aggressive"},
	"dark":      {"Techno/Dark", "Dark House", "Downtempo/Dark", "Atmospheric/Textural", "Ambient/Atmospheric", "Melancholic/Sad"},
	"happy":     {"Upbeat/Moderate", "House/Dance", "Disco/Funk", "Club/Groovy"},
	"party":     {"House/Dance", "Disco/Funk", "Club/Groovy", "Energetic/Rave"},
}

type JamendoTrack struct{ Path, Tag string }

func JamendoLabel(tag string) []string {
	return jamendoMoods[tag]
}

func ParseJamendo(r io.Reader) ([]JamendoTrack, error) {
	var out []JamendoTrack
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64<<10), 1<<20)
	sc.Scan()
	for sc.Scan() {
		f := strings.Split(sc.Text(), "\t")
		if len(f) < 6 {
			continue
		}
		var wanted []string
		for _, raw := range f[5:] {
			tag, ok := strings.CutPrefix(raw, jamendoTagPrefix)
			if _, mapped := jamendoMoods[tag]; ok && mapped && !slices.Contains(wanted, tag) {
				wanted = append(wanted, tag)
			}
		}
		if len(wanted) != 1 {
			continue
		}
		if !jamendoPath.MatchString(f[3]) {
			return nil, fmt.Errorf("unexpected track path %q", f[3])
		}
		out = append(out, JamendoTrack{f[3], wanted[0]})
	}
	return out, sc.Err()
}

func PickSlice(tracks []JamendoTrack, perTag int) []JamendoTrack {
	byTag := map[string][]JamendoTrack{}
	for _, t := range tracks {
		byTag[t.Tag] = append(byTag[t.Tag], t)
	}
	tags := make([]string, 0, len(byTag))
	for tag := range byTag {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	var out []JamendoTrack
	for _, tag := range tags {
		ts := byTag[tag]
		sort.Slice(ts, func(i, j int) bool { return ts[i].Path < ts[j].Path })
		n := min(perTag, len(ts))
		for i := 0; i < n; i++ {
			out = append(out, ts[i*len(ts)/n])
		}
	}
	return out
}

func FetchJamendoTrack(c *http.Client, audioBase string, t JamendoTrack, audioDir string) (string, error) {
	dest := filepath.Join(audioDir, t.Tag, strings.ReplaceAll(strings.TrimSuffix(t.Path, ".mp3"), "/", "_")+".low.mp3")
	url := strings.TrimRight(audioBase, "/") + "/" + strings.TrimSuffix(t.Path, ".mp3") + ".low.mp3"
	return fetchFile(c, url, dest, nil)
}

func fetchFile(c *http.Client, url, dest string, verify func([]byte) error) (string, error) {
	if _, err := os.Stat(dest); err == nil {
		return dest, nil
	}
	resp, err := c.Get(url) // #nosec G107 -- base is a flag, path is allowlisted
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: %s", url, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxTrackBytes+1))
	if err != nil {
		return "", err
	}
	if len(body) > maxTrackBytes {
		return "", fmt.Errorf("%s: larger than %d bytes", url, maxTrackBytes)
	}
	if verify != nil {
		if err := verify(body); err != nil {
			return "", fmt.Errorf("%s: %w", url, err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
		return "", err
	}
	tmp := dest + ".part"
	if err := writeFile(tmp, body); err != nil {
		return "", err
	}
	return dest, os.Rename(tmp, dest)
}
