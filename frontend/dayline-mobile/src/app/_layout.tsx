import { useEffect, useMemo } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import {
  PlusJakartaSans_400Regular,
  PlusJakartaSans_500Medium,
  PlusJakartaSans_600SemiBold,
  PlusJakartaSans_700Bold,
  useFonts,
} from '@expo-google-fonts/plus-jakarta-sans';
import { Stack } from 'expo-router';
import * as SplashScreen from 'expo-splash-screen';
import { StatusBar } from 'expo-status-bar';
import { StyleSheet, View } from 'react-native';
import { SafeAreaProvider } from 'react-native-safe-area-context';

import { LoadingState } from '@/components/AsyncState';
import { AuthProvider, useAuth } from '@/context/AuthContext';
import { SettingsProvider } from '@/context/SettingsContext';
import { type ThemeColors } from '@/theme/tokens';
import { ThemeProvider, useTheme } from '@/theme/ThemeContext';

SplashScreen.preventAutoHideAsync().catch(() => undefined);

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { retry: 1, staleTime: 30_000 },
  },
});

function RootNavigator() {
  const { status } = useAuth();
  const { colors } = useTheme();
  const styles = useMemo(() => createStyles(colors), [colors]);

  // Auth status resolves asynchronously (session read from storage). Until it
  // does, hold off on mounting the Stack at all: Stack.Protected registers
  // only the (auth) screens while status === 'loading' (its guard is
  // `status !== 'signedIn'`), so a cold-start deep link into a protected
  // route (e.g. /product/[slug], /discover, /tenant/[tenantKey]) can't be
  // matched at initial-URL resolution time and falls back to the default
  // route before the real status - and the real screen set - is known.
  // Waiting here lets expo-router resolve the deep link against the correct,
  // final screen set on first mount.
  if (status === 'loading') {
    return (
      <View style={styles.container}>
        <LoadingState label="" />
      </View>
    );
  }

  return (
    <Stack screenOptions={{ headerShown: false, contentStyle: { backgroundColor: colors.surface } }}>
      <Stack.Protected guard={status === 'signedIn'}>
        <Stack.Screen name="(tabs)" />
        <Stack.Screen name="product/[slug]/index" />
        <Stack.Screen name="product/[slug]/confirm" options={{ presentation: 'formSheet' }} />
        <Stack.Screen name="product/[slug]/success" options={{ presentation: 'fullScreenModal' }} />
        <Stack.Screen name="content/[id]" />
        <Stack.Screen name="tenant/[tenantKey]" />
        <Stack.Screen name="notifications" />
      </Stack.Protected>
      <Stack.Protected guard={status !== 'signedIn'}>
        <Stack.Screen name="(auth)" />
      </Stack.Protected>
    </Stack>
  );
}

const createStyles = (colors: ThemeColors) => StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: colors.surface,
  },
});

function ThemedShell() {
  const { scheme } = useTheme();
  return (
    <>
      <RootNavigator />
      <StatusBar style={scheme === 'dark' ? 'light' : 'dark'} />
    </>
  );
}

export default function RootLayout() {
  const [fontsLoaded, fontError] = useFonts({
    PlusJakartaSans_400Regular,
    PlusJakartaSans_500Medium,
    PlusJakartaSans_600SemiBold,
    PlusJakartaSans_700Bold,
  });

  useEffect(() => {
    if (fontsLoaded || fontError) {
      SplashScreen.hideAsync().catch(() => undefined);
    }
  }, [fontsLoaded, fontError]);

  if (!fontsLoaded && !fontError) {
    return null;
  }

  return (
    <SafeAreaProvider>
      <QueryClientProvider client={queryClient}>
        <AuthProvider>
          <SettingsProvider>
            <ThemeProvider>
              <ThemedShell />
            </ThemeProvider>
          </SettingsProvider>
        </AuthProvider>
      </QueryClientProvider>
    </SafeAreaProvider>
  );
}
