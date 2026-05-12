import Link from "next/link";
import { Badge } from "@/components/ui/badge";
import type { ComponentData } from "@/lib/types";

export default function ComponentCard({ component }: { component: ComponentData }) {
  return (
    <Link
      href={`/components/${component.name}`}
      className="flex items-center justify-between px-4 py-3 rounded-lg border border-border bg-card hover:bg-accent/30 transition-colors"
    >
      <div className="flex items-center gap-3">
        <div className="w-10 h-10 rounded-full bg-muted flex items-center justify-center text-muted-foreground text-sm font-bold uppercase">
          {component.name[0]}
        </div>
        <div>
          <div className="font-medium text-card-foreground">{component.name}</div>
          <div className="text-sm text-muted-foreground">
            {component.componentKind} / {component.stack} &middot; {component.owner}
          </div>
        </div>
      </div>
      <div className="flex items-center gap-2">
        <Badge variant="secondary">{component.componentKind}</Badge>
        <Badge variant="outline">{component.stack}</Badge>
      </div>
    </Link>
  );
}
