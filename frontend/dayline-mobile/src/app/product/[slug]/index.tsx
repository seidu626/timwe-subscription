import { MaterialIcons } from '@expo/vector-icons';
import { Image } from 'expo-image';
import { router, useLocalSearchParams } from 'expo-router';
import { Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { Button } from '@/components/Button';
import { Card } from '@/components/Card';
import { ErrorState, LoadingState } from '@/components/AsyncState';
import { useSettings } from '@/context/SettingsContext';
import { useCatalogProduct } from '@/hooks/useCatalog';
import { colors, radii, spacing, typography } from '@/theme/tokens';
import { formatBillingCycle, formatCurrency } from '@/utils/format';

export default function ProductDetailScreen() {
  const { slug } = useLocalSearchParams<{ slug: string }>();
  const { isPending, isError, error, refetch, product } = useCatalogProduct(slug);
  const { dataSaverEnabled } = useSettings();

  return (
    <SafeAreaView style={styles.root} edges={['top', 'left', 'right']}>
      <View style={styles.header}>
        <Pressable onPress={() => router.back()} accessibilityRole="button" accessibilityLabel="Go back" style={styles.headerButton}>
          <MaterialIcons name="arrow-back" size={22} color={colors.onSurfaceVariant} />
        </Pressable>
        <Text style={styles.headerTitle}>{product?.name ?? 'Product'}</Text>
        <View style={styles.headerButton} />
      </View>

      {isPending ? <LoadingState label="Loading product…" /> : null}
      {isError ? (
        <ErrorState
          title="Couldn't load this product"
          message={error instanceof Error ? error.message : undefined}
          onRetry={refetch}
        />
      ) : null}
      {!isPending && !isError && !product ? (
        <ErrorState title="Product not found" message="This product may no longer be available." />
      ) : null}

      {product ? (
        <ScrollView contentContainerStyle={styles.scrollContent}>
          {product.artwork_url && !dataSaverEnabled ? (
            <Image source={{ uri: product.artwork_url }} style={styles.hero} contentFit="cover" />
          ) : (
            <View style={styles.heroPlaceholder}>
              <MaterialIcons name="menu-book" size={40} color={colors.primary} />
            </View>
          )}

          <Text style={styles.title}>
            {product.name} — {product.tagline}
          </Text>
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
        </ScrollView>
      ) : null}
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  root: {
    flex: 1,
    backgroundColor: colors.surface,
  },
  header: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    paddingHorizontal: spacing.stackMd,
    height: 56,
  },
  headerButton: {
    width: 40,
    height: 40,
    alignItems: 'center',
    justifyContent: 'center',
  },
  headerTitle: {
    ...typography.headlineMd,
    fontSize: 18,
    color: colors.primary,
  },
  scrollContent: {
    paddingHorizontal: spacing.containerMargin,
    paddingBottom: spacing.sectionGap,
    gap: spacing.stackLg,
  },
  hero: {
    width: '100%',
    height: 220,
    borderRadius: radii.md,
    backgroundColor: colors.surfaceVariant,
  },
  heroPlaceholder: {
    width: '100%',
    height: 220,
    borderRadius: radii.md,
    backgroundColor: colors.surfaceVariant,
    alignItems: 'center',
    justifyContent: 'center',
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
