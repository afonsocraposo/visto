type APIError = { error?: string };

export class APIRequestError extends Error {
  readonly status: number;

  constructor(message: string, status: number) {
    super(message);
    this.name = "APIRequestError";
    this.status = status;
  }
}

export function retryTransientRequest(failureCount: number, error: unknown): boolean {
  if (failureCount >= 2) return false;
  if (error instanceof APIRequestError) return error.status === 429 || error.status >= 500;
  return error instanceof TypeError;
}

export const connectionUnavailableEvent = "visto:connection-unavailable";

function reportConnectionUnavailable() {
  if (typeof window !== "undefined") {
    window.dispatchEvent(new Event(connectionUnavailableEvent));
  }
}

export async function request<T>(
  path: string,
  init: RequestInit,
  fallback: string | ((status: number) => string),
): Promise<T> {
  let response: Response;
  try {
    response = await fetch(path, init);
  } catch (error) {
    reportConnectionUnavailable();
    throw error;
  }
  if (response.headers.get("X-Visto-Offline") === "true") reportConnectionUnavailable();
  if (!response.ok) {
    const body = (await response.json().catch(() => ({}))) as APIError;
    throw new APIRequestError(
      body.error || (typeof fallback === "string" ? fallback : fallback(response.status)),
      response.status,
    );
  }
  if (response.status === 204) return undefined as T;
  if (response.headers.get("Content-Type")?.startsWith("text/csv")) {
    return response.text() as Promise<T>;
  }
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
    return request<T>(
      path,
      body === undefined ? { method: "POST" } : jsonInit("POST", body),
      fallback,
    );
  },
  patch<T = void>(path: string, body: unknown, fallback = "Request failed."): Promise<T> {
    return request<T>(path, jsonInit("PATCH", body), fallback);
  },
  put<T = void>(path: string, body: unknown, fallback = "Request failed."): Promise<T> {
    return request<T>(path, jsonInit("PUT", body), fallback);
  },
  delete(path: string, fallback = "Request failed.", body?: unknown): Promise<void> {
    return request<void>(
      path,
      body === undefined ? { method: "DELETE" } : jsonInit("DELETE", body),
      fallback,
    );
  },
};
