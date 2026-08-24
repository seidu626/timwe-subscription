import { useEffect, useMemo } from 'react';
import { MaterialIcons } from '@expo/vector-icons';
import { router, useLocalSearchParams } from 'expo-router';
import { Linking, Platform, Pressable, ScrollView, Share, StyleSheet, Text, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { Button } from '@/components/Button';
import { Card } from '@/components/Card';
import { ErrorState, LoadingState } from '@/components/AsyncState';
import { useFeedItem, useMarkFeedItemRead } from '@/hooks/useFeed';
import { radii, spacing, typography, type ThemeColors } from '@/theme/tokens';
import { useTheme } from '@/theme/ThemeContext';
import { estimateReadTime, formatRelativeDay, parseHttpUrl } from '@/utils/format';

export default function ContentDetailScreen() {
  const { colors } = useTheme();
  const styles = useMemo(() => createStyles(colors), [colors]);
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
          <View style={styles.iconCircle}>
            <MaterialIcons name="arrow-back" size={20} color={colors.onSurface} />
          </View>
        </Pressable>
        <Pressable
          onPress={handleShare}
          accessibilityRole="button"
          accessibilityLabel="Share"
          style={styles.headerButton}
          disabled={!item}
        >
          <View style={styles.iconCircle}>
            <MaterialIcons name="share" size={20} color={item ? colors.onSurface : colors.outline} />
          </View>
        </Pressable>
      </View>

      {isPending ? <LoadingState label="Loading briefing…" /> : null}
      {isError ? (
        <ErrorState
          title="Couldn't load this briefing"
          message={error instanceof Error ? error.message : undefined}
          onRetry={refetch}
        />
      ) : null}

      {item ? (
        <ScrollView contentContainerStyle={styles.scrollContent} showsVerticalScrollIndicator={false}>
          <View style={styles.publicationHeader}>
            <View style={styles.publicationBadge}>
              <MaterialIcons name="newspaper" size={16} color={colors.primary} />
              <Text style={styles.publicationText}>{item.product_name.toUpperCase()}</Text>
            </View>
            <View style={styles.metaBadge}>
              <MaterialIcons name="schedule" size={14} color={colors.outline} />
              <Text style={styles.metaText}>{estimateReadTime(item.body)}</Text>
              <Text style={styles.metaDot}>•</Text>
              <Text style={styles.metaText}>{formatRelativeDay(item.published_at)}</Text>
            </View>
          </View>

          <Text style={styles.title}>{item.title}</Text>

          <View style={styles.divider} />

          <Text style={styles.body}>{item.body}</Text>

          {linkUrl ? (
            <Card style={styles.linkCard} padded={false}>
              <View style={styles.linkCardInner}>
                <View style={styles.linkHeader}>
                  <MaterialIcons name="link" size={20} color={colors.primary} />
                  <Text style={styles.linkHeaderTitle}>Extended Resource</Text>
                </View>
                {linkHost ? <Text style={styles.linkHost}>Read full material on {linkHost}</Text> : null}
                <Button 
                  label={item.cta_label || 'Open External Resource'} 
                  onPress={handleOpenLink} 
                  icon={<MaterialIcons name="open-in-new" size={18} color={colors.onPrimary} />}
                  style={styles.linkCtaButton} 
                />
              </View>
            </Card>
          ) : null}
        </ScrollView>
      ) : null}
    </SafeAreaView>
  );
}

const createStyles = (colors: ThemeColors) => StyleSheet.create({
  root: {
    flex: 1,
    backgroundColor: colors.background,
  },
  header: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    paddingHorizontal: spacing.containerMargin,
    height: 56,
  },
  headerButton: {
    width: 40,
    height: 40,
    alignItems: 'center',
    justifyContent: 'center',
  },
  iconCircle: {
    width: 38,
    height: 38,
    borderRadius: 19,
    backgroundColor: colors.surfaceContainerLowest,
    borderWidth: 1,
    borderColor: colors.cardBorder,
    alignItems: 'center',
    justifyContent: 'center',
  },
  scrollContent: {
    paddingHorizontal: spacing.containerMargin,
    paddingTop: 16,
    paddingBottom: spacing.sectionGap + 24,
    gap: 16,
  },
  publicationHeader: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: 10,
    flexWrap: 'wrap',
  },
  publicationBadge: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 6,
    paddingHorizontal: 10,
    paddingVertical: 4,
    borderRadius: radii.sm,
    backgroundColor: colors.primarySoft,
    borderWidth: 1,
    borderColor: 'rgba(52, 211, 153, 0.2)',
  },
  publicationText: {
    ...typography.labelSm,
    fontSize: 11,
    fontWeight: '800',
    letterSpacing: 0.6,
    color: colors.primary,
  },
  metaBadge: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 5,
  },
  metaText: {
    ...typography.labelSm,
    fontSize: 12,
    color: colors.outline,
  },
  metaDot: {
    color: colors.outline,
    fontSize: 12,
  },
  title: {
    ...typography.headlineLgMobile,
    fontSize: 26,
    lineHeight: 34,
    fontWeight: '800',
    letterSpacing: -0.3,
    color: colors.onSurface,
  },
  divider: {
    height: 1,
    backgroundColor: colors.cardBorder,
    marginVertical: 4,
  },
  body: {
    ...typography.bodyLg,
    fontSize: 17,
    lineHeight: 28,
    color: colors.onSurface,
    letterSpacing: 0.1,
  },
  linkCard: {
    marginTop: 16,
    backgroundColor: colors.surfaceContainerLowest,
    borderColor: colors.cardBorder,
    overflow: 'hidden',
  },
  linkCardInner: {
    padding: 16,
    gap: 10,
  },
  linkHeader: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 6,
  },
  linkHeaderTitle: {
    ...typography.headlineMd,
    fontSize: 15,
    fontWeight: '700',
    color: colors.onSurface,
  },
  linkHost: {
    ...typography.labelSm,
    fontSize: 12,
    color: colors.onSurfaceVariant,
  },
  linkCtaButton: {
    width: '100%',
    marginTop: 4,
  },
});
