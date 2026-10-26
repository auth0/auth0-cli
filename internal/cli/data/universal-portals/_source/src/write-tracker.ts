import type { ApiClient, HttpMethod } from "./api.js";

// Records whether a request that changes the tenant succeeded. Failed requests do
// not count, so the preflight probes (rejected with 400) do not count either.
export function trackWrites(api: ApiClient): { api: ApiClient; wrote: () => boolean } {
  let wrote = false;
  return {
    api: {
      async request<T>(method: HttpMethod, path: string, body?: unknown): Promise<T> {
        const result = await api.request<T>(method, path, body);
        if (method !== "GET") wrote = true;
        return result;
      },
    },
    wrote: () => wrote,
  };
}
