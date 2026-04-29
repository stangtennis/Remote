//go:build !windows

package clipboard

import "fmt"

// SessionHelper is a no-op stub on non-Windows platforms — Windows is the
// only OS where the agent runs in a different session than the user.
type SessionHelper struct{}

func NewSessionHelper() *SessionHelper                                 { return &SessionHelper{} }
func (h *SessionHelper) Start() error                                  { return fmt.Errorf("session helper only on windows") }
func (h *SessionHelper) Stop()                                         {}
func (h *SessionHelper) SetOnTextChange(_ func(string))                {}
func (h *SessionHelper) SetOnImageChange(_ func([]byte))               {}
func (h *SessionHelper) SetText(_ string) error                        { return nil }
func (h *SessionHelper) SetImage(_ []byte) error                       { return nil }
func (h *SessionHelper) RememberText(_ string)                         {}
func (h *SessionHelper) RememberImage(_ []byte)                        {}

// RunHelper is unreachable on non-windows; kept for build symmetry.
func RunHelper(_ string) error { return fmt.Errorf("clipboard helper only on windows") }
