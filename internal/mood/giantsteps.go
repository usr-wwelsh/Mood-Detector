package mood

import (
	"archive/tar"
	"compress/gzip"
	"crypto/md5" // #nosec G501 -- dataset publishes md5 sums; integrity check only
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	maxAnnotationBytes = 4 << 10
	maxTarEntries      = 20000
	maxTarBytes        = 64 << 20
)

var (
	gsID  = regexp.MustCompile(`^[0-9]{1,9}\.LOFI$`)
	gsMD5 = regexp.MustCompile(`^[0-9a-f]{32}$`)
)

var gsStyles = map[string][]string{
	"techno":               {"Techno/Dark", "Techno/Industrial"},
	"hardcore-hard-techno": {"Techno/Industrial", "Hard/Aggressive", "Energetic/Rave"},
	"hard-dance":           {"Energetic/Rave", "Hard/Aggressive", "Techno/Industrial"},
	"house":                {"House/Dance", "Dark House", "Club/Groovy"},
	"deep-house":           {"House/Dance", "Dark House", "Club/Groovy"},
	"tech-house":           {"House/Dance", "Dark House", "Club/Groovy"},
	"progressive-house":    {"House/Dance", "Dark House", "Club/Groovy", "Driving Electronic"},
	"electro-house":        {"House/Dance", "Dark House", "Club/Groovy", "Energetic/Rave"},
	"drum-and-bass":        {"Drum & Bass"},
	"breaks":               {"Breakbeat/Fast", "Club/Groovy"},
	"trance":               {"Driving Electronic", "Energetic/Rave", "Fast/Atmospheric"},
	"psy-trance":           {"Driving Electronic", "Energetic/Rave", "Fast/Atmospheric"},
	"chill-out":            {"Ambient/Chill", "Downtempo/Relaxed", "Downtempo/Dark"},
	"glitch-hop":           {"Glitch/Experimental"},
	"indie-dance-nu-disco": {"Disco/Funk", "House/Dance"},
	"minimal":              {"Minimal/Sparse", "Techno/Dark"},
}

type GSTrack struct {
	ID    string
	BPM   float64
	Genre string
	MD5   string
}

func GSLabel(t GSTrack) Label {
	return Label{Style: gsStyles[t.Genre], Tempo: t.BPM}
}

func ParseGiantSteps(r io.Reader) ([]GSTrack, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(io.LimitReader(gz, maxTarBytes))
	byID := map[string]*GSTrack{}
	get := func(id string) *GSTrack {
		if byID[id] == nil {
			byID[id] = &GSTrack{ID: id}
		}
		return byID[id]
	}
	for n := 0; ; n++ {
		if n > maxTarEntries {
			return nil, fmt.Errorf("tarball has more than %d entries", maxTarEntries)
		}
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		_, rel, _ := strings.Cut(h.Name, "/")
		if path.Clean(rel) != rel {
			return nil, fmt.Errorf("unexpected tar path %q", h.Name)
		}
		dir, file := path.Split(rel)
		var kind string
		switch dir {
		case "md5/":
			kind = ".md5"
		case "annotations_v2/tempo/":
			kind = ".bpm"
		case "annotations/genre/":
			kind = ".genre"
		default:
			continue
		}
		id, ok := strings.CutSuffix(file, kind)
		if !ok {
			continue
		}
		if !gsID.MatchString(id) {
			return nil, fmt.Errorf("unexpected track id %q", id)
		}
		raw, err := io.ReadAll(io.LimitReader(tr, maxAnnotationBytes))
		if err != nil {
			return nil, err
		}
		val := strings.TrimSpace(string(raw))
		t := get(id)
		switch kind {
		case ".md5":
			if !gsMD5.MatchString(val) {
				return nil, fmt.Errorf("%s: malformed md5", id)
			}
			t.MD5 = val
		case ".bpm":
			bpm, err := strconv.ParseFloat(val, 64)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", id, err)
			}
			t.BPM = bpm
		case ".genre":
			t.Genre = val
		}
	}
	var out []GSTrack
	for _, t := range byID {
		if t.BPM > 0 && t.MD5 != "" {
			out = append(out, *t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func PickTempoSlice(tracks []GSTrack, n, outliers int) []GSTrack {
	ts := append([]GSTrack(nil), tracks...)
	sort.Slice(ts, func(i, j int) bool {
		if ts[i].BPM != ts[j].BPM {
			return ts[i].BPM < ts[j].BPM
		}
		return ts[i].ID < ts[j].ID
	})
	if n >= len(ts) {
		return ts
	}
	outliers = min(outliers, n/2)
	out := append([]GSTrack(nil), ts[:outliers]...)
	middle := ts[outliers : len(ts)-outliers]
	spread := n - 2*outliers
	for i := 0; i < spread; i++ {
		out = append(out, middle[i*len(middle)/spread])
	}
	return append(out, ts[len(ts)-outliers:]...)
}

func FetchGiantStepsTrack(c *http.Client, audioBase string, t GSTrack, audioDir string) (string, error) {
	url := strings.TrimRight(audioBase, "/") + "/" + t.ID + ".mp3"
	dest := filepath.Join(audioDir, "giantsteps", t.ID+".mp3")
	return fetchFile(c, url, dest, func(b []byte) error {
		sum := md5.Sum(b) // #nosec G401 -- integrity check against published sums
		if got := hex.EncodeToString(sum[:]); got != t.MD5 {
			return fmt.Errorf("md5 %s, want %s", got, t.MD5)
		}
		return nil
	})
}
