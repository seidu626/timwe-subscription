import { useMemo } from 'react';
import { StyleSheet, Text, View } from 'react-native';

import { radii, typography, type ThemeColors } from '@/theme/tokens';
import { useTheme } from '@/theme/ThemeContext';

export function Chip({ label, tone = 'neutral' }: { label: string; tone?: 'neutral' | 'accent' | 'primary' }) {
  const { colors } = useTheme();
  const styles = useMemo(() => createStyles(colors), [colors]);
  const toneStyles = useMemo(() => createToneStyles(colors), [colors]);
  const toneLabelStyles = useMemo(() => createToneLabelStyles(colors), [colors]);
  return (
    <View style={[styles.chip, toneStyles[tone]]}>
      <Text style={[styles.label, toneLabelStyles[tone]]}>{label}</Text>
    </View>
  );
}

const createStyles = (colors: ThemeColors) => StyleSheet.create({
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

const createToneStyles = (colors: ThemeColors) => StyleSheet.create({
  neutral: { backgroundColor: colors.surfaceContainerHigh },
  accent: { backgroundColor: 'rgba(253,183,65,0.2)', borderWidth: 1, borderColor: 'rgba(253,183,65,0.3)' },
  primary: { backgroundColor: colors.primaryContainer },
});

const createToneLabelStyles = (colors: ThemeColors) => StyleSheet.create({
  neutral: { color: colors.onSurfaceVariant },
  accent: { color: colors.secondary },
  primary: { color: colors.onPrimaryContainer },
});
