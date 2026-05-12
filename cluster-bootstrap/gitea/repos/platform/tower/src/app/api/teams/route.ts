import { NextResponse } from "next/server";
import { listOrgs } from "@/lib/gitea";

export async function GET() {
  const orgs = await listOrgs();
  return NextResponse.json(orgs);
}
