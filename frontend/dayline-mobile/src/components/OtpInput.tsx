import { useRef, useState } from 'react';
import { StyleSheet, TextInput, View } from 'react-native';

import { colors, focusRing, radii, typography } from '@/theme/tokens';

const LENGTH = 6;

interface OtpInputProps {
  value: string;
  onChange: (value: string) => void;
  autoFocus?: boolean;
}

export function OtpInput({ value, onChange, autoFocus }: OtpInputProps) {
  const inputs = useRef<Array<TextInput | null>>([]);
  const [focusedIndex, setFocusedIndex] = useState<number | null>(autoFocus ? 0 : null);
  const digits = value.padEnd(LENGTH, ' ').split('').slice(0, LENGTH);

  function setDigitAt(index: number, digit: string) {
    const next = digits.slice();
    next[index] = digit || ' ';
    onChange(next.join('').replace(/\s+$/, ''));
  }

  function fillFrom(index: number, incoming: string) {
    const next = digits.slice();
    let cursor = index;
    for (const ch of incoming) {
      if (cursor >= LENGTH) break;
      next[cursor] = ch;
      cursor += 1;
    }
    onChange(next.join('').replace(/\s+$/, ''));
    inputs.current[Math.min(cursor, LENGTH - 1)]?.focus();
  }

  function handleChangeText(index: number, text: string) {
    const cleaned = text.replace(/\D/g, '');
    // Keyboard autofill and paste deliver the whole code into one box -
    // ignore the digit already rendered there, then distribute the rest.
    const existing = digits[index].trim();
    const incoming =
      existing && cleaned.startsWith(existing) && cleaned.length > 1 ? cleaned.slice(1) : cleaned;
    if (incoming.length >= LENGTH) {
      onChange(incoming.slice(0, LENGTH));
      inputs.current[LENGTH - 1]?.focus();
      return;
    }
    if (incoming.length > 1) {
      fillFrom(index, incoming);
      return;
    }
    const digit = incoming.slice(-1);
    setDigitAt(index, digit);
    if (digit && index < LENGTH - 1) {
      inputs.current[index + 1]?.focus();
    }
  }

  function handleKeyPress(index: number, key: string) {
    if (key === 'Backspace' && !digits[index]?.trim() && index > 0) {
      inputs.current[index - 1]?.focus();
      setDigitAt(index - 1, '');
    }
  }

  return (
    <View style={styles.row}>
      {digits.map((digit, index) => (
        <TextInput
          // eslint-disable-next-line react/no-array-index-key
          key={index}
          ref={(ref) => {
            inputs.current[index] = ref;
          }}
          value={digit.trim()}
          onChangeText={(text) => handleChangeText(index, text)}
          onKeyPress={({ nativeEvent }) => handleKeyPress(index, nativeEvent.key)}
          onFocus={() => setFocusedIndex(index)}
          onBlur={() => setFocusedIndex((current) => (current === index ? null : current))}
          keyboardType="number-pad"
          autoComplete="sms-otp"
          textContentType="oneTimeCode"
          maxLength={LENGTH}
          autoFocus={autoFocus && index === 0}
          style={[styles.box, focusedIndex === index && styles.boxFocused]}
          accessibilityLabel={`Digit ${index + 1} of ${LENGTH}`}
          testID={`otp-digit-${index}`}
        />
      ))}
    </View>
  );
}

const styles = StyleSheet.create({
  row: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    width: '100%',
  },
  box: {
    width: 48,
    height: 56,
    textAlign: 'center',
    backgroundColor: colors.surfaceBright,
    borderWidth: 1,
    borderColor: colors.surfaceVariant,
    borderRadius: radii.md,
    color: colors.onSurface,
    ...typography.headlineMd,
  },
  // Fix-in-implementation: emerald (brand primary) focus ring, not blue.
  boxFocused: {
    borderColor: focusRing.color,
    borderWidth: focusRing.width,
  },
});
