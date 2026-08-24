import { useMemo } from 'react';
import { MaterialIcons } from '@expo/vector-icons';
import { Image } from 'expo-image';
import { Link } from 'expo-router';
import { ScrollView, StyleSheet, Text, View } from 'react-native';
import Animated from 'react-native-reanimated';

import { Card } from '@/components/Card';
import { ProductRow } from '@/components/ProductRow';
import { ScreenContainer } from '@/components/ScreenContainer';
import { EmptyState, ErrorState, LoadingState } from '@/components/AsyncState';
import { AnimatedPressable } from '@/components/AnimatedPressable';
import { useMarketplace } from '@/hooks/useCatalog';
import { useFeed } from '@/hooks/useFeed';
import { radii, spacing, typography, type ThemeColors } from '@/theme/tokens';
import { useTheme } from '@/theme/ThemeContext';
import { formatBillingCycle, formatCurrency, formatProductName, pluralize } from '@/utils/format';
import type { CatalogProduct, MarketplaceTenant } from '@/api/types';

// A tenant section previews at most this many products before the shopper
// must open the full storefront; keeps Discover a single bounded stream
// regardless of catalog size.
const TENANT_PREVIEW_LIMIT = 3;

export default function DiscoverScreen() {
  const { colors } = useTheme();
  const styles = useMemo(() => createStyles(colors), [colors]);
  const catalog = useMarketplace();
  const feed = useFeed();
  const unreadCount = feed.data?.filter((item) => !item.read).length ?? 0;
  
  // The backend orders featured products first within each tenant section
  // (app_featured_rank NULLS LAST), so a flat filter keeps that order.
  const featuredProducts = catalog.data?.flatMap((tenant) => tenant.products.filter((p) => p.featured)) ?? [];

  return (
    <ScreenContainer
      scroll
      withTabBarClearance
      refreshing={catalog.isRefetching}
      onRefresh={() => {
        void catalog.refetch();
        void feed.refetch();
      }}
    >
      <View style={styles.headerRow}>
        <View>
          <Text style={styles.eyebrow}>MARKETPLACE & CHANNELS</Text>
          <Text style={styles.pageTitle}>Discover</Text>
        </View>
      </View>

      {/* Daily Digest Portal Banner */}
      <Link href="/(tabs)/today" asChild>
        <AnimatedPressable accessibilityRole="button">
          <Card style={styles.todayCard} variant="glow" padded={false}>
            <View style={styles.todayCardInner}>
              <View style={styles.todayIcon}>
                <MaterialIcons name="auto-awesome" size={24} color={colors.primary} />
              </View>
              <View style={styles.todayTextGroup}>
                <View style={styles.todayHeaderLine}>
                  <Text style={styles.todayTitle}>Your Daily Digest</Text>
                  {unreadCount > 0 ? (
                    <View style={styles.unreadBadge}>
                      <Text style={styles.unreadBadgeText}>{unreadCount} NEW</Text>
                    </View>
                  ) : null}
                </View>
                <Text style={styles.todaySubtitle}>Check out the top updates tailored just for you this morning.</Text>
              </View>
              <MaterialIcons name="chevron-right" size={22} color={colors.primary} />
            </View>
          </Card>
        </AnimatedPressable>
      </Link>

      {/* Featured Spotlight Carousel */}
      {featuredProducts.length > 0 ? (
        <View style={styles.sectionContainer}>
          <View style={styles.sectionHeaderRow}>
            <View style={styles.sectionTitleWithIcon}>
              <MaterialIcons name="stars" size={20} color={colors.secondary} />
              <Text style={styles.sectionTitle}>Featured Channels</Text>
            </View>
            <Text style={styles.sectionSubtitle}>Popular right now</Text>
          </View>

          <ScrollView
            horizontal
            showsHorizontalScrollIndicator={false}
            contentContainerStyle={styles.featuredRow}
            style={styles.featuredScroller}
          >
            {featuredProducts.map((product) => (
              <FeaturedCard key={`${product.tenant}-${product.slug}`} product={product} />
            ))}
          </ScrollView>
        </View>
      ) : null}

      {/* Marketplace Catalog */}
      <View style={styles.sectionContainer}>
        <View style={styles.sectionHeaderRow}>
          <View style={styles.sectionTitleWithIcon}>
            <MaterialIcons name="storefront" size={20} color={colors.primary} />
            <Text style={styles.sectionTitle}>Curated Publishers</Text>
          </View>
          <Text style={styles.sectionSubtitle}>Subscribe directly with mobile airtime</Text>
        </View>

        {catalog.isPending ? <LoadingState label="Loading marketplace…" /> : null}

        {catalog.isError ? (
          <ErrorState
            title="Couldn't load the marketplace"
            message={catalog.error instanceof Error ? catalog.error.message : undefined}
            onRetry={() => catalog.refetch()}
          />
        ) : null}

        {catalog.isSuccess && catalog.data.length === 0 ? (
          <EmptyState icon="explore" title="No products available yet" message="Check back soon for new content." />
        ) : null}

        <View style={styles.tenantList}>
          {catalog.data?.map((tenant) => (
            <TenantSection key={tenant.tenant_key} tenant={tenant} />
          ))}
        </View>
      </View>
    </ScreenContainer>
  );
}

