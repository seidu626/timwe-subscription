import { useMemo } from 'react';
import { StyleSheet, View, type StyleProp, type ViewStyle } from 'react-native';

import { radii, shadow, spacing, type ThemeColors } from '@/theme/tokens';
import { useTheme } from '@/theme/ThemeContext';

export type CardVariant = 'default' | 'elevated' | 'outlined' | 'glow' | 'accent';

interface CardProps {
  children: React.ReactNode;
  style?: StyleProp<ViewStyle>;
  padded?: boolean;
  variant?: CardVariant;
}

export function Card({ children, style, padded = true, variant = 'default' }: CardProps) {
  const { colors } = useTheme();
  const styles = useMemo(() => createStyles(colors), [colors]);
  
  const variantStyle = useMemo(() => {
    switch (variant) {
      case 'elevated':
        return styles.elevated;
      case 'outlined':
        return styles.outlined;
      case 'glow':
        return styles.glow;
      case 'accent':
        return styles.accent;
      default:
        return styles.default;
    }
  }, [variant, styles]);

  return <View style={[styles.card, variantStyle, padded && styles.padded, style]}>{children}</View>;
}

const createStyles = (colors: ThemeColors) => StyleSheet.create({
  card: {
    backgroundColor: colors.surfaceContainerLowest,
    borderRadius: radii.lg,
    borderWidth: 1,
    borderColor: colors.cardBorder,
    ...shadow.card,
  },
  default: {},
  elevated: {
    backgroundColor: colors.surfaceContainerLow,
    borderColor: colors.outlineVariant,
  },
  outlined: {
    backgroundColor: 'transparent',
    borderColor: colors.cardBorder,
    shadowOpacity: 0,
    elevation: 0,
  },
  glow: {
    borderColor: colors.primary,
    ...shadow.glow,
  },
  accent: {
    backgroundColor: colors.secondaryContainer,
    borderColor: 'rgba(245, 158, 11, 0.3)',
  },
  padded: {
    padding: spacing.containerMargin,
  },
});
