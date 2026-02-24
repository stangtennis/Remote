package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/pion/webrtc/v3"
	"github.com/stangtennis/Remote/controller/internal/config"
	rtc "github.com/stangtennis/Remote/controller/internal/webrtc"
)

// runHeadless runs the controller without GUI — for testing WebRTC end-to-end
func runHeadless() {
	log.Println("=== Controller Headless Mode ===")
	log.Printf("Version: %s", VersionInfo)

	// Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Get credentials from env or saved creds
	email := os.Getenv("AGENT_EMAIL")
	password := os.Getenv("AGENT_PASSWORD")
	if email == "" || password == "" {
		log.Fatal("AGENT_EMAIL and AGENT_PASSWORD env vars required for headless mode")
	}

	// Authenticate
	log.Printf("Authenticating as %s...", email)
	authResp, err := supabaseSignIn(cfg.SupabaseURL, cfg.SupabaseAnonKey, email, password)
	if err != nil {
		log.Fatalf("Authentication failed: %v", err)
	}
	log.Printf("Authenticated: user_id=%s", authResp.UserID)

	// Fetch devices
	log.Println("Fetching devices...")
	devices, err := fetchDevices(cfg.SupabaseURL, cfg.SupabaseAnonKey, authResp.AccessToken, authResp.UserID)
	if err != nil {
		log.Fatalf("Failed to fetch devices: %v", err)
	}

	log.Printf("Found %d devices", len(devices))

	// Find first online device
	var targetDevice *headlessDevice
	for i, d := range devices {
		status := "offline"
		if !d.LastSeen.IsZero() && time.Since(d.LastSeen) < 2*time.Minute {
			status = "online"
		}
		log.Printf("  [%d] %s (%s) - %s (last seen: %s)", i, d.DeviceName, d.Platform, status, d.LastSeen.Format(time.RFC3339))
		if status == "online" && targetDevice == nil {
			targetDevice = &devices[i]
		}
	}

	if targetDevice == nil {
		log.Fatal("No online devices found!")
	}

	log.Printf("Connecting to: %s (ID: %s)", targetDevice.DeviceName, targetDevice.DeviceID)

	// Create WebRTC client
	client, err := rtc.NewClient()
	if err != nil {
		log.Fatalf("Failed to create WebRTC client: %v", err)
	}

	// Track connection state
	connected := make(chan bool, 1)
	framesReceived := 0

	client.SetOnFrame(func(frameData []byte) {
		framesReceived++
		if framesReceived == 1 {
			log.Printf("First video frame received! (%d bytes)", len(frameData))
		}
		if framesReceived%30 == 0 {
			log.Printf("Frames received: %d", framesReceived)
		}
	})

	client.SetOnConnected(func() {
		log.Println("WebRTC CONNECTED!")
		connected <- true
	})

	client.SetOnDisconnected(func() {
		log.Println("WebRTC DISCONNECTED")
	})

	// Get ICE servers
	iceServers := headlessFetchICEServers(cfg.SupabaseURL, cfg.SupabaseAnonKey, authResp.AccessToken)
	log.Printf("ICE servers: %d configured", len(iceServers))

	// Create peer connection
	if err := client.CreatePeerConnection(iceServers); err != nil {
		log.Fatalf("Failed to create peer connection: %v", err)
	}

	// Create signaling client
	signalingClient := rtc.NewSignalingClient(cfg.SupabaseURL, cfg.SupabaseAnonKey, authResp.AccessToken)

	// Create session
	log.Println("Creating WebRTC session...")
	session, err := signalingClient.CreateSession(targetDevice.DeviceID, authResp.UserID)
	if err != nil {
		log.Fatalf("Failed to create session: %v", err)
	}
	log.Printf("Session created: %s", session.SessionID)

	// Create and send offer
	log.Println("Creating WebRTC offer...")
	offerJSON, err := client.CreateOffer()
	if err != nil {
		log.Fatalf("Failed to create offer: %v", err)
	}
	log.Printf("Offer created (%d bytes)", len(offerJSON))

	log.Println("Sending offer...")
	if err := signalingClient.SendOffer(session.SessionID, offerJSON); err != nil {
		log.Fatalf("Failed to send offer: %v", err)
	}
	log.Println("Offer sent, waiting for answer...")

	// Wait for answer
	answerJSON, err := signalingClient.WaitForAnswer(session.SessionID, 30*time.Second)
	if err != nil {
		log.Fatalf("Failed to get answer: %v", err)
	}
	log.Printf("Answer received (%d bytes)", len(answerJSON))

	// Set answer
	if err := client.SetAnswer(answerJSON); err != nil {
		log.Fatalf("Failed to set answer: %v", err)
	}
	log.Println("Answer set, waiting for connection...")

	// Wait for connection or timeout
	select {
	case <-connected:
		log.Println("=== CONNECTION ESTABLISHED ===")
	case <-time.After(30 * time.Second):
		log.Fatal("Timeout waiting for WebRTC connection")
	}

	// Run for specified duration to collect frames
	duration := 10 * time.Second
	if d := os.Getenv("HEADLESS_DURATION"); d != "" {
		if parsed, err := time.ParseDuration(d); err == nil {
			duration = parsed
		}
	}

	log.Printf("Collecting frames for %s...", duration)
	time.Sleep(duration)

	// Report results
	log.Println("=== HEADLESS TEST RESULTS ===")
	log.Printf("Device: %s (%s)", targetDevice.DeviceName, targetDevice.Platform)
	log.Printf("Frames received: %d", framesReceived)
	log.Printf("Duration: %s", duration)
	if framesReceived > 0 {
		fps := float64(framesReceived) / duration.Seconds()
		log.Printf("Average FPS: %.1f", fps)
		log.Println("RESULT: PASS")
	} else {
		log.Println("RESULT: FAIL (no frames received)")
		os.Exit(1)
	}

	// Cleanup
	signalingClient.DeleteSession(session.SessionID)
	log.Println("Session cleaned up")
}

