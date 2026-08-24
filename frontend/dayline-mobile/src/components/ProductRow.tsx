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
        <Card style={styles.productRow}>
          <View style={styles.productLeft}>
            {product.artwork_url ? (
              <Animated.View sharedTransitionTag={`product-hero-${product.slug}`}>
                <Image source={{ uri: product.artwork_url }} style={styles.productArtwork} contentFit="cover" />
              </Animated.View>
            ) : (
              <Animated.View sharedTransitionTag={`product-hero-fallback-${product.slug}`} style={styles.productIcon}>
                <MaterialIcons name="menu-book" size={22} color={colors.primary} />
              </Animated.View>
            )}
            <View style={styles.productTextGroup}>
              <Animated.Text sharedTransitionTag={`product-title-${product.slug}`} style={styles.productName} numberOfLines={2} ellipsizeMode="tail">
                {formatProductName(product.name)}
              </Animated.Text>
              <Text style={styles.productTagline} numberOfLines={1} ellipsizeMode="tail">
                {product.tagline}
              </Text>
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
      </AnimatedPressable>
    </Link>
  );
}

const createStyles = (colors: ThemeColors) => StyleSheet.create({
  productRow: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
  },
  productLeft: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.stackMd,
    flex: 1,
    minWidth: 0,
  },
  productIcon: {
    width: 48,
    height: 48,
    borderRadius: radii.md,
    backgroundColor: colors.surfaceVariant,
    alignItems: 'center',
    justifyContent: 'center',
    flexShrink: 0,
  },
  productArtwork: {
    width: 48,
    height: 48,
    borderRadius: radii.md,
    backgroundColor: colors.surfaceVariant,
    flexShrink: 0,
  },
  productTextGroup: {
    flex: 1,
    minWidth: 0,
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
    flexShrink: 0,
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
