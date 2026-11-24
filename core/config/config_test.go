package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLoadFromMissingFileReturnsDefaults(t *testing.T) {
	got, err := LoadFrom(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if err != nil {
		t.Fatalf("a missing configuration file must not be an error: %v", err)
	}
	if got.Language != "" {
		t.Errorf("Language = %q, want the zero value", got.Language)
	}
}

func TestSaveThenLoadRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")

	if err := SaveTo(path, Config{Language: "es"}); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}
	got, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if got.Language != "es" {
		t.Errorf("Language = %q, want es", got.Language)
	}
}

func TestSaveCreatesAPrivateFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix file modes are not meaningful on Windows")
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err := SaveTo(path, Config{Language: "en"}); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != filePerm {
		t.Errorf("mode = %o, want %o", perm, filePerm)
	}
}

func TestLoadFromRejectsMalformedJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("{not json"), filePerm); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFrom(path); err == nil {
		t.Error("expected an error for malformed JSON")
	}
}

func TestLoadAndSaveUseTheUserConfigDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AppData", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	path, err := Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if !strings.HasPrefix(path, dir) {
		t.Fatalf("Path = %q, want it inside %q", path, dir)
	}

	empty, err := Load()
	if err != nil {
		t.Fatalf("Load before any save must return defaults: %v", err)
	}
	if empty.Language != "" {
		t.Errorf("Language = %q, want empty", empty.Language)
	}

	if err := Save(Config{Language: "es"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Language != "es" {
		t.Errorf("Language = %q, want es", got.Language)
	}
}

func TestLoadFromRejectsAnOversizedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, make([]byte, maxFileSize+1), filePerm); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFrom(path); err == nil {
		t.Error("expected an error for a file past the size cap")
	}
}

func TestErrorsDoNotRevealTheConfigPath(t *testing.T) {
	dir := t.TempDir()
	malformed := filepath.Join(dir, "malformed.json")
	oversized := filepath.Join(dir, "oversized.json")
	directory := filepath.Join(dir, "a-directory")
	if err := os.WriteFile(malformed, []byte("{not json"), filePerm); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oversized, make([]byte, maxFileSize+1), filePerm); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(directory, dirPerm); err != nil {
		t.Fatal(err)
	}
	blocked := filepath.Join(malformed, "nested", "config.json")

	errs := map[string]error{}
	for name, path := range map[string]string{"malformed": malformed, "oversized": oversized, "directory": directory} {
		_, errs[name] = LoadFrom(path)
	}
	errs["unwritable"] = SaveTo(blocked, Config{Language: "es"})

	for name, err := range errs {
		if err == nil {
			t.Errorf("%s: expected an error", name)
			continue
		}
		if strings.Contains(err.Error(), dir) || strings.Contains(err.Error(), filepath.ToSlash(dir)) {
			t.Errorf("%s: error reveals the local path: %v", name, err)
		}
	}
}

func TestPathIsInsideTheUserConfigDir(t *testing.T) {
	got, err := Path()
	if err != nil {
		t.Skipf("no user configuration directory on this machine: %v", err)
	}
	if filepath.Base(got) != fileName || filepath.Base(filepath.Dir(got)) != dirName {
		t.Errorf("Path = %q, want .../%s/%s", got, dirName, fileName)
	}
}
