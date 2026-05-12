// Package reconciler implements the level-triggered reconciliation loop.
//
// It reads the desired state from the service-catalog, computes the expected
// gitops-deploy artifacts, diffs against what is currently committed, and
// writes only the files that changed. It also ensures that each component's
// source repository has a .tekton/push.yaml pipeline and a PAC webhook.
//
// Write operations use go-git (clone, modify working tree, commit-and-push)
// so that all gitops-deploy changes land in a single atomic commit.
//
// All repos (service-catalog, gitops-deploy, and component source repos) are
// persistent shallow clones (depth=1) under DataDir, pulled on subsequent
// reconciliations. Component repos whose catalog entries are removed are
// cleaned up automatically.
package reconciler

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/obs-platform/ria/internal/catalog"
	"github.com/obs-platform/ria/internal/config"
	"github.com/obs-platform/ria/internal/git"
	"github.com/obs-platform/ria/internal/gitea"
)

// Reconciler holds the dependencies for a single reconciliation pass.
type Reconciler struct {
	cfg    *config.Config
	client *gitea.Client

	// mu serialises reconciliation runs so that at most one is active.
	mu sync.Mutex

	// Persistent repo handles — initialised on first Run(), reused thereafter.
	catalogRepo    *git.Repo
	deployRepo     *git.Repo
	componentRepos map[string]*git.Repo // keyed by "owner/name"
}

// New creates a Reconciler.
func New(cfg *config.Config, client *gitea.Client) *Reconciler {
	return &Reconciler{cfg: cfg, client: client}
}

// Run performs a full reconciliation. It is safe to call from multiple
// goroutines; concurrent invocations are serialised.
func (r *Reconciler) Run() {
	r.mu.Lock()
	defer r.mu.Unlock()

	log.Println("reconciler: starting reconciliation")

	// ---- Ensure persistent clones are ready (clone or pull) ---------------
	if err := r.ensureCatalogRepo(); err != nil {
		log.Printf("reconciler: ERROR preparing service-catalog clone: %v", err)
		return
	}
	if err := r.ensureDeployRepo(); err != nil {
		log.Printf("reconciler: ERROR preparing gitops-deploy clone: %v", err)
		return
	}

	// ---- Read the desired state from the local service-catalog clone ------
	components, err := catalog.FetchAll(r.catalogRepo.Dir())
	if err != nil {
		log.Printf("reconciler: ERROR fetching catalog: %v", err)
		return
	}
	log.Printf("reconciler: found %d component(s) in catalog", len(components))

	if len(components) == 0 {
		log.Println("reconciler: nothing to reconcile")
		return
	}

	// Collect sorted component names for the appset overlay files.
	var names []string
	for _, comp := range components {
		names = append(names, comp.Metadata.Name)
	}
	sort.Strings(names)

	// ---- Apply all gitops-deploy changes atomically ----------------------
	if err := r.reconcileGitopsDeploy(components, names); err != nil {
		log.Printf("reconciler: ERROR gitops-deploy: %v", err)
	}

	// ---- Per-component source repos: .tekton/ and webhooks ----------------
	activeComponents := make(map[string]bool, len(components))
	for _, comp := range components {
		name := comp.Metadata.Name
		owner := comp.Team()
		repoName := comp.RepoName()
		activeComponents[owner+"/"+repoName] = true

		if err := r.reconcileTektonPush(comp); err != nil {
			log.Printf("reconciler: ERROR component %q tekton push: %v", name, err)
		}
		if err := r.reconcileWebhook(comp); err != nil {
			log.Printf("reconciler: ERROR component %q webhook: %v", name, err)
		}
	}

	// ---- Clean up persistent clones for removed components ---------------
	r.cleanupStaleComponentRepos(activeComponents)

	log.Println("reconciler: reconciliation complete")
}

// ---------------------------------------------------------------------------
// Persistent repo lifecycle
// ---------------------------------------------------------------------------

