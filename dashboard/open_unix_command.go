//go:build darwin || linux

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var runOpenCommand = func(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Env = withDefaultOpenPath(os.Environ())
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	text := strings.TrimSpace(string(out))
	if text == "" {
		return err
	}
	return fmt.Errorf("%w: %s", err, text)
}

var lookPath = exec.LookPath

func withDefaultOpenPath(env []string) []string {
	const key = "PATH="
	defaultPath := strings.Join([]string{
		"/usr/local/bin",
		"/usr/bin",
		"/bin",
		"/snap/bin",
		"/snap/codex/current/usr/bin",
		"/snap/codex/34/usr/bin",
	}, ":")

	for i, entry := range env {
		if !strings.HasPrefix(entry, key) {
			continue
		}
		current := strings.TrimPrefix(entry, key)
		if current == "" {
			env[i] = key + defaultPath
			return env
		}
		env[i] = key + current + ":" + defaultPath
		return env
	}
	return append(env, key+defaultPath)
}

func findCommand(name string) (string, error) {
	for _, dir := range []string{
		"/usr/local/bin",
		"/usr/bin",
		"/bin",
		"/snap/bin",
		"/snap/codex/current/usr/bin",
		"/snap/codex/34/usr/bin",
	} {
		candidate := filepath.Join(dir, name)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}

	if path, err := lookPath(name); err == nil {
		return path, nil
	}
	return "", exec.ErrNotFound
}
