import Link from "next/link";
import { notFound } from "next/navigation";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import DeleteButton from "@/components/delete-button";
import SloTab from "@/components/slo-tab";
import type { ComponentData } from "@/lib/types";

const ARGOCD_URL = process.env.ARGOCD_URL || "https://localhost:8080";

async function getComponent(name: string): Promise<ComponentData | null> {
  const base = process.env.INTERNAL_URL || "http://localhost:3000";
  const res = await fetch(`${base}/api/components/${name}`, { cache: "no-store" });
  if (!res.ok) return null;
  return res.json();
}

export default async function ComponentDetail({ params }: { params: Promise<{ name: string }> }) {
  const { name } = await params;
  const component = await getComponent(name);
  if (!component) notFound();

  const details = [
    { label: "Kind", value: component.componentKind, badge: true },
    { label: "Stack", value: component.stack, badge: true },
    { label: "Owner", value: component.owner },
    { label: "Repository", value: component.repo },
    { label: "Documentation", value: component.documentation, link: true },
    { label: "Source code", value: component.sourceCode, link: true },
    { label: "Issues", value: component.issues, link: true },
    { label: "Created at", value: component.createdAt },
    { label: "Last updated on", value: component.lastUpdatedOn },
  ];

  const stageAppUrl = `${ARGOCD_URL}/applications/argocd/${component.name}-stage`;
  const prodAppUrl = `${ARGOCD_URL}/applications/argocd/${component.name}-prod`;

  return (
    <div className="p-8">
      <div className="flex items-center gap-3 mb-6">
        <Link href="/" className="text-muted-foreground hover:text-foreground text-sm">
          &larr; Back
        </Link>
        <h1 className="text-2xl font-semibold">{component.name}</h1>
      </div>

      <Tabs defaultValue="general">
        <TabsList>
          <TabsTrigger value="general">General</TabsTrigger>
          <TabsTrigger value="deployments">Deployments</TabsTrigger>
          <TabsTrigger value="slo">SLO</TabsTrigger>
        </TabsList>

        <TabsContent value="general" className="mt-6">
          {component.description && (
            <p className="text-muted-foreground mb-6">{component.description}</p>
          )}

          <h2 className="text-lg font-medium mb-4">Details</h2>
          <Card>
            <CardContent className="p-0">
              {details.map((d, i) => (
                <div key={d.label}>
                  <div className="flex items-center justify-between px-4 py-3">
                    <span className="text-sm text-muted-foreground">{d.label}</span>
                    {d.link && d.value ? (
                      <a href={d.value} target="_blank" rel="noopener noreferrer" className="text-sm text-primary hover:underline">
                        {d.value}
                      </a>
                    ) : d.badge ? (
                      <Badge variant="secondary">{d.value}</Badge>
                    ) : (
                      <span className="text-sm">{d.value || "—"}</span>
                    )}
                  </div>
                  {i < details.length - 1 && <Separator />}
                </div>
              ))}
            </CardContent>
          </Card>

          {component.infrastructure?.postgres && (
            <>
              <h2 className="text-lg font-medium mt-6 mb-4">Infrastructure</h2>
              <Card>
                <CardContent className="p-0">
                  {component.infrastructure.postgres.stage && (
                    <>
                      <div className="px-4 py-3">
                        <span className="text-sm font-medium">Stage</span>
                      </div>
                      <Separator />
                      <div className="flex items-center justify-between px-4 py-3">
                        <span className="text-sm text-muted-foreground">PostgreSQL Size</span>
                        <span className="text-sm">{component.infrastructure.postgres.stage.size}</span>
                      </div>
                      <Separator />
                      <div className="flex items-center justify-between px-4 py-3">
                        <span className="text-sm text-muted-foreground">PostgreSQL Replicas</span>
                        <span className="text-sm">{component.infrastructure.postgres.stage.replicas}</span>
                      </div>
                    </>
                  )}
                  {component.infrastructure.postgres.prod && (
                    <>
                      <Separator />
                      <div className="px-4 py-3">
                        <span className="text-sm font-medium">Prod</span>
                      </div>
                      <Separator />
                      <div className="flex items-center justify-between px-4 py-3">
                        <span className="text-sm text-muted-foreground">PostgreSQL Size</span>
                        <span className="text-sm">{component.infrastructure.postgres.prod.size}</span>
                      </div>
                      <Separator />
                      <div className="flex items-center justify-between px-4 py-3">
                        <span className="text-sm text-muted-foreground">PostgreSQL Replicas</span>
                        <span className="text-sm">{component.infrastructure.postgres.prod.replicas}</span>
                      </div>
                    </>
                  )}
                </CardContent>
              </Card>
            </>
          )}

          {Object.keys(component.labels).length > 0 && (
            <>
              <h2 className="text-lg font-medium mt-6 mb-3">Labels</h2>
              <div className="flex gap-2 flex-wrap">
                {Object.entries(component.labels).map(([k, v]) => (
                  <Badge key={k} variant="outline">{k}: {v}</Badge>
                ))}
              </div>
            </>
          )}

          <div className="mt-8">
            <DeleteButton name={component.name} />
          </div>
        </TabsContent>

        <TabsContent value="deployments" className="mt-6">
          <h2 className="text-lg font-medium mb-4">ArgoCD Applications</h2>
          <Card>
            <CardContent className="p-0">
              <div className="flex items-center justify-between px-4 py-3">
                <span className="text-sm text-muted-foreground">Stage</span>
                <a href={stageAppUrl} target="_blank" rel="noopener noreferrer" className="text-sm text-primary hover:underline">
                  {component.name}-stage
                </a>
              </div>
              <Separator />
              <div className="flex items-center justify-between px-4 py-3">
                <span className="text-sm text-muted-foreground">Production</span>
                <a href={prodAppUrl} target="_blank" rel="noopener noreferrer" className="text-sm text-primary hover:underline">
                  {component.name}-prod
                </a>
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="slo" className="mt-6">
          <SloTab name={component.name} />
        </TabsContent>
      </Tabs>
    </div>
  );
}
