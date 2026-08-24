import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { getNotificationPrefs, registerDevice, setNotificationPref } from '@/api/devices';
import type { DevicePlatform, NotificationChannel, NotificationPrefsResponse } from '@/api/types';
import { queryKeys } from './queryKeys';

export function useRegisterDevice() {
  return useMutation({
    mutationFn: ({ fcmToken, platform }: { fcmToken: string; platform: DevicePlatform }) =>
      registerDevice(fcmToken, platform),
  });
}

export function useNotificationPrefs() {
  return useQuery({
    queryKey: queryKeys.notificationPrefs,
    queryFn: () => getNotificationPrefs(),
    // Collapse the wire list into a slug -> channel map; products with no
    // stored row are absent and the screen falls back to its BOTH default.
    select: (data) =>
      Object.fromEntries(data.prefs.map((pref) => [pref.product_slug, pref.channel])) as Record<
        string,
        NotificationChannel
      >,
  });
}

export function useSetNotificationPref() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ productSlug, channel }: { productSlug: string; channel: NotificationChannel }) =>
      setNotificationPref(productSlug, channel),
    onSuccess: (_data, { productSlug, channel }) => {
      // Write the confirmed pref straight into the cache so the screen stays
      // consistent without an extra round trip.
      queryClient.setQueryData<NotificationPrefsResponse>(queryKeys.notificationPrefs, (prev) => {
        const others = prev?.prefs.filter((pref) => pref.product_slug !== productSlug) ?? [];
        return { prefs: [...others, { product_slug: productSlug, channel }] };
      });
    },
  });
}
