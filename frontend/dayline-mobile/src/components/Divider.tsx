import { useMemo } from 'react';
import { StyleSheet, View } from 'react-native';

import { type ThemeColors } from '@/theme/tokens';
import { useTheme } from '@/theme/ThemeContext';

export function Divider() {
  const { colors } = useTheme();
  const styles = useMemo(() => createStyles(colors), [colors]);
  return <View style={styles.divider} />;
}

const createStyles = (colors: ThemeColors) => StyleSheet.create({
  divider: {
    height: StyleSheet.hairlineWidth,
    backgroundColor: colors.outlineVariant,
    opacity: 0.6,
  },
});
