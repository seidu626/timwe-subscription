import { Redirect } from 'expo-router';

import { LoadingState } from '@/components/AsyncState';
import { colors } from '@/theme/tokens';
import { useAuth } from '@/context/AuthContext';
import { StyleSheet, View } from 'react-native';

export default function Index() {
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

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: colors.background,
  },
});