function FeaturedCard({ product }: { product: CatalogProduct }) {
  const { colors } = useTheme();
  const styles = useMemo(() => createStyles(colors), [colors]);
  return (
    <Link href={{ pathname: '/product/[slug]', params: { slug: product.slug } }} asChild>
      <AnimatedPressable accessibilityRole="button">
        <Card style={styles.featuredCard} padded={false}>
          <View style={styles.featuredArtworkWrapper}>
            {product.artwork_url ? (
              <Animated.View sharedTransitionTag={`product-hero-${product.slug}`} style={styles.featuredArtworkContainer}>
                <Image source={{ uri: product.artwork_url }} style={styles.featuredArtwork} contentFit="cover" />
              </Animated.View>
            ) : (
              <Animated.View sharedTransitionTag={`product-hero-fallback-${product.slug}`} style={styles.featuredArtworkFallback}>
                <MaterialIcons name="menu-book" size={32} color={colors.primary} />
              </Animated.View>
            )}
            
            {/* Category Overlay Tag */}
            <View style={styles.featuredCategoryPill}>
              <Text style={styles.featuredCategoryText}>
                {product.category ? product.category.toUpperCase() : 'EDITORIAL'}
              </Text>
            </View>

            {/* Price Chip Overlay */}
            <View style={styles.featuredPriceOverlay}>
              <Text style={styles.featuredPriceText}>
                {formatCurrency(product.price, product.currency)}
                {formatBillingCycle(product.billing_cycle)}
              </Text>
            </View>
          </View>

          <View style={styles.featuredContent}>
            <Animated.Text sharedTransitionTag={`product-title-${product.slug}`} style={styles.featuredName} numberOfLines={2} ellipsizeMode="tail">
              {formatProductName(product.name)}
            </Animated.Text>
            {product.tagline ? (
              <Text style={styles.featuredTagline} numberOfLines={1} ellipsizeMode="tail">
                {product.tagline}
              </Text>
            ) : null}

            <View style={styles.featuredFooter}>
              <View style={styles.featuredPublisher}>
                <Text style={styles.featuredMeta} numberOfLines={1} ellipsizeMode="tail">
                  {product.tenant_name}
                </Text>
              </View>
              {product.subscriber_count ? (
                <Text style={styles.featuredSubscribers}>
                  {product.subscriber_count.toLocaleString()} readers
                </Text>
              ) : null}
            </View>
          </View>
        </Card>
      </AnimatedPressable>
    </Link>
  );
}

