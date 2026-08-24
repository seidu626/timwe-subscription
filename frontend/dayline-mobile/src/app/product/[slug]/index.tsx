import { useMemo } from 'react';
import { MaterialIcons } from '@expo/vector-icons';
import { Image } from 'expo-image';
import { router, useLocalSearchParams } from 'expo-router';
import { StyleSheet, Text, View } from 'react-native';
import { SafeAreaView, useSafeAreaInsets } from 'react-native-safe-area-context';
import Animated, { 
  useAnimatedScrollHandler, 
  useAnimatedStyle, 
  useSharedValue, 
  interpolate, 
  Extrapolation 
} from 'react-native-reanimated';
import * as Haptics from 'expo-haptics';

import { Button } from '@/components/Button';
import { Card } from '@/components/Card';
import { AnimatedPressable } from '@/components/AnimatedPressable';
import { ErrorState, LoadingState } from '@/components/AsyncState';
import { useSettings } from '@/context/SettingsContext';
import { useCatalogProduct } from '@/hooks/useCatalog';
import { radii, spacing, typography, type ThemeColors } from '@/theme/tokens';
import { useTheme } from '@/theme/ThemeContext';
import { formatBillingCycle, formatCurrency, formatProductName } from '@/utils/format';

const HERO_HEIGHT = 300;

