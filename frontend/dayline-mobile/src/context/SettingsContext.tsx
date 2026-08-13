import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import * as SecureStore from 'expo-secure-store';

const DATA_SAVER_KEY = 'dayline_data_saver_enabled';

interface SettingsContextValue {
  dataSaverEnabled: boolean;
  setDataSaverEnabled: (enabled: boolean) => void;
}

const SettingsContext = createContext<SettingsContextValue | null>(null);

export function SettingsProvider({ children }: { children: ReactNode }) {
  const [dataSaverEnabled, setDataSaverEnabledState] = useState(false);

  useEffect(() => {
    SecureStore.getItemAsync(DATA_SAVER_KEY).then((value) => {
      if (value === '1') setDataSaverEnabledState(true);
    });
  }, []);

  function setDataSaverEnabled(enabled: boolean) {
    setDataSaverEnabledState(enabled);
    SecureStore.setItemAsync(DATA_SAVER_KEY, enabled ? '1' : '0').catch(() => undefined);
  }

  const value = useMemo(() => ({ dataSaverEnabled, setDataSaverEnabled }), [dataSaverEnabled]);

  return <SettingsContext.Provider value={value}>{children}</SettingsContext.Provider>;
}

export function useSettings(): SettingsContextValue {
  const ctx = useContext(SettingsContext);
  if (!ctx) {
    throw new Error('useSettings must be used within a SettingsProvider');
  }
  return ctx;
}
