package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	serverSecret []byte
	password     string
)

func initAuth(store *Store) {
	password = os.Getenv("CLIPBOARD_PASSWORD")
	if password == "" {
		password = "clipboooard"
	}

	secret, err := store.GetSecret()
	if err != nil || secret == "" {
		b := make([]byte, 32)
		rand.Read(b)
		serverSecret = b
		store.SetSecret(base64.StdEncoding.EncodeToString(b))
	} else {
		b, err := base64.StdEncoding.DecodeString(secret)
		if err != nil {
			b = make([]byte, 32)
			rand.Read(b)
		}
		serverSecret = b
	}
}

func makeToken() string {
	expiry := time.Now().Add(90 * 24 * time.Hour).Unix()
	data := fmt.Sprintf("%d", expiry)
	mac := hmac.New(sha256.New, serverSecret)
	mac.Write([]byte(data))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("%d:%s", expiry, sig)))
}

func checkToken(r *http.Request) bool {
	tokenStr := ""
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		tokenStr = strings.TrimPrefix(auth, "Bearer ")
	}
	if tokenStr == "" {
		tokenStr = r.URL.Query().Get("token")
	}
	if tokenStr == "" {
		return false
	}
	return verifyToken(tokenStr)
}

func verifyToken(tokenStr string) bool {
	b, err := base64.RawURLEncoding.DecodeString(tokenStr)
	if err != nil {
		return false
	}
	parts := strings.SplitN(string(b), ":", 2)
	if len(parts) != 2 {
		return false
	}
	var expiry int64
	fmt.Sscanf(parts[0], "%d", &expiry)
	if time.Now().Unix() > expiry {
		return false
	}
	mac := hmac.New(sha256.New, serverSecret)
	mac.Write([]byte(parts[0]))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(parts[1]), []byte(expected))
}

func getClientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		parts := strings.SplitN(fwd, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	host := r.RemoteAddr
	if idx := strings.LastIndex(host, ":"); idx > 0 {
		return host[:idx]
	}
	return host
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	ip := getClientIP(r)

	ban, ok := h.store.CheckBan(ip)
	if ok {
		remaining := time.Until(ban.BanUntil).Round(time.Second)
		writeJSON(w, http.StatusTooManyRequests, map[string]interface{}{
			"error":     "too many attempts",
			"banUntil":  ban.BanUntil,
			"remaining": remaining.String(),
		})
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	if req.Password != password {
		count := h.store.RecordFail(ip)
		banDur := time.Duration(0)
		if count >= 3 {
			banDur = time.Duration(math.Pow(2, float64(count-3))) * time.Hour
			h.store.SetBan(ip, count, time.Now().Add(banDur))
			writeJSON(w, http.StatusTooManyRequests, map[string]interface{}{
				"error":     "wrong password",
				"banUntil":  time.Now().Add(banDur),
				"remaining": banDur.String(),
			})
			return
		}
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"error":     "wrong password",
			"remaining": 3 - count,
		})
		return
	}

	h.store.ResetBan(ip)
	token := makeToken()
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !checkToken(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}
