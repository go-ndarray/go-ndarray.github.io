// Copyright (c) the go-ndarray authors.
// SPDX-License-Identifier: BSD-3-Clause

package playground

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// captureDir is where a test writes a rendered frame for a person to look at:
// $PLAYGROUND_CAPTURE_DIR, else <user config dir>/go-ndarray-playground/captures.
// It is durable (unlike t.TempDir) and it is refused when it lies inside a git
// work tree, so a capture can never be one `git add` away from publication.
func captureDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("PLAYGROUND_CAPTURE_DIR")
	if dir == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			t.Skipf("no user config dir: %v", err)
		}
		dir = filepath.Join(base, "go-ndarray-playground", "captures")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if tree := gitTreeOf(abs); tree != "" {
		t.Fatalf("refusing to write captures inside the git work tree %s", tree)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		t.Fatal(err)
	}
	return abs
}

// gitTreeOf returns the first ancestor of dir (or dir itself) holding a .git,
// or "" when there is none up to the filesystem root.
func gitTreeOf(dir string) string {
	for d := dir; ; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, ".git")); err == nil {
			return d
		}
		if filepath.Dir(d) == d {
			return ""
		}
	}
}

// savePNG writes an RGBA frame to the capture directory and logs where.
func savePNG(t *testing.T, name string, buf []byte, w, h int) {
	t.Helper()
	img := &image.RGBA{Pix: buf, Stride: 4 * w, Rect: image.Rect(0, 0, w, h)}
	path := filepath.Join(captureDir(t), name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	t.Logf("capture: %s", path)
}
