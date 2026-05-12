// Command ria runs the RIA reconciler service.
//
// RIA is a level-triggered reconciler that reads component definitions from
// the platform/service-catalog Gitea repository and generates infrastructure
// artifacts (Kubernetes manifests, Kustomize overlays, Argo CD ApplicationSets,
// Tekton pipelines, and Gitea webhooks) in the platform/gitops-deploy
// repository.
//
// It exposes a small HTTP server:
//
//	POST /webhook  — receives Gitea push events and triggers reconciliation
//	GET  /health   — liveness/readiness probe
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/obs-platform/ria/internal/config"
	"github.com/obs-platform/ria/internal/gitea"
	"github.com/obs-platform/ria/internal/reconciler"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Ensure the persistent clone directory exists.
	reposDir := filepath.Join(cfg.DataDir, "repos")
	if err := os.MkdirAll(reposDir, 0o755); err != nil {
		log.Fatalf("creating data dir %s: %v", reposDir, err)
	}

	client := gitea.NewClient(cfg.GiteaURL, cfg.GiteaUser, cfg.GiteaPass)
	rec := reconciler.New(cfg, client)

	// reconcileCh is a buffered channel that collapses multiple incoming
	// webhook events into a single reconciliation run. The buffer size of 1
	// means: "there is at least one pending event". Any further sends while
	// a reconciliation is already queued are dropped — that is fine because
	// the reconciler is level-triggered and will converge the full state on
	// every run.
	reconcileCh := make(chan struct{}, 1)

	// Background goroutine that drains the channel and runs reconciliation.
	go func() {
		for range reconcileCh {
			rec.Run()
		}
	}()

	// Trigger an initial reconciliation on startup so we converge
	// immediately rather than waiting for the first webhook.
	reconcileCh <- struct{}{}

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// We do not inspect the webhook payload — any push to the
		// service-catalog triggers a full reconciliation. This keeps the
		// handler simple and the reconciler idempotent.
		select {
		case reconcileCh <- struct{}{}:
			log.Println("webhook: reconciliation queued")
		default:
			log.Println("webhook: reconciliation already queued, skipping")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
	})

	log.Printf("ria: listening on %s", cfg.ListenAddr)
	if err := http.ListenAndServe(cfg.ListenAddr, mux); err != nil {
		log.Fatalf("http: %v", err)
	}
}