func (r *Reconciler) ensureCatalogRepo() error {
	cloneURL := git.CloneURL(r.cfg.GiteaURL, r.cfg.CatalogOwner, r.cfg.CatalogRepo)
	dir := filepath.Join(r.cfg.DataDir, "repos", r.cfg.CatalogRepo)

	if r.catalogRepo != nil {
		log.Printf("reconciler: pulling service-catalog at %s", dir)
		if err := r.catalogRepo.Pull(); err != nil {
			// Force-pushes (e.g., from reseed) break shallow clones.
			// Recover by deleting the local clone and re-cloning.
			log.Printf("reconciler: pull failed (%v), re-cloning service-catalog", err)
			r.catalogRepo.Close()
			r.catalogRepo = nil
			os.RemoveAll(dir)
		} else {
			return nil
		}
	}

	// First run: clone (or open+pull if dir already exists on disk).
	log.Printf("reconciler: cloning/opening service-catalog at %s", dir)
	repo, err := git.Clone(cloneURL, r.cfg.GiteaUser, r.cfg.GiteaPass, dir)
	if err != nil {
		return fmt.Errorf("cloning service-catalog: %w", err)
	}
	r.catalogRepo = repo
	return nil
}

func (r *Reconciler) ensureDeployRepo() error {
	cloneURL := git.CloneURL(r.cfg.GiteaURL, r.cfg.DeployOwner, r.cfg.DeployRepo)
	dir := filepath.Join(r.cfg.DataDir, "repos", r.cfg.DeployRepo)

	if r.deployRepo != nil {
		log.Printf("reconciler: pulling gitops-deploy at %s", dir)
		if err := r.deployRepo.Pull(); err != nil {
			log.Printf("reconciler: pull failed (%v), re-cloning gitops-deploy", err)
			r.deployRepo.Close()
			r.deployRepo = nil
			os.RemoveAll(dir)
		} else {
			return nil
		}
	}

	// First run: clone (or open+pull if dir already exists on disk).
	log.Printf("reconciler: cloning/opening gitops-deploy at %s", dir)
	repo, err := git.Clone(cloneURL, r.cfg.GiteaUser, r.cfg.GiteaPass, dir)
	if err != nil {
		return fmt.Errorf("cloning gitops-deploy: %w", err)
	}
	r.deployRepo = repo
	return nil
}

func (r *Reconciler) ensureComponentRepo(owner, name string) (*git.Repo, error) {
	key := owner + "/" + name
	cloneURL := git.CloneURL(r.cfg.GiteaURL, owner, name)
	dir := filepath.Join(r.cfg.DataDir, "repos", "components", owner, name)

	if r.componentRepos == nil {
		r.componentRepos = make(map[string]*git.Repo)
	}

	if repo, ok := r.componentRepos[key]; ok {
		log.Printf("reconciler: pulling component %s at %s", key, dir)
		if err := repo.Pull(); err != nil {
			log.Printf("reconciler: pull failed (%v), re-cloning component %s", err, key)
			repo.Close()
			delete(r.componentRepos, key)
			os.RemoveAll(dir)
		} else {
			return repo, nil
		}
	}

	// First encounter: clone (or open+pull if dir already exists on disk).
	log.Printf("reconciler: cloning/opening component %s at %s", key, dir)
	repo, err := git.Clone(cloneURL, r.cfg.GiteaUser, r.cfg.GiteaPass, dir)
	if err != nil {
		return nil, err
	}
	r.componentRepos[key] = repo
	return repo, nil
}

// cleanupStaleComponentRepos removes persistent clones for components that are
// no longer present in the service-catalog.
func (r *Reconciler) cleanupStaleComponentRepos(active map[string]bool) {
	// Clean up in-memory handles.
	for key, repo := range r.componentRepos {
		if !active[key] {
			log.Printf("reconciler: removing stale component repo handle for %s", key)
			repo.Close()
			delete(r.componentRepos, key)
		}
	}

	// Clean up on-disk directories under $DATA_DIR/repos/components/<owner>/<name>.
	componentsDir := filepath.Join(r.cfg.DataDir, "repos", "components")
	owners, err := os.ReadDir(componentsDir)
	if err != nil {
		// Directory may not exist yet on first run; that is fine.
		return
	}
	for _, ownerEntry := range owners {
		if !ownerEntry.IsDir() {
			continue
		}
		ownerPath := filepath.Join(componentsDir, ownerEntry.Name())
		repos, err := os.ReadDir(ownerPath)
		if err != nil {
			continue
		}
		for _, repoEntry := range repos {
			if !repoEntry.IsDir() {
				continue
			}
			key := ownerEntry.Name() + "/" + repoEntry.Name()
			if !active[key] {
				dirPath := filepath.Join(ownerPath, repoEntry.Name())
				log.Printf("reconciler: removing stale component clone at %s", dirPath)
				if err := os.RemoveAll(dirPath); err != nil {
					log.Printf("reconciler: WARNING: failed to remove %s: %v", dirPath, err)
				}
			}
		}
		// Remove the owner directory if it is now empty.
		remaining, _ := os.ReadDir(ownerPath)
		if len(remaining) == 0 {
			os.Remove(ownerPath)
		}
	}
}

