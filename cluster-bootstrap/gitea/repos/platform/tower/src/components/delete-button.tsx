"use client";

import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";

export default function DeleteButton({ name }: { name: string }) {
  const router = useRouter();

  const handleDelete = async () => {
    if (!confirm(`Delete component "${name}"?`)) return;
    const res = await fetch(`/api/components/${name}`, { method: "DELETE" });
    if (res.ok) router.push("/");
  };

  return (
    <Button variant="destructive" onClick={handleDelete}>
      Delete Component
    </Button>
  );
}
