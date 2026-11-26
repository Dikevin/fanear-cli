package source

import (
	"os"
	"path/filepath"
	"testing"
)

func mkday(t *testing.T, records string) {
	t.Helper()
	d := filepath.Join(records, "2026", "11", "24")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "11月24日 饭耳总结.md"), []byte("#"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestResolveAnyLevel(t *testing.T) {
	root := t.TempDir()
	records := filepath.Join(root, "share", "饭耳", "记录")
	mkday(t, records)
	for _, p := range []string{root, filepath.Join(root, "share"), records} {
		got, err := Resolve(p)
		if err != nil || got != records {
			t.Errorf("Resolve(%s) = %q, %v", p, got, err)
		}
	}
	if _, err := Resolve(t.TempDir()); err == nil {
		t.Error("empty dir should not resolve")
	}
	// A year folder without any FanEar file is not a records folder.
	if err := os.MkdirAll(filepath.Join(root, "other", "2026", "01", "01"), 0o755); err != nil {
		t.Fatal(err)
	}
	if IsRecords(filepath.Join(root, "other")) {
		t.Error("plain date folders are not FanEar records")
	}
}

func TestDiscoverMac(t *testing.T) {
	home := t.TempDir()
	icloud := filepath.Join(home, "Library", "Mobile Documents", "iCloud~dev~fandao~fanear", "Documents", "记录")
	gdrive := filepath.Join(home, "Library", "CloudStorage", "GoogleDrive-someone@example.com", "My Drive", "饭耳", "记录")
	mkday(t, gdrive)
	o := Options{Getenv: func(string) string { return "" }, Home: home, GOOS: "darwin"}

	c := Discover(o)
	if len(c) != 1 || c[0].Records != gdrive || c[0].Label != "Google Drive 桌面版" {
		t.Fatalf("Discover = %+v", c)
	}
	mkday(t, icloud)
	r, err := Locate(o)
	if err != nil || r.Records != icloud {
		t.Fatalf("iCloud should win: %+v %v", r, err)
	}
	if got := Discover(Options{Getenv: o.Getenv, Home: home, GOOS: "linux"}); got != nil {
		t.Fatalf("no auto-discovery off macOS, got %+v", got)
	}
}

func TestFindRestore(t *testing.T) {
	root := t.TempDir()
	records := filepath.Join(root, "Documents", "记录")
	mkday(t, records)
	if FindRestore(records) != "" {
		t.Fatal("no backup yet")
	}
	// A hidden backup folder is recognised by its content.
	rp := filepath.Join(root, ".backup")
	dev := filepath.Join(rp, "DEV-2")
	for _, d := range []string{filepath.Join(rp, "DEV-1"), dev} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "manifest.json"), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(rp, "main-phone.json"), []byte(`{"deviceID":"DEV-2"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := FindRestore(records); got != dev {
		t.Fatalf("FindRestore = %q, want %q", got, dev)
	}
}

func TestConfigPath(t *testing.T) {
	env := map[string]string{"XDG_CONFIG_HOME": "/x"}
	o := Options{Getenv: func(k string) string { return env[k] }, Home: "/h"}
	if got := ConfigPath(o); got != "/x/fanear/config.json" {
		t.Fatal(got)
	}
	env = nil
	if got := ConfigPath(o); got != "/h/.config/fanear/config.json" {
		t.Fatal(got)
	}
}
