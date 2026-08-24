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

import { Button } from '@/components/Button';
import { Card } from '@/components/Card';
import { AnimatedPressable } from '@/components/AnimatedPressable';
import { ErrorState, LoadingState } from '@/components/AsyncState';
import { useSettings } from '@/context/SettingsContext';
import { useCatalogProduct } from '@/hooks/useCatalog';
import { radii, spacing, typography, type ThemeColors } from '@/theme/tokens';
import { useTheme } from '@/theme/ThemeContext';
import { formatBillingCycle, formatCurrency, formatProductName } from '@/utils/format';

const HERO_HEIGHT = 280;

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
      [HERO_HEIGHT - 100, HERO_HEIGHT - 50],
      [0, 1],
      Extrapolation.CLAMP
    );
    return {
      opacity,
      backgroundColor: colors.surface,
    };
  });

  const heroAnimatedStyle = useAnimatedStyle(() => {
    // Parallax effect on scroll down, zoom effect on over-scroll up
    const translateY = interpolate(
      scrollY.value,
      [-100, 0, HERO_HEIGHT],
      [-50, 0, HERO_HEIGHT * 0.5],
      Extrapolation.CLAMP
    );
    const scale = interpolate(
      scrollY.value,
      [-100, 0],
      [1.5, 1],
      Extrapolation.CLAMP
    );
    return {
      transform: [{ translateY }, { scale }],
    };
  });

  return (
    <View style={styles.root}>
      <Animated.View style={[styles.header, { paddingTop: insets.top }, headerAnimatedStyle]}>
        <Text style={styles.headerTitle} numberOfLines={1} ellipsizeMode="tail">
          {product?.name ?? 'Product'}
        </Text>
      </Animated.View>

      <View style={[styles.headerControls, { top: insets.top }]}>
        <AnimatedPressable onPress={() => router.back()} accessibilityRole="button" accessibilityLabel="Go back" style={styles.headerButton}>
          <View style={styles.iconBackground}>
            <MaterialIcons name="arrow-back" size={22} color={colors.onSurface} />
          </View>
        </AnimatedPressable>
      </View>

      {isPending ? (
        <SafeAreaView style={styles.centered} edges={['top', 'left', 'right']}>
          <LoadingState label="Loading product…" />
        </SafeAreaView>
      ) : null}
      
      {isError ? (
        <SafeAreaView style={styles.centered} edges={['top', 'left', 'right']}>
          <ErrorState
            title="Couldn't load this product"
            message={error instanceof Error ? error.message : undefined}
            onRetry={refetch}
          />
        </SafeAreaView>
      ) : null}
      
      {!isPending && !isError && !product ? (
        <SafeAreaView style={styles.centered} edges={['top', 'left', 'right']}>
          <ErrorState title="Product not found" message="This product may no longer be available." />
        </SafeAreaView>
      ) : null}

      {product ? (
        <Animated.ScrollView 
          contentContainerStyle={styles.scrollContent}
          onScroll={scrollHandler}
          scrollEventThrottle={16}
          showsVerticalScrollIndicator={false}
        >
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
          </Animated.View>

          <View style={styles.contentBody}>
            <Animated.Text sharedTransitionTag={`product-title-${product.slug}`} style={styles.title}>
              {formatProductName(product.name)} — {product.tagline}
            </Animated.Text>
            <Text style={styles.description}>{product.description}</Text>

            {product.subscriber_count ? (
              <View style={styles.subscriberRow}>
                <MaterialIcons name="groups" size={18} color={colors.onSurfaceVariant} />
                <Text style={styles.subscriberText}>{product.subscriber_count.toLocaleString()} subscribers</Text>
              </View>
            ) : null}

            {product.sample_content ? (
              <Card style={styles.previewCard}>
                <View style={styles.previewHeader}>
                  <MaterialIcons name="history" size={18} color={colors.primary} />
                  <Text style={styles.previewLabel}>Sample content</Text>
                </View>
                <Text style={styles.previewQuote}>&ldquo;{product.sample_content}&rdquo;</Text>
              </Card>
            ) : null}

            <Card style={styles.pricingCard}>
              <MaterialIcons name="stars" size={28} color={colors.secondary} />
              <Text style={styles.pricingTitle}>Premium Access</Text>
              <Text style={styles.price}>
                {formatCurrency(product.price, product.currency)}
                <Text style={styles.priceCycle}> {formatBillingCycle(product.billing_cycle)}</Text>
              </Text>
              <Text style={styles.disclosure}>Billed via your mobile network. Auto-renews, cancel anytime.</Text>
              <Button
                label="Subscribe Now"
                onPress={() => router.push({ pathname: '/product/[slug]/confirm', params: { slug: product.slug } })}
                icon={<MaterialIcons name="arrow-forward" size={20} color={colors.onPrimary} />}
                style={styles.subscribeButton}
              />
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
    backgroundColor: colors.surface,
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
    height: 96,
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    borderBottomWidth: StyleSheet.hairlineWidth,
    borderBottomColor: colors.outlineVariant,
  },
  headerControls: {
    position: 'absolute',
    left: spacing.stackSm,
    zIndex: 20,
    width: 48,
    height: 48,
    justifyContent: 'center',
    alignItems: 'center',
  },
  iconBackground: {
    width: 40,
    height: 40,
    borderRadius: 20,
    backgroundColor: colors.surfaceContainerLowest,
    justifyContent: 'center',
    alignItems: 'center',
    shadowColor: '#000',
    shadowOpacity: 0.1,
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
    fontSize: 18,
    color: colors.primary,
    flex: 1,
    textAlign: 'center',
    paddingHorizontal: 60, // Clear the back button
  },
  scrollContent: {
    paddingBottom: spacing.sectionGap,
  },
  heroContainer: {
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
  contentBody: {
    paddingHorizontal: spacing.containerMargin,
    paddingTop: spacing.stackLg,
    gap: spacing.stackLg,
    backgroundColor: colors.surface,
    borderTopLeftRadius: radii.xl,
    borderTopRightRadius: radii.xl,
    marginTop: -20, // Overlap the hero image slightly
  },
  title: {
    ...typography.headlineLgMobile,
    color: colors.onSurface,
  },
  description: {
    ...typography.bodyLg,
    color: colors.onSurfaceVariant,
  },
  subscriberRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.stackSm,
  },
  subscriberText: {
    ...typography.labelMd,
    color: colors.onSurfaceVariant,
  },
  previewCard: {
    gap: spacing.stackMd,
  },
  previewHeader: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.stackSm,
  },
  previewLabel: {
    ...typography.labelMd,
    color: colors.primary,
    textTransform: 'uppercase',
  },
  previewQuote: {
    ...typography.bodyMd,
    color: colors.onSurfaceVariant,
    fontStyle: 'italic',
    borderLeftWidth: 4,
    borderLeftColor: colors.primary,
    paddingLeft: spacing.stackMd,
  },
  pricingCard: {
    alignItems: 'center',
    gap: spacing.stackSm,
    backgroundColor: 'rgba(253,183,65,0.15)',
    borderWidth: 1,
    borderColor: 'rgba(253,183,65,0.4)',
  },
  pricingTitle: {
    ...typography.headlineMd,
    fontSize: 20,
    color: colors.onSecondaryFixed,
  },
  price: {
    ...typography.headlineLgMobile,
    color: colors.secondary,
  },
  priceCycle: {
    ...typography.bodyMd,
    color: colors.onSecondaryFixedVariant,
  },
  disclosure: {
    ...typography.labelSm,
    color: colors.onSecondaryFixedVariant,
    textAlign: 'center',
  },
  subscribeButton: {
    width: '100%',
    marginTop: spacing.stackSm,
  },
});
