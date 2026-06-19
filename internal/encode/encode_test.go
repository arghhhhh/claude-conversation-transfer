package encode

import "testing"

func TestEncodeWindows(t *testing.T) {
	got := Encode(`C:\Users\joss\Desktop\Projects\Mine`)
	want := "C--Users-joss-Desktop-Projects-Mine"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestEncodePOSIX(t *testing.T) {
	got := Encode("/Users/joss/work/Mine")
	want := "-Users-joss-work-Mine"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestDetectOS(t *testing.T) {
	cases := map[string]OS{
		`C:\Users\x`:  Windows,
		`C:/Users/x`:  Windows,
		`/Users/joss`: POSIX,
		`/home/joss`:  POSIX,
	}
	for in, want := range cases {
		if got := DetectOS(in); got != want {
			t.Errorf("DetectOS(%q)=%v want %v", in, got, want)
		}
	}
}

func TestDecodeFilenameHint(t *testing.T) {
	cwd, os, err := DecodeFilenameHint("C--Users-joss-Desktop-Projects-Mine")
	if err != nil {
		t.Fatal(err)
	}
	if os != Windows || cwd != `C:\Users\joss\Desktop\Projects\Mine` {
		t.Fatalf("got %q %v", cwd, os)
	}
	cwd, os, err = DecodeFilenameHint("-Users-joss-work-Mine")
	if err != nil {
		t.Fatal(err)
	}
	if os != POSIX || cwd != "/Users/joss/work/Mine" {
		t.Fatalf("got %q %v", cwd, os)
	}
	// Underscore variant.
	cwd, os, _ = DecodeFilenameHint("C__Users_joss_work")
	if os != Windows || cwd != `C:\Users\joss\work` {
		t.Fatalf("underscore variant: got %q %v", cwd, os)
	}
}
