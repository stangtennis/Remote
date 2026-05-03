//go:build !darwin

package encoder

// tryVideoToolbox stub på non-macOS — VideoToolbox er kun Apple.
func tryVideoToolbox(cfg Config) Encoder {
	return nil
}
