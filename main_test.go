package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/navidrome/navidrome/plugins/pdk/go/pdk"
	"github.com/stretchr/testify/mock"
)

// allowLogging lets pdk.Log run on the host, where it is a mock that panics
// without an expectation.
func allowLogging(t *testing.T) {
	pdk.ResetMock()
	pdk.PDKMock.On("Log", mock.Anything, mock.Anything).Return()
	t.Cleanup(pdk.ResetMock)
}

func TestNameVariants(t *testing.T) {
	tests := []struct {
		name  string
		first string   // the least invasive spelling must come first
		wants []string // every spelling that must be offered
		never []string
	}{
		{name: "Metallica"},
		{name: "Boney M.", first: "Boney M", wants: []string{"Boney M"}},
		{name: "P.O.D.", first: "P.O.D", wants: []string{"P.O.D"}},
		{name: "W.A.S.P.", first: "W.A.S.P", wants: []string{"W.A.S.P"}},
		{name: "AC/DC", first: "ACDC", wants: []string{"ACDC", "AC-DC", "AC_DC", "AC+DC", "AC DC"}},
		{name: "Doom:vs", first: "Doomvs", wants: []string{"Doomvs", "Doom-vs", "Doom_vs", "Doom vs"}},
		{name: "Mötley Crüe", first: "Motley Crue", wants: []string{"Motley Crue"}},
		{name: "Motörhead", first: "Motorhead", wants: []string{"Motorhead"}},
		{name: "Blue Öyster Cult", first: "Blue Oyster Cult", wants: []string{"Blue Oyster Cult"}},
		{name: "Mø", first: "Mo", wants: []string{"Mo"}},
		{name: "Hüsker Dü", first: "Husker Du", wants: []string{"Husker Du"}},
		{name: "Sigur Rós: Ágætis byrjun", wants: []string{"Sigur Ros- Agaetis byrjun", "Sigur Ros Agaetis byrjun", "Sigur Rós- Ágætis byrjun"}},
		{name: "Who?", first: "Who", wants: []string{"Who", "Who-"}, never: []string{""}},
		{name: "...", never: []string{"", "...", ".."}},
		{name: "???", never: []string{"", "???"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := nameVariants(tc.name)
			if tc.first == "" && len(tc.wants) == 0 && tc.never == nil && len(got) != 0 {
				t.Fatalf("nameVariants(%q) = %q, want none", tc.name, got)
			}
			if tc.first != "" && (len(got) == 0 || got[0] != tc.first) {
				t.Errorf("nameVariants(%q) starts with %q, want %q (all: %q)", tc.name, first(got), tc.first, got)
			}
			for _, want := range tc.wants {
				if !slices.Contains(got, want) {
					t.Errorf("nameVariants(%q) lacks %q (all: %q)", tc.name, want, got)
				}
			}
			for _, bad := range tc.never {
				if slices.Contains(got, bad) {
					t.Errorf("nameVariants(%q) offers %q (all: %q)", tc.name, bad, got)
				}
			}
			if slices.Contains(got, tc.name) {
				t.Errorf("nameVariants(%q) repeats the original name (all: %q)", tc.name, got)
			}
			seen := map[string]bool{}
			for _, v := range got {
				if seen[v] {
					t.Errorf("nameVariants(%q) repeats %q (all: %q)", tc.name, v, got)
				}
				seen[v] = true
			}
		})
	}
}

func first(items []string) string {
	if len(items) == 0 {
		return ""
	}
	return items[0]
}

func TestStripDiacritics(t *testing.T) {
	tests := map[string]string{
		"Mötley Crüe":    "Motley Crue",
		"Sólstafir":      "Solstafir",
		"Björk":          "Bjork",
		"Mø":             "Mo",
		"Łukasz":         "Lukasz",
		"Straße":         "Strasse",
		"Ólafur Arnalds": "Olafur Arnalds",
		"Dёргать!":       "Dергать!", // Cyrillic io decomposes to ie + diaeresis; the Latin D stays
		"Metallica":      "Metallica",
		"":               "",
	}
	for in, want := range tests {
		if got := stripDiacritics(in); got != want {
			t.Errorf("stripDiacritics(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFindMatchingDir(t *testing.T) {
	allowLogging(t)
	parent := t.TempDir()
	for _, dir := range []string{"ACDC", "AC-DC", "Boney M", "Doom-vs", "Motley Crue", "P.O.D", "W.A.S.P", "metallica", "Slayer"} {
		if err := os.Mkdir(filepath.Join(parent, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// a file, not a directory, must never match
	if err := os.WriteFile(filepath.Join(parent, "Iron Maiden"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		artist string
		want   string // directory name; "" means no match
	}{
		{artist: "Metallica", want: "metallica"}, // case-insensitive, as before
		{artist: "Slayer", want: "Slayer"},
		{artist: "AC/DC", want: "ACDC"}, // removal wins over replacement
		{artist: "Boney M.", want: "Boney M"},
		{artist: "Doom:vs", want: "Doom-vs"},
		{artist: "Mötley Crüe", want: "Motley Crue"},
		{artist: "P.O.D.", want: "P.O.D"},
		{artist: "W.A.S.P.", want: "W.A.S.P"},
		{artist: "Iron Maiden"},
		{artist: "Megadeth"},
		{artist: "Pöd"}, // "Pod" is not a directory; "P.O.D" must not be matched by accident
	}
	for _, tc := range tests {
		t.Run(tc.artist, func(t *testing.T) {
			got, ok := findMatchingDir(parent, tc.artist)
			if tc.want == "" {
				if ok {
					t.Fatalf("findMatchingDir(%q) = %q, want no match", tc.artist, got)
				}
				return
			}
			if !ok || got != filepath.Join(parent, tc.want) {
				t.Fatalf("findMatchingDir(%q) = %q, %v; want %q", tc.artist, got, ok, filepath.Join(parent, tc.want))
			}
		})
	}

	if _, ok := findMatchingDir(filepath.Join(parent, "does-not-exist"), "Metallica"); ok {
		t.Fatal("findMatchingDir on a missing parent must not match")
	}
}