// ---------------------------------------------------------------------------
// gitops-deploy: write all files, single atomic commit
// ---------------------------------------------------------------------------

func (r *Reconciler) reconcileGitopsDeploy(components []catalog.Component, names []string) error {
	repo := r.deployRepo

	// --- Per-component artifacts -------------------------------------------
	for _, comp := range components {
		name := comp.Metadata.Name
		log.Printf("reconciler: generating gitops-deploy artifacts for %q", name)

		data := ComponentData{
			Name:        name,
			Team:        comp.Team(),
			Port:        comp.Port(),
			Stack:       comp.Spec.Stack,
			Owner:       comp.Team(),
			Repo:        comp.RepoName(),
			HasPostgres: comp.HasPostgres(),
			HasSLO:      comp.HasSLO(),
		}

		if err := r.writeComponentBase(repo, data, comp); err != nil {
			return fmt.Errorf("component %q base: %w", name, err)
		}
		if err := r.writeComponentOverlays(repo, data, comp); err != nil {
			return fmt.Errorf("component %q overlays: %w", name, err)
		}
		if err := r.writeComponentCI(repo, data); err != nil {
			return fmt.Errorf("component %q ci overlay: %w", name, err)
		}
		if err := r.writeAppSet(repo, data); err != nil {
			return fmt.Errorf("component %q appset: %w", name, err)
		}
	}

	// --- Global appset overlays -------------------------------------------
	if err := r.writeAppsetOverlays(repo, names); err != nil {
		return fmt.Errorf("appset overlays: %w", err)
	}

	// --- Commit and push if anything changed -------------------------------
	changed, err := repo.HasChanges()
	if err != nil {
		return fmt.Errorf("checking for changes: %w", err)
	}
	if !changed {
		log.Println("reconciler: gitops-deploy is up to date, nothing to commit")
		return nil
	}

	log.Println("reconciler: committing and pushing gitops-deploy changes")
	if err := repo.CommitAndPush("ria: reconcile components from service-catalog"); err != nil {
		return fmt.Errorf("commit and push: %w", err)
	}
	log.Println("reconciler: gitops-deploy push complete")
	return nil
}

// ---------------------------------------------------------------------------
// Per-component: gitops-deploy/components/<name>/base
// ---------------------------------------------------------------------------

func (r *Reconciler) writeComponentBase(repo *git.Repo, d ComponentData, comp catalog.Component) error {
	prefix := fmt.Sprintf("components/%s/base", d.Name)
	files := map[string]func() (string, error){
		"namespace.yaml":      func() (string, error) { return render(tmplNamespace, d) },
		"deployment.yaml":     func() (string, error) { return render(tmplDeployment, d) },
		"service.yaml":        func() (string, error) { return render(tmplService, d) },
		"servicemonitor.yaml": func() (string, error) { return render(tmplServiceMonitor, d) },
		"kustomization.yaml":  func() (string, error) { return render(tmplBaseKustomization, d) },
	}

	return writeFilesToRepo(repo, prefix, files)
}

// ---------------------------------------------------------------------------
// Per-component: gitops-deploy/components/<name>/overlays/{stage,prod}
// ---------------------------------------------------------------------------

