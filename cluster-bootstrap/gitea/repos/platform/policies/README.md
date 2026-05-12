# Platform Policies

This repo defines Vault policies and access control for secrets management across the platform.

## How secrets work

Each component has its own path in Vault under `secret/data/<component-name>/`. Secrets stored there are automatically synced into the component's Kubernetes namespace via External Secrets Operator (ESO).

### Current setup

Platform services (Tower, RIA) already use this pattern:

- **Tower**: credentials stored at `secret/data/platform/tower` in Vault, synced via an `ExternalSecret` CR into the `tower-stage`/`tower-prod` namespaces as a K8s Secret `tower-gitea-creds`.
- **RIA**: same pattern at `secret/data/platform/ria`.
- **PAC token**: stored at `secret/data/platform/pac`, synced via a `ClusterExternalSecret` to all namespaces labeled `obs-platform/pac-enabled: "true"`.

### How a product component would get secrets

When a component is registered in Tower, it can declare secrets it needs. The flow:

1. **Tower** stores the secret metadata in the component spec in `platform/service-catalog`:

```yaml
spec:
  secrets:
    - name: api-key
      vaultKey: api_key
    - name: stripe-token
      vaultKey: stripe_token
```

2. **A team member or admin** stores the actual secret values in Vault at `secret/data/<component-name>/`:

```bash
vault kv put secret/<component-name> api_key="sk_live_..." stripe_token="tok_..."
```

3. **RIA** sees the secrets metadata and generates:
   - An `ExternalSecret` CR in `gitops-deploy/components/<name>/base/` that pulls from Vault
   - `secretKeyRef` entries in the deployment env vars

The generated ExternalSecret would look like:

```yaml
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: <component-name>-secrets
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: vault
    kind: ClusterSecretStore
  target:
    name: <component-name>-secrets
  data:
    - secretKey: api-key
      remoteRef:
        key: <component-name>
        property: api_key
    - secretKey: stripe-token
      remoteRef:
        key: <component-name>
        property: stripe_token
```

And the deployment would have:

```yaml
env:
  - name: API_KEY
    valueFrom:
      secretKeyRef:
        name: <component-name>-secrets
        key: api-key
  - name: STRIPE_TOKEN
    valueFrom:
      secretKeyRef:
        name: <component-name>-secrets
        key: stripe-token
```

## Vault policies

This repo is where Vault policies should be defined — controlling which service accounts and users can access which secret paths.

### Policy structure

Each team gets a policy file that grants access to their components' secrets:

```
policies/
  team-alpha.hcl    # access to secret/data/team-alpha/*
  team-beta.hcl     # access to secret/data/team-beta/*
  platform.hcl      # access to secret/data/platform/*
```

### Example policy (team-alpha.hcl)

```hcl
# Allow read/write to all secrets under team-alpha's components
path "secret/data/team-alpha/*" {
  capabilities = ["create", "read", "update", "delete", "list"]
}

path "secret/metadata/team-alpha/*" {
  capabilities = ["list", "read"]
}
```

### How policies get applied

In production, RIA would:

1. Read the component's owner team from the catalog
2. Generate or update the team's Vault policy in this repo
3. Apply the policy to Vault (via the Vault API or a separate controller)

Currently, the Vault `setup.sh` creates basic team policies during bootstrap. This repo is the intended source of truth for managing them declaratively as the platform grows.

## Current limitations

- Secrets metadata in the component spec and the corresponding ExternalSecret/env generation by RIA are **not yet implemented** — this is a planned feature.
- Vault policies are created by `setup.sh` during bootstrap, not managed from this repo yet.
- In production, a Vault policy controller would watch this repo and reconcile policies automatically.
