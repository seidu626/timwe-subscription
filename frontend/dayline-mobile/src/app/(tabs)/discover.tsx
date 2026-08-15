import { MaterialIcons } from '@expo/vector-icons';
import { Link } from 'expo-router';
import { Pressable, StyleSheet, Text, View } from 'react-native';

import { Card } from '@/components/Card';
import { ProductRow } from '@/components/ProductRow';
import { ScreenContainer } from '@/components/ScreenContainer';
import { EmptyState, ErrorState, LoadingState } from '@/components/AsyncState';
import { useMarketplace } from '@/hooks/useCatalog';
import { useFeed } from '@/hooks/useFeed';
import { colors, radii, spacing, typography } from '@/theme/tokens';
import { pluralize } from '@/utils/format';
import type { MarketplaceTenant } from '@/api/types';

// A tenant section previews at most this many products before the shopper
// must open the full storefront; keeps Discover a single bounded stream
// regardless of catalog size.
const TENANT_PREVIEW_LIMIT = 3;

export default function DiscoverScreen() {
  const catalog = useMarketplace();
  const feed = useFeed();
  const unreadCount = feed.data?.filter((item) => !item.read).length ?? 0;

  return (
    <ScreenContainer scroll withTabBarClearance>
      <Text style={styles.pageTitle}>Discover</Text>

      <Link href="/(tabs)/today" asChild>
        <Pressable accessibilityRole="button">
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
        </Pressable>
      </Link>

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

function TenantSection({ tenant }: { tenant: MarketplaceTenant }) {
  const previewProducts = tenant.products.slice(0, TENANT_PREVIEW_LIMIT);
  const remaining = tenant.products.length - previewProducts.length;

  return (
    <View style={styles.tenantSection}>
      <View style={styles.tenantHeader}>
        <View style={styles.tenantBadge}>
          <MaterialIcons name="storefront" size={18} color={colors.primary} />
        </View>
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
          <Pressable accessibilityRole="button" style={styles.viewAllRow}>
            <Text style={styles.viewAllText}>View all {pluralize(tenant.products.length, 'product')}</Text>
            <MaterialIcons name="chevron-right" size={20} color={colors.primary} />
          </Pressable>
        </Link>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
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
    backgroundColor: 'rgba(15,110,82,0.12)',
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
    backgroundColor: 'rgba(15,110,82,0.12)',
    alignItems: 'center',
    justifyContent: 'center',
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