export default function ProductDetailScreen() {
  const { colors } = useTheme();
  const styles = useMemo(() => createStyles(colors), [colors]);
  const { slug } = useLocalSearchParams<{ slug: string }>();
  const { isPending, isError, error, refetch, product } = useCatalogProduct(slug);
  const { dataSaverEnabled } = useSettings();
  const insets = useSafeAreaInsets();

  const scrollY = useSharedValue(0);

  const scrollHandler = useAnimatedScrollHandler({
    onScroll: (event) => {
      scrollY.value = event.contentOffset.y;
    },
  });

  const headerAnimatedStyle = useAnimatedStyle(() => {
    const opacity = interpolate(
      scrollY.value,
      [HERO_HEIGHT - 100, HERO_HEIGHT - 40],
      [0, 1],
      Extrapolation.CLAMP
    );
    return {
      opacity,
      backgroundColor: colors.surface,
    };
  });

  const heroAnimatedStyle = useAnimatedStyle(() => {
    const translateY = interpolate(
      scrollY.value,
      [-100, 0, HERO_HEIGHT],
      [-50, 0, HERO_HEIGHT * 0.5],
      Extrapolation.CLAMP
    );
    const scale = interpolate(
      scrollY.value,
      [-100, 0],
      [1.4, 1],
      Extrapolation.CLAMP
    );
    return {
      transform: [{ translateY }, { scale }],
    };
  });

  const handleSubscribePress = () => {
    Haptics.impactAsync(Haptics.ImpactFeedbackStyle.Medium);
    if (product) {
      router.push({ pathname: '/product/[slug]/confirm', params: { slug: product.slug } });
    }
  };

  return (
    <View style={styles.root}>
      {/* Sticky Blurred Nav Header on Scroll */}
      <Animated.View style={[styles.header, { paddingTop: insets.top }, headerAnimatedStyle]}>
        <Text style={styles.headerTitle} numberOfLines={1} ellipsizeMode="tail">
          {product?.name ? formatProductName(product.name) : 'Channel'}
        </Text>
      </Animated.View>

      {/* Floating Back Action */}
      <View style={[styles.headerControls, { top: insets.top + 8 }]}>
        <AnimatedPressable onPress={() => router.back()} accessibilityRole="button" accessibilityLabel="Go back" style={styles.headerButton}>
          <View style={styles.iconBackground}>
            <MaterialIcons name="arrow-back" size={20} color={colors.onSurface} />
          </View>
        </AnimatedPressable>
      </View>

      {isPending ? (
        <SafeAreaView style={styles.centered} edges={['top', 'left', 'right']}>
          <LoadingState label="Loading channel…" />
        </SafeAreaView>
      ) : null}
      
      {isError ? (
        <SafeAreaView style={styles.centered} edges={['top', 'left', 'right']}>
          <ErrorState
            title="Couldn't load this channel"
            message={error instanceof Error ? error.message : undefined}
            onRetry={refetch}
          />
        </SafeAreaView>
      ) : null}
      
      {!isPending && !isError && !product ? (
        <SafeAreaView style={styles.centered} edges={['top', 'left', 'right']}>
          <ErrorState title="Channel not found" message="This publication may no longer be available." />
        </SafeAreaView>
      ) : null}

      {product ? (
        <Animated.ScrollView 
          contentContainerStyle={styles.scrollContent}
          onScroll={scrollHandler}
          scrollEventThrottle={16}
          showsVerticalScrollIndicator={false}
        >
          {/* Cinematic Hero Media */}
          <Animated.View style={[styles.heroContainer, heroAnimatedStyle]}>
            {product.artwork_url && !dataSaverEnabled ? (
              <Animated.View sharedTransitionTag={`product-hero-${product.slug}`} style={styles.heroWrapper}>
                <Image source={{ uri: product.artwork_url }} style={styles.hero} contentFit="cover" />
              </Animated.View>
            ) : (
              <Animated.View sharedTransitionTag={`product-hero-fallback-${product.slug}`} style={styles.heroPlaceholder}>
                <MaterialIcons name="menu-book" size={64} color={colors.primary} />
              </Animated.View>
            )}
            <View style={styles.heroGradientOverlay} />
          </Animated.View>

          {/* Content Sheet */}
          <View style={styles.contentBody}>
            {/* Publisher Trust & Category Bar */}
            <View style={styles.publisherBar}>
              <View style={styles.publisherInfo}>
                <View style={styles.publisherBadge}>
                  <MaterialIcons name="verified" size={16} color={colors.primary} />
                </View>
                <Text style={styles.publisherName}>{product.tenant_name ?? 'Official Partner'}</Text>
                <Text style={styles.publisherDot}>•</Text>
                <Text style={styles.categoryPill}>
                  {product.category ? product.category.toUpperCase() : 'DAILY TIPS'}
                </Text>
              </View>

              {product.subscriber_count ? (
                <View style={styles.subscriberPill}>
                  <MaterialIcons name="groups" size={14} color={colors.outline} />
                  <Text style={styles.subscriberText}>
                    {product.subscriber_count.toLocaleString()} readers
                  </Text>
                </View>
              ) : null}
            </View>

            {/* Title & Tagline */}
            <View style={styles.titleSection}>
              <Animated.Text sharedTransitionTag={`product-title-${product.slug}`} style={styles.title}>
                {formatProductName(product.name)}
              </Animated.Text>
              {product.tagline ? (
                <Text style={styles.tagline}>{product.tagline}</Text>
              ) : null}
            </View>

            {/* Description */}
            <Text style={styles.description}>{product.description}</Text>

            {/* Feature Highlights Grid */}
            <View style={styles.featureGrid}>
              <View style={styles.featureItem}>
                <MaterialIcons name="alarm" size={20} color={colors.primary} />
                <View style={styles.featureItemText}>
                  <Text style={styles.featureTitle}>Daily 7:00 AM</Text>
                  <Text style={styles.featureSubtitle}>Fresh morning briefing</Text>
                </View>
              </View>

              <View style={styles.featureItem}>
                <MaterialIcons name="lightbulb-outline" size={20} color={colors.secondary} />
                <View style={styles.featureItemText}>
                  <Text style={styles.featureTitle}>Actionable Tips</Text>
                  <Text style={styles.featureSubtitle}>2-min practical read</Text>
                </View>
              </View>

              <View style={styles.featureItem}>
                <MaterialIcons name="sms" size={20} color={colors.primary} />
                <View style={styles.featureItemText}>
                  <Text style={styles.featureTitle}>SMS & Feed</Text>
                  <Text style={styles.featureSubtitle}>Direct to your phone</Text>
                </View>
              </View>

              <View style={styles.featureItem}>
                <MaterialIcons name="check-circle-outline" size={20} color={colors.primary} />
                <View style={styles.featureItemText}>
                  <Text style={styles.featureTitle}>Cancel Anytime</Text>
                  <Text style={styles.featureSubtitle}>1-tap unsubscribe</Text>
                </View>
              </View>
            </View>

            {/* Sample Content Quotation */}
            {product.sample_content ? (
              <Card style={styles.previewCard} padded={false}>
                <View style={styles.previewCardInner}>
                  <View style={styles.previewHeader}>
                    <MaterialIcons name="format-quote" size={22} color={colors.primary} />
                    <Text style={styles.previewLabel}>SAMPLE BRIEFING EXCERPT</Text>
                  </View>
                  <Text style={styles.previewQuote}>&ldquo;{product.sample_content}&rdquo;</Text>
                </View>
              </Card>
            ) : null}

            {/* Conversion-Optimized Pricing Card */}
            <Card style={styles.pricingCard} padded={false}>
              <View style={styles.pricingCardInner}>
                <View style={styles.pricingBadge}>
                  <MaterialIcons name="stars" size={16} color={colors.secondary} />
                  <Text style={styles.pricingBadgeText}>PREMIUM ACCESS</Text>
                </View>

                <View style={styles.priceRow}>
                  <Text style={styles.priceAmount}>
                    {formatCurrency(product.price, product.currency)}
                  </Text>
                  <Text style={styles.priceCycle}>/{formatBillingCycle(product.billing_cycle).replace(/^\//, '')}</Text>
                </View>

                <Text style={styles.disclosure}>
                  Billed directly via your MTN / Telecel airtime. Auto-renews daily, cancel anytime.
                </Text>

                <Button
                  label="Subscribe Now"
                  onPress={handleSubscribePress}
                  icon={<MaterialIcons name="arrow-forward" size={18} color={colors.onPrimary} />}
                  style={styles.subscribeButton}
                />
              </View>
            </Card>
          </View>
        </Animated.ScrollView>
      ) : null}
    </View>
  );
}

const createStyles = (colors: ThemeColors) => StyleSheet.create({
  root: {
    flex: 1,
    backgroundColor: colors.background,
  },
  centered: {
    flex: 1,
    justifyContent: 'center',
  },
  header: {
    position: 'absolute',
    top: 0,
    left: 0,
    right: 0,
    zIndex: 10,
    height: 90,
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    borderBottomWidth: 1,
    borderBottomColor: colors.cardBorder,
  },
  headerControls: {
    position: 'absolute',
    left: spacing.containerMargin,
    zIndex: 20,
  },
  iconBackground: {
    width: 38,
    height: 38,
    borderRadius: 19,
    backgroundColor: colors.surfaceContainerLowest,
    borderWidth: 1,
    borderColor: colors.cardBorder,
    justifyContent: 'center',
    alignItems: 'center',
    shadowColor: '#000',
    shadowOpacity: 0.15,
    shadowRadius: 8,
    shadowOffset: { width: 0, height: 2 },
    elevation: 4,
  },
  headerButton: {
    alignItems: 'center',
    justifyContent: 'center',
  },
  headerTitle: {
    ...typography.headlineMd,
    fontSize: 16,
    fontWeight: '700',
    color: colors.onSurface,
    flex: 1,
    textAlign: 'center',
    paddingHorizontal: 60,
  },
  scrollContent: {
    paddingBottom: spacing.sectionGap + 20,
  },
  heroContainer: {
    position: 'relative',
    width: '100%',
    height: HERO_HEIGHT,
    backgroundColor: colors.surfaceVariant,
    overflow: 'hidden',
  },
  heroWrapper: {
    width: '100%',
    height: '100%',
  },
  hero: {
    width: '100%',
    height: '100%',
  },
  heroPlaceholder: {
    width: '100%',
    height: '100%',
    backgroundColor: colors.surfaceVariant,
    alignItems: 'center',
    justifyContent: 'center',
  },
  heroGradientOverlay: {
    position: 'absolute',
    bottom: 0,
    left: 0,
    right: 0,
    height: 100,
    backgroundColor: 'transparent',
  },
  contentBody: {
    paddingHorizontal: spacing.containerMargin,
    paddingTop: 24,
    gap: 20,
    backgroundColor: colors.background,
    borderTopLeftRadius: radii.xl,
    borderTopRightRadius: radii.xl,
    marginTop: -24,
  },
  publisherBar: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
  },
  publisherInfo: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 6,
  },
  publisherBadge: {
    width: 24,
    height: 24,
    borderRadius: 12,
    backgroundColor: colors.primarySoft,
    alignItems: 'center',
    justifyContent: 'center',
  },
  publisherName: {
    ...typography.labelSm,
    fontSize: 13,
    fontWeight: '700',
    color: colors.onSurface,
  },
  publisherDot: {
    color: colors.outline,
    fontSize: 12,
  },
  categoryPill: {
    ...typography.labelSm,
    fontSize: 10,
    fontWeight: '800',
    letterSpacing: 0.6,
    color: colors.primary,
  },
  subscriberPill: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 4,
    paddingHorizontal: 8,
    paddingVertical: 3,
    borderRadius: radii.full,
    backgroundColor: colors.surfaceContainerLow,
    borderWidth: 1,
    borderColor: colors.cardBorder,
  },
  subscriberText: {
    ...typography.labelSm,
    fontSize: 11,
    color: colors.onSurfaceVariant,
  },
  titleSection: {
    gap: 6,
  },
  title: {
    ...typography.headlineLgMobile,
    fontSize: 26,
    lineHeight: 32,
    fontWeight: '800',
    color: colors.onSurface,
  },
  tagline: {
    ...typography.bodyLg,
    fontSize: 16,
    lineHeight: 22,
    color: colors.onSurfaceVariant,
  },
  description: {
    ...typography.bodyMd,
    fontSize: 14,
    lineHeight: 22,
    color: colors.onSurfaceVariant,
  },
  featureGrid: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: 10,
    paddingVertical: 4,
  },
  featureItem: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 10,
    width: '48%',
    padding: 12,
    borderRadius: radii.md,
    backgroundColor: colors.surfaceContainerLowest,
    borderWidth: 1,
    borderColor: colors.cardBorder,
  },
  featureItemText: {
    flex: 1,
    minWidth: 0,
    gap: 1,
  },
  featureTitle: {
    ...typography.labelSm,
    fontSize: 12,
    fontWeight: '700',
    color: colors.onSurface,
  },
  featureSubtitle: {
    ...typography.labelSm,
    fontSize: 10,
    color: colors.outline,
  },
  previewCard: {
    backgroundColor: colors.surfaceContainerLowest,
    borderColor: colors.cardBorder,
    overflow: 'hidden',
  },
  previewCardInner: {
    padding: 16,
    gap: 8,
  },
  previewHeader: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 6,
  },
  previewLabel: {
    ...typography.labelSm,
    fontSize: 10,
    fontWeight: '800',
    letterSpacing: 0.8,
    color: colors.primary,
  },
  previewQuote: {
    ...typography.bodyMd,
    fontSize: 14,
    lineHeight: 22,
    fontStyle: 'italic',
    color: colors.onSurface,
    paddingLeft: 12,
    borderLeftWidth: 3,
    borderLeftColor: colors.primary,
  },
  pricingCard: {
    backgroundColor: colors.surfaceContainerLowest,
    borderColor: 'rgba(245, 158, 11, 0.35)',
    borderWidth: 1,
    overflow: 'hidden',
  },
  pricingCardInner: {
    padding: 20,
    alignItems: 'center',
    gap: 10,
  },
  pricingBadge: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 6,
    paddingHorizontal: 10,
    paddingVertical: 4,
    borderRadius: radii.full,
    backgroundColor: colors.secondaryContainer,
    borderWidth: 1,
    borderColor: 'rgba(245, 158, 11, 0.25)',
  },
  pricingBadgeText: {
    ...typography.labelSm,
    fontSize: 11,
    fontWeight: '800',
    letterSpacing: 0.6,
    color: colors.secondary,
  },
  priceRow: {
    flexDirection: 'row',
    alignItems: 'baseline',
    gap: 4,
    marginVertical: 4,
  },
  priceAmount: {
    ...typography.headlineLgMobile,
    fontSize: 32,
    fontWeight: '800',
    color: colors.onSurface,
  },
  priceCycle: {
    ...typography.bodyLg,
    fontSize: 15,
    fontWeight: '600',
    color: colors.onSurfaceVariant,
  },
  disclosure: {
    ...typography.labelSm,
    fontSize: 12,
    lineHeight: 16,
    color: colors.outline,
    textAlign: 'center',
    paddingHorizontal: 12,
  },
  subscribeButton: {
    width: '100%',
    marginTop: 6,
  },
});
