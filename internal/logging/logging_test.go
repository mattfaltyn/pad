package logging

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRotatingFileWriterBoundsAndRetainsLogs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pad.log")
	w, err := newRotatingFileWriter(path, 8, 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range []string{"first\n", "second\n", "third\n", "fourth\n"} {
		if _, err := w.Write([]byte(entry)); err != nil {
			t.Fatal(err)
		}
	}
	for _, candidate := range []string{path, path + ".1", path + ".2"} {
		info, err := os.Stat(candidate)
		if err != nil {
			t.Fatalf("expected %s: %v", candidate, err)
		}
		if info.Size() > 8 {
			t.Fatalf("%s size = %d, want <= 8", candidate, info.Size())
		}
	}
	if _, err := os.Stat(path + ".3"); !os.IsNotExist(err) {
		t.Fatalf("unexpected third backup: %v", err)
	}
}
