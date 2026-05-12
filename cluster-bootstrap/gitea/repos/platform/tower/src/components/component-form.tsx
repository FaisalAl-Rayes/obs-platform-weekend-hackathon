"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import type { ComponentCreate, InfrastructureConfig, CustomSLO } from "@/lib/types";

export default function ComponentForm() {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [teams, setTeams] = useState<{ name: string }[]>([]);
  const [requiresPostgres, setRequiresPostgres] = useState(false);
  const [infraPostgres, setInfraPostgres] = useState({
    stage: { size: "200Mi", replicas: 1 },
    prod: { size: "500Mi", replicas: 2 },
  });
  const [sloAvailability, setSloAvailability] = useState({ target: 99.9, window: "30d" });
  const [customSlos, setCustomSlos] = useState<CustomSLO[]>([]);
  const [form, setForm] = useState<ComponentCreate>({
    name: "",
    description: "",
    componentKind: "service",
    stack: "python-service",
    owner: "",
    repo: "",
    slo: {
      availability: { target: 99.9, window: "30d" },
      custom: [],
    },
  });

  useEffect(() => {
    if (open) {
      fetch("/api/teams")
        .then((r) => r.json())
        .then((data) => {
          setTeams(data);
          if (data.length > 0 && !form.owner) {
            setForm((prev) => ({ ...prev, owner: data[0].name }));
          }
        });
    }
  }, [open]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      const payload: ComponentCreate = { ...form };
      if (requiresPostgres) {
        payload.infrastructure = { postgres: infraPostgres };
      }
      payload.slo = {
        availability: sloAvailability,
        custom: customSlos.length > 0 ? customSlos : undefined,
      };
      const res = await fetch("/api/components", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      });
      if (!res.ok) {
        const err = await res.json();
        alert(err.detail || "Failed to create component");
        return;
      }
      setOpen(false);
      setForm({ name: "", description: "", componentKind: "service", stack: "python-service", owner: "", repo: "", slo: { availability: { target: 99.9, window: "30d" }, custom: [] } });
      setRequiresPostgres(false);
      setInfraPostgres({ stage: { size: "200Mi", replicas: 1 }, prod: { size: "500Mi", replicas: 2 } });
      setSloAvailability({ target: 99.9, window: "30d" });
      setCustomSlos([]);
      router.refresh();
    } finally {
      setLoading(false);
    }
  };

  const update = (field: keyof ComponentCreate, value: string) =>
    setForm((prev) => ({ ...prev, [field]: value }));

  if (!open) {
    return <Button onClick={() => setOpen(true)}>Register Component</Button>;
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="bg-card border border-border rounded-lg p-6 w-full max-w-lg shadow-lg">
        <h2 className="text-lg font-semibold mb-4">Register Component</h2>
        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <div className="grid gap-2">
            <Label htmlFor="name">Name</Label>
            <Input id="name" value={form.name} onChange={(e) => update("name", e.target.value)} placeholder="my-service" required />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="description">Description</Label>
            <Input id="description" value={form.description} onChange={(e) => update("description", e.target.value)} placeholder="What does this component do?" />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div className="grid gap-2">
              <Label htmlFor="componentKind">Kind</Label>
              <select id="componentKind" value={form.componentKind} onChange={(e) => update("componentKind", e.target.value)} className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm">
                <option value="service">Service</option>
                <option value="library">Library</option>
                <option value="website">Website</option>
              </select>
            </div>
            <div className="grid gap-2">
              <Label htmlFor="stack">Stack</Label>
              <select id="stack" value={form.stack} onChange={(e) => update("stack", e.target.value)} className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm">
                <option value="python-service">Python Service</option>
                <option value="go-service">Go Service</option>
              </select>
            </div>
          </div>
          <div className="grid gap-2">
            <Label htmlFor="owner">Owner</Label>
            <select id="owner" value={form.owner} onChange={(e) => update("owner", e.target.value)} className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm">
              {teams.map((t) => (
                <option key={t.name} value={t.name}>{t.name}</option>
              ))}
            </select>
          </div>
          <div className="grid gap-2">
            <Label htmlFor="repo">Repository Name</Label>
            <Input id="repo" value={form.repo} onChange={(e) => update("repo", e.target.value)} placeholder="sample-app" required />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="documentation">Documentation URL</Label>
            <Input id="documentation" value={form.documentation ?? ""} onChange={(e) => update("documentation", e.target.value)} placeholder="https://..." />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="sourceCode">Source Code URL</Label>
            <Input id="sourceCode" value={form.sourceCode ?? ""} onChange={(e) => update("sourceCode", e.target.value)} placeholder="https://..." />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="issues">Issues URL</Label>
            <Input id="issues" value={form.issues ?? ""} onChange={(e) => update("issues", e.target.value)} placeholder="https://..." />
          </div>
          <div className="flex items-center gap-2">
            <input
              id="requiresPostgres"
              type="checkbox"
              checked={requiresPostgres}
              onChange={(e) => setRequiresPostgres(e.target.checked)}
              className="h-4 w-4 rounded border-input"
            />
            <Label htmlFor="requiresPostgres">Requires PostgreSQL</Label>
          </div>
          {requiresPostgres && (
            <div className="border border-border rounded-md p-4 flex flex-col gap-4">
              <p className="text-sm font-medium">PostgreSQL Infrastructure</p>
              <div>
                <p className="text-sm text-muted-foreground mb-2">Stage</p>
                <div className="grid grid-cols-2 gap-4">
                  <div className="grid gap-2">
                    <Label htmlFor="stage-size">Size</Label>
                    <Input
                      id="stage-size"
                      value={infraPostgres.stage.size}
                      onChange={(e) =>
                        setInfraPostgres((prev) => ({
                          ...prev,
                          stage: { ...prev.stage, size: e.target.value },
                        }))
                      }
                    />
                  </div>
                  <div className="grid gap-2">
                    <Label htmlFor="stage-replicas">Replicas</Label>
                    <Input
                      id="stage-replicas"
                      type="number"
                      min={1}
                      value={infraPostgres.stage.replicas}
                      onChange={(e) =>
                        setInfraPostgres((prev) => ({
                          ...prev,
                          stage: { ...prev.stage, replicas: parseInt(e.target.value) || 1 },
                        }))
                      }
                    />
                  </div>
                </div>
              </div>
              <div>
                <p className="text-sm text-muted-foreground mb-2">Prod</p>
                <div className="grid grid-cols-2 gap-4">
                  <div className="grid gap-2">
                    <Label htmlFor="prod-size">Size</Label>
                    <Input
                      id="prod-size"
                      value={infraPostgres.prod.size}
                      onChange={(e) =>
                        setInfraPostgres((prev) => ({
                          ...prev,
                          prod: { ...prev.prod, size: e.target.value },
                        }))
                      }
                    />
                  </div>
                  <div className="grid gap-2">
                    <Label htmlFor="prod-replicas">Replicas</Label>
                    <Input
                      id="prod-replicas"
                      type="number"
                      min={1}
                      value={infraPostgres.prod.replicas}
                      onChange={(e) =>
                        setInfraPostgres((prev) => ({
                          ...prev,
                          prod: { ...prev.prod, replicas: parseInt(e.target.value) || 1 },
                        }))
                      }
                    />
                  </div>
                </div>
              </div>
            </div>
          )}
          <div className="border border-border rounded-md p-4 flex flex-col gap-4">
            <p className="text-sm font-medium">Availability SLO</p>
            <div className="grid grid-cols-2 gap-4">
              <div className="grid gap-2">
                <Label htmlFor="slo-target">Availability Target (%)</Label>
                <Input
                  id="slo-target"
                  type="number"
                  step="0.1"
                  min={0}
                  max={100}
                  value={sloAvailability.target}
                  onChange={(e) =>
                    setSloAvailability((prev) => ({
                      ...prev,
                      target: parseFloat(e.target.value) || 99.9,
                    }))
                  }
                  required
                />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="slo-window">Window</Label>
                <Input
                  id="slo-window"
                  value={sloAvailability.window}
                  onChange={(e) =>
                    setSloAvailability((prev) => ({
                      ...prev,
                      window: e.target.value,
                    }))
                  }
                  placeholder="30d"
                  required
                />
              </div>
            </div>
            {customSlos.length > 0 && (
              <div className="flex flex-col gap-3">
                <p className="text-sm text-muted-foreground">Custom SLOs</p>
                {customSlos.map((slo, idx) => (
                  <div key={idx} className="border border-border rounded-md p-3 flex flex-col gap-2">
                    <div className="grid grid-cols-2 gap-4">
                      <div className="grid gap-1">
                        <Label>Name</Label>
                        <Input
                          value={slo.name}
                          onChange={(e) => {
                            const updated = [...customSlos];
                            updated[idx] = { ...updated[idx], name: e.target.value };
                            setCustomSlos(updated);
                          }}
                          placeholder="success-rate"
                          required
                        />
                      </div>
                      <div className="grid gap-1">
                        <Label>PromQL Query</Label>
                        <Input
                          value={slo.query}
                          onChange={(e) => {
                            const updated = [...customSlos];
                            updated[idx] = { ...updated[idx], query: e.target.value };
                            setCustomSlos(updated);
                          }}
                          placeholder="rate(http_requests_total{status=~'2..'}[5m])"
                          required
                        />
                      </div>
                    </div>
                    <div className="grid grid-cols-2 gap-4">
                      <div className="grid gap-1">
                        <Label>Target (%)</Label>
                        <Input
                          type="number"
                          step="0.1"
                          min={0}
                          max={100}
                          value={slo.target}
                          onChange={(e) => {
                            const updated = [...customSlos];
                            updated[idx] = { ...updated[idx], target: parseFloat(e.target.value) || 99.9 };
                            setCustomSlos(updated);
                          }}
                          required
                        />
                      </div>
                      <div className="grid gap-1">
                        <Label>Window</Label>
                        <Input
                          value={slo.window}
                          onChange={(e) => {
                            const updated = [...customSlos];
                            updated[idx] = { ...updated[idx], window: e.target.value };
                            setCustomSlos(updated);
                          }}
                          placeholder="7d"
                          required
                        />
                      </div>
                    </div>
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      className="self-end"
                      onClick={() => setCustomSlos(customSlos.filter((_, i) => i !== idx))}
                    >
                      Remove
                    </Button>
                  </div>
                ))}
              </div>
            )}
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() =>
                setCustomSlos([...customSlos, { name: "", query: "", target: 99.9, window: "7d" }])
              }
            >
              Add Custom SLO
            </Button>
          </div>
          <div className="flex gap-2 justify-end">
            <Button type="button" variant="outline" onClick={() => setOpen(false)}>Cancel</Button>
            <Button type="submit" disabled={loading || !form.name || !form.owner || !form.repo}>
              {loading ? "Registering..." : "Register"}
            </Button>
          </div>
        </form>
      </div>
    </div>
  );
}
