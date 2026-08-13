import * as SecureStore from 'expo-secure-store';
import { Platform } from 'react-native';

// expo-secure-store has no web implementation and throws at runtime in the
// browser. Web builds (dev preview / react-native-web) fall back to
// localStorage; native builds keep hardware-backed SecureStore.
const isWeb = Platform.OS === 'web';

export async function getItemAsync(key: string): Promise<string | null> {
  if (isWeb) {
    return typeof localStorage === 'undefined' ? null : localStorage.getItem(key);
  }
  return SecureStore.getItemAsync(key);
}

export async function setItemAsync(key: string, value: string): Promise<void> {
  if (isWeb) {
    if (typeof localStorage !== 'undefined') localStorage.setItem(key, value);
    return;
  }
  return SecureStore.setItemAsync(key, value);
}

export async function deleteItemAsync(key: string): Promise<void> {
  if (isWeb) {
    if (typeof localStorage !== 'undefined') localStorage.removeItem(key);
    return;
  }
  return SecureStore.deleteItemAsync(key);
}
