import { useMemo } from 'react';
import { MaterialIcons } from '@expo/vector-icons';
import { router, useLocalSearchParams } from 'expo-router';
import { StyleSheet, Text, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { Button } from '@/components/Button';
import { useCatalogProduct } from '@/hooks/useCatalog';
import { spacing, typography, type ThemeColors } from '@/theme/tokens';
import { useTheme } from '@/theme/ThemeContext';
import { formatProductName } from '@/utils/format';

export default function SubscriptionSuccessScreen() {
  const { colors } = useTheme();
  const styles = useMemo(() => createStyles(colors), [colors]);
  const { slug } = useLocalSearchParams<{ slug: string }>();
  const { product } = useCatalogProduct(slug);

  return (
    <SafeAreaView style={styles.root} edges={['top', 'bottom', 'left', 'right']}>
      <View style={styles.content}>
        <View style={styles.iconCircleOuter}>
          <View style={styles.iconCircle}>
            <MaterialIcons name="check" size={44} color={colors.onPrimary} />
          </View>
        </View>

        <Text style={styles.title}>You&apos;re All Set!</Text>
        <Text style={styles.subtitle}>
          {product ? `You're now subscribed to ${formatProductName(product.name)}.` : 'Your subscription is now active.'} Fresh content will be delivered to your Today feed daily.
        </Text>

        <View style={styles.actions}>
          <Button
            label="Go to Today's Feed"
            onPress={() => router.replace('/(tabs)/today')}
            icon={<MaterialIcons name="arrow-forward" size={18} color={colors.onPrimary} />}
            style={styles.actionButton}
          />
          <Button
            label="Browse More Channels"
            variant="secondary"
            onPress={() => router.replace('/(tabs)/discover')}
            style={styles.actionButton}
          />
        </View>
      </View>
    </SafeAreaView>
  );
}

const createStyles = (colors: ThemeColors) => StyleSheet.create({
  root: {
    flex: 1,
    backgroundColor: colors.background,
  },
  content: {
    flex: 1,
    alignItems: 'center',
    justifyContent: 'center',
    paddingHorizontal: spacing.sectionGap,
    gap: 16,
  },
  iconCircleOuter: {
    width: 104,
    height: 104,
    borderRadius: 52,
    backgroundColor: colors.primarySoft,
    borderWidth: 1,
    borderColor: 'rgba(52, 211, 153, 0.3)',
    alignItems: 'center',
    justifyContent: 'center',
    marginBottom: 8,
  },
  iconCircle: {
    width: 80,
    height: 80,
    borderRadius: 40,
    backgroundColor: colors.primary,
    alignItems: 'center',
    justifyContent: 'center',
    shadowColor: '#10b981',
    shadowOpacity: 0.3,
    shadowRadius: 16,
    shadowOffset: { width: 0, height: 4 },
    elevation: 6,
  },
  title: {
    ...typography.displayLg,
    fontSize: 30,
    fontWeight: '800',
    color: colors.onSurface,
    textAlign: 'center',
  },
  subtitle: {
    ...typography.bodyLg,
    fontSize: 15,
    lineHeight: 22,
    color: colors.onSurfaceVariant,
    textAlign: 'center',
    paddingHorizontal: 8,
  },
  actions: {
    width: '100%',
    gap: 12,
    marginTop: 24,
  },
  actionButton: {
    width: '100%',
  },
});