function TenantSection({ tenant }: { tenant: MarketplaceTenant }) {
  const { colors } = useTheme();
  const styles = useMemo(() => createStyles(colors), [colors]);
  const previewProducts = tenant.products.slice(0, TENANT_PREVIEW_LIMIT);
  const remaining = tenant.products.length - previewProducts.length;

  return (
    <View style={styles.tenantSection}>
      <View style={styles.tenantHeader}>
        <View style={styles.tenantIdentity}>
          {tenant.branding?.logo_url ? (
            <Image source={{ uri: tenant.branding.logo_url }} style={styles.tenantLogo} contentFit="cover" />
          ) : (
            <View style={styles.tenantBadge}>
              <MaterialIcons name="storefront" size={20} color={colors.primary} />
            </View>
          )}
          <View style={styles.tenantTextGroup}>
            <View style={styles.tenantNameRow}>
              <Text style={styles.tenantName} numberOfLines={1} ellipsizeMode="tail">
                {tenant.tenant_name}
              </Text>
            </View>
            <Text style={styles.tenantMeta}>
              {pluralize(tenant.products.length, 'channel')}
            </Text>
          </View>
        </View>

        <Link href={{ pathname: '/tenant/[tenantKey]', params: { tenantKey: tenant.tenant_key } }} asChild>
          <AnimatedPressable accessibilityRole="button" style={styles.storefrontButton}>
            <Text style={styles.storefrontButtonText}>Storefront</Text>
            <MaterialIcons name="chevron-right" size={16} color={colors.primary} />
          </AnimatedPressable>
        </Link>
      </View>

      <View style={styles.productList}>
        {previewProducts.map((product) => (
          <ProductRow key={product.slug} product={product} />
        ))}
      </View>

      {remaining > 0 ? (
        <Link href={{ pathname: '/tenant/[tenantKey]', params: { tenantKey: tenant.tenant_key } }} asChild>
          <AnimatedPressable accessibilityRole="button" style={styles.viewAllRow}>
            <Text style={styles.viewAllText}>View all {pluralize(tenant.products.length, 'channel')}</Text>
            <MaterialIcons name="arrow-forward" size={16} color={colors.primary} />
          </AnimatedPressable>
        </Link>
      ) : null}
    </View>
  );
}

