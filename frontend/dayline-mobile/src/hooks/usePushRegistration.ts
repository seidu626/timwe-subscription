import { useEffect, useSyncExternalStore } from 'react';
import * as Notifications from 'expo-notifications';
import { Platform } from 'react-native';

import { useAuth } from '@/context/AuthContext';
import { useRegisterDevice } from './useDevices';

export type PushRegistrationStatus =
  | { state: 'idle' }
  | { state: 'registered' }
  | { state: 'denied' }
  | { state: 'failed'; detail: string };

// Module-level store so the registration outcome (which happens once at
// sign-in, mounted in the tab layout) is visible to any screen that wants to
// explain why push delivery is off, without threading context providers.
let currentStatus: PushRegistrationStatus = { state: 'idle' };
const listeners = new Set<() => void>();

function setStatus(next: PushRegistrationStatus) {
  currentStatus = next;
  listeners.forEach((listener) => listener());
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

export function usePushRegistrationStatus(): PushRegistrationStatus {
  return useSyncExternalStore(subscribe, () => currentStatus, () => currentStatus);
}

/**
 * Registers this device's push token with the backend once signed in. The
 * contract's device field is named `fcm_token`; on iOS `getDevicePushTokenAsync`
 * returns the native APNs token rather than an FCM token unless the Firebase
 * iOS config is present (see docs/dayline-push-setup.md). Registration is
 * best-effort and never blocks app usage, but the outcome is recorded in the
 * status store above so the Notifications screen can surface failures.
 */
export function usePushRegistration() {
  const { status } = useAuth();
  const registerDevice = useRegisterDevice();

  useEffect(() => {
    // Web has no device push token; leaving the store 'idle' keeps the
    // Notifications screen banner from flagging an expected non-registration.
    if (status !== 'signedIn' || Platform.OS === 'web') return;
    let cancelled = false;

    async function register() {
      try {
        const current = await Notifications.getPermissionsAsync();
        let finalStatus = current.status;
        if (finalStatus !== 'granted') {
          const requested = await Notifications.requestPermissionsAsync();
          finalStatus = requested.status;
        }
        if (cancelled) return;
        if (finalStatus !== 'granted') {
          setStatus({ state: 'denied' });
          return;
        }

        const tokenResponse = await Notifications.getDevicePushTokenAsync();
        if (cancelled) return;

        await registerDevice.mutateAsync({
          fcmToken: tokenResponse.data,
          platform: Platform.OS === 'ios' ? 'ios' : 'android',
        });
        if (cancelled) return;
        setStatus({ state: 'registered' });
      } catch (err) {
        if (cancelled) return;
        setStatus({
          state: 'failed',
          detail: err instanceof Error ? err.message : 'Unknown error',
        });
      }
    }

    register();
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [status]);
}
