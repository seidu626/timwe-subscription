import { useMemo } from 'react';
import { MaterialIcons } from '@expo/vector-icons';
import { Image } from 'expo-image';
import { Link } from 'expo-router';
import { StyleSheet, Text, View } from 'react-native';
import Animated from 'react-native-reanimated';

import { Card } from '@/components/Card';
import { AnimatedPressable } from '@/components/AnimatedPressable';
import { radii, spacing, typography, type ThemeColors } from '@/theme/tokens';
import { useTheme } from '@/theme/ThemeContext';
import { formatBillingCycle, formatCurrency, formatProductName } from '@/utils/format';
import type { CatalogProduct } from '@/api/types';

export function ProductRow({ product }: { product: CatalogProduct }) {
  const { colors } = useTheme();
  const styles = useMemo(() => createStyles(colors), [colors]);
  return (
    <Link href={{ pathname: '/product/[slug]', params: { slug: product.slug } }} asChild>
      <AnimatedPressable accessibilityRole="button">
        <Card style={styles.productRow} padded={false}>
          <View style={styles.contentContainer}>
            <View style={styles.productLeft}>
              {product.artwork_url ? (
                <Animated.View sharedTransitionTag={`product-hero-${product.slug}`} style={styles.artworkContainer}>
                  <Image source={{ uri: product.artwork_url }} style={styles.productArtwork} contentFit="cover" />
                </Animated.View>
              ) : (
                <Animated.View sharedTransitionTag={`product-hero-fallback-${product.slug}`} style={styles.productIcon}>
                  <MaterialIcons name="menu-book" size={24} color={colors.primary} />
                </Animated.View>
              )}
              <View style={styles.productTextGroup}>
                {product.category ? (
                  <Text style={styles.categoryTag}>{product.category.toUpperCase()}</Text>
                ) : null}
                <Animated.Text sharedTransitionTag={`product-title-${product.slug}`} style={styles.productName} numberOfLines={2} ellipsizeMode="tail">
                  {formatProductName(product.name)}
                </Animated.Text>
                {product.tagline ? (
                  <Text style={styles.productTagline} numberOfLines={1} ellipsizeMode="tail">
                    {product.tagline}
                  </Text>
                ) : null}
              </View>
            </View>
            <View style={styles.productRight}>
              <View style={styles.priceChip}>
                <Text style={styles.priceChipText}>
                  {formatCurrency(product.price, product.currency)}
                  {formatBillingCycle(product.billing_cycle)}
                </Text>
              </View>
              <MaterialIcons name="chevron-right" size={20} color={colors.outline} />
            </View>
          </View>
        </Card>
      </AnimatedPressable>
    </Link>
  );
}

const createStyles = (colors: ThemeColors) => StyleSheet.create({
  productRow: {
    overflow: 'hidden',
  },
  contentContainer: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    paddingHorizontal: 16,
    paddingVertical: 14,
    gap: spacing.stackMd,
  },
  productLeft: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 14,
    flex: 1,
    minWidth: 0,
  },
  artworkContainer: {
    width: 54,
    height: 54,
    borderRadius: radii.md,
    overflow: 'hidden',
    borderWidth: 1,
    borderColor: colors.cardBorder,
    backgroundColor: colors.surfaceVariant,
    flexShrink: 0,
  },
  productArtwork: {
    width: '100%',
    height: '100%',
  },
  productIcon: {
    width: 54,
    height: 54,
    borderRadius: radii.md,
    backgroundColor: colors.primarySoft,
    borderWidth: 1,
    borderColor: colors.cardBorder,
    alignItems: 'center',
    justifyContent: 'center',
    flexShrink: 0,
  },
  productTextGroup: {
    flex: 1,
    minWidth: 0,
    gap: 2,
  },
  categoryTag: {
    ...typography.labelSm,
    fontSize: 10,
    lineHeight: 12,
    letterSpacing: 0.6,
    color: colors.primary,
    fontWeight: '700',
    marginBottom: 2,
  },
  productName: {
    ...typography.headlineMd,
    fontSize: 15,
    lineHeight: 20,
    fontWeight: '700',
    color: colors.onSurface,
  },
  productTagline: {
    ...typography.bodyMd,
    fontSize: 13,
    lineHeight: 18,
    color: colors.onSurfaceVariant,
  },
  productRight: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
    flexShrink: 0,
  },
  priceChip: {
    backgroundColor: colors.secondaryContainer,
    borderWidth: 1,
    borderColor: 'rgba(245, 158, 11, 0.3)',
    borderRadius: radii.full,
    paddingHorizontal: 10,
    paddingVertical: 4,
  },
  priceChipText: {
    ...typography.labelSm,
    fontSize: 11,
    lineHeight: 15,
    fontWeight: '600',
    color: colors.secondary,
  },
});
