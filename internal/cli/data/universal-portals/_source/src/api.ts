// Management API client. Requests go through the CLI's loopback proxy, which
// swaps the proxy token for the real access token, so this module never holds it.

export type HttpMethod = "GET" | "POST" | "PATCH" | "DELETE";

export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly errorCode: string | undefined,
    message: string,
    readonly body: unknown,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

export interface ApiClient {
  request<T = unknown>(method: HttpMethod, path: string, body?: unknown): Promise<T>;
}

export type FetchLike = (url: string, init: RequestInit) => Promise<Response>;

export function createApiClient(baseUrl: string, token: string, fetchImpl: FetchLike = fetch): ApiClient {
  const root = `${baseUrl.replace(/\/$/, "")}/api/v2/`;
  return {
    async request<T>(method: HttpMethod, path: string, body?: unknown): Promise<T> {
      const headers: Record<string, string> = { authorization: `Bearer ${token}`, accept: "application/json" };
      const init: RequestInit = { method, headers };
      if (body !== undefined) {
        headers["content-type"] = "application/json";
        init.body = JSON.stringify(body);
      }
      const response = await fetchImpl(root + path, init);
      const text = await response.text();
      const parsed = text ? safeJson(text) : undefined;
      if (!response.ok) {
        throw toApiError(response.status, parsed);
      }
      return parsed as T;
    },
  };
}

export function toApiError(status: number, body: unknown): ApiError {
  const record = isRecord(body) ? body : {};
  const message = typeof record.message === "string" ? record.message : `HTTP ${status}`;
  const errorCode = typeof record.errorCode === "string" ? record.errorCode : undefined;
  return new ApiError(status, errorCode, message, body);
}

// Management API list endpoints return either an array or an object keyed by
// the resource name, depending on the endpoint and the query parameters.
export function listItems<T>(body: unknown, key: string): T[] {
  if (Array.isArray(body)) return body as T[];
  if (isRecord(body) && Array.isArray(body[key])) return body[key] as T[];
  return [];
}

// The page size and the most pages one listing may read. The cap stops an endpoint
// that ignores paging from looping forever.
const PAGE_SIZE = 100;
const MAX_PAGES = 100;

// Reads every page of a list endpoint. A short page is the last one, so the total is
// not needed, and endpoints that reject extra query parameters are not affected.
export async function listAll<T>(api: ApiClient, path: string, key: string): Promise<T[]> {
  const separator = path.includes("?") ? "&" : "?";
  const items: T[] = [];
  for (let page = 0; page < MAX_PAGES; page += 1) {
    const batch = listItems<T>(await api.request("GET", `${path}${separator}page=${page}&per_page=${PAGE_SIZE}`), key);
    items.push(...batch);
    if (batch.length < PAGE_SIZE) return items;
  }
  throw new Error(`${path} returned more than ${MAX_PAGES * PAGE_SIZE} items. The listing did not stop paging.`);
}

function safeJson(text: string): unknown {
  try {
    return JSON.parse(text);
  } catch {
    return text;
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}