func (r *Reconciler) writeComponentOverlays(repo *git.Repo, d ComponentData, comp catalog.Component) error {
	stagePrefix := fmt.Sprintf("components/%s/overlays/stage", d.Name)
	stageFiles := map[string]func() (string, error){
		"kustomization.yaml": func() (string, error) { return render(tmplOverlayStageKustomization, d) },
	}
	if stageCfg := comp.PostgresStage(); stageCfg != nil {
		pgData := PostgresData{Name: d.Name, Size: stageCfg.Size, Replicas: stageCfg.Replicas}
		stageFiles["postgres.yaml"] = func() (string, error) { return render(tmplPostgresCluster, pgData) }
	}
	if err := writeFilesToRepo(repo, stagePrefix, stageFiles); err != nil {
		return err
	}

	prodPrefix := fmt.Sprintf("components/%s/overlays/prod", d.Name)
	prodFiles := map[string]func() (string, error){
		"kustomization.yaml": func() (string, error) { return render(tmplOverlayProdKustomization, d) },
	}
	if prodCfg := comp.PostgresProd(); prodCfg != nil {
		pgData := PostgresData{Name: d.Name, Size: prodCfg.Size, Replicas: prodCfg.Replicas}
		prodFiles["postgres.yaml"] = func() (string, error) { return render(tmplPostgresCluster, pgData) }
	}
	if d.HasSLO {
		slo := comp.Spec.SLO
		ruleData := PrometheusRuleData{
			Name:          d.Name,
			NameSafe:      strings.ReplaceAll(d.Name, "-", "_"),
			Target:        fmt.Sprintf("%.4f", slo.Availability.Target/100.0),
			TargetPercent: fmt.Sprintf("%.1f", slo.Availability.Target),
			Window:        slo.Availability.Window,
		}
		for _, cs := range slo.Custom {
			ruleData.CustomSLOs = append(ruleData.CustomSLOs, CustomSLOData{
				Name:          cs.Name,
				Query:         cs.Query,
				Target:        fmt.Sprintf("%.4f", cs.Target/100.0),
				TargetPercent: fmt.Sprintf("%.1f", cs.Target),
				Window:        cs.Window,
			})
		}
		prodFiles["prometheusrule.yaml"] = func() (string, error) { return render(tmplPrometheusRule, ruleData) }
	}
	return writeFilesToRepo(repo, prodPrefix, prodFiles)
}

// ---------------------------------------------------------------------------
// Per-component: gitops-deploy/components/<name>/overlays/ci
// ---------------------------------------------------------------------------

func (r *Reconciler) writeComponentCI(repo *git.Repo, d ComponentData) error {
	prefix := fmt.Sprintf("components/%s/overlays/ci", d.Name)
	files := map[string]func() (string, error){
		"namespace.yaml":      func() (string, error) { return render(tmplCINamespace, d) },
		"pac-repository.yaml": func() (string, error) { return render(tmplPACRepository, d) },
		"kustomization.yaml":  func() (string, error) { return render(tmplCIKustomization, d) },
	}
	return writeFilesToRepo(repo, prefix, files)
}

// ---------------------------------------------------------------------------
// Per-component: gitops-deploy/appsets/base/<name>
// ---------------------------------------------------------------------------

func (r *Reconciler) writeAppSet(repo *git.Repo, d ComponentData) error {
	prefix := fmt.Sprintf("appsets/base/%s", d.Name)
	files := map[string]func() (string, error){
		d.Name + "-appset.yaml": func() (string, error) { return render(tmplAppSet, d) },
		"kustomization.yaml":    func() (string, error) { return render(tmplAppSetKustomization, d) },
	}
	return writeFilesToRepo(repo, prefix, files)
}

// ---------------------------------------------------------------------------
// Global: gitops-deploy/appsets/overlays/{stage,prod,ci}/kustomization.yaml
// ---------------------------------------------------------------------------

