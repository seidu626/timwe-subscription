import * as SecureStore from '@/utils/secureStorage';

const SESSION_KEY = 'dayline_app_session';

export interface StoredSession {
  token: string;
  msisdn: string;
  tenant: string;
  expiresAt: number; // epoch ms
}

export async function readSession(): Promise<StoredSession | null> {
  const raw = await SecureStore.getItemAsync(SESSION_KEY);
  if (!raw) return null;
  try {
    const parsed = JSON.parse(raw) as StoredSession;
    if (!parsed.token || !parsed.expiresAt) return null;
    if (parsed.expiresAt <= Date.now()) {
      await clearSession();
      return null;
    }
    return parsed;
  } catch {
    return null;
  }
}

export async function writeSession(session: StoredSession): Promise<void> {
  await SecureStore.setItemAsync(SESSION_KEY, JSON.stringify(session));
}

export async function clearSession(): Promise<void> {
  await SecureStore.deleteItemAsync(SESSION_KEY);
}

export async function readToken(): Promise<string | null> {
  const session = await readSession();
  return session?.token ?? null;
}
