import { StyleSheet, Text, View } from 'react-native';

import { colors, radii, typography } from '@/theme/tokens';

export function Chip({ label, tone = 'neutral' }: { label: string; tone?: 'neutral' | 'accent' | 'primary' }) {
  return (
    <View style={[styles.chip, toneStyles[tone]]}>
      <Text style={[styles.label, toneLabelStyles[tone]]}>{label}</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  chip: {
    paddingHorizontal: 12,
    paddingVertical: 4,
    borderRadius: radii.full,
    alignSelf: 'flex-start',
  },
  label: {
    ...typography.labelSm,
  },
});

const toneStyles = StyleSheet.create({
  neutral: { backgroundColor: colors.surfaceContainerHigh },
  accent: { backgroundColor: 'rgba(253,183,65,0.2)', borderWidth: 1, borderColor: 'rgba(253,183,65,0.3)' },
  primary: { backgroundColor: colors.primaryContainer },
});

const toneLabelStyles = StyleSheet.create({
  neutral: { color: colors.onSurfaceVariant },
  accent: { color: colors.secondary },
  primary: { color: colors.onPrimaryContainer },
});
