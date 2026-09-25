type APIErrorBody = { error?: string };
import { APIRequestError, connectionUnavailableEvent } from "./api";

export async function customFetch<T>(url: string, options: RequestInit): Promise<T> {
  let response: Response;
  try {
    const apiPath = url.startsWith("/api/v1/") ? url : `/api/v1${url}`;
    response = await fetch(apiPath, options);
  } catch (error) {
    if (typeof window !== "undefined") window.dispatchEvent(new Event(connectionUnavailableEvent));
    throw error;
  }
  if (response.headers.get("X-Visto-Offline") === "true" && typeof window !== "undefined") {
    window.dispatchEvent(new Event(connectionUnavailableEvent));
  }
  if (!response.ok) {
    const body = (await response.json().catch(() => ({}))) as APIErrorBody;
    throw new APIRequestError(
      body.error || `Request failed (${response.status}).`,
      response.status,
    );
  }
  if (response.status === 204) return undefined as T;
  return response.json() as Promise<T>;
}
