import { request } from "./api";

export async function customFetch<T>(url: string, options: RequestInit): Promise<T> {
  const apiPath = url.startsWith("/api/v1/") ? url : `/api/v1${url}`;
  return request<T>(apiPath, options, (status) => `Request failed (${status}).`);
}
