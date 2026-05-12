const GITEA_URL = process.env.GITEA_URL || "http://gitea-http.gitea.svc:3000";
const GITEA_USER = process.env.GITEA_USER || "gitea_admin";
const GITEA_PASSWORD = process.env.GITEA_PASSWORD || "admin1234";
const GITEA_ORG = process.env.GITEA_ORG || "platform";
const GITEA_REPO = process.env.GITEA_REPO || "service-catalog";

const BASE = `${GITEA_URL}/api/v1/repos/${GITEA_ORG}/${GITEA_REPO}/contents`;
const AUTH = Buffer.from(`${GITEA_USER}:${GITEA_PASSWORD}`).toString("base64");

const headers = {
  Authorization: `Basic ${AUTH}`,
  "Content-Type": "application/json",
};

export async function listFiles(): Promise<{ name: string }[]> {
  const res = await fetch(BASE, { headers, cache: "no-store" });
  if (!res.ok) return [];
  const entries = await res.json();
  return entries.filter((e: { name: string }) => e.name.endsWith(".yaml") || e.name.endsWith(".yml"));
}

export async function readFile(filename: string): Promise<{ data: Record<string, unknown>; sha: string } | null> {
  const res = await fetch(`${BASE}/${filename}`, { headers, cache: "no-store" });
  if (!res.ok) return null;
  const json = await res.json();
  const { parse } = await import("yaml");
  const content = Buffer.from(json.content, "base64").toString("utf-8");
  return { data: parse(content), sha: json.sha };
}

export async function createFile(filename: string, content: Record<string, unknown>, message: string) {
  const { stringify } = await import("yaml");
  const encoded = Buffer.from(stringify(content)).toString("base64");
  const res = await fetch(`${BASE}/${filename}`, {
    method: "POST",
    headers,
    body: JSON.stringify({ content: encoded, message }),
  });
  if (!res.ok) throw new Error(`Failed to create ${filename}: ${res.status}`);
  return res.json();
}

export async function updateFile(filename: string, content: Record<string, unknown>, sha: string, message: string) {
  const { stringify } = await import("yaml");
  const encoded = Buffer.from(stringify(content)).toString("base64");
  const res = await fetch(`${BASE}/${filename}`, {
    method: "PUT",
    headers,
    body: JSON.stringify({ content: encoded, sha, message }),
  });
  if (!res.ok) throw new Error(`Failed to update ${filename}: ${res.status}`);
  return res.json();
}

export async function deleteFile(filename: string, sha: string, message: string) {
  const res = await fetch(`${BASE}/${filename}`, {
    method: "DELETE",
    headers,
    body: JSON.stringify({ sha, message }),
  });
  if (!res.ok) throw new Error(`Failed to delete ${filename}: ${res.status}`);
}

export async function listOrgs(): Promise<{ name: string; description: string }[]> {
  const res = await fetch(`${GITEA_URL}/api/v1/orgs`, { headers, cache: "no-store" });
  if (!res.ok) return [];
  const orgs = await res.json();
  return orgs
    .map((o: { username: string; description: string }) => ({
      name: o.username,
      description: o.description || "",
    }));
}

export async function fileExists(filename: string): Promise<string | null> {
  const res = await fetch(`${BASE}/${filename}`, { headers, cache: "no-store" });
  if (!res.ok) return null;
  const json = await res.json();
  return json.sha;
}