const createStyles = (colors: ThemeColors) => StyleSheet.create({
  headerRow: {
    marginBottom: spacing.stackLg,
  },
  eyebrow: {
    ...typography.labelSm,
    fontSize: 11,
    letterSpacing: 0.8,
    color: colors.primary,
    fontWeight: '700',
    marginBottom: 4,
  },
  pageTitle: {
    ...typography.headlineLgMobile,
    fontSize: 30,
    fontWeight: '800',
    letterSpacing: -0.5,
    color: colors.onSurface,
  },
  todayCard: {
    marginBottom: spacing.sectionGap,
    overflow: 'hidden',
    backgroundColor: colors.surfaceContainerLowest,
    borderColor: 'rgba(52, 211, 153, 0.25)',
  },
  todayCardInner: {
    flexDirection: 'row',
    alignItems: 'center',
    padding: 16,
    gap: spacing.stackMd,
  },
  todayIcon: {
    width: 44,
    height: 44,
    borderRadius: 22,
    backgroundColor: colors.primarySoft,
    borderWidth: 1,
    borderColor: 'rgba(52, 211, 153, 0.3)',
    alignItems: 'center',
    justifyContent: 'center',
  },
  todayTextGroup: {
    flex: 1,
    gap: 3,
  },
  todayHeaderLine: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
  },
  todayTitle: {
    ...typography.headlineMd,
    fontSize: 16,
    lineHeight: 20,
    fontWeight: '700',
    color: colors.onSurface,
  },
  todaySubtitle: {
    ...typography.bodyMd,
    fontSize: 13,
    lineHeight: 18,
    color: colors.onSurfaceVariant,
  },
  unreadBadge: {
    paddingHorizontal: 7,
    paddingVertical: 2,
    borderRadius: radii.full,
    backgroundColor: colors.error,
  },
  unreadBadgeText: {
    ...typography.labelSm,
    fontSize: 10,
    fontWeight: '800',
    letterSpacing: 0.5,
    color: colors.onError,
  },
  sectionContainer: {
    marginBottom: spacing.sectionGap,
  },
  sectionHeaderRow: {
    marginBottom: spacing.stackMd,
  },
  sectionTitleWithIcon: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 6,
  },
  sectionTitle: {
    ...typography.headlineMd,
    fontSize: 19,
    fontWeight: '700',
    color: colors.onSurface,
  },
  sectionSubtitle: {
    ...typography.bodyMd,
    fontSize: 13,
    color: colors.onSurfaceVariant,
    marginTop: 2,
  },
  featuredScroller: {
    marginHorizontal: -spacing.containerMargin,
    paddingHorizontal: spacing.containerMargin,
  },
  featuredRow: {
    gap: 14,
    paddingRight: spacing.containerMargin,
  },
  featuredCard: {
    width: 250,
    overflow: 'hidden',
    backgroundColor: colors.surfaceContainerLowest,
  },
  featuredArtworkWrapper: {
    position: 'relative',
    width: '100%',
    height: 130,
    backgroundColor: colors.surfaceVariant,
  },
  featuredArtworkContainer: {
    width: '100%',
    height: '100%',
  },
  featuredArtwork: {
    width: '100%',
    height: '100%',
  },
  featuredArtworkFallback: {
    width: '100%',
    height: '100%',
    backgroundColor: colors.primarySoft,
    alignItems: 'center',
    justifyContent: 'center',
  },
  featuredCategoryPill: {
    position: 'absolute',
    top: 10,
    left: 10,
    backgroundColor: 'rgba(13, 17, 14, 0.75)',
    paddingHorizontal: 8,
    paddingVertical: 3,
    borderRadius: radii.sm,
    borderWidth: 1,
    borderColor: 'rgba(255, 255, 255, 0.15)',
  },
  featuredCategoryText: {
    ...typography.labelSm,
    fontSize: 9,
    fontWeight: '800',
    letterSpacing: 0.6,
    color: colors.primary,
  },
  featuredPriceOverlay: {
    position: 'absolute',
    bottom: 10,
    right: 10,
    backgroundColor: 'rgba(13, 17, 14, 0.85)',
    paddingHorizontal: 8,
    paddingVertical: 4,
    borderRadius: radii.full,
    borderWidth: 1,
    borderColor: 'rgba(245, 158, 11, 0.4)',
  },
  featuredPriceText: {
    ...typography.labelSm,
    fontSize: 11,
    fontWeight: '700',
    color: colors.secondary,
  },
  featuredContent: {
    padding: 14,
    gap: 4,
  },
  featuredName: {
    ...typography.headlineMd,
    fontSize: 16,
    lineHeight: 21,
    fontWeight: '700',
    color: colors.onSurface,
  },
  featuredTagline: {
    ...typography.bodyMd,
    fontSize: 13,
    lineHeight: 18,
    color: colors.onSurfaceVariant,
  },
  featuredFooter: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginTop: 8,
    paddingTop: 8,
    borderTopWidth: 1,
    borderTopColor: colors.cardBorder,
  },
  featuredPublisher: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 4,
    flex: 1,
    minWidth: 0,
  },
  featuredMeta: {
    ...typography.labelSm,
    fontSize: 12,
    fontWeight: '600',
    color: colors.onSurfaceVariant,
  },
  featuredSubscribers: {
    ...typography.labelSm,
    fontSize: 11,
    color: colors.outline,
  },
  tenantList: {
    gap: 28,
  },
  tenantSection: {
    gap: 12,
  },
  tenantHeader: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: spacing.stackSm,
  },
  tenantIdentity: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 10,
    flex: 1,
    minWidth: 0,
  },
  tenantBadge: {
    width: 36,
    height: 36,
    borderRadius: radii.md,
    backgroundColor: colors.primarySoft,
    borderWidth: 1,
    borderColor: colors.cardBorder,
    alignItems: 'center',
    justifyContent: 'center',
    flexShrink: 0,
  },
  tenantLogo: {
    width: 36,
    height: 36,
    borderRadius: radii.md,
    backgroundColor: colors.surfaceVariant,
    borderWidth: 1,
    borderColor: colors.cardBorder,
    flexShrink: 0,
  },
  tenantTextGroup: {
    flex: 1,
    minWidth: 0,
    gap: 1,
  },
  tenantNameRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 4,
  },
  tenantName: {
    ...typography.headlineMd,
    fontSize: 16,
    lineHeight: 20,
    fontWeight: '700',
    color: colors.onSurface,
  },
  tenantMeta: {
    ...typography.labelSm,
    fontSize: 12,
    color: colors.onSurfaceVariant,
  },
  storefrontButton: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingHorizontal: 10,
    paddingVertical: 5,
    borderRadius: radii.full,
    backgroundColor: colors.primarySoft,
    borderWidth: 1,
    borderColor: 'rgba(52, 211, 153, 0.2)',
    gap: 2,
  },
  storefrontButtonText: {
    ...typography.labelSm,
    fontSize: 12,
    fontWeight: '600',
    color: colors.primary,
  },
  productList: {
    gap: 10,
  },
  viewAllRow: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    gap: 6,
    paddingVertical: 10,
    borderRadius: radii.md,
    borderWidth: 1,
    borderColor: colors.cardBorder,
    backgroundColor: colors.surfaceContainerLow,
  },
  viewAllText: {
    ...typography.labelMd,
    fontSize: 13,
    fontWeight: '600',
    color: colors.primary,
  },
});
