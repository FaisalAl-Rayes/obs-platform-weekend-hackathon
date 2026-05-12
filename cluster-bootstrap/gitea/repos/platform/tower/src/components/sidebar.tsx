"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { cn } from "@/lib/utils";

export default function Sidebar() {
  const pathname = usePathname();

  return (
    <aside className="w-56 border-r border-border flex flex-col py-4 px-3 shrink-0">
      <div className="flex items-center gap-2 px-2 mb-8">
        <div className="w-8 h-8 rounded-full bg-primary flex items-center justify-center text-primary-foreground text-sm font-bold">
          T
        </div>
        <span className="font-semibold text-lg">Tower</span>
      </div>
      <nav className="flex flex-col gap-1">
        <Link
          href="/"
          className={cn(
            "flex items-center gap-3 px-3 py-2 rounded-md text-sm transition-colors",
            pathname === "/"
              ? "bg-accent text-accent-foreground"
              : "text-muted-foreground hover:bg-accent/50"
          )}
        >
          Home
        </Link>
      </nav>
    </aside>
  );
}
