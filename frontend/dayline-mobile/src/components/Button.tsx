import { useMemo } from 'react';
import { ActivityIndicator, Pressable, StyleSheet, Text, View, type StyleProp, type ViewStyle } from 'react-native';

import { radii, spacing, typography, type ThemeColors } from '@/theme/tokens';
import { useTheme } from '@/theme/ThemeContext';

type Variant = 'primary' | 'secondary' | 'accent' | 'text' | 'inverse';

interface ButtonProps {
  label: string;
  onPress: () => void;
  variant?: Variant;
  disabled?: boolean;
  loading?: boolean;
  icon?: React.ReactNode;
  style?: StyleProp<ViewStyle>;
  testID?: string;
}

export function Button({
  label,
  onPress,
  variant = 'primary',
  disabled,
  loading,
  icon,
  style,
  testID,
}: ButtonProps) {
  const { colors } = useTheme();
  const styles = useMemo(() => createStyles(colors), [colors]);
  const variantStyles = useMemo(() => createVariantStyles(colors), [colors]);
  const labelStyles = useMemo(() => createLabelStyles(colors), [colors]);
  const isDisabled = disabled || loading;
  return (
    <Pressable
      accessibilityRole="button"
      accessibilityState={{ disabled: isDisabled }}
      onPress={onPress}
      disabled={isDisabled}
      testID={testID}
      style={({ pressed }) => [
        styles.base,
        variantStyles[variant],
        isDisabled && styles.disabled,
        pressed && !isDisabled && styles.pressed,
        style,
      ]}
    >
      {loading ? (
        <ActivityIndicator color={variant === 'primary' || variant === 'accent' ? colors.onPrimary : colors.primary} />
      ) : (
        <View style={styles.content}>
          <Text style={[styles.label, labelStyles[variant]]}>{label}</Text>
          {icon}
        </View>
      )}
    </Pressable>
  );
}

const createStyles = (colors: ThemeColors) => StyleSheet.create({
  base: {
    paddingVertical: 16,
    paddingHorizontal: spacing.containerMargin,
    borderRadius: radii.md,
    alignItems: 'center',
    justifyContent: 'center',
  },
  content: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.base,
  },
  label: {
    ...typography.labelMd,
  },
  disabled: {
    opacity: 0.5,
  },
  pressed: {
    opacity: 0.85,
  },
});

const createVariantStyles = (colors: ThemeColors) => StyleSheet.create({
  primary: { backgroundColor: colors.primary },
  secondary: {
    backgroundColor: 'transparent',
    borderWidth: 1,
    borderColor: colors.primary,
  },
  accent: { backgroundColor: colors.secondaryContainer },
  text: { backgroundColor: 'transparent', paddingHorizontal: spacing.base },
  inverse: { backgroundColor: colors.onPrimary },
});

const createLabelStyles = (colors: ThemeColors) => StyleSheet.create({
  primary: { color: colors.onPrimary },
  secondary: { color: colors.primary },
  accent: { color: colors.onSecondaryContainer },
  text: { color: colors.primary },
  inverse: { color: colors.primaryContainer },
});
