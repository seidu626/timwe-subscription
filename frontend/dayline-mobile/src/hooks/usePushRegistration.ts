import { useEffect } from 'react';
import * as Notifications from 'expo-notifications';
import { Platform } from 'react-native';

import { useAuth } from '@/context/AuthContext';
import { useRegisterDevice } from './useDevices';

/**
 * Registers this device's push token with the backend once signed in. The
 * contract's device field is named `fcm_token`; on iOS `getDevicePushTokenAsync`
 * returns the native APNs token rather than an FCM token (Expo does not proxy
 * APNs through FCM without additional native config) — documented deviation,
 * see result capsule.
 */
export function usePushRegistration() {
  const { status } = useAuth();
  const registerDevice = useRegisterDevice();

  useEffect(() => {
    if (status !== 'signedIn') return;
    let cancelled = false;

    async function register() {
      try {
        const current = await Notifications.getPermissionsAsync();
        let finalStatus = current.status;
        if (finalStatus !== 'granted') {
          const requested = await Notifications.requestPermissionsAsync();
          finalStatus = requested.status;
        }
        if (finalStatus !== 'granted' || cancelled) return;

        const tokenResponse = await Notifications.getDevicePushTokenAsync();
        if (cancelled) return;

        registerDevice.mutate({
          fcmToken: tokenResponse.data,
          platform: Platform.OS === 'ios' ? 'ios' : 'android',
        });
      } catch {
        // Push registration is best-effort (e.g. unsupported in the current
        // runtime/simulator); it must never block app usage.
      }
    }

    register();
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [status]);
}
