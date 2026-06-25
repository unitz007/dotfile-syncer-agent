package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// startWebhookServer listens for GitHub push webhooks on the given port.
// When a push to `branch` is received (and the HMAC signature is valid when
// `secret` is non-empty), onPush is called in a goroutine so the HTTP
// response is returned immediately.
func startWebhookServer(port int, secret, branch string, onPush func()) {
	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if secret != "" {
			if !validWebhookSig(secret, body, r.Header.Get("X-Hub-Signature-256")) {
				Warnln("webhook: invalid signature — request rejected")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}

		if r.Header.Get("X-GitHub-Event") != "push" {
			w.WriteHeader(http.StatusOK)
			return
		}

		var payload struct {
			Ref string `json:"ref"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if payload.Ref != "refs/heads/"+branch {
			w.WriteHeader(http.StatusOK)
			return
		}

		Infoln(fmt.Sprintf("webhook: push to %s — triggering sync ⚡", branch))
		w.WriteHeader(http.StatusOK)
		go onPush()
	})

	addr := fmt.Sprintf(":%d", port)
	Infoln(fmt.Sprintf("Webhook listener started on %s/webhook 🪝", addr))
	if err := http.ListenAndServe(addr, mux); err != nil {
		Error("webhook server error: " + err.Error())
	}
}

func validWebhookSig(secret string, body []byte, sig string) bool {
	if !strings.HasPrefix(sig, "sha256=") {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(sig), []byte(expected))
}
