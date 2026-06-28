//go:build !windows && !darwin

package credentials

import "fmt"

func saveSecret(key, value string) error {
	return fmt.Errorf("OS secret store is not available on this platform")
}

func loadSecret(key string) (string, error) {
	return "", nil
}

func deleteSecret(key string) error {
	return nil
}
