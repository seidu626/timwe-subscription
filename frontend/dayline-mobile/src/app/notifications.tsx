import { useState, useMemo } from 'react';
import { MaterialIcons } from '@expo/vector-icons';
import { router } from 'expo-router';
import { Pressable, StyleSheet, Text, View } from 'react-native';
import * as Haptics from 'expo-haptics';

import { Card } from '@/components/Card';
import { EmptyState, ErrorState, LoadingState } from '@/components/AsyncState';
import { ScreenContainer } from '@/components/ScreenContainer';
import { useFeed } from '@/hooks/useFeed';
import { useNotificationPrefs, useSetNotificationPref } from '@/hooks/useDevices';
import { usePushRegistrationStatus } from '@/hooks/usePushRegistration';
import { useSubscriptions } from '@/hooks/useSubscriptions';
import { radii, spacing, typography, type ThemeColors } from '@/theme/tokens';
import { useTheme } from '@/theme/ThemeContext';
import { formatProductName, formatRelativeDay } from '@/utils/format';
import type { NotificationChannel } from '@/api/types';

const CHANNELS: { value: NotificationChannel; label: string; icon: keyof typeof MaterialIcons.glyphMap }[] = [
  { value: 'PUSH', label: 'Push', icon: 'notifications' },
  { value: 'SMS', label: 'SMS', icon: 'sms' },
  { value: 'BOTH', label: 'Both', icon: 'all-inclusive' },
];

export default function NotificationsScreen() {
  const { colors } = useTheme();
  const styles = useMemo(() => createStyles(colors), [colors]);
  const subscriptions = useSubscriptions();
  const feed = useFeed();
  const setPref = useSetNotificationPref();
  const serverPrefs = useNotificationPrefs();
  const pushStatus = usePushRegistrationStatus();

  const [prefOverrides, setPrefOverrides] = useState<Record<string, NotificationChannel>>({});
  const activeSubscriptions = subscriptions.data?.filter((s) => s.status === 'ACTIVE') ?? [];

  function selectChannel(productSlug: string, channel: NotificationChannel) {
    Haptics.impactAsync(Haptics.ImpactFeedbackStyle.Light);
    setPrefOverrides((prev) => ({ ...prev, [productSlug]: channel }));
    setPref.mutate({ productSlug, channel });
  }

  return (
    <ScreenContainer scroll withTabBarClearance>
      {/* Header */}
      <View style={styles.header}>
        <Pressable onPress={() => router.back()} accessibilityRole="button" accessibilityLabel="Go back" style={styles.headerButton}>
          <View style={styles.iconCircle}>
            <MaterialIcons name="arrow-back" size={20} color={colors.onSurface} />
          </View>
        </Pressable>
        <Text style={styles.headerTitle}>Notifications</Text>
        <View style={styles.headerSpacer} />
      </View>

      {/* Push Permission Warning */}
      {pushStatus.state === 'denied' || pushStatus.state === 'failed' ? (
        <View style={styles.pushBanner}>
          <MaterialIcons name="notifications-off" size={22} color={colors.error} />
          <View style={styles.pushBannerTextGroup}>
            <Text style={styles.pushBannerTitle}>
              {pushStatus.state === 'denied' ? 'Push Notifications Disabled' : "Push Notifications Unavailable"}
            </Text>
            <Text style={styles.pushBannerBody}>
              {pushStatus.state === 'denied'
                ? 'Enable notifications in your system settings to receive instant morning updates. SMS delivery is active.'
                : `Device could not register for push. SMS delivery remains active.`}
            </Text>
          </View>
        </View>
      ) : null}

      <Text style={styles.sectionTitle}>Delivery Preferences</Text>

      {subscriptions.isPending ? <LoadingState label="Loading subscriptions…" /> : null}
      {subscriptions.isError ? (
        <ErrorState
          title="Couldn't load preferences"
          message={subscriptions.error instanceof Error ? subscriptions.error.message : undefined}
          onRetry={() => subscriptions.refetch()}
        />
      ) : null}
      {subscriptions.isSuccess && activeSubscriptions.length === 0 ? (
        <EmptyState
          icon="notifications-none"
          title="No active channels"
          message="Subscribe to a channel in Discover to configure delivery methods."
        />
      ) : null}

      <View style={styles.prefList}>
        {activeSubscriptions.map((subscription) => {
          const current =
            prefOverrides[subscription.product_slug] ??
            serverPrefs.data?.[subscription.product_slug] ??
            'BOTH';
          return (
            <Card key={subscription.ref} style={styles.prefCard} padded={false}>
              <View style={styles.prefCardInner}>
                <View style={styles.prefTopRow}>
                  <MaterialIcons name="newspaper" size={18} color={colors.primary} />
                  <Text style={styles.prefProductName} numberOfLines={1} ellipsizeMode="tail">
                    {formatProductName(subscription.product_name)}
                  </Text>
                </View>

                <View style={styles.channelRow}>
                  {CHANNELS.map((channel) => {
                    const active = current === channel.value;
                    return (
                      <Pressable
                        key={channel.value}
                        onPress={() => selectChannel(subscription.product_slug, channel.value)}
                        accessibilityRole="button"
                        accessibilityState={{ selected: active }}
                        style={[styles.channelPill, active && styles.channelPillActive]}
                      >
                        <MaterialIcons 
                          name={channel.icon} 
                          size={14} 
                          color={active ? colors.onPrimary : colors.onSurfaceVariant} 
                        />
                        <Text style={[styles.channelPillText, active && styles.channelPillTextActive]}>
                          {channel.label}
                        </Text>
                      </Pressable>
                    );
                  })}
                </View>
              </View>
            </Card>
          );
        })}
      </View>

      <Text style={[styles.sectionTitle, styles.historyTitle]}>Recent Deliveries</Text>

      {feed.isPending ? <LoadingState label="Loading history…" /> : null}
      {feed.isError ? (
        <ErrorState
          title="Couldn't load delivery history"
          message={feed.error instanceof Error ? feed.error.message : undefined}
          onRetry={() => feed.refetch()}
        />
      ) : null}
      {feed.isSuccess && feed.data.length === 0 ? (
        <EmptyState icon="history" title="No deliveries yet" message="Delivered daily briefings will be listed here." />
      ) : null}

      <View style={styles.historyList}>
        {feed.data?.map((item) => (
          <Card key={item.id} style={styles.historyCard} padded={false}>
            <View style={styles.historyRow}>
              <View style={styles.historyCheckCircle}>
                <MaterialIcons name="check" size={14} color={colors.primary} />
              </View>
              <View style={styles.historyTextGroup}>
                <Text style={styles.historyItemTitle} numberOfLines={2} ellipsizeMode="tail">
                  {item.title}
                </Text>
                <Text style={styles.historyMeta} numberOfLines={1} ellipsizeMode="tail">
                  {item.product_name} • {formatRelativeDay(item.published_at)}
                </Text>
              </View>
              {item.content_kind === 'LINK' ? (
                <MaterialIcons name="open-in-new" size={16} color={colors.outline} />
              ) : (
                <MaterialIcons name="chevron-right" size={20} color={colors.outline} />
              )}
            </View>
          </Card>
        ))}
      </View>
    </ScreenContainer>
  );
}

