//go:build darwin

package encoder

// tryVideoToolbox forsøger at initialisere VideoToolbox-encoderen på macOS.
// Returnerer nil hvis ikke tilgængelig (FFmpeg mangler eller ingen HW).
func tryVideoToolbox(cfg Config) Encoder {
	if !IsVideoToolboxAvailable() {
		return nil
	}
	enc := NewVideoToolboxEncoder()
	if err := enc.Init(cfg); err != nil {
		return nil
	}
	return enc
}
