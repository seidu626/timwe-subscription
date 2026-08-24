import { useMemo, useState } from 'react';
import { MaterialIcons } from '@expo/vector-icons';
import { Image } from 'expo-image';
import { router, useLocalSearchParams } from 'expo-router';
import { StyleSheet, Text, View } from 'react-native';
import Animated, { FadeIn, FadeOut, LinearTransition } from 'react-native-reanimated';
import * as Haptics from 'expo-haptics';

import { ProductRow } from '@/components/ProductRow';
import { EmptyState, ErrorState, LoadingState } from '@/components/AsyncState';
import { ScreenContainer } from '@/components/ScreenContainer';
import { AnimatedPressable } from '@/components/AnimatedPressable';
import { useMarketplaceTenant } from '@/hooks/useCatalog';
import { radii, spacing, typography, type ThemeColors } from '@/theme/tokens';
import { useTheme } from '@/theme/ThemeContext';
import { pluralize } from '@/utils/format';

// Category chips earn their place once a storefront actually has more than
// one category to browse by; a single-category (or uncategorized) catalog
// stays chip-free regardless of product count.
const CATEGORY_FILTER_MIN_CATEGORIES = 2;
const ALL_CATEGORY = 'All';

export default function TenantStorefrontScreen() {
  const { colors } = useTheme();
  const styles = useMemo(() => createStyles(colors), [colors]);
  const { tenantKey } = useLocalSearchParams<{ tenantKey: string }>();
  const { isPending, isError, error, refetch, tenant } = useMarketplaceTenant(tenantKey);
  const [selectedCategory, setSelectedCategory] = useState(ALL_CATEGORY);

  const categoryCounts = useMemo(() => {
    if (!tenant) return [];
    const counts = new Map<string, number>();
    for (const product of tenant.products) {
      if (!product.category) continue;
      counts.set(product.category, (counts.get(product.category) ?? 0) + 1);
    }
    if (counts.size < CATEGORY_FILTER_MIN_CATEGORIES) return [];
    return [
      { label: ALL_CATEGORY, count: tenant.products.length },
      ...Array.from(counts.entries()).map(([category, count]) => ({ label: category, count })),
    ];
  }, [tenant]);

  const visibleProducts = useMemo(() => {
    if (!tenant) return [];
    if (selectedCategory === ALL_CATEGORY) return tenant.products;
    return tenant.products.filter((product) => product.category === selectedCategory);
  }, [tenant, selectedCategory]);

  const handleCategoryPress = (category: string) => {
    setSelectedCategory(category);
    Haptics.impactAsync(Haptics.ImpactFeedbackStyle.Light);
  };

  return (
    <ScreenContainer scroll withTabBarClearance>
      {/* Header Bar */}
      <View style={styles.header}>
        <AnimatedPressable onPress={() => router.back()} accessibilityRole="button" accessibilityLabel="Go back" style={styles.headerButton}>
          <View style={styles.backIconCircle}>
            <MaterialIcons name="arrow-back" size={20} color={colors.onSurface} />
          </View>
        </AnimatedPressable>
        <Text style={styles.headerTitle} numberOfLines={1} ellipsizeMode="tail">
          {tenant?.tenant_name ?? 'Storefront'}
        </Text>
        <View style={styles.headerSpacer} />
      </View>

      {isPending ? <LoadingState label="Loading storefront…" /> : null}
      {isError ? (
        <ErrorState
          title="Couldn't load this storefront"
          message={error instanceof Error ? error.message : undefined}
          onRetry={refetch}
        />
      ) : null}
      {!isPending && !isError && !tenant ? (
        <ErrorState title="Storefront not found" message="This partner may no longer be available." />
      ) : null}

      {tenant ? (
        <>
          {/* Brand Hero Showcase */}
          <View style={styles.showcaseCard}>
            {tenant.branding?.banner_url ? (
              <Image source={{ uri: tenant.branding.banner_url }} style={styles.banner} contentFit="contain" />
            ) : (
              <View style={styles.bannerFallback}>
                <MaterialIcons name="storefront" size={48} color={colors.primary} />
              </View>
            )}
          </View>

          {/* Identity & Stats Row */}
          <View style={styles.identitySection}>
            <View style={styles.identityTopRow}>
              {tenant.branding?.logo_url ? (
                <Image source={{ uri: tenant.branding.logo_url }} style={styles.logo} contentFit="cover" />
              ) : (
                <View style={styles.logoFallback}>
                  <MaterialIcons name="storefront" size={24} color={colors.primary} />
                </View>
              )}
              <View style={styles.identityTextGroup}>
                <View style={styles.nameRow}>
                  <Text style={styles.tenantName}>{tenant.tenant_name}</Text>
                  <MaterialIcons name="verified" size={18} color={colors.primary} />
                </View>
                <Text style={styles.meta}>
                  {pluralize(tenant.products.length, 'active channel')} • Verified Publisher
                </Text>
              </View>
            </View>
          </View>

          {/* Category Filter Chips */}
          {categoryCounts.length > 0 ? (
            <View style={styles.categoryRow}>
              {categoryCounts.map((entry) => {
                const active = entry.label === selectedCategory;
                return (
                  <AnimatedPressable
                    key={entry.label}
                    onPress={() => handleCategoryPress(entry.label)}
                    accessibilityRole="button"
                    accessibilityState={{ selected: active }}
                    style={[styles.categoryPill, active && styles.categoryPillActive]}
                  >
                    <Text style={[styles.categoryPillText, active && styles.categoryPillTextActive]}>
                      {entry.label} ({entry.count})
                    </Text>
                  </AnimatedPressable>
                );
              })}
            </View>
          ) : null}

          {/* Product Catalog List */}
          {visibleProducts.length === 0 ? (
            <Animated.View layout={LinearTransition.springify()}>
              <EmptyState icon="explore" title="No channels in this category" message="Try a different filter." />
            </Animated.View>
          ) : (
            <View style={styles.productList}>
              {visibleProducts.map((product) => (
                <Animated.View 
                  key={product.slug}
                  layout={LinearTransition.springify()}
                  entering={FadeIn}
                  exiting={FadeOut}
                >
                  <ProductRow product={product} />
                </Animated.View>
              ))}
            </View>
          )}
        </>
      ) : null}
    </ScreenContainer>
  );
}

