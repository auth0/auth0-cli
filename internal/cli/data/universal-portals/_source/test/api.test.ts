import { strict as assert } from "node:assert";
import { test } from "node:test";

import { ApiClient, ApiError, createApiClient, listAll, listAllByCheckpoint, listItems, type FetchLike } from "../src/api.js";

function fakeFetch(status: number, body: unknown): { fetch: FetchLike; calls: { url: string; init: RequestInit }[] } {
  const calls: { url: string; init: RequestInit }[] = [];
  const fetch: FetchLike = async (url, init) => {
    calls.push({ url, init });
    const text = body === undefined ? "" : JSON.stringify(body);
    return new Response(text, { status, headers: { "content-type": "application/json" } });
  };
  return { fetch, calls };
}

test("listAll reads every page until a short one", async () => {
  const everything = Array.from({ length: 250 }, (_, index) => ({ id: `item_${index}` }));
  const api = {
    async request(_method: string, path: string) {
      const page = Number(new URL(`https://x/${path}`).searchParams.get("page"));
      return everything.slice(page * 100, (page + 1) * 100);
    },
  } as ApiClient;

  const items = await listAll<{ id: string }>(api, "things", "things");

  assert.equal(items.length, 250);
  assert.equal(items[249].id, "item_249");
});

test("listAll stops with an error when an endpoint ignores paging", async () => {
  const api = {
    async request() {
      return Array.from({ length: 100 }, (_, index) => ({ id: `item_${index}` }));
    },
  } as ApiClient;

  await assert.rejects(listAll(api, "things", "things"), /did not stop paging/);
});

test("requests go to the proxy with the bearer token and JSON body", async () => {
  const { fetch, calls } = fakeFetch(200, { id: "x" });
  const api = createApiClient("https://127.0.0.1:4433/", "secret-token", fetch);
  const result = await api.request<{ id: string }>("POST", "clients", { name: "a" });

  assert.equal(result.id, "x");
  assert.equal(calls[0].url, "https://127.0.0.1:4433/api/v2/clients");
  assert.equal(calls[0].init.method, "POST");
  assert.deepEqual(calls[0].init.headers, {
    authorization: "Bearer secret-token",
    accept: "application/json",
    "content-type": "application/json",
  });
  assert.equal(calls[0].init.body, JSON.stringify({ name: "a" }));
});

test("a non-2xx response becomes an ApiError with status, code and message", async () => {
  const { fetch } = fakeFetch(403, {
    statusCode: 403,
    message: "Insufficient scope, expected any of: create:clients",
    errorCode: "insufficient_scope",
  });
  const api = createApiClient("https://proxy", "t", fetch);

  await assert.rejects(api.request("POST", "clients", {}), (error: unknown) => {
    assert.ok(error instanceof ApiError);
    assert.equal(error.status, 403);
    assert.equal(error.errorCode, "insufficient_scope");
    assert.match(error.message, /create:clients/);
    return true;
  });
});

test("listItems reads both array responses and keyed responses", () => {
  assert.deepEqual(listItems([{ id: 1 }], "portals"), [{ id: 1 }]);
  assert.deepEqual(listItems({ portals: [{ id: 2 }] }, "portals"), [{ id: 2 }]);
  assert.deepEqual(listItems({ other: [] }, "portals"), []);
});

test("listAllByCheckpoint follows the next token until there is none, and never sends page numbers", async () => {
  const everything = Array.from({ length: 250 }, (_, index) => ({ id: `item_${index}` }));
  const paths: string[] = [];
  const api = {
    async request(_method: string, path: string) {
      paths.push(path);
      const params = new URL(`https://x/${path}`).searchParams;
      const start = params.has("from") ? everything.findIndex((item) => item.id === params.get("from")) : 0;
      const take = Number(params.get("take"));
      const next = everything[start + take];
      return { things: everything.slice(start, start + take), ...(next ? { next: next.id } : {}) };
    },
  } as ApiClient;

  const items = await listAllByCheckpoint<{ id: string }>(api, "things?strategy=auth0", "things");

  assert.equal(items.length, 250);
  assert.deepEqual(paths, [
    "things?strategy=auth0&take=100",
    "things?strategy=auth0&take=100&from=item_100",
    "things?strategy=auth0&take=100&from=item_200",
  ]);
});

test("listAllByCheckpoint stops with an error when the token never runs out", async () => {
  const api = {
    async request() {
      return { things: [{ id: "a" }], next: "again" };
    },
  } as ApiClient;

  await assert.rejects(listAllByCheckpoint(api, "things", "things"), /did not stop paging/);
});
