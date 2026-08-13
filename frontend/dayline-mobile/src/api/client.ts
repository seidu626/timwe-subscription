import { API_BASE_URL } from '@/config';
import { ApiError, messageForCode } from './errors';
import { readToken } from './session';

interface RequestOptions {
  method?: 'GET' | 'POST' | 'PUT' | 'DELETE';
  body?: unknown;
  /** Set false for the two unauthenticated routes (otp request/verify, catalog). */
  auth?: boolean;
}

interface ApiErrorBody {
  error?: { code?: string; message?: string };
}

/**
 * Typed fetch wrapper for the Dayline app API (docs/dayline-app-api-contract.md).
 * - Attaches `Authorization: Bearer <jwt>` from secure storage unless auth:false.
 * - Maps the contract's error envelope to user-readable ApiError instances.
 * - Treats 204/202 as bodiless success.
 */
export async function apiRequest<T>(path: string, options: RequestOptions = {}): Promise<T> {
  if (!API_BASE_URL) {
    throw new ApiError('CONFIG_ERROR', messageForCode('CONFIG_ERROR'), 0);
  }

  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (options.auth !== false) {
    const token = await readToken();
    if (token) {
      headers.Authorization = `Bearer ${token}`;
    }
  }

  let response: Response;
  try {
    response = await fetch(`${API_BASE_URL}${path}`, {
      method: options.method ?? 'GET',
      headers,
      body: options.body !== undefined ? JSON.stringify(options.body) : undefined,
    });
  } catch {
    throw new ApiError('NETWORK_ERROR', messageForCode('NETWORK_ERROR'), 0);
  }

  if (response.status === 204 || response.status === 202) {
    return undefined as T;
  }

  const raw = await response.text();
  const data = raw ? (JSON.parse(raw) as unknown) : undefined;

  if (!response.ok) {
    const body = (data ?? {}) as ApiErrorBody;
    const code = body.error?.code ?? 'UNKNOWN';
    throw new ApiError(code, messageForCode(code, body.error?.message), response.status);
  }

  return data as T;
}
