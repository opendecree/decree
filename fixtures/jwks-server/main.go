// Package main is a minimal JWKS server for JWT e2e testing.
//
// It exposes:
//
//	/.well-known/jwks.json — JWKS with all active and retired keys
//	POST /admin/sign        — mint a JWT signed by the active key
//	POST /admin/rotate      — add a new signing key (old key stays in JWKS)
//	GET  /admin/health      — liveness probe
//
// All key material lives in memory. The server is not suitable for production.
package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

// --- state ---

type keyEntry struct {
	kid string
	key *rsa.PrivateKey
}

var (
	mu         sync.RWMutex
	keys       []*keyEntry // all keys; last entry is the active signing key
	kidCounter int
)

// --- main ---

func main() {
	// healthcheck subcommand: used by Docker health check.
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		resp, err := http.Get("http://localhost:" + listenPort() + "/admin/health")
		if err != nil || resp.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		os.Exit(0)
	}

	k, err := generateKey()
	if err != nil {
		log.Fatalf("generate initial key: %v", err)
	}
	keys = append(keys, k)

	issuer := os.Getenv("ISSUER")
	if issuer == "" {
		issuer = "decree-e2e"
	}
	port := listenPort()

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/jwks.json", handleJWKS)
	mux.HandleFunc("/admin/sign", makeSignHandler(issuer))
	mux.HandleFunc("/admin/rotate", handleRotate)
	mux.HandleFunc("/admin/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mu.RLock()
	activekid := keys[0].kid
	mu.RUnlock()
	log.Printf("jwks-server listening on :%s (issuer=%s, initial_kid=%s)", port, issuer, activekid)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("listen: %v", err)
	}
}

func listenPort() string {
	if p := os.Getenv("PORT"); p != "" {
		return p
	}
	return "8090"
}

// --- key generation ---

func generateKey() (*keyEntry, error) {
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	mu.Lock()
	kidCounter++
	kid := "key-" + strconv.Itoa(kidCounter)
	mu.Unlock()
	return &keyEntry{kid: kid, key: k}, nil
}

// --- handlers ---

func handleJWKS(w http.ResponseWriter, _ *http.Request) {
	type jwk struct {
		Kty string `json:"kty"`
		Use string `json:"use"`
		Alg string `json:"alg"`
		Kid string `json:"kid"`
		N   string `json:"n"`
		E   string `json:"e"`
	}
	type resp struct {
		Keys []jwk `json:"keys"`
	}

	mu.RLock()
	snapshot := make([]*keyEntry, len(keys))
	copy(snapshot, keys)
	mu.RUnlock()

	var r resp
	for _, ke := range snapshot {
		pub := ke.key.Public().(*rsa.PublicKey)
		r.Keys = append(r.Keys, jwk{
			Kty: "RSA",
			Use: "sig",
			Alg: "RS256",
			Kid: ke.kid,
			N:   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
			E:   encodeExponent(pub.E),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(r)
}

type signRequest struct {
	Role      string   `json:"role"`
	Subject   string   `json:"subject"`
	TenantIDs []string `json:"tenant_ids"`
	ExpiresIn string   `json:"expires_in"` // duration string, e.g. "5m" or "-1s"
}

type signResponse struct {
	Token string `json:"token"`
	Kid   string `json:"kid"`
}

func makeSignHandler(issuer string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req signRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
			return
		}
		if req.ExpiresIn == "" {
			req.ExpiresIn = "5m"
		}
		expiry, err := time.ParseDuration(req.ExpiresIn)
		if err != nil {
			http.Error(w, "bad expires_in: "+err.Error(), http.StatusBadRequest)
			return
		}
		if req.TenantIDs == nil {
			req.TenantIDs = []string{}
		}

		mu.RLock()
		active := keys[len(keys)-1]
		mu.RUnlock()

		now := time.Now()
		claims := map[string]any{
			"iss":        issuer,
			"sub":        req.Subject,
			"iat":        now.Unix(),
			"exp":        now.Add(expiry).Unix(),
			"role":       req.Role,
			"tenant_ids": req.TenantIDs,
		}

		token, err := signJWT(active.key, active.kid, claims)
		if err != nil {
			http.Error(w, "sign failed: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(signResponse{Token: token, Kid: active.kid})
	}
}

type rotateResponse struct {
	RetiredKID string `json:"retired_kid"`
	ActiveKID  string `json:"active_kid"`
}

func handleRotate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Generate outside lock — RSA key generation is slow.
	newKey, err := generateKey()
	if err != nil {
		http.Error(w, "generate key failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	mu.Lock()
	retiredKID := keys[len(keys)-1].kid
	keys = append(keys, newKey)
	mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rotateResponse{
		RetiredKID: retiredKID,
		ActiveKID:  newKey.kid,
	})
}

// --- JWT signing (RS256, stdlib only) ---

func signJWT(key *rsa.PrivateKey, kid string, claims map[string]any) (string, error) {
	headerJSON, err := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT", "kid": kid})
	if err != nil {
		return "", fmt.Errorf("marshal header: %w", err)
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal claims: %w", err)
	}

	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)
	sigInput := headerB64 + "." + claimsB64

	h := crypto.SHA256.New()
	h.Write([]byte(sigInput))
	digest := h.Sum(nil)

	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest)
	if err != nil {
		return "", fmt.Errorf("rsa sign: %w", err)
	}

	return sigInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func encodeExponent(e int) string {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, uint32(e))
	for len(b) > 1 && b[0] == 0 {
		b = b[1:]
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
