package gitx

import (
	"fmt"
	"strings"
)

// HasOrigin reports whether the repository has a remote named origin.
func HasOrigin(dir string) bool {
	_, err := run(dir, "remote", "get-url", "origin")
	return err == nil
}

// OriginURL returns the origin remote URL.
func OriginURL(dir string) (string, error) {
	out, err := run(dir, "remote", "get-url", "origin")
	if err != nil {
		return "", err
	}
	u := strings.TrimSpace(out)
	if u == "" {
		return "", fmt.Errorf("origin URL empty")
	}
	return u, nil
}

// PushOrigin pushes branch to origin and sets upstream.
func PushOrigin(dir, branch string) error {
	dir = strings.TrimSpace(dir)
	branch = strings.TrimSpace(branch)
	if dir == "" || branch == "" {
		return fmt.Errorf("dir and branch are required")
	}
	if !HasOrigin(dir) {
		return fmt.Errorf("no origin remote")
	}
	_, err := run(dir, "push", "-u", "origin", branch)
	return err
}
