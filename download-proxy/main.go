package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// JWT validation for Supabase tokens
// Supabase uses HS256 (HMAC-SHA256) with the JWT secret

var (
	downloadsDir string
	jwtSecret    []byte
	allowedFiles = map[string]bool{
		"remote-agent.exe":         true,
		"remote-agent-console.exe": true,
		"controller.exe":           true,
		"input-helper.exe":         true,
	}
)

// JWTHeader represents the JWT header
type JWTHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

// JWTClaims represents the JWT payload
type JWTClaims struct {
	Sub  string `json:"sub"`
	Role string `json:"role"`
	Exp  int64  `json:"exp"`
	Iat  int64  `json:"iat"`
}

func main() {
	// Configuration from environment
	downloadsDir = getEnv("DOWNLOADS_DIR", "/home/dennis/caddy/downloads")
	jwtSecretStr := getEnv("SUPABASE_JWT_SECRET", "")
	listenAddr := getEnv("LISTEN_ADDR", ":8099")

	if jwtSecretStr == "" {
		log.Fatal("❌ SUPABASE_JWT_SECRET is required")
	}
	jwtSecret = []byte(jwtSecretStr)

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleDownload)
	mux.HandleFunc("/health", handleHealth)

	log.Printf("🔒 Download proxy starting on %s", listenAddr)
	log.Printf("📁 Serving files from: %s", downloadsDir)
	log.Printf("📋 Allowed files: %v", allowedFilesList())

	if err := http.ListenAndServe(listenAddr, mux); err != nil {
		log.Fatalf("❌ Server failed: %v", err)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func handleDownload(w http.ResponseWriter, r *http.Request) {
	// CORS preflight
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract filename from path
	filename := strings.TrimPrefix(r.URL.Path, "/")
	if filename == "" || !allowedFiles[filename] {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	// Extract JWT from Authorization header or query param
	token := ""
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		token = strings.TrimPrefix(authHeader, "Bearer ")
	}
	if token == "" {
		token = r.URL.Query().Get("token")
	}

	if token == "" {
		http.Error(w, `{"error":"Missing authorization token. Use Authorization header or ?token= query param"}`, http.StatusUnauthorized)
		return
	}

	// Validate JWT
	claims, err := validateJWT(token)
	if err != nil {
		log.Printf("⚠️ Invalid token for %s: %v", filename, err)
		http.Error(w, fmt.Sprintf(`{"error":"Invalid token: %s"}`, err.Error()), http.StatusUnauthorized)
		return
	}

	log.Printf("✅ Authorized download: %s by user %s", filename, claims.Sub)

	// Serve the file
	filePath := filepath.Join(downloadsDir, filename)
	f, err := os.Open(filePath)
	if err != nil {
		log.Printf("❌ File not found: %s", filePath)
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	// Set download headers
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", stat.Size()))

	io.Copy(w, f)
}

// validateJWT validates a Supabase JWT token using HS256
func validateJWT(tokenStr string) (*JWTClaims, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid token format")
	}

	// Decode header
	headerBytes, err := base64URLDecode(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid header encoding")
	}

	var header JWTHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, fmt.Errorf("invalid header JSON")
	}

	if header.Alg != "HS256" {
		return nil, fmt.Errorf("unsupported algorithm: %s", header.Alg)
	}

	// Verify signature using HMAC-SHA256
	signingInput := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, jwtSecret)
	mac.Write([]byte(signingInput))
	expectedSig := mac.Sum(nil)

	actualSig, err := base64URLDecode(parts[2])
	if err != nil {
		return nil, fmt.Errorf("invalid signature encoding")
	}

	if !hmac.Equal(expectedSig, actualSig) {
		return nil, fmt.Errorf("signature verification failed")
	}

	// Decode claims
	claimsBytes, err := base64URLDecode(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid claims encoding")
	}

	var claims JWTClaims
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		return nil, fmt.Errorf("invalid claims JSON")
	}

	// Check expiration
	if claims.Exp > 0 && time.Now().Unix() > claims.Exp {
		return nil, fmt.Errorf("token expired")
	}

	return &claims, nil
}

// base64URLDecode decodes base64url-encoded string (JWT standard)
func base64URLDecode(s string) ([]byte, error) {
	// Add padding if needed
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}
	return base64.URLEncoding.DecodeString(s)
}

func allowedFilesList() []string {
	files := make([]string, 0, len(allowedFiles))
	for f := range allowedFiles {
		files = append(files, f)
	}
	return files
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
