import ComponentCard from "@/components/component-card";
import ComponentForm from "@/components/component-form";
import type { ComponentData } from "@/lib/types";

async function getComponents(): Promise<ComponentData[]> {
  const base = process.env.INTERNAL_URL || "http://localhost:3000";
  const res = await fetch(`${base}/api/components`, { cache: "no-store" });
  if (!res.ok) return [];
  return res.json();
}

export default async function Home() {
  const components = await getComponents();

  return (
    <div className="p-8">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-semibold">Home</h1>
          <p className="text-muted-foreground text-sm mt-1">
            Components &middot; {components.length}
          </p>
        </div>
        <ComponentForm />
      </div>
      {components.length === 0 ? (
        <p className="text-muted-foreground">No components registered yet.</p>
      ) : (
        <div className="flex flex-col gap-3">
          {components.map((c) => (
            <ComponentCard key={c.name} component={c} />
          ))}
        </div>
      )}
    </div>
  );
}
