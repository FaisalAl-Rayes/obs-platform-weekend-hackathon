// Package catalog reads and parses component definitions from a local clone
// of the service-catalog repository.
package catalog

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Component is the parsed representation of a service-catalog YAML file.
type Component struct {
	APIVersion string        `yaml:"apiVersion"`
	Kind       string        `yaml:"kind"`
	Metadata   ComponentMeta `yaml:"metadata"`
	Spec       ComponentSpec `yaml:"spec"`
}

// ComponentMeta holds metadata fields from the component manifest.
type ComponentMeta struct {
	Name   string            `yaml:"name"`
	Labels map[string]string `yaml:"labels"`
}

// PostgresConfig holds sizing parameters for a CloudNativePG Cluster.
type PostgresConfig struct {
	Size     string `yaml:"size"`
	Replicas int    `yaml:"replicas"`
}

// InfrastructurePostgres groups per-environment Postgres configurations.
type InfrastructurePostgres struct {
	Stage *PostgresConfig `yaml:"stage,omitempty"`
	Prod  *PostgresConfig `yaml:"prod,omitempty"`
}

// CustomSLO defines a user-provided SLO based on an arbitrary PromQL query.
type CustomSLO struct {
	Name   string  `yaml:"name"`
	Query  string  `yaml:"query"`
	Target float64 `yaml:"target"`
	Window string  `yaml:"window"`
}

// AvailabilitySLO defines the baseline availability SLO for a component.
type AvailabilitySLO struct {
	Target float64 `yaml:"target"`
	Window string  `yaml:"window"`
}

// SLOConfig groups all SLO definitions for a component.
type SLOConfig struct {
	Availability AvailabilitySLO `yaml:"availability"`
	Custom       []CustomSLO     `yaml:"custom,omitempty"`
}

// Infrastructure declares infrastructure dependencies for a component.
type Infrastructure struct {
	Postgres *InfrastructurePostgres `yaml:"postgres,omitempty"`
}

// ComponentSpec holds the spec fields from the component manifest.
type ComponentSpec struct {
	Description    string          `yaml:"description"`
	Kind           string          `yaml:"kind"`
	Stack          string          `yaml:"stack"`
	Owner          string          `yaml:"owner"`
	Repo           string          `yaml:"repo"`
	Documentation  string          `yaml:"documentation"`
	SourceCode     string          `yaml:"sourceCode"`
	Issues         string          `yaml:"issues"`
	CreatedAt      string          `yaml:"createdAt"`
	LastUpdatedOn  string          `yaml:"lastUpdatedOn"`
	Infrastructure *Infrastructure `yaml:"infrastructure,omitempty"`
	SLO            *SLOConfig     `yaml:"slo,omitempty"`
}

// Team returns the team label, falling back to the spec owner.
func (c *Component) Team() string {
	if t, ok := c.Metadata.Labels["team"]; ok && t != "" {
		return t
	}
	return c.Spec.Owner
}

// RepoName returns the Gitea repository name for this component.
// Falls back to the component name if repo is not set.
func (c *Component) RepoName() string {
	if c.Spec.Repo != "" {
		return c.Spec.Repo
	}
	return c.Metadata.Name
}

// Port returns the default container port for this component's stack.
func (c *Component) Port() int {
	switch c.Spec.Stack {
	case "go-service":
		return 8080
	default:
		// python-service and anything else defaults to 8000.
		return 8000
	}
}

// TaskFile returns the Tekton task filename that matches this stack.
func (c *Component) TaskFile() string {
	switch c.Spec.Stack {
	case "go-service":
		return "go-build.yaml"
	case "node-service":
		return "node-build.yaml"
	default:
		return "python-build.yaml"
	}
}

// TaskName returns the Tekton task name (without .yaml) that matches this stack.
func (c *Component) TaskName() string {
	switch c.Spec.Stack {
	case "go-service":
		return "go-build"
	case "node-service":
		return "node-build"
	default:
		return "python-build"
	}
}

// HasSLO reports whether the component declares any SLO configuration.
func (c *Component) HasSLO() bool {
	return c.Spec.SLO != nil
}

// HasPostgres reports whether the component declares any Postgres infrastructure.
func (c *Component) HasPostgres() bool {
	return c.Spec.Infrastructure != nil && c.Spec.Infrastructure.Postgres != nil
}

// PostgresStage returns the stage Postgres configuration, or nil if none is declared.
func (c *Component) PostgresStage() *PostgresConfig {
	if !c.HasPostgres() {
		return nil
	}
	return c.Spec.Infrastructure.Postgres.Stage
}

// PostgresProd returns the prod Postgres configuration, or nil if none is declared.
func (c *Component) PostgresProd() *PostgresConfig {
	if !c.HasPostgres() {
		return nil
	}
	return c.Spec.Infrastructure.Postgres.Prod
}

// FetchAll reads every YAML file in the root of catalogDir (a local clone of
// the service-catalog repo) and returns the parsed components. Files that fail
// to parse are logged and skipped so that one bad entry does not block the
// entire reconciliation.
func FetchAll(catalogDir string) ([]Component, error) {
	entries, err := os.ReadDir(catalogDir)
	if err != nil {
		return nil, fmt.Errorf("listing catalog directory %s: %w", catalogDir, err)
	}

	var components []Component
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := filepath.Ext(entry.Name())
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		path := filepath.Join(catalogDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("WARN: skipping catalog file %s: %v", entry.Name(), err)
			continue
		}

		var comp Component
		if err := yaml.Unmarshal(data, &comp); err != nil {
			log.Printf("WARN: skipping catalog file %s: invalid YAML: %v", entry.Name(), err)
			continue
		}

		if comp.Kind != "Component" {
			log.Printf("WARN: skipping catalog file %s: kind is %q, expected Component", entry.Name(), comp.Kind)
			continue
		}
		if strings.TrimSpace(comp.Metadata.Name) == "" {
			log.Printf("WARN: skipping catalog file %s: missing metadata.name", entry.Name())
			continue
		}

		components = append(components, comp)
	}

	return components, nil
}
