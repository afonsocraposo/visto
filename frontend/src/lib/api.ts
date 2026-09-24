type APIError = { error?: string };

async function request<T>(path: string, init: RequestInit, fallback: string): Promise<T> {
  const response = await fetch(path, init);
  if (!response.ok) {
    const body = await response.json().catch(() => ({})) as APIError;
    throw new Error(body.error || fallback);
  }
  if (response.status === 204) return undefined as T;
  return response.json() as Promise<T>;
}

function jsonInit(method: string, body: unknown): RequestInit {
  return {
    method,
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  };
}

export const api = {
  get<T>(path: string, fallback = "Request failed."): Promise<T> {
    return request<T>(path, {}, fallback);
  },
  post<T = void>(path: string, body?: unknown, fallback = "Request failed."): Promise<T> {
    return request<T>(path, body === undefined ? { method: "POST" } : jsonInit("POST", body), fallback);
  },
  patch<T = void>(path: string, body: unknown, fallback = "Request failed."): Promise<T> {
    return request<T>(path, jsonInit("PATCH", body), fallback);
  },
  delete(path: string, fallback = "Request failed."): Promise<void> {
    return request<void>(path, { method: "DELETE" }, fallback);
  },
};