func (r *Reconciler) writeAppsetOverlays(repo *git.Repo, componentNames []string) error {
	overlayData := AppsetOverlayData{Components: componentNames}

	type overlayDef struct {
		path string
		tmpl func() (string, error)
	}

	overlays := []overlayDef{
		{
			path: "appsets/overlays/stage",
			tmpl: func() (string, error) { return render(tmplAppsetsOverlayStage, overlayData) },
		},
		{
			path: "appsets/overlays/prod",
			tmpl: func() (string, error) { return render(tmplAppsetsOverlayProd, overlayData) },
		},
		{
			path: "appsets/overlays/ci",
			tmpl: func() (string, error) { return render(tmplAppsetsOverlayCI, overlayData) },
		},
	}

	for _, o := range overlays {
		files := map[string]func() (string, error){
			"kustomization.yaml": o.tmpl,
		}
		if err := writeFilesToRepo(repo, o.path, files); err != nil {
			return fmt.Errorf("overlay %s: %w", o.path, err)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Per-component: .tekton/push.yaml in the component's source repo
// ---------------------------------------------------------------------------

func (r *Reconciler) reconcileTektonPush(comp catalog.Component) error {
	name := comp.Metadata.Name
	owner := comp.Team()
	repoName := comp.RepoName()

	data := TektonPushData{
		Name:     name,
		TaskFile: comp.TaskFile(),
		TaskName: comp.TaskName(),
	}

	content, err := render(tmplTektonPush, data)
	if err != nil {
		return fmt.Errorf("rendering tekton push: %w", err)
	}

	repo, err := r.ensureComponentRepo(owner, repoName)
	if err != nil {
		if strings.Contains(err.Error(), "authentication") || strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404") {
			log.Printf("reconciler: repo %s/%s not accessible, skipping tekton push", owner, repoName)
			return nil
		}
		return fmt.Errorf("preparing %s/%s: %w", owner, repoName, err)
	}

	path := ".tekton/push.yaml"

	// Read existing content to avoid unnecessary commits.
	existing, err := repo.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading existing tekton push: %w", err)
	}

	if strings.TrimRight(existing, "\n") == strings.TrimRight(content, "\n") {
		log.Printf("reconciler: %s/%s %s is up to date", owner, name, path)
		return nil
	}

	if err := repo.WriteFile(path, content); err != nil {
		return fmt.Errorf("writing tekton push: %w", err)
	}

	changed, err := repo.HasChanges()
	if err != nil {
		return fmt.Errorf("checking for changes: %w", err)
	}
	if !changed {
		return nil
	}

	log.Printf("reconciler: committing tekton push for %s/%s", owner, name)
	commitMsg := fmt.Sprintf("ria: reconcile %s tekton push pipeline", name)
	if err := repo.CommitAndPush(commitMsg); err != nil {
		return fmt.Errorf("commit and push tekton push: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Per-component: Gitea webhook -> PAC
// ---------------------------------------------------------------------------

func (r *Reconciler) reconcileWebhook(comp catalog.Component) error {
	owner := comp.Team()
	repoName := comp.RepoName()

	hooks, err := r.client.ListWebhooks(owner, repoName)
	if err != nil {
		if strings.Contains(err.Error(), "status 404") {
			log.Printf("reconciler: repo %s/%s not found, skipping webhook", owner, repoName)
			return nil
		}
		return fmt.Errorf("listing webhooks: %w", err)
	}

	targetURL := r.cfg.WebhookTargetURL
	for _, h := range hooks {
		if h.Config.URL == targetURL {
			return nil // webhook already exists
		}
	}

	opt := gitea.CreateHookOption{
		Type: "gitea",
		Config: map[string]string{
			"url":          targetURL,
			"content_type": "json",
		},
		Events: []string{"push", "pull_request"},
		Active: true,
	}
	return r.client.CreateWebhook(owner, repoName, opt)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// writeFilesToRepo renders each template and writes the result to the cloned
// repo's working tree. Files are only written if their content differs from
// what is already on disk, keeping the git diff minimal.
func writeFilesToRepo(repo *git.Repo, prefix string, files map[string]func() (string, error)) error {
	for filename, tmplFunc := range files {
		content, err := tmplFunc()
		if err != nil {
			return fmt.Errorf("rendering %s/%s: %w", prefix, filename, err)
		}

		path := prefix + "/" + filename

		// Read the existing file to avoid unnecessary writes.
		existing, err := repo.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading existing %s: %w", path, err)
		}

		if strings.TrimRight(existing, "\n") == strings.TrimRight(content, "\n") {
			continue // already up to date
		}

		if err := repo.WriteFile(path, content); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}
	}
	return nil
}
