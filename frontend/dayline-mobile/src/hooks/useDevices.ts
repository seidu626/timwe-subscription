import { useMutation } from '@tanstack/react-query';

import { registerDevice, setNotificationPref } from '@/api/devices';
import type { DevicePlatform, NotificationChannel } from '@/api/types';

export function useRegisterDevice() {
  return useMutation({
    mutationFn: ({ fcmToken, platform }: { fcmToken: string; platform: DevicePlatform }) =>
      registerDevice(fcmToken, platform),
  });
}

export function useSetNotificationPref() {
  return useMutation({
    mutationFn: ({ productSlug, channel }: { productSlug: string; channel: NotificationChannel }) =>
      setNotificationPref(productSlug, channel),
  });
}
