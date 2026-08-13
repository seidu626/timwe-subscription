import { MaterialIcons } from '@expo/vector-icons';
import { Link } from 'expo-router';
import { Pressable, StyleSheet, Text, View } from 'react-native';

import { Card } from '@/components/Card';
import { ScreenContainer } from '@/components/ScreenContainer';
import { EmptyState, ErrorState, LoadingState } from '@/components/AsyncState';
import { useCatalog } from '@/hooks/useCatalog';
import { useFeed } from '@/hooks/useFeed';
import { colors, radii, spacing, typography } from '@/theme/tokens';
import { formatBillingCycle, formatCurrency } from '@/utils/format';
import type { CatalogProduct } from '@/api/types';

export default function DiscoverScreen() {
  const catalog = useCatalog();
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

      <Text style={styles.sectionTitle}>Discover products</Text>

      {catalog.isPending ? <LoadingState label="Loading products…" /> : null}

      {catalog.isError ? (
        <ErrorState
          title="Couldn't load products"
          message={catalog.error instanceof Error ? catalog.error.message : undefined}
          onRetry={() => catalog.refetch()}
        />
      ) : null}

      {catalog.isSuccess && catalog.data.length === 0 ? (
        <EmptyState icon="explore" title="No products available yet" message="Check back soon for new content." />
      ) : null}

      <View style={styles.productList}>
        {catalog.data?.map((product) => (
          <ProductRow key={product.slug} product={product} />
        ))}
      </View>
    </ScreenContainer>
  );
}

function ProductRow({ product }: { product: CatalogProduct }) {
  return (
    <Link href={{ pathname: '/product/[slug]', params: { slug: product.slug } }} asChild>
      <Pressable accessibilityRole="button">
        <Card style={styles.productRow}>
          <View style={styles.productLeft}>
            <View style={styles.productIcon}>
              <MaterialIcons name="menu-book" size={22} color={colors.primary} />
            </View>
            <View>
              <Text style={styles.productName}>{product.name}</Text>
              <Text style={styles.productTagline}>{product.tagline}</Text>
            </View>
          </View>
          <View style={styles.productRight}>
            <View style={styles.priceChip}>
              <Text style={styles.priceChipText}>
                {formatCurrency(product.price, product.currency)}
                {formatBillingCycle(product.billing_cycle)}
              </Text>
            </View>
            <MaterialIcons name="chevron-right" size={22} color={colors.onSurfaceVariant} />
          </View>
        </Card>
      </Pressable>
    </Link>
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
  productList: {
    gap: spacing.stackMd,
  },
  productRow: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
  },
  productLeft: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.stackMd,
    flexShrink: 1,
  },
  productIcon: {
    width: 48,
    height: 48,
    borderRadius: radii.md,
    backgroundColor: colors.surfaceVariant,
    alignItems: 'center',
    justifyContent: 'center',
  },
  productName: {
    ...typography.headlineMd,
    fontSize: 18,
    lineHeight: 24,
    color: colors.onSurface,
  },
  productTagline: {
    ...typography.bodyMd,
    fontSize: 14,
    lineHeight: 20,
    color: colors.onSurfaceVariant,
  },
  productRight: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.stackMd,
  },
  priceChip: {
    backgroundColor: 'rgba(253,183,65,0.2)',
    borderWidth: 1,
    borderColor: 'rgba(253,183,65,0.3)',
    borderRadius: 9999,
    paddingHorizontal: 12,
    paddingVertical: 4,
  },
  priceChipText: {
    ...typography.labelSm,
    color: colors.secondary,
  },
});
