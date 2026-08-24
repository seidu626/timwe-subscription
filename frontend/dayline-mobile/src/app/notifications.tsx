import { useState , useMemo } from 'react';
import { MaterialIcons } from '@expo/vector-icons';
import { router } from 'expo-router';
import { Pressable, StyleSheet, Text, View } from 'react-native';

import { Card } from '@/components/Card';
import { EmptyState, ErrorState, LoadingState } from '@/components/AsyncState';
import { ScreenContainer } from '@/components/ScreenContainer';
import { useFeed } from '@/hooks/useFeed';
import { useNotificationPrefs, useSetNotificationPref } from '@/hooks/useDevices';
import { usePushRegistrationStatus } from '@/hooks/usePushRegistration';
import { useSubscriptions } from '@/hooks/useSubscriptions';
import { radii, spacing, typography, type ThemeColors } from '@/theme/tokens';
import { useTheme } from '@/theme/ThemeContext';
import { formatRelativeDay } from '@/utils/format';
import type { NotificationChannel } from '@/api/types';

const CHANNELS: { value: NotificationChannel; label: string }[] = [
  { value: 'PUSH', label: 'Push' },
  { value: 'SMS', label: 'SMS' },
  { value: 'BOTH', label: 'Both' },
];

export default function NotificationsScreen() {
  const { colors } = useTheme();
  const styles = useMemo(() => createStyles(colors), [colors]);
  const subscriptions = useSubscriptions();
  const feed = useFeed();
  const setPref = useSetNotificationPref();
  const serverPrefs = useNotificationPrefs();
  const pushStatus = usePushRegistrationStatus();
  // Server prefs are the base; local overrides keep taps instant while the
  // PUT is in flight. Products with no stored row default to "Both".
  const [prefOverrides, setPrefOverrides] = useState<Record<string, NotificationChannel>>({});
  // Prefs only apply to active subscriptions; cancelled/failed rows must not
  // leave the section silently blank.
  const activeSubscriptions = subscriptions.data?.filter((s) => s.status === 'ACTIVE') ?? [];

  function selectChannel(productSlug: string, channel: NotificationChannel) {
    setPrefOverrides((prev) => ({ ...prev, [productSlug]: channel }));
    setPref.mutate({ productSlug, channel });
  }

  return (
    <ScreenContainer scroll>
      <View style={styles.header}>
        <Pressable onPress={() => router.back()} accessibilityRole="button" accessibilityLabel="Go back" style={styles.headerButton}>
          <MaterialIcons name="arrow-back" size={22} color={colors.onSurfaceVariant} />
        </Pressable>
        <Text style={styles.headerTitle}>Notifications</Text>
        <View style={styles.headerButton} />
      </View>

      {pushStatus.state === 'denied' || pushStatus.state === 'failed' ? (
        <View style={styles.pushBanner}>
          <MaterialIcons name="notifications-off" size={20} color={colors.error} />
          <View style={styles.pushBannerTextGroup}>
            <Text style={styles.pushBannerTitle}>
              {pushStatus.state === 'denied' ? 'Push notifications are off' : "Push notifications aren't set up"}
            </Text>
            <Text style={styles.pushBannerBody}>
              {pushStatus.state === 'denied'
                ? 'Allow notifications for Dayline in your system settings, then reopen the app. SMS delivery still works.'
                : `This device could not register for push (${pushStatus.detail}). SMS delivery still works.`}
            </Text>
          </View>
        </View>
      ) : null}

      <Text style={styles.sectionTitle}>Delivery preferences</Text>

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
          title="No active subscriptions"
          message="Subscribe to a product to manage its delivery preferences."
        />
      ) : null}

      <View style={styles.prefList}>
        {activeSubscriptions.map((subscription) => {
            const current =
              prefOverrides[subscription.product_slug] ??
              serverPrefs.data?.[subscription.product_slug] ??
              'BOTH';
            return (
              <Card key={subscription.ref} style={styles.prefCard}>
                <Text style={styles.prefProductName} numberOfLines={2} ellipsizeMode="tail">
                  {subscription.product_name}
                </Text>
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
                        <Text style={[styles.channelPillText, active && styles.channelPillTextActive]}>
                          {channel.label}
                        </Text>
                      </Pressable>
                    );
                  })}
                </View>
              </Card>
            );
          })}
      </View>

      <Text style={[styles.sectionTitle, styles.historyTitle]}>Delivery history</Text>

      {feed.isPending ? <LoadingState label="Loading history…" /> : null}
      {feed.isError ? (
        <ErrorState
          title="Couldn't load delivery history"
          message={feed.error instanceof Error ? feed.error.message : undefined}
          onRetry={() => feed.refetch()}
        />
      ) : null}
      {feed.isSuccess && feed.data.length === 0 ? (
        <EmptyState icon="history" title="No deliveries yet" message="Delivered content will be listed here." />
      ) : null}

      <View style={styles.historyList}>
        {feed.data?.map((item) => (
          <View key={item.id} style={styles.historyRow}>
            <MaterialIcons name="check-circle" size={16} color={colors.primary} />
            <View style={styles.historyTextGroup}>
              <Text style={styles.historyItemTitle} numberOfLines={2} ellipsizeMode="tail">
                {item.title}
              </Text>
              <Text style={styles.historyMeta} numberOfLines={1} ellipsizeMode="tail">
                {item.product_name} • {formatRelativeDay(item.published_at)}
              </Text>
            </View>
            {item.content_kind === 'LINK' ? (
              <MaterialIcons name="open-in-new" size={14} color={colors.onSurfaceVariant} style={styles.linkGlyph} />
            ) : null}
          </View>
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
    width: 40,
    height: 40,
    alignItems: 'center',
    justifyContent: 'center',
  },
  headerTitle: {
    ...typography.headlineMd,
    fontSize: 18,
    color: colors.primary,
  },
  pushBanner: {
    flexDirection: 'row',
    gap: spacing.stackSm,
    alignItems: 'flex-start',
    backgroundColor: 'rgba(186,26,26,0.08)',
    borderRadius: radii.md,
    padding: spacing.stackMd,
    marginBottom: spacing.stackLg,
  },
  pushBannerTextGroup: {
    flex: 1,
    minWidth: 0,
    gap: 2,
  },
  pushBannerTitle: {
    ...typography.labelMd,
    color: colors.error,
  },
  pushBannerBody: {
    ...typography.labelSm,
    color: colors.onSurfaceVariant,
    lineHeight: 18,
  },
  sectionTitle: {
    ...typography.headlineMd,
    color: colors.onSurface,
    marginBottom: spacing.stackMd,
  },
  historyTitle: {
    marginTop: spacing.sectionGap - spacing.stackLg,
  },
  prefList: {
    gap: spacing.stackMd,
  },
  prefCard: {
    gap: spacing.stackMd,
  },
  prefProductName: {
    ...typography.headlineMd,
    fontSize: 16,
    color: colors.onSurface,
  },
  channelRow: {
    flexDirection: 'row',
    gap: spacing.stackSm,
  },
  channelPill: {
    paddingHorizontal: spacing.stackMd,
    paddingVertical: 8,
    borderRadius: radii.full,
    borderWidth: 1,
    borderColor: colors.outlineVariant,
  },
  channelPillActive: {
    backgroundColor: colors.primary,
    borderColor: colors.primary,
  },
  channelPillText: {
    ...typography.labelSm,
    color: colors.onSurfaceVariant,
  },
  channelPillTextActive: {
    color: colors.onPrimary,
  },
  historyList: {
    gap: spacing.stackMd,
  },
  historyRow: {
    flexDirection: 'row',
    gap: spacing.stackSm,
    alignItems: 'flex-start',
  },
  historyTextGroup: {
    flex: 1,
    minWidth: 0,
    gap: 2,
  },
  historyItemTitle: {
    ...typography.bodyMd,
    color: colors.onSurface,
  },
  historyMeta: {
    ...typography.labelSm,
    color: colors.onSurfaceVariant,
  },
  linkGlyph: {
    flexShrink: 0,
    marginTop: 2,
  },
});