// Lightweight auth struct for headless mode
type headlessAuthResponse struct {
	AccessToken string
	UserID      string
}

type headlessDevice struct {
	DeviceID   string    `json:"device_id"`
	DeviceName string    `json:"device_name"`
	Platform   string    `json:"platform"`
	Status     string    `json:"status"`
	LastSeen   time.Time `json:"last_seen"`
}

func supabaseSignIn(supabaseURL, anonKey, email, password string) (*headlessAuthResponse, error) {
	payload := map[string]string{"email": email, "password": password}
	jsonData, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", supabaseURL+"/auth/v1/token?grant_type=password", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("apikey", anonKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := readBody(resp)
		return nil, fmt.Errorf("auth failed (status %d): %s", resp.StatusCode, body)
	}

	var result struct {
		AccessToken string `json:"access_token"`
		User        struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &headlessAuthResponse{
		AccessToken: result.AccessToken,
		UserID:      result.User.ID,
	}, nil
}

func fetchDevices(supabaseURL, anonKey, authToken, userID string) ([]headlessDevice, error) {
	url := fmt.Sprintf("%s/rest/v1/remote_devices?owner_id=eq.%s&select=*", supabaseURL, userID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("apikey", anonKey)
	req.Header.Set("Authorization", "Bearer "+authToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var devices []headlessDevice
	if err := json.NewDecoder(resp.Body).Decode(&devices); err != nil {
		return nil, err
	}
	return devices, nil
}

func headlessFetchICEServers(supabaseURL, anonKey, authToken string) []webrtc.ICEServer {
	// Try TURN edge function
	client := &http.Client{Timeout: 5 * time.Second}
	req, _ := http.NewRequest("POST", supabaseURL+"/functions/v1/turn-credentials", nil)
	req.Header.Set("Authorization", "Bearer "+authToken)
	req.Header.Set("apikey", anonKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err == nil && resp.StatusCode == 200 {
		defer resp.Body.Close()
		var result struct {
			ICEServers []struct {
				URLs       interface{} `json:"urls"`
				Username   string      `json:"username,omitempty"`
				Credential string      `json:"credential,omitempty"`
			} `json:"iceServers"`
		}
		if json.NewDecoder(resp.Body).Decode(&result) == nil {
			var servers []webrtc.ICEServer
			for _, s := range result.ICEServers {
				var urls []string
				switch v := s.URLs.(type) {
				case string:
					urls = []string{v}
				case []interface{}:
					for _, u := range v {
						if str, ok := u.(string); ok {
							urls = append(urls, str)
						}
					}
				}
				server := webrtc.ICEServer{URLs: urls}
				if s.Username != "" {
					server.Username = s.Username
					server.Credential = s.Credential
				}
				servers = append(servers, server)
			}
			if len(servers) > 0 {
				log.Printf("TURN credentials fetched from edge function")
				return servers
			}
		}
	}

	// Fallback: STUN only + env TURN
	servers := []webrtc.ICEServer{
		{URLs: []string{"stun:stun.l.google.com:19302"}},
	}
	if ts := os.Getenv("TURN_SERVER"); ts != "" {
		servers = append(servers, webrtc.ICEServer{
			URLs:       []string{"turn:" + ts, "turn:" + ts + "?transport=tcp"},
			Username:   os.Getenv("TURN_USERNAME"),
			Credential: os.Getenv("TURN_PASSWORD"),
		})
	}
	return servers
}

func readBody(resp *http.Response) (string, error) {
	body := make([]byte, 1024)
	n, _ := resp.Body.Read(body)
	return string(body[:n]), nil
}