const createStyles = (colors: ThemeColors) => StyleSheet.create({
  header: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginBottom: spacing.stackLg,
  },
  headerButton: {
    width: 38,
    height: 38,
    alignItems: 'center',
    justifyContent: 'center',
  },
  iconCircle: {
    width: 36,
    height: 36,
    borderRadius: 18,
    backgroundColor: colors.surfaceContainerLowest,
    borderWidth: 1,
    borderColor: colors.cardBorder,
    alignItems: 'center',
    justifyContent: 'center',
  },
  headerSpacer: {
    width: 38,
  },
  headerTitle: {
    ...typography.headlineMd,
    fontSize: 18,
    fontWeight: '700',
    color: colors.onSurface,
    textAlign: 'center',
  },
  pushBanner: {
    flexDirection: 'row',
    gap: 12,
    alignItems: 'flex-start',
    backgroundColor: 'rgba(239, 68, 68, 0.08)',
    borderWidth: 1,
    borderColor: 'rgba(239, 68, 68, 0.25)',
    borderRadius: radii.md,
    padding: 14,
    marginBottom: 20,
  },
  pushBannerTextGroup: {
    flex: 1,
    minWidth: 0,
    gap: 2,
  },
  pushBannerTitle: {
    ...typography.labelMd,
    fontSize: 13,
    fontWeight: '700',
    color: colors.error,
  },
  pushBannerBody: {
    ...typography.labelSm,
    fontSize: 12,
    color: colors.onSurfaceVariant,
    lineHeight: 18,
  },
  sectionTitle: {
    ...typography.headlineMd,
    fontSize: 18,
    fontWeight: '700',
    color: colors.onSurface,
    marginBottom: 12,
  },
  historyTitle: {
    marginTop: 28,
  },
  prefList: {
    gap: 12,
  },
  prefCard: {
    backgroundColor: colors.surfaceContainerLowest,
    overflow: 'hidden',
  },
  prefCardInner: {
    padding: 16,
    gap: 12,
  },
  prefTopRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
  },
  prefProductName: {
    ...typography.headlineMd,
    fontSize: 15,
    fontWeight: '700',
    color: colors.onSurface,
    flex: 1,
  },
  channelRow: {
    flexDirection: 'row',
    gap: 8,
  },
  channelPill: {
    flex: 1,
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    gap: 6,
    paddingVertical: 8,
    borderRadius: radii.full,
    borderWidth: 1,
    borderColor: colors.cardBorder,
    backgroundColor: colors.surfaceContainerLow,
  },
  channelPillActive: {
    backgroundColor: colors.primary,
    borderColor: colors.primary,
  },
  channelPillText: {
    ...typography.labelSm,
    fontSize: 12,
    fontWeight: '600',
    color: colors.onSurfaceVariant,
  },
  channelPillTextActive: {
    color: colors.onPrimary,
    fontWeight: '700',
  },
  historyList: {
    gap: 10,
  },
  historyCard: {
    backgroundColor: colors.surfaceContainerLowest,
    overflow: 'hidden',
  },
  historyRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 12,
    padding: 14,
  },
  historyCheckCircle: {
    width: 28,
    height: 28,
    borderRadius: 14,
    backgroundColor: colors.primarySoft,
    alignItems: 'center',
    justifyContent: 'center',
    flexShrink: 0,
  },
  historyTextGroup: {
    flex: 1,
    minWidth: 0,
    gap: 2,
  },
  historyItemTitle: {
    ...typography.headlineMd,
    fontSize: 14,
    lineHeight: 19,
    fontWeight: '600',
    color: colors.onSurface,
  },
  historyMeta: {
    ...typography.labelSm,
    fontSize: 11,
    color: colors.outline,
  },
});
