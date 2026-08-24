import { useMemo } from 'react';
import { MaterialIcons } from '@expo/vector-icons';
import { Link, router } from 'expo-router';
import { FlatList, Pressable, RefreshControl, StyleSheet, Text, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import Animated, { FadeInUp } from 'react-native-reanimated';

import { Card } from '@/components/Card';
import { EmptyState, ErrorState, LoadingState } from '@/components/AsyncState';
import { AnimatedPressable } from '@/components/AnimatedPressable';
import { useFeed } from '@/hooks/useFeed';
import { radii, spacing, typography, type ThemeColors } from '@/theme/tokens';
import { useTheme } from '@/theme/ThemeContext';
import { TAB_BAR_CLEARANCE } from '@/theme/layout';
import { estimateReadTime, formatRelativeDay, truncate } from '@/utils/format';
import type { FeedItem } from '@/api/types';

export default function TodayScreen() {
  const { colors } = useTheme();
  const styles = useMemo(() => createStyles(colors), [colors]);
  const feed = useFeed();
  const unreadItems = feed.data?.filter((item) => !item.read) ?? [];

  return (
    <SafeAreaView style={styles.root} edges={['top', 'left', 'right']}>
      <View style={styles.header}>
        <View style={styles.headerLeft}>
          <Text style={styles.eyebrow}>YOUR DAILY BRIEFING</Text>
          <Text style={styles.title}>Today</Text>
        </View>
        <Pressable
          onPress={() => router.push('/notifications')}
          accessibilityRole="button"
          accessibilityLabel="Notifications"
          style={styles.bellButton}
        >
          <MaterialIcons name="notifications-none" size={22} color={colors.onSurface} />
          {unreadItems.length > 0 ? <View style={styles.bellDot} /> : null}
        </Pressable>
      </View>

      {feed.isPending ? <LoadingState label="Loading today's digest…" /> : null}

      {feed.isError ? (
        <ErrorState
          title="Couldn't load your digest"
          message={feed.error instanceof Error ? feed.error.message : undefined}
          onRetry={() => feed.refetch()}
        />
      ) : null}

      {feed.isSuccess && feed.data.length === 0 ? (
        <EmptyState
          icon="check-circle"
          title="You're all caught up for today!"
          message="New content from your subscriptions lands here as it's delivered."
        />
      ) : null}

      {feed.isSuccess && feed.data.length > 0 ? (
        <FlatList
          data={feed.data}
          keyExtractor={(item) => item.id}
          contentContainerStyle={styles.list}
          renderItem={({ item, index }) => (
            <Animated.View entering={FadeInUp.delay(index * 60).duration(300)}>
              <FeedCard item={item} />
            </Animated.View>
          )}
          refreshControl={
            <RefreshControl refreshing={feed.isRefetching} onRefresh={() => feed.refetch()} tintColor={colors.primary} />
          }
        />
      ) : null}
    </SafeAreaView>
  );
}

function FeedCard({ item }: { item: FeedItem }) {
  const { colors } = useTheme();
  const styles = useMemo(() => createStyles(colors), [colors]);
  return (
    <Link href={{ pathname: '/content/[id]', params: { id: item.id } }} asChild>
      <AnimatedPressable accessibilityRole="button">
        <Card style={styles.card} padded={false}>
          <View style={styles.cardInner}>
            <View style={styles.cardTopRow}>
              <View style={styles.publisherPill}>
                <MaterialIcons name="newspaper" size={14} color={colors.primary} />
                <Text style={styles.publisherText}>{item.product_name.toUpperCase()}</Text>
              </View>
              {!item.read ? (
                <View style={styles.newBadge}>
                  <Text style={styles.newBadgeText}>NEW</Text>
                </View>
              ) : null}
            </View>

            <Text style={styles.cardTitle}>{item.title}</Text>
            <Text style={styles.cardBody}>{truncate(item.body, 140)}</Text>

            <View style={styles.cardFooter}>
              <View style={styles.cardMetaRow}>
                <MaterialIcons name="schedule" size={14} color={colors.outline} />
                <Text style={styles.cardMeta}>{estimateReadTime(item.body)}</Text>
                <Text style={styles.cardMetaDot}>•</Text>
                <Text style={styles.cardMeta}>{formatRelativeDay(item.published_at)}</Text>
              </View>
              {item.content_kind === 'LINK' ? (
                <View style={styles.linkIndicator}>
                  <Text style={styles.linkText}>Read article</Text>
                  <MaterialIcons name="arrow-forward" size={14} color={colors.primary} />
                </View>
              ) : (
                <MaterialIcons name="chevron-right" size={20} color={colors.outline} />
              )}
            </View>
          </View>
        </Card>
      </AnimatedPressable>
    </Link>
  );
}

const createStyles = (colors: ThemeColors) => StyleSheet.create({
  root: {
    flex: 1,
    backgroundColor: colors.background,
  },
  header: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    paddingHorizontal: spacing.containerMargin,
    paddingTop: spacing.stackMd,
    paddingBottom: spacing.stackSm,
  },
  headerLeft: {
    gap: 2,
  },
  eyebrow: {
    ...typography.labelSm,
    fontSize: 11,
    fontWeight: '700',
    letterSpacing: 0.8,
    color: colors.primary,
  },
  title: {
    ...typography.headlineLgMobile,
    fontSize: 30,
    fontWeight: '800',
    letterSpacing: -0.5,
    color: colors.onSurface,
  },
  bellButton: {
    width: 42,
    height: 42,
    borderRadius: 21,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.surfaceContainerLowest,
    borderWidth: 1,
    borderColor: colors.cardBorder,
    position: 'relative',
  },
  bellDot: {
    position: 'absolute',
    top: 10,
    right: 10,
    width: 8,
    height: 8,
    borderRadius: 4,
    backgroundColor: colors.error,
  },
  list: {
    paddingHorizontal: spacing.containerMargin,
    paddingTop: spacing.stackMd,
    paddingBottom: TAB_BAR_CLEARANCE + 16,
    gap: 12,
  },
  card: {
    backgroundColor: colors.surfaceContainerLowest,
    overflow: 'hidden',
  },
  cardInner: {
    padding: 16,
    gap: 8,
  },
  cardTopRow: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
  },
  publisherPill: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 5,
  },
  publisherText: {
    ...typography.labelSm,
    fontSize: 11,
    fontWeight: '800',
    letterSpacing: 0.6,
    color: colors.primary,
  },
  newBadge: {
    paddingHorizontal: 7,
    paddingVertical: 2,
    borderRadius: radii.full,
    backgroundColor: colors.secondaryContainer,
    borderWidth: 1,
    borderColor: 'rgba(245, 158, 11, 0.3)',
  },
  newBadgeText: {
    ...typography.labelSm,
    fontSize: 10,
    fontWeight: '800',
    letterSpacing: 0.5,
    color: colors.secondary,
  },
  cardTitle: {
    ...typography.headlineMd,
    fontSize: 17,
    lineHeight: 23,
    fontWeight: '700',
    color: colors.onSurface,
  },
  cardBody: {
    ...typography.bodyMd,
    fontSize: 14,
    lineHeight: 20,
    color: colors.onSurfaceVariant,
  },
  cardFooter: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginTop: 4,
    paddingTop: 8,
    borderTopWidth: 1,
    borderTopColor: colors.cardBorder,
  },
  cardMetaRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 5,
  },
  cardMeta: {
    ...typography.labelSm,
    fontSize: 12,
    color: colors.outline,
  },
  cardMetaDot: {
    color: colors.outline,
    fontSize: 12,
  },
  linkIndicator: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 4,
  },
  linkText: {
    ...typography.labelSm,
    fontSize: 12,
    fontWeight: '600',
    color: colors.primary,
  },
});
