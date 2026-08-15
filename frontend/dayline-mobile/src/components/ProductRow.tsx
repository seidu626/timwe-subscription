import { MaterialIcons } from '@expo/vector-icons';
import { Link } from 'expo-router';
import { Pressable, StyleSheet, Text, View } from 'react-native';

import { Card } from '@/components/Card';
import { colors, radii, spacing, typography } from '@/theme/tokens';
import { formatBillingCycle, formatCurrency } from '@/utils/format';
import type { CatalogProduct } from '@/api/types';

// Shared by Discover and the tenant storefront so both render products
// identically. flex:1 + minWidth:0 on the text wrapper lets long product
// names shrink instead of overlapping the price chip/chevron (minWidth:0 is
// required on web; flex children otherwise never shrink below their
// intrinsic content width).
export function ProductRow({ product }: { product: CatalogProduct }) {
  return (
    <Link href={{ pathname: '/product/[slug]', params: { slug: product.slug } }} asChild>
      <Pressable accessibilityRole="button">
        <Card style={styles.productRow}>
          <View style={styles.productLeft}>
            <View style={styles.productIcon}>
              <MaterialIcons name="menu-book" size={22} color={colors.primary} />
            </View>
            <View style={styles.productTextGroup}>
              <Text style={styles.productName} numberOfLines={2} ellipsizeMode="tail">
                {product.name}
              </Text>
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
      </Pressable>
    </Link>
  );
}

const styles = StyleSheet.create({
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
