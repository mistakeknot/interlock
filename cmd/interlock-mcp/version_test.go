package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionFlagPrintsAndExitsBeforeServing(t *testing.T) {
	var out, errOut bytes.Buffer
	code, handled := handleVersionFlag([]string{"--version"}, &out, &errOut)
	if !handled || code != 0 {
		t.Fatalf("--version: handled=%v code=%d", handled, code)
	}
	if !strings.HasPrefix(out.String(), "interlock-mcp "+version) {
		t.Fatalf("output %q does not carry the version", out.String())
	}
}

func TestUnknownFlagIsRejected(t *testing.T) {
	var out, errOut bytes.Buffer
	code, handled := handleVersionFlag([]string{"--bogus"}, &out, &errOut)
	if !handled || code != 2 || !strings.Contains(errOut.String(), "unknown flag") {
		t.Fatalf("--bogus: handled=%v code=%d err=%q", handled, code, errOut.String())
	}
}

func TestNoArgsMeansServe(t *testing.T) {
	var out, errOut bytes.Buffer
	if _, handled := handleVersionFlag(nil, &out, &errOut); handled {
		t.Fatal("no arguments must fall through to serving")
	}
}
