package main

import (
	"fmt"
	"image/png"
	"os"

	"github.com/kbinani/screenshot"
)

func main() {
	n := screenshot.NumActiveDisplays()
	fmt.Printf("📺 Active displays: %d\n", n)
	
	if n == 0 {
		fmt.Println("❌ No displays found!")
		return
	}
	
	bounds := screenshot.GetDisplayBounds(0)
	fmt.Printf("📐 Display 0 bounds: %v\n", bounds)
	
	fmt.Println("📸 Attempting capture...")
	img, err := screenshot.CaptureRect(bounds)
	if err != nil {
		fmt.Printf("❌ Capture failed: %v\n", err)
		return
	}
	
	fmt.Println("✅ Capture successful!")
	fmt.Printf("   Size: %dx%d\n", img.Bounds().Dx(), img.Bounds().Dy())
	
	// Save to file
	file, err := os.Create("test_screenshot.png")
	if err != nil {
		fmt.Printf("❌ Failed to create file: %v\n", err)
		return
	}
	defer file.Close()
	
	png.Encode(file, img)
	fmt.Println("💾 Saved to test_screenshot.png")
}
