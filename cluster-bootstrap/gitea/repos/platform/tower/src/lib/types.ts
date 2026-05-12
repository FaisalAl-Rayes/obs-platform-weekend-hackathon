export interface PostgresConfig {
  size: string;
  replicas: number;
}

export interface InfrastructureConfig {
  postgres?: {
    stage?: PostgresConfig;
    prod?: PostgresConfig;
  };
}

export interface CustomSLO {
  name: string;
  query: string;       // PromQL expression that returns 0-1
  target: number;      // e.g. 99.9
  window: string;      // e.g. "30d"
}

export interface SLOConfig {
  availability: {
    target: number;    // e.g. 99.9 (percentage)
    window: string;    // e.g. "30d"
  };
  custom?: CustomSLO[];
}

export interface ComponentData {
  name: string;
  description: string;
  componentKind: string;
  stack: string;
  owner: string;
  repo: string;
  documentation: string | null;
  sourceCode: string | null;
  issues: string | null;
  createdAt: string;
  lastUpdatedOn: string;
  labels: Record<string, string>;
  infrastructure?: InfrastructureConfig;
  slo?: SLOConfig;
  sha?: string;
}

export interface ComponentCreate {
  name: string;
  description: string;
  componentKind: string;
  stack: string;
  owner: string;
  repo: string;
  documentation?: string;
  sourceCode?: string;
  issues?: string;
  labels?: Record<string, string>;
  infrastructure?: InfrastructureConfig;
  slo: SLOConfig;
}

export interface ComponentUpdate {
  description?: string;
  componentKind?: string;
  stack?: string;
  owner?: string;
  repo?: string;
  documentation?: string;
  sourceCode?: string;
  issues?: string;
  labels?: Record<string, string>;
  infrastructure?: InfrastructureConfig;
  slo?: SLOConfig;
}
