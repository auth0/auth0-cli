import type { ApiClient, HttpMethod } from "./api.js";

// A request a real run would send. Secrets in the body are replaced by CLIENT_SECRET_PLACEHOLDER.
export interface PlannedRequest {
  method: Exclude<HttpMethod, "GET">;
  path: string;
  body?: unknown;
}

export const CLIENT_SECRET_PLACEHOLDER = "<client secret>";

// Sends every read to the tenant and records each other request instead of sending it. Setup runs
// as it does for real, so the created, reused and updated decisions and the payloads all come from
// the same code. The IDs of resources a real run would create are placeholders, and a read about one
// of them returns an empty list, because that resource does not exist yet.
export function planWrites(api: ApiClient): { api: ApiClient; requests: () => PlannedRequest[] } {
  const requests: PlannedRequest[] = [];
  const placeholders: string[] = [];
  let counter = 0;

  return {
    api: {
      async request<T>(method: HttpMethod, path: string, body?: unknown): Promise<T> {
        if (method === "GET") {
          if (placeholders.some((id) => path.includes(encodeURIComponent(id)))) return [] as T;
          return api.request<T>(method, path, body);
        }

        requests.push({ method: method as PlannedRequest["method"], path, body: redact(body) });
        if (method !== "POST") return {} as T;

        const id = `<new:${path.split("/")[0]}:${++counter}>`;
        placeholders.push(id);
        return placeholderResponse(path, body, id) as T;
      },
    },
    requests: () => JSON.parse(JSON.stringify(requests)) as PlannedRequest[],
  };
}

// The part of a create response that setup reads back: the ID, and for clients the secret.
function placeholderResponse(path: string, body: unknown, id: string): unknown {
  const fields = isRecord(body) ? body : {};
  if (path === "clients") return { ...fields, client_id: id, client_secret: CLIENT_SECRET_PLACEHOLDER };
  if (path === "forms/import") return { form: { id, nodes: [] }, flows: [] };
  return { ...fields, id };
}

// Copies the value and replaces every client secret, so a secret read from the tenant for a reused
// application can never reach the output.
function redact(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(redact);
  if (isRecord(value)) {
    return Object.fromEntries(
      Object.entries(value).map(([key, item]) => [key, key === "client_secret" ? CLIENT_SECRET_PLACEHOLDER : redact(item)]),
    );
  }
  return value;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}
