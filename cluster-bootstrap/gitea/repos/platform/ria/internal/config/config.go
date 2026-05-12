// Package config provides environment-based configuration for RIA.
package config

import (
	"fmt"
	"os"
)

// Config holds all configuration values read from environment variables.
type Config struct {
	// GiteaURL is the base URL for the Gitea API.
	// In-cluster this is the service DNS name; externally it may differ.
	GiteaURL  string
	GiteaUser string
	GiteaPass string

	// ListenAddr is the address the HTTP server binds to.
	ListenAddr string

	// CatalogOwner and CatalogRepo identify the service-catalog repository.
	CatalogOwner string
	CatalogRepo  string

	// DeployOwner and DeployRepo identify the gitops-deploy repository.
	DeployOwner string
	DeployRepo  string

	// WebhookTargetURL is the URL that PAC webhooks should point to.
	WebhookTargetURL string

	// DataDir is the base directory for persistent repo clones.
	DataDir string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() (*Config, error) {
	cfg := &Config{
		GiteaURL:         envOrDefault("GITEA_URL", "http://gitea-http.gitea.svc:3000"),
		GiteaUser:        os.Getenv("GITEA_USER"),
		GiteaPass:        os.Getenv("GITEA_PASSWORD"),
		ListenAddr:       envOrDefault("LISTEN_ADDR", ":8080"),
		CatalogOwner:     envOrDefault("CATALOG_OWNER", "platform"),
		CatalogRepo:      envOrDefault("CATALOG_REPO", "service-catalog"),
		DeployOwner:      envOrDefault("DEPLOY_OWNER", "platform"),
		DeployRepo:       envOrDefault("DEPLOY_REPO", "gitops-deploy"),
		WebhookTargetURL: envOrDefault("WEBHOOK_TARGET_URL", "http://pipelines-as-code-controller.pipelines-as-code.svc:8080"),
		DataDir:          envOrDefault("DATA_DIR", "/data"),
	}

	if cfg.GiteaUser == "" {
		return nil, fmt.Errorf("GITEA_USER is required")
	}
	if cfg.GiteaPass == "" {
		return nil, fmt.Errorf("GITEA_PASSWORD is required")
	}

	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
