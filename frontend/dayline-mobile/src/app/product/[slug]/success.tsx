import { MaterialIcons } from '@expo/vector-icons';
import { router, useLocalSearchParams } from 'expo-router';
import { StyleSheet, Text, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { Button } from '@/components/Button';
import { useCatalogProduct } from '@/hooks/useCatalog';
import { colors, radii, spacing, typography } from '@/theme/tokens';

export default function SubscriptionSuccessScreen() {
  const { slug } = useLocalSearchParams<{ slug: string }>();
  const { product } = useCatalogProduct(slug);

  return (
    <SafeAreaView style={styles.root} edges={['top', 'bottom', 'left', 'right']}>
      <View style={styles.content}>
        <View style={styles.iconCircle}>
          <MaterialIcons name="check" size={48} color={colors.onPrimary} />
        </View>
        <Text style={styles.title}>You&apos;re in!</Text>
        <Text style={styles.subtitle}>
          {product ? `You're subscribed to ${product.name}.` : 'Your subscription is active.'} New content will
          appear in your Today feed.
        </Text>

        <View style={styles.actions}>
          <Button
            label="Explore more products"
            onPress={() => router.replace('/(tabs)/discover')}
            style={styles.actionButton}
          />
          <Button
            label="Manage anytime in Subscriptions"
            variant="secondary"
            onPress={() => router.replace('/(tabs)/subscriptions')}
            style={styles.actionButton}
          />
        </View>
      </View>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  root: {
    flex: 1,
    backgroundColor: colors.surface,
  },
  content: {
    flex: 1,
    alignItems: 'center',
    justifyContent: 'center',
    paddingHorizontal: spacing.sectionGap,
    gap: spacing.stackMd,
  },
  iconCircle: {
    width: 96,
    height: 96,
    borderRadius: radii.full,
    backgroundColor: colors.primary,
    alignItems: 'center',
    justifyContent: 'center',
    marginBottom: spacing.stackMd,
  },
  title: {
    ...typography.displayLg,
    fontSize: 32,
    color: colors.onSurface,
    textAlign: 'center',
  },
  subtitle: {
    ...typography.bodyLg,
    color: colors.onSurfaceVariant,
    textAlign: 'center',
  },
  actions: {
    width: '100%',
    gap: spacing.stackMd,
    marginTop: spacing.sectionGap - spacing.stackLg,
  },
  actionButton: {
    width: '100%',
  },
});
