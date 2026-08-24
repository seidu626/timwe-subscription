import { apiRequest } from './client';
import type { DevicePlatform, NotificationChannel, NotificationPrefsResponse } from './types';

export function registerDevice(fcmToken: string, platform: DevicePlatform): Promise<void> {
  return apiRequest<void>('/v1/app/devices', {
    method: 'POST',
    body: { fcm_token: fcmToken, platform },
  });
}

export function getNotificationPrefs(): Promise<NotificationPrefsResponse> {
  return apiRequest<NotificationPrefsResponse>('/v1/app/notification-prefs', { method: 'GET' });
}

export function setNotificationPref(
  productSlug: string,
  channel: NotificationChannel,
): Promise<void> {
  return apiRequest<void>('/v1/app/notification-prefs', {
    method: 'PUT',
    body: { product_slug: productSlug, channel },
  });
}
