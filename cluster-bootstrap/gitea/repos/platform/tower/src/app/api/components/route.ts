import { NextResponse } from "next/server";
import { listFiles, readFile, createFile, fileExists } from "@/lib/gitea";
import { toComponentData, buildComponentYaml } from "@/lib/component";
import type { ComponentCreate } from "@/lib/types";

export async function GET() {
  const files = await listFiles();
  const components = [];
  for (const f of files) {
    const result = await readFile(f.name);
    if (result) {
      components.push(toComponentData(result.data, result.sha));
    }
  }
  return NextResponse.json(components);
}

export async function POST(request: Request) {
  const body: ComponentCreate = await request.json();
  const filename = `${body.name}.yaml`;

  const existing = await fileExists(filename);
  if (existing) {
    return NextResponse.json({ detail: `Component '${body.name}' already exists` }, { status: 409 });
  }

  const yamlData = buildComponentYaml(body);
  await createFile(filename, yamlData, `tower: register component ${body.name}`);
  return NextResponse.json(toComponentData(yamlData), { status: 201 });
}
