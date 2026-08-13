import { useState } from 'react';
import { StyleSheet, Text, TextInput, View } from 'react-native';

import { colors, focusRing, radii, spacing, typography } from '@/theme/tokens';
import { formatLocalInput } from '@/utils/phone';

interface PhoneInputProps {
  value: string;
  onChange: (value: string) => void;
  autoFocus?: boolean;
}

export function PhoneInput({ value, onChange, autoFocus }: PhoneInputProps) {
  const [focused, setFocused] = useState(false);

  return (
    <View style={[styles.group, focused && styles.groupFocused]}>
      <View style={styles.prefix}>
        <Text style={styles.flag}>🇬🇭</Text>
        <Text style={styles.prefixText}>+233</Text>
      </View>
      <TextInput
        value={value}
        onChangeText={(text) => onChange(formatLocalInput(text))}
        onFocus={() => setFocused(true)}
        onBlur={() => setFocused(false)}
        placeholder="24 123 4567"
        placeholderTextColor={colors.outlineVariant}
        keyboardType="phone-pad"
        autoComplete="tel-national"
        autoFocus={autoFocus}
        style={styles.input}
        accessibilityLabel="Phone number"
        testID="phone-input"
      />
    </View>
  );
}

const styles = StyleSheet.create({
  group: {
    flexDirection: 'row',
    alignItems: 'center',
    backgroundColor: colors.surfaceBright,
    borderWidth: 1,
    borderColor: colors.cardBorder,
    borderRadius: radii.md,
    overflow: 'hidden',
  },
  groupFocused: {
    borderColor: focusRing.color,
    borderWidth: focusRing.width,
  },
  prefix: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.stackSm,
    paddingHorizontal: spacing.stackMd,
    paddingVertical: 16,
    borderRightWidth: 1,
    borderRightColor: colors.cardBorder,
    backgroundColor: colors.surfaceContainerLow,
  },
  flag: {
    fontSize: 18,
  },
  prefixText: {
    ...typography.bodyLg,
    color: colors.onSurface,
    fontFamily: typography.labelMd.fontFamily,
  },
  input: {
    flex: 1,
    paddingVertical: 16,
    paddingHorizontal: spacing.stackMd,
    color: colors.onSurface,
    ...typography.bodyLg,
  },
});
