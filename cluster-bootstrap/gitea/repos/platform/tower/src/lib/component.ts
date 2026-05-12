import type { ComponentData, ComponentCreate, InfrastructureConfig, SLOConfig } from "./types";

export function toComponentData(data: Record<string, unknown>, sha?: string): ComponentData {
  const spec = data.spec as Record<string, unknown> || {};
  const metadata = data.metadata as Record<string, unknown> || {};
  return {
    name: (metadata.name as string) || "",
    description: (spec.description as string) || "",
    componentKind: (spec.kind as string) || "service",
    stack: (spec.stack as string) || "",
    owner: (spec.owner as string) || "",
    repo: (spec.repo as string) || "",
    documentation: (spec.documentation as string) || null,
    sourceCode: (spec.sourceCode as string) || null,
    issues: (spec.issues as string) || null,
    createdAt: (spec.createdAt as string) || "",
    lastUpdatedOn: (spec.lastUpdatedOn as string) || "",
    labels: (metadata.labels as Record<string, string>) || {},
    infrastructure: (spec.infrastructure as InfrastructureConfig) || undefined,
    slo: (spec.slo as SLOConfig) || undefined,
    sha,
  };
}

export function buildComponentYaml(create: ComponentCreate): Record<string, unknown> {
  const today = new Date().toISOString().split("T")[0];
  return {
    apiVersion: "tower.obs-platform.local/v1",
    kind: "Component",
    metadata: {
      name: create.name,
      labels: { ...create.labels, team: create.owner },
    },
    spec: {
      description: create.description,
      kind: create.componentKind,
      stack: create.stack,
      owner: create.owner,
      repo: create.repo,
      documentation: create.documentation || null,
      sourceCode: create.sourceCode || null,
      issues: create.issues || null,
      createdAt: today,
      lastUpdatedOn: today,
      ...(create.infrastructure ? { infrastructure: create.infrastructure } : {}),
      slo: create.slo,
    },
  };
}
