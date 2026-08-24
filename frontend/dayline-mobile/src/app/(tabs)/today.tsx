import { useMemo } from 'react';
import { MaterialIcons } from '@expo/vector-icons';
import { Link, router } from 'expo-router';
import { FlatList, Pressable, RefreshControl, StyleSheet, Text, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { Card } from '@/components/Card';
import { Chip } from '@/components/Chip';
import { EmptyState, ErrorState, LoadingState } from '@/components/AsyncState';
import { useFeed } from '@/hooks/useFeed';
import { spacing, typography, type ThemeColors } from '@/theme/tokens';
import { useTheme } from '@/theme/ThemeContext';
import { TAB_BAR_CLEARANCE } from '@/theme/layout';
import { estimateReadTime, formatRelativeDay, truncate } from '@/utils/format';
import type { FeedItem } from '@/api/types';

export default function TodayScreen() {
  const { colors } = useTheme();
  const styles = useMemo(() => createStyles(colors), [colors]);
  const feed = useFeed();

  return (
    <SafeAreaView style={styles.root} edges={['top', 'left', 'right']}>
      <View style={styles.header}>
        <View>
          <Text style={styles.eyebrow}>Today</Text>
          <Text style={styles.title}>Your daily digest</Text>
        </View>
        <Pressable
          onPress={() => router.push('/notifications')}
          accessibilityRole="button"
          accessibilityLabel="Notifications"
          style={styles.bellButton}
        >
          <MaterialIcons name="notifications" size={24} color={colors.primary} />
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
          renderItem={({ item }) => <FeedCard item={item} />}
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
      <Pressable accessibilityRole="button">
        <Card style={styles.card}>
          <View style={styles.cardTopRow}>
            <Chip label={item.product_name.toUpperCase()} tone="primary" />
            {!item.read ? <Chip label="NEW" tone="accent" /> : null}
          </View>
          <Text style={styles.cardTitle}>{item.title}</Text>
          <Text style={styles.cardBody}>{truncate(item.body, 160)}</Text>
          <View style={styles.cardMetaRow}>
            <MaterialIcons name="schedule" size={14} color={colors.onSurfaceVariant} />
            <Text style={styles.cardMeta}>{estimateReadTime(item.body)}</Text>
            <Text style={styles.cardMetaDot}>•</Text>
            <Text style={styles.cardMeta}>{formatRelativeDay(item.published_at)}</Text>
            {item.content_kind === 'LINK' ? (
              <MaterialIcons name="open-in-new" size={14} color={colors.onSurfaceVariant} style={styles.linkGlyph} />
            ) : null}
          </View>
        </Card>
      </Pressable>
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
  },
  eyebrow: {
    ...typography.labelSm,
    color: colors.onSurfaceVariant,
  },
  title: {
    ...typography.headlineLgMobile,
    color: colors.primary,
  },
  bellButton: {
    width: 40,
    height: 40,
    borderRadius: 20,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.surfaceContainerLow,
  },
  list: {
    paddingHorizontal: spacing.containerMargin,
    paddingTop: spacing.stackLg,
    paddingBottom: TAB_BAR_CLEARANCE,
    gap: spacing.stackMd,
  },
  card: {
    gap: spacing.stackSm,
  },
  cardTopRow: {
    flexDirection: 'row',
    gap: spacing.stackSm,
  },
  cardTitle: {
    ...typography.headlineMd,
    fontSize: 20,
    lineHeight: 26,
    color: colors.onSurface,
  },
  cardBody: {
    ...typography.bodyMd,
    color: colors.onSurfaceVariant,
  },
  cardMetaRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.stackSm,
    marginTop: spacing.stackSm,
  },
  cardMeta: {
    ...typography.labelSm,
    color: colors.onSurfaceVariant,
  },
  cardMetaDot: {
    color: colors.onSurfaceVariant,
  },
  linkGlyph: {
    flexShrink: 0,
  },
});
