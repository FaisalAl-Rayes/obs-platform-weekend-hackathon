import { NextResponse } from "next/server";
import { readFile, updateFile, deleteFile } from "@/lib/gitea";
import { toComponentData } from "@/lib/component";
import type { ComponentUpdate } from "@/lib/types";

export async function GET(_request: Request, { params }: { params: Promise<{ name: string }> }) {
  const { name } = await params;
  const filename = `${name}.yaml`;
  const result = await readFile(filename);
  if (!result) {
    return NextResponse.json({ detail: `Component '${name}' not found` }, { status: 404 });
  }
  return NextResponse.json(toComponentData(result.data, result.sha));
}

export async function PUT(request: Request, { params }: { params: Promise<{ name: string }> }) {
  const { name } = await params;
  const filename = `${name}.yaml`;
  const result = await readFile(filename);
  if (!result) {
    return NextResponse.json({ detail: `Component '${name}' not found` }, { status: 404 });
  }

  const body: ComponentUpdate = await request.json();
  const data = result.data;
  const spec = data.spec as Record<string, unknown>;
  const metadata = data.metadata as Record<string, unknown>;

  if (body.description !== undefined) spec.description = body.description;
  if (body.componentKind !== undefined) spec.kind = body.componentKind;
  if (body.stack !== undefined) spec.stack = body.stack;
  if (body.owner !== undefined) {
    spec.owner = body.owner;
    (metadata.labels as Record<string, string>).team = body.owner;
  }
  if (body.repo !== undefined) spec.repo = body.repo;
  if (body.documentation !== undefined) spec.documentation = body.documentation;
  if (body.sourceCode !== undefined) spec.sourceCode = body.sourceCode;
  if (body.issues !== undefined) spec.issues = body.issues;
  if (body.labels !== undefined) {
    metadata.labels = { ...body.labels, team: spec.owner as string };
  }
  if (body.infrastructure !== undefined) spec.infrastructure = body.infrastructure;
  if (body.slo !== undefined) spec.slo = body.slo;

  spec.lastUpdatedOn = new Date().toISOString().split("T")[0];

  await updateFile(filename, data, result.sha, `tower: update component ${name}`);
  return NextResponse.json(toComponentData(data));
}

export async function DELETE(_request: Request, { params }: { params: Promise<{ name: string }> }) {
  const { name } = await params;
  const filename = `${name}.yaml`;
  const result = await readFile(filename);
  if (!result) {
    return NextResponse.json({ detail: `Component '${name}' not found` }, { status: 404 });
  }

  await deleteFile(filename, result.sha, `tower: delete component ${name}`);
  return new NextResponse(null, { status: 204 });
}
