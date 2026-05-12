# Team Alpha Pipelines

Custom Tekton Tasks and Pipelines for team-alpha components.

## When to use this

By default, components use the **global pipeline tasks** from `platform/pipelines/tasks/` (e.g., `python-build`, `go-build`). Use this repo when your component needs a custom build step that doesn't fit the standard tasks.

## How to create a custom task

1. Create a Tekton Task YAML in this repo, e.g., `tasks/custom-build.yaml`:

```yaml
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: custom-build
spec:
  params:
    - name: repo_url
      type: string
    - name: revision
      type: string
      default: main
    - name: image
      type: string
    - name: image_latest
      type: string
      default: ""
  workspaces:
    - name: source
  steps:
    - name: clone
      image: alpine/git:2.43.0
      script: |
        git clone $(params.repo_url) $(workspaces.source.path)/repo
        cd $(workspaces.source.path)/repo
        git checkout $(params.revision)

    - name: custom-step
      image: your-custom-image:latest
      workingDir: $(workspaces.source.path)/repo
      script: |
        # Your custom build/test logic here

    - name: build-and-push
      image: gcr.io/kaniko-project/executor:latest
      args:
        - --dockerfile=$(workspaces.source.path)/repo/Dockerfile
        - --context=$(workspaces.source.path)/repo
        - --destination=$(params.image)
        - --destination=$(params.image_latest)
        - --insecure
        - --skip-tls-verify
```

2. Commit and push to this repo.

## How to reference it from your component

In your component's `.tekton/push.yaml`, change the task annotation to point here instead of `platform/pipelines`:

```yaml
annotations:
  pipelinesascode.tekton.dev/task: "http://gitea-http.gitea.svc:3000/team-alpha/pipelines/raw/branch/main/tasks/custom-build.yaml"
```

And update the `taskRef` name to match:

```yaml
taskRef:
  name: custom-build
```

## Notes

- Task names must be unique across all tasks referenced in a single PipelineRun.
- The task URL must use the in-cluster Gitea address (`gitea-http.gitea.svc:3000`), not `localhost:3000`.
- If RIA regenerates the `.tekton/push.yaml` (e.g., after a component update in Tower), it will overwrite your custom reference with the global task. To prevent this, coordinate with the platform team.
