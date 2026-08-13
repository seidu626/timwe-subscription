import { StyleSheet, View } from 'react-native';

import { colors } from '@/theme/tokens';

export function Divider() {
  return <View style={styles.divider} />;
}

const styles = StyleSheet.create({
  divider: {
    height: StyleSheet.hairlineWidth,
    backgroundColor: colors.outlineVariant,
    opacity: 0.6,
  },
});
