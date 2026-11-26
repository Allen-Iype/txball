package score

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPath(t *testing.T) {
	tests := []struct {
		name string
		xdg  string
		home string
		want string
	}{
		{"defaults to ~/.config", "", "/home/u", "/home/u/.config/txball/highscore"},
		{"honours XDG_CONFIG_HOME", "/cfg", "/home/u", "/cfg/txball/highscore"},
		{"ignores a relative XDG_CONFIG_HOME", "cfg", "/home/u", "/home/u/.config/txball/highscore"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", tt.xdg)
			t.Setenv("HOME", tt.home)
			got, err := Path()
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("Path() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		content *string // nil means no file
		want    int
		wantErr bool
	}{
		{"missing file is zero", nil, 0, false},
		{"plain number", ptr("1230\n"), 1230, false},
		{"no trailing newline", ptr("40"), 40, false},
		{"surrounding whitespace", ptr("  70 \r\n"), 70, false},
		{"zero", ptr("0\n"), 0, false},
		{"empty file", ptr(""), 0, true},
		{"not a number", ptr("lots\n"), 0, true},
		{"negative", ptr("-5\n"), 0, true},
		{"trailing junk", ptr("12 monkeys\n"), 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "highscore")
			if tt.content != nil {
				if err := os.WriteFile(path, []byte(*tt.content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			got, err := Load(path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("Load = %d, want %d", got, tt.want)
			}
		})
	}
}

func ptr(s string) *string { return &s }

func TestSaveRoundTrip(t *testing.T) {
	// The directory doesn't exist yet: Save must create it.
	path := filepath.Join(t.TempDir(), "nested", "txball", "highscore")

	for _, n := range []int{120, 99990, 5} { // also overwrites with a lower value when told to
		if err := Save(path, n); err != nil {
			t.Fatalf("Save(%d): %v", n, err)
		}
		got, err := Load(path)
		if err != nil || got != n {
			t.Fatalf("Load after Save(%d) = %d, %v", n, got, err)
		}
	}

	data, _ := os.ReadFile(path)
	if string(data) != "5\n" {
		t.Errorf("file content = %q, want %q", data, "5\n")
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Errorf("directory holds %d entries, want only the score file (no leftover temp files)", len(entries))
	}
}

func TestSaveReportsFailure(t *testing.T) {
	// A file where the directory should be.
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Save(filepath.Join(blocker, "highscore"), 1); err == nil {
		t.Error("Save succeeded under a path that is a file")
	}
}
