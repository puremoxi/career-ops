//go:build linux

package main

import (
	"errors"
	"os/exec"
	"testing"
)

func TestOpenWithDefaultAppUsesDiscoveredXDGOpen(t *testing.T) {
	previousRun := runOpenCommand
	previousLookPath := lookPath
	defer func() {
		runOpenCommand = previousRun
		lookPath = previousLookPath
	}()

	lookPath = func(file string) (string, error) {
		if file == "xdg-open" {
			return "/snap/codex/current/usr/bin/xdg-open", nil
		}
		return "", exec.ErrNotFound
	}

	var gotName string
	var gotArgs []string
	runOpenCommand = func(name string, args ...string) error {
		gotName = name
		gotArgs = append([]string(nil), args...)
		return nil
	}

	target := "/tmp/example.pdf"
	if err := openWithDefaultApp(target); err != nil {
		t.Fatalf("openWithDefaultApp returned error: %v", err)
	}
	if gotName != "/snap/codex/current/usr/bin/xdg-open" {
		t.Fatalf("command name = %q, want %q", gotName, "/snap/codex/current/usr/bin/xdg-open")
	}
	if len(gotArgs) != 1 || gotArgs[0] != target {
		t.Fatalf("command args = %#v, want single target arg %q", gotArgs, target)
	}
}

func TestOpenWithDefaultAppFallsBackToGio(t *testing.T) {
	previousRun := runOpenCommand
	previousLookPath := lookPath
	defer func() {
		runOpenCommand = previousRun
		lookPath = previousLookPath
	}()

	lookPath = func(file string) (string, error) {
		switch file {
		case "xdg-open":
			return "", exec.ErrNotFound
		case "gio":
			return "/usr/bin/gio", nil
		default:
			return "", exec.ErrNotFound
		}
	}

	var gotName string
	var gotArgs []string
	runOpenCommand = func(name string, args ...string) error {
		gotName = name
		gotArgs = append([]string(nil), args...)
		return nil
	}

	target := "/tmp/example.pdf"
	if err := openWithDefaultApp(target); err != nil {
		t.Fatalf("openWithDefaultApp returned error: %v", err)
	}
	if gotName != "/usr/bin/gio" {
		t.Fatalf("command name = %q, want %q", gotName, "/usr/bin/gio")
	}
	wantArgs := []string{"open", target}
	if len(gotArgs) != len(wantArgs) || gotArgs[0] != wantArgs[0] || gotArgs[1] != wantArgs[1] {
		t.Fatalf("command args = %#v, want %#v", gotArgs, wantArgs)
	}
}

func TestOpenWithDefaultAppReturnsHelpfulErrorWithoutOpeners(t *testing.T) {
	previousRun := runOpenCommand
	previousLookPath := lookPath
	defer func() {
		runOpenCommand = previousRun
		lookPath = previousLookPath
	}()

	lookPath = func(file string) (string, error) {
		return "", exec.ErrNotFound
	}
	runOpenCommand = func(name string, args ...string) error {
		return exec.ErrNotFound
	}

	err := openWithDefaultApp("/tmp/example.pdf")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, exec.ErrNotFound) && err.Error() != "could not find a desktop opener; install xdg-utils or ensure xdg-open is on PATH" {
		t.Fatalf("unexpected error: %v", err)
	}
}
