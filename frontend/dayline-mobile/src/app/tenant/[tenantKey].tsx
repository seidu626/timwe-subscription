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

// Category chips only earn their place once a storefront is large enough
// that browsing the flat list stops being the fastest path; small catalogs
// stay chip-free.
const CATEGORY_FILTER_THRESHOLD = 15;
const ALL_CATEGORY = 'All';

export default function TenantStorefrontScreen() {
  const { colors } = useTheme();
  const styles = useMemo(() => createStyles(colors), [colors]);
  const { tenantKey } = useLocalSearchParams<{ tenantKey: string }>();
  const { isPending, isError, error, refetch, tenant } = useMarketplaceTenant(tenantKey);
  const [selectedCategory, setSelectedCategory] = useState(ALL_CATEGORY);

  const categoryCounts = useMemo(() => {
    if (!tenant || tenant.products.length <= CATEGORY_FILTER_THRESHOLD) return [];
    const counts = new Map<string, number>();
    for (const product of tenant.products) {
      if (!product.category) continue;
      counts.set(product.category, (counts.get(product.category) ?? 0) + 1);
    }
    if (counts.size === 0) return [];
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
    <ScreenContainer scroll>
      <View style={styles.header}>
        <AnimatedPressable onPress={() => router.back()} accessibilityRole="button" accessibilityLabel="Go back" style={styles.headerButton}>
          <MaterialIcons name="arrow-back" size={22} color={colors.onSurfaceVariant} />
        </AnimatedPressable>
        <Text style={styles.headerTitle} numberOfLines={1} ellipsizeMode="tail">
          {tenant?.tenant_name ?? 'Storefront'}
        </Text>
        <View style={styles.headerButton} />
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
        <ErrorState title="Storefront not found" message="This tenant may no longer be available." />
      ) : null}

      {tenant ? (
        <>
          {tenant.branding?.banner_url ? (
            <Image source={{ uri: tenant.branding.banner_url }} style={styles.banner} contentFit="cover" />
          ) : null}

          <View style={styles.identityRow}>
            {tenant.branding?.logo_url ? (
              <Image source={{ uri: tenant.branding.logo_url }} style={styles.logo} contentFit="cover" />
            ) : null}
            <Text style={styles.meta}>{pluralize(tenant.products.length, 'product')}</Text>
          </View>

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

          {visibleProducts.length === 0 ? (
            <Animated.View layout={LinearTransition.springify()}>
              <EmptyState icon="explore" title="No products in this category" message="Try a different filter." />
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
    gap: spacing.stackSm,
    marginBottom: spacing.stackLg,
  },
  headerButton: {
    width: 40,
    height: 40,
    alignItems: 'center',
    justifyContent: 'center',
    flexShrink: 0,
  },
  headerTitle: {
    ...typography.headlineMd,
    fontSize: 18,
    color: colors.primary,
    flex: 1,
    minWidth: 0,
    textAlign: 'center',
  },
  banner: {
    width: '100%',
    height: 120,
    borderRadius: radii.md,
    backgroundColor: colors.surfaceVariant,
    marginBottom: spacing.stackMd,
  },
  identityRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.stackSm,
    marginBottom: spacing.stackLg,
  },
  logo: {
    width: 28,
    height: 28,
    borderRadius: radii.md,
    backgroundColor: colors.surfaceVariant,
    flexShrink: 0,
  },
  meta: {
    ...typography.labelSm,
    color: colors.onSurfaceVariant,
  },
  categoryRow: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: spacing.stackSm,
    marginBottom: spacing.stackLg,
  },
  categoryPill: {
    paddingHorizontal: spacing.stackMd,
    paddingVertical: 8,
    borderRadius: radii.full,
    borderWidth: 1,
    borderColor: colors.outlineVariant,
  },
  categoryPillActive: {
    backgroundColor: colors.primary,
    borderColor: colors.primary,
  },
  categoryPillText: {
    ...typography.labelSm,
    color: colors.onSurfaceVariant,
  },
  categoryPillTextActive: {
    color: colors.onPrimary,
  },
  productList: {
    gap: spacing.stackMd,
  },
});
