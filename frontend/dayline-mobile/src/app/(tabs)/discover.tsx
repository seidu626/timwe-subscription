import { useMemo } from 'react';
import { MaterialIcons } from '@expo/vector-icons';
import { Image } from 'expo-image';
import { Link } from 'expo-router';
import { Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';
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
import { formatProductName, pluralize } from '@/utils/format';
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
      <Text style={styles.pageTitle}>Discover</Text>

      <Link href="/(tabs)/today" asChild>
        <AnimatedPressable accessibilityRole="button">
          <Card style={styles.todayCard}>
            <View style={styles.todayIcon}>
              <MaterialIcons name="auto-awesome" size={22} color={colors.primary} />
            </View>
            <View style={styles.todayTextGroup}>
              <Text style={styles.todayTitle}>Your Daily Digest is Ready</Text>
              <Text style={styles.todaySubtitle}>Check out the top updates tailored just for you this morning.</Text>
            </View>
            {unreadCount > 0 ? (
              <View style={styles.unreadBadge}>
                <Text style={styles.unreadBadgeText}>{unreadCount}</Text>
              </View>
            ) : null}
          </Card>
        </AnimatedPressable>
      </Link>

      {featuredProducts.length > 0 ? (
        <>
          <Text style={styles.sectionTitle}>Featured</Text>
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
        </>
      ) : null}

      <Text style={styles.sectionTitle}>Marketplace</Text>

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
    </ScreenContainer>
  );
}

function FeaturedCard({ product }: { product: CatalogProduct }) {
  const { colors } = useTheme();
  const styles = useMemo(() => createStyles(colors), [colors]);
  return (
    <Link href={{ pathname: '/product/[slug]', params: { slug: product.slug } }} asChild>
      <AnimatedPressable accessibilityRole="button">
        <Card style={styles.featuredCard}>
          {product.artwork_url ? (
            <Animated.View sharedTransitionTag={`product-hero-${product.slug}`}>
              <Image source={{ uri: product.artwork_url }} style={styles.featuredArtwork} contentFit="cover" />
            </Animated.View>
          ) : (
            <Animated.View sharedTransitionTag={`product-hero-fallback-${product.slug}`} style={styles.featuredArtworkFallback}>
              <MaterialIcons name="auto-awesome" size={28} color={colors.primary} />
            </Animated.View>
          )}
          <Animated.Text sharedTransitionTag={`product-title-${product.slug}`} style={styles.featuredName} numberOfLines={1} ellipsizeMode="tail">
            {formatProductName(product.name)}
          </Animated.Text>
          <Text style={styles.featuredMeta} numberOfLines={1} ellipsizeMode="tail">
            {product.tenant_name}
          </Text>
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
        {tenant.branding?.logo_url ? (
          <Image source={{ uri: tenant.branding.logo_url }} style={styles.tenantLogo} contentFit="cover" />
        ) : (
          <View style={styles.tenantBadge}>
            <MaterialIcons name="storefront" size={18} color={colors.primary} />
          </View>
        )}
        <View style={styles.tenantTextGroup}>
          <Text style={styles.tenantName} numberOfLines={1} ellipsizeMode="tail">
            {tenant.tenant_name}
          </Text>
          <Text style={styles.tenantMeta}>{pluralize(tenant.products.length, 'product')}</Text>
        </View>
      </View>
      <View style={styles.productList}>
        {previewProducts.map((product) => (
          <ProductRow key={product.slug} product={product} />
        ))}
      </View>
      {remaining > 0 ? (
        <Link href={{ pathname: '/tenant/[tenantKey]', params: { tenantKey: tenant.tenant_key } }} asChild>
          <AnimatedPressable accessibilityRole="button" style={styles.viewAllRow}>
            <Text style={styles.viewAllText}>View all {pluralize(tenant.products.length, 'product')}</Text>
            <MaterialIcons name="chevron-right" size={20} color={colors.primary} />
          </AnimatedPressable>
        </Link>
      ) : null}
    </View>
  );
}

const createStyles = (colors: ThemeColors) => StyleSheet.create({
  pageTitle: {
    ...typography.headlineLgMobile,
    color: colors.primary,
    marginBottom: spacing.sectionGap - spacing.stackLg,
  },
  todayCard: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.stackMd,
    marginBottom: spacing.sectionGap,
  },
  todayIcon: {
    width: 48,
    height: 48,
    borderRadius: 24,
    backgroundColor: colors.primarySoft,
    alignItems: 'center',
    justifyContent: 'center',
  },
  todayTextGroup: {
    flex: 1,
    gap: 2,
  },
  todayTitle: {
    ...typography.headlineMd,
    fontSize: 18,
    lineHeight: 24,
    color: colors.onSurface,
  },
  todaySubtitle: {
    ...typography.bodyMd,
    fontSize: 14,
    lineHeight: 20,
    color: colors.onSurfaceVariant,
  },
  unreadBadge: {
    width: 24,
    height: 24,
    borderRadius: 12,
    backgroundColor: colors.error,
    alignItems: 'center',
    justifyContent: 'center',
  },
  unreadBadgeText: {
    ...typography.labelSm,
    color: colors.onError,
  },
  sectionTitle: {
    ...typography.headlineMd,
    color: colors.onSurface,
    marginBottom: spacing.stackMd,
  },
  featuredScroller: {
    marginBottom: spacing.sectionGap,
  },
  featuredRow: {
    gap: spacing.stackMd,
  },
  featuredCard: {
    width: 160,
    gap: spacing.stackSm,
  },
  featuredArtwork: {
    width: '100%',
    height: 96,
    borderRadius: radii.md,
    backgroundColor: colors.surfaceVariant,
  },
  featuredArtworkFallback: {
    width: '100%',
    height: 96,
    borderRadius: radii.md,
    backgroundColor: colors.primarySoft,
    alignItems: 'center',
    justifyContent: 'center',
  },
  featuredName: {
    ...typography.headlineMd,
    fontSize: 15,
    lineHeight: 20,
    color: colors.onSurface,
  },
  featuredMeta: {
    ...typography.labelSm,
    color: colors.onSurfaceVariant,
  },
  tenantList: {
    gap: spacing.sectionGap,
  },
  tenantSection: {
    gap: spacing.stackMd,
  },
  tenantHeader: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.stackSm,
  },
  tenantBadge: {
    width: 32,
    height: 32,
    borderRadius: radii.md,
    backgroundColor: colors.primarySoft,
    alignItems: 'center',
    justifyContent: 'center',
    flexShrink: 0,
  },
  tenantLogo: {
    width: 32,
    height: 32,
    borderRadius: radii.md,
    backgroundColor: colors.surfaceVariant,
    flexShrink: 0,
  },
  tenantTextGroup: {
    gap: 0,
    flex: 1,
    minWidth: 0,
  },
  tenantName: {
    ...typography.headlineMd,
    fontSize: 16,
    lineHeight: 22,
    color: colors.onSurface,
  },
  tenantMeta: {
    ...typography.labelSm,
    color: colors.onSurfaceVariant,
  },
  productList: {
    gap: spacing.stackMd,
  },
  viewAllRow: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    gap: spacing.stackSm,
    paddingVertical: spacing.stackSm,
  },
  viewAllText: {
    ...typography.labelMd,
    color: colors.primary,
  },
});
