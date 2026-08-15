import { useEffect } from 'react';
import { MaterialIcons } from '@expo/vector-icons';
import { router, useLocalSearchParams } from 'expo-router';
import { Linking, Platform, Pressable, ScrollView, Share, StyleSheet, Text, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { Button } from '@/components/Button';
import { Chip } from '@/components/Chip';
import { ErrorState, LoadingState } from '@/components/AsyncState';
import { useFeedItem, useMarkFeedItemRead } from '@/hooks/useFeed';
import { colors, spacing, typography } from '@/theme/tokens';
import { estimateReadTime, formatRelativeDay, parseHttpUrl } from '@/utils/format';

export default function ContentDetailScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const { data: item, isPending, isError, error, refetch } = useFeedItem(id);
  const markRead = useMarkFeedItemRead();

  useEffect(() => {
    if (item && !item.read) {
      markRead.mutate(item.id);
    }
    // Only fire once per loaded item.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [item?.id]);

  async function handleShare() {
    if (!item) return;
    try {
      await Share.share({ message: `${item.title}\n\n${item.body}` });
    } catch {
      // Share sheet dismissal/errors are non-fatal; no feedback needed.
    }
  }

  // Only http/https destinations are safe to hand to Linking/window.open;
  // anything else (missing link_url, custom scheme, malformed string)
  // renders no CTA at all rather than a broken button.
  const linkUrl =
    item?.content_kind === 'LINK' && item.link_url && parseHttpUrl(item.link_url) ? item.link_url : null;
  const linkHost = linkUrl ? parseHttpUrl(linkUrl)?.hostname : null;

  function handleOpenLink() {
    if (!linkUrl) return;
    if (Platform.OS === 'web') {
      window.open(linkUrl, '_blank', 'noopener');
    } else {
      Linking.openURL(linkUrl);
    }
  }

  return (
    <SafeAreaView style={styles.root} edges={['top', 'left', 'right']}>
      <View style={styles.header}>
        <Pressable onPress={() => router.back()} accessibilityRole="button" accessibilityLabel="Go back" style={styles.headerButton}>
          <MaterialIcons name="arrow-back" size={22} color={colors.onSurfaceVariant} />
        </Pressable>
        <Pressable
          onPress={handleShare}
          accessibilityRole="button"
          accessibilityLabel="Share"
          style={styles.headerButton}
          disabled={!item}
        >
          <MaterialIcons name="share" size={22} color={item ? colors.onSurfaceVariant : colors.outline} />
        </Pressable>
      </View>

      {isPending ? <LoadingState label="Loading article…" /> : null}
      {isError ? (
        <ErrorState
          title="Couldn't load this article"
          message={error instanceof Error ? error.message : undefined}
          onRetry={refetch}
        />
      ) : null}

      {item ? (
        <ScrollView contentContainerStyle={styles.scrollContent}>
          <Chip label={item.product_name.toUpperCase()} tone="primary" />
          <Text style={styles.title}>{item.title}</Text>
          <View style={styles.metaRow}>
            <MaterialIcons name="schedule" size={14} color={colors.onSurfaceVariant} />
            <Text style={styles.metaText}>{estimateReadTime(item.body)}</Text>
            <Text style={styles.metaDot}>•</Text>
            <Text style={styles.metaText}>{formatRelativeDay(item.published_at)}</Text>
          </View>
          <Text style={styles.body}>{item.body}</Text>
          {linkUrl ? (
            <View style={styles.linkCta}>
              <Button label={item.cta_label || 'Open link'} onPress={handleOpenLink} style={styles.linkCtaButton} />
              {linkHost ? <Text style={styles.linkHost}>Opens {linkHost}</Text> : null}
            </View>
          ) : null}
        </ScrollView>
      ) : null}
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  root: {
    flex: 1,
    backgroundColor: colors.surface,
  },
  header: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    paddingHorizontal: spacing.stackMd,
    height: 56,
  },
  headerButton: {
    width: 40,
    height: 40,
    alignItems: 'center',
    justifyContent: 'center',
  },
  scrollContent: {
    paddingHorizontal: spacing.containerMargin,
    paddingBottom: spacing.sectionGap,
    gap: spacing.stackMd,
  },
  title: {
    ...typography.headlineLgMobile,
    color: colors.onSurface,
  },
  metaRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.stackSm,
  },
  metaText: {
    ...typography.labelSm,
    color: colors.onSurfaceVariant,
  },
  metaDot: {
    color: colors.onSurfaceVariant,
  },
  body: {
    ...typography.bodyLg,
    color: colors.onSurfaceVariant,
    marginTop: spacing.stackSm,
  },
  linkCta: {
    width: '100%',
    marginTop: spacing.stackSm,
    alignItems: 'center',
    gap: spacing.stackSm,
  },
  linkCtaButton: {
    width: '100%',
  },
  linkHost: {
    ...typography.labelSm,
    color: colors.onSurfaceVariant,
  },
});
