package main

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "ssm.png")

	var stdout bytes.Buffer

	err := run([]string{"-o", path, "-px", "3", "-phrase", "8"}, &stdout)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"boundary before beat   8", "A′", "wrote " + path} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("output lacks %q:\n%s", want, stdout.String())
		}
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	img, err := png.Decode(f)
	if cerr := f.Close(); err == nil {
		err = cerr
	}

	if err != nil {
		t.Fatal(err)
	}

	// 6 phrases of 8 beats at 3 px, plus the novelty panel.
	if b := img.Bounds(); b.Dx() != 144 || b.Dy() != 144+panelGap+panelHeight {
		t.Fatalf("image %v", b)
	}
}

func TestRunErrors(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{
		{"-px", "0"},
		{"-phrase", "0"},
		{"-unknown"},
		{"-o", filepath.Join(t.TempDir(), "missing", "x.png")},
	} {
		if err := run(args, &bytes.Buffer{}); err == nil {
			t.Fatalf("args %v: no error", args)
		}
	}
}
