//go:build !windows

package auth

// lockCredentialsACL is a no-op on non-Windows: os.WriteFile already
// honoured the 0600 mode passed at write time.
func lockCredentialsACL(_ string) {}
