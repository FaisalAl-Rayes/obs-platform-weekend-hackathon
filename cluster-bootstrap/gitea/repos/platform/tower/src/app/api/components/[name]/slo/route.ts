import { NextResponse } from "next/server";
import { readFile } from "@/lib/gitea";
import { toComponentData } from "@/lib/component";
import type { SLOConfig } from "@/lib/types";

const PROMETHEUS_URL =
  process.env.PROMETHEUS_URL ||
  "http://monitoring-kube-prometheus-prometheus.monitoring.svc:9090";

function parseWindowToMinutes(window: string): number {
  const match = window.match(/^(\d+)([dhm])$/);
  if (!match) return 30 * 24 * 60; // default 30 days
  const value = parseInt(match[1]);
  switch (match[2]) {
    case "d":
      return value * 24 * 60;
    case "h":
      return value * 60;
    case "m":
      return value;
    default:
      return 30 * 24 * 60;
  }
}

async function queryPrometheus(query: string): Promise<number | null> {
  try {
    const url = `${PROMETHEUS_URL}/api/v1/query?query=${encodeURIComponent(query)}`;
    const res = await fetch(url, { cache: "no-store" });
    if (!res.ok) return null;
    const json = await res.json();
    if (
      json.status === "success" &&
      json.data?.result?.length > 0 &&
      json.data.result[0].value?.length === 2
    ) {
      return parseFloat(json.data.result[0].value[1]);
    }
    return null;
  } catch {
    return null;
  }
}

export async function GET(
  _request: Request,
  { params }: { params: Promise<{ name: string }> }
) {
  const { name } = await params;
  const filename = `${name}.yaml`;
  const result = await readFile(filename);
  if (!result) {
    return NextResponse.json(
      { detail: `Component '${name}' not found` },
      { status: 404 }
    );
  }

  const component = toComponentData(result.data, result.sha);
  const sloConfig: SLOConfig | undefined = component.slo;
  if (!sloConfig) {
    return NextResponse.json(
      { detail: "No SLO configuration found for this component" },
      { status: 404 }
    );
  }

  // Query availability from Prometheus
  const availWindow = sloConfig.availability.window;
  // Query prod namespace only — SLO alerts are for production
  const availQuery = `avg_over_time(up{job="${name}", namespace="${name}-prod"}[${availWindow}])`;
  const rawAvailability = await queryPrometheus(availQuery);
  // Prometheus up metric returns 0-1, convert to percentage
  const currentAvailability =
    rawAvailability !== null ? rawAvailability * 100 : null;

  const windowMinutes = parseWindowToMinutes(availWindow);
  const target = sloConfig.availability.target;

  let errorBudgetTotal: number | null = null;
  let errorBudgetRemaining: number | null = null;
  let errorBudgetRemainingPercent: number | null = null;
  let availStatus: "met" | "breached" | "unknown" = "unknown";

  if (currentAvailability !== null) {
    errorBudgetTotal = windowMinutes * (1 - target / 100);
    const used = windowMinutes * (1 - currentAvailability / 100);
    errorBudgetRemaining = errorBudgetTotal - used;
    errorBudgetRemainingPercent =
      errorBudgetTotal > 0
        ? Math.round((errorBudgetRemaining / errorBudgetTotal) * 1000) / 10
        : 100;
    availStatus = currentAvailability >= target ? "met" : "breached";
  }

  // Query custom SLOs
  const customResults: {
    name: string;
    target: number;
    window: string;
    current: number | null;
    status: "met" | "breached" | "unknown";
  }[] = [];

  if (sloConfig.custom) {
    for (const custom of sloConfig.custom) {
      const rawValue = await queryPrometheus(custom.query);
      const currentValue = rawValue !== null ? rawValue * 100 : null;
      customResults.push({
        name: custom.name,
        target: custom.target,
        window: custom.window,
        current: currentValue,
        status:
          currentValue !== null
            ? currentValue >= custom.target
              ? "met"
              : "breached"
            : "unknown",
      });
    }
  }

  return NextResponse.json({
    availability: {
      target,
      window: availWindow,
      current: currentAvailability,
      errorBudgetTotal,
      errorBudgetRemaining,
      errorBudgetRemainingPercent,
      status: availStatus,
    },
    custom: customResults,
  });
}
