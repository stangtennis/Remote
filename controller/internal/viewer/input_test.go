package viewer

import "testing"

func TestScaleLocalToRemote_IdentityWhenSameSize(t *testing.T) {
	x, y := ScaleLocalToRemote(100, 200, 1920, 1080, 1920, 1080)
	if x != 100 || y != 200 {
		t.Fatalf("expected (100, 200), got (%v, %v)", x, y)
	}
}

func TestScaleLocalToRemote_ScalesUp(t *testing.T) {
	// Canvas 960x540, remote 1920x1080 → 2x scale
	x, y := ScaleLocalToRemote(100, 200, 960, 540, 1920, 1080)
	if x != 200 || y != 400 {
		t.Fatalf("expected (200, 400), got (%v, %v)", x, y)
	}
}

func TestScaleLocalToRemote_ScalesDown(t *testing.T) {
	// Canvas 3840x2160, remote 1920x1080 → 0.5x scale
	x, y := ScaleLocalToRemote(400, 600, 3840, 2160, 1920, 1080)
	if x != 200 || y != 300 {
		t.Fatalf("expected (200, 300), got (%v, %v)", x, y)
	}
}

func TestScaleLocalToRemote_AsymmetricStretch(t *testing.T) {
	// Canvas 1000x1000, remote 1920x1080 → different x/y scales
	x, y := ScaleLocalToRemote(500, 500, 1000, 1000, 1920, 1080)
	if x != 960 {
		t.Fatalf("expected x=960, got %v", x)
	}
	if y != 540 {
		t.Fatalf("expected y=540, got %v", y)
	}
}

func TestScaleLocalToRemote_ZeroCanvasReturnsOrigin(t *testing.T) {
	x, y := ScaleLocalToRemote(100, 100, 0, 0, 1920, 1080)
	if x != 0 || y != 0 {
		t.Fatalf("expected (0,0) when canvas is zero, got (%v, %v)", x, y)
	}
}

func TestScaleLocalToRemote_OriginMapsToOrigin(t *testing.T) {
	x, y := ScaleLocalToRemote(0, 0, 800, 600, 1920, 1080)
	if x != 0 || y != 0 {
		t.Fatalf("expected (0,0), got (%v, %v)", x, y)
	}
}

func TestScaleLocalToRemote_CornerMapsToCorner(t *testing.T) {
	x, y := ScaleLocalToRemote(800, 600, 800, 600, 1920, 1080)
	if x != 1920 || y != 1080 {
		t.Fatalf("expected (1920, 1080) bottom-right corner, got (%v, %v)", x, y)
	}
}
