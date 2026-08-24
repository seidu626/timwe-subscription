import { useMemo } from 'react';
import { Redirect } from 'expo-router';

import { LoadingState } from '@/components/AsyncState';
import { type ThemeColors } from '@/theme/tokens';
import { useTheme } from '@/theme/ThemeContext';
import { useAuth } from '@/context/AuthContext';
import { StyleSheet, View } from 'react-native';

export default function Index() {
  const { colors } = useTheme();
  const styles = useMemo(() => createStyles(colors), [colors]);
  const { status } = useAuth();

  if (status === 'loading') {
    return (
      <View style={styles.container}>
        <LoadingState label="" />
      </View>
    );
  }

  if (status === 'signedIn') {
    return <Redirect href="/(tabs)/today" />;
  }

  return <Redirect href="/(auth)/welcome" />;
}

const createStyles = (colors: ThemeColors) => StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: colors.background,
  },
});
