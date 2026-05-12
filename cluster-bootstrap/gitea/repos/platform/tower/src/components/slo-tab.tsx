"use client";

import { useEffect, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";

interface AvailabilityResult {
  target: number;
  window: string;
  current: number | null;
  errorBudgetTotal: number | null;
  errorBudgetRemaining: number | null;
  errorBudgetRemainingPercent: number | null;
  status: "met" | "breached" | "unknown";
}

interface CustomSLOResult {
  name: string;
  target: number;
  window: string;
  current: number | null;
  status: "met" | "breached" | "unknown";
}

interface SLOResponse {
  availability: AvailabilityResult;
  custom: CustomSLOResult[];
}

function StatusBadge({ status }: { status: "met" | "breached" | "unknown" }) {
  if (status === "met") {
    return <Badge className="bg-green-600 text-white hover:bg-green-700">Met</Badge>;
  }
  if (status === "breached") {
    return <Badge className="bg-red-600 text-white hover:bg-red-700">Breached</Badge>;
  }
  return <Badge variant="secondary">Unknown</Badge>;
}

export default function SloTab({ name }: { name: string }) {
  const [data, setData] = useState<SLOResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetch(`/api/components/${name}/slo`)
      .then((res) => {
        if (!res.ok) throw new Error("Failed to load SLO data");
        return res.json();
      })
      .then((json) => setData(json))
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [name]);

  if (loading) {
    return <p className="text-sm text-muted-foreground">Loading SLO data...</p>;
  }

  if (error) {
    return <p className="text-sm text-destructive">{error}</p>;
  }

  if (!data) {
    return <p className="text-sm text-muted-foreground">No SLO configuration found.</p>;
  }

  const avail = data.availability;

  return (
    <div className="flex flex-col gap-6">
      <div>
        <h2 className="text-lg font-medium mb-4">Availability SLO</h2>
        <Card>
          <CardContent className="p-0">
            <div className="flex items-center justify-between px-4 py-3">
              <span className="text-sm text-muted-foreground">Target</span>
              <span className="text-sm">{avail.target}%</span>
            </div>
            <Separator />
            <div className="flex items-center justify-between px-4 py-3">
              <span className="text-sm text-muted-foreground">Window</span>
              <span className="text-sm">{avail.window}</span>
            </div>
            <Separator />
            <div className="flex items-center justify-between px-4 py-3">
              <span className="text-sm text-muted-foreground">Current Availability</span>
              <span className="text-sm">
                {avail.current !== null ? `${avail.current.toFixed(2)}%` : "N/A"}
              </span>
            </div>
            <Separator />
            <div className="flex items-center justify-between px-4 py-3">
              <span className="text-sm text-muted-foreground">Error Budget Remaining</span>
              <span className="text-sm">
                {avail.errorBudgetRemaining !== null
                  ? `${avail.errorBudgetRemaining.toFixed(1)} min (${avail.errorBudgetRemainingPercent}%)`
                  : "N/A"}
              </span>
            </div>
            <Separator />
            <div className="flex items-center justify-between px-4 py-3">
              <span className="text-sm text-muted-foreground">Status</span>
              <StatusBadge status={avail.status} />
            </div>
          </CardContent>
        </Card>
      </div>

      {data.custom.length > 0 && (
        <div>
          <h2 className="text-lg font-medium mb-4">Custom SLOs</h2>
          <div className="flex flex-col gap-4">
            {data.custom.map((slo) => (
              <Card key={slo.name}>
                <CardContent className="p-0">
                  <div className="px-4 py-3">
                    <span className="text-sm font-medium">{slo.name}</span>
                  </div>
                  <Separator />
                  <div className="flex items-center justify-between px-4 py-3">
                    <span className="text-sm text-muted-foreground">Target</span>
                    <span className="text-sm">{slo.target}%</span>
                  </div>
                  <Separator />
                  <div className="flex items-center justify-between px-4 py-3">
                    <span className="text-sm text-muted-foreground">Window</span>
                    <span className="text-sm">{slo.window}</span>
                  </div>
                  <Separator />
                  <div className="flex items-center justify-between px-4 py-3">
                    <span className="text-sm text-muted-foreground">Current</span>
                    <span className="text-sm">
                      {slo.current !== null ? `${slo.current.toFixed(2)}%` : "N/A"}
                    </span>
                  </div>
                  <Separator />
                  <div className="flex items-center justify-between px-4 py-3">
                    <span className="text-sm text-muted-foreground">Status</span>
                    <StatusBadge status={slo.status} />
                  </div>
                </CardContent>
              </Card>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
