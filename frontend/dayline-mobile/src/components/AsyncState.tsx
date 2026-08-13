import { MaterialIcons } from '@expo/vector-icons';
import { ActivityIndicator, StyleSheet, Text, View } from 'react-native';

import { Button } from './Button';
import { colors, spacing, typography } from '@/theme/tokens';

interface StateProps {
  title: string;
  message?: string;
}

export function LoadingState({ label = 'Loading…' }: { label?: string }) {
  return (
    <View style={styles.container}>
      <ActivityIndicator color={colors.primary} size="large" />
      <Text style={styles.message}>{label}</Text>
    </View>
  );
}

export function EmptyState({ title, message, icon = 'inbox' }: StateProps & { icon?: keyof typeof MaterialIcons.glyphMap }) {
  return (
    <View style={styles.container}>
      <MaterialIcons name={icon} size={40} color={colors.outline} />
      <Text style={styles.title}>{title}</Text>
      {message ? <Text style={styles.message}>{message}</Text> : null}
    </View>
  );
}

export function ErrorState({ title, message, onRetry }: StateProps & { onRetry?: () => void }) {
  return (
    <View style={styles.container}>
      <MaterialIcons name="error-outline" size={40} color={colors.error} />
      <Text style={styles.title}>{title}</Text>
      {message ? <Text style={styles.message}>{message}</Text> : null}
      {onRetry ? <Button label="Try again" variant="secondary" onPress={onRetry} style={styles.retry} /> : null}
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    alignItems: 'center',
    justifyContent: 'center',
    paddingHorizontal: spacing.containerMargin,
    paddingVertical: spacing.sectionGap,
    gap: spacing.stackSm,
  },
  title: {
    ...typography.headlineMd,
    color: colors.onSurface,
    textAlign: 'center',
  },
  message: {
    ...typography.bodyMd,
    color: colors.onSurfaceVariant,
    textAlign: 'center',
  },
  retry: {
    marginTop: spacing.stackMd,
    minWidth: 160,
  },
});
