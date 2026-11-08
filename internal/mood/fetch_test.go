package mood

import (
	"strings"
	"testing"
)

const jamendoTSV = "TRACK_ID\tARTIST_ID\tALBUM_ID\tPATH\tDURATION\tTAGS\n" +
	"track_1\ta\tb\t10/1.mp3\t200\tmood/theme---sad\n" +
	"track_2\ta\tb\t10/2.mp3\t200\tmood/theme---sad\tmood/theme---film\n" +
	"track_3\ta\tb\t10/3.mp3\t200\tmood/theme---sad\tmood/theme---happy\n" +
	"track_4\ta\tb\t10/4.mp3\t200\tmood/theme---film\n" +
	"track_5\ta\tb\t10/5.mp3\t200\tmood/theme---happy\n"

func TestParseJamendoKeepsTracksWithExactlyOneMappedMood(t *testing.T) {
	got, err := ParseJamendo(strings.NewReader(jamendoTSV))
	if err != nil {
		t.Fatal(err)
	}
	want := []JamendoTrack{{"10/1.mp3", "sad"}, {"10/2.mp3", "sad"}, {"10/5.mp3", "happy"}}
	if len(got) != len(want) {
		t.Fatalf("%+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%+v", got)
		}
	}
}

func TestParseJamendoRejectsPathsOutsideAllowlist(t *testing.T) {
	for _, p := range []string{"../etc/passwd", "10/../../x.mp3", "/abs/1.mp3", "10/1.sh", "10/1.mp3?x=1"} {
		tsv := "H\n" + "t\ta\tb\t" + p + "\t1\tmood/theme---sad\n"
		if _, err := ParseJamendo(strings.NewReader(tsv)); err == nil {
			t.Fatalf("accepted %q", p)
		}
	}
}

func TestPickSliceIsDeterministicAndCapsEachMood(t *testing.T) {
	var all []JamendoTrack
	for i := 0; i < 100; i++ {
		all = append(all, JamendoTrack{Path: "1/" + strings.Repeat("1", i%7+1) + string(rune('0'+i%10)) + ".mp3", Tag: "sad"})
	}
	all = append(all, JamendoTrack{"2/1.mp3", "happy"})
	a, b := PickSlice(all, 5), PickSlice(all, 5)
	counts := map[string]int{}
	for i, tr := range a {
		counts[tr.Tag]++
		if tr != b[i] {
			t.Fatal("not deterministic")
		}
	}
	if counts["sad"] != 5 || counts["happy"] != 1 {
		t.Fatalf("%v", counts)
	}
}

func TestPickSliceSpreadsAcrossCatalogueNotJustTheFirstTracks(t *testing.T) {
	var all []JamendoTrack
	for i := 0; i < 100; i++ {
		all = append(all, JamendoTrack{Path: "1/" + string(rune('a'+i/26)) + string(rune('a'+i%26)) + ".mp3", Tag: "sad"})
	}
	got := PickSlice(all, 4)
	if got[len(got)-1].Path == all[3].Path {
		t.Fatalf("%+v", got)
	}
}

func TestJamendoLabelMapsToClassifierMoods(t *testing.T) {
	if l := JamendoLabel("sad"); len(l) == 0 || l[0] != "Melancholic/Sad" {
		t.Fatalf("%v", l)
	}
	if JamendoLabel("nonsense") != nil {
		t.Fatal("unmapped tag returned labels")
	}
}
