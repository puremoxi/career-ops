//go:build linux

package main

import "fmt"

func openWithDefaultApp(target string) error {
	if opener, err := findCommand("xdg-open"); err == nil {
		openErr := runOpenCommand(opener, target)
		if openErr == nil {
			return nil
		}
		if gio, gioErr := findCommand("gio"); gioErr == nil {
			if err := runOpenCommand(gio, "open", target); err == nil {
				return nil
			}
		}
		return openErr
	}

	if opener, err := findCommand("gio"); err == nil {
		return runOpenCommand(opener, "open", target)
	}

	return fmt.Errorf("could not find a desktop opener; install xdg-utils or ensure xdg-open is on PATH")
}