const createStyles = (colors: ThemeColors) => StyleSheet.create({
  header: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginBottom: spacing.stackLg,
  },
  headerButton: {
    width: 38,
    height: 38,
    alignItems: 'center',
    justifyContent: 'center',
  },
  backIconCircle: {
    width: 36,
    height: 36,
    borderRadius: 18,
    backgroundColor: colors.surfaceContainerLowest,
    borderWidth: 1,
    borderColor: colors.cardBorder,
    alignItems: 'center',
    justifyContent: 'center',
  },
  headerSpacer: {
    width: 38,
  },
  headerTitle: {
    ...typography.headlineMd,
    fontSize: 17,
    fontWeight: '700',
    color: colors.onSurface,
    flex: 1,
    textAlign: 'center',
  },
  showcaseCard: {
    width: '100%',
    height: 140,
    borderRadius: radii.lg,
    overflow: 'hidden',
    backgroundColor: colors.surfaceContainerLowest,
    borderWidth: 1,
    borderColor: colors.cardBorder,
    marginBottom: 16,
    alignItems: 'center',
    justifyContent: 'center',
    padding: 12,
  },
  banner: {
    width: '100%',
    height: '100%',
  },
  bannerFallback: {
    width: '100%',
    height: '100%',
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: colors.primarySoft,
  },
  identitySection: {
    marginBottom: 20,
  },
  identityTopRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 12,
  },
  logo: {
    width: 48,
    height: 48,
    borderRadius: radii.md,
    backgroundColor: colors.surfaceContainerLowest,
    borderWidth: 1,
    borderColor: colors.cardBorder,
    flexShrink: 0,
  },
  logoFallback: {
    width: 48,
    height: 48,
    borderRadius: radii.md,
    backgroundColor: colors.primarySoft,
    borderWidth: 1,
    borderColor: colors.cardBorder,
    alignItems: 'center',
    justifyContent: 'center',
    flexShrink: 0,
  },
  identityTextGroup: {
    flex: 1,
    minWidth: 0,
    gap: 2,
  },
  nameRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 6,
  },
  tenantName: {
    ...typography.headlineMd,
    fontSize: 20,
    fontWeight: '800',
    color: colors.onSurface,
  },
  meta: {
    ...typography.labelSm,
    fontSize: 12,
    color: colors.onSurfaceVariant,
  },
  categoryRow: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: 8,
    marginBottom: 20,
  },
  categoryPill: {
    paddingHorizontal: 14,
    paddingVertical: 7,
    borderRadius: radii.full,
    borderWidth: 1,
    borderColor: colors.cardBorder,
    backgroundColor: colors.surfaceContainerLowest,
  },
  categoryPillActive: {
    backgroundColor: colors.primary,
    borderColor: colors.primary,
  },
  categoryPillText: {
    ...typography.labelSm,
    fontSize: 12,
    fontWeight: '600',
    color: colors.onSurfaceVariant,
  },
  categoryPillTextActive: {
    color: colors.onPrimary,
    fontWeight: '700',
  },
  productList: {
    gap: 12,
  },
});
