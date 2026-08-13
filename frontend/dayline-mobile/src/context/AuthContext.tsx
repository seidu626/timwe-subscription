import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';

import { TENANT_KEY } from '@/config';
import { clearSession, readSession, writeSession } from '@/api/session';

type AuthStatus = 'loading' | 'signedOut' | 'signedIn';

interface AuthContextValue {
  status: AuthStatus;
  msisdn: string | null;
  tenant: string;
  signIn: (params: { token: string; msisdn: string; expiresInSeconds: number }) => Promise<void>;
  signOut: () => Promise<void>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [status, setStatus] = useState<AuthStatus>('loading');
  const [msisdn, setMsisdn] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    readSession().then((session) => {
      if (cancelled) return;
      if (session) {
        setMsisdn(session.msisdn);
        setStatus('signedIn');
      } else {
        setStatus('signedOut');
      }
    });
    return () => {
      cancelled = true;
    };
  }, []);

  const signIn = useCallback(
    async ({ token, msisdn: signedInMsisdn, expiresInSeconds }: { token: string; msisdn: string; expiresInSeconds: number }) => {
      await writeSession({
        token,
        msisdn: signedInMsisdn,
        tenant: TENANT_KEY,
        expiresAt: Date.now() + expiresInSeconds * 1000,
      });
      setMsisdn(signedInMsisdn);
      setStatus('signedIn');
    },
    [],
  );

  const signOut = useCallback(async () => {
    await clearSession();
    setMsisdn(null);
    setStatus('signedOut');
  }, []);

  const value = useMemo<AuthContextValue>(
    () => ({ status, msisdn, tenant: TENANT_KEY, signIn, signOut }),
    [status, msisdn, signIn, signOut],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return ctx;
}
