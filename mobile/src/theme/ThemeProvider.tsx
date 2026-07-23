import React, { createContext, useContext, useMemo } from 'react';
import { useColorScheme } from 'react-native';
import { tokens, ThemeTokens } from './tokens';
import { useUIStore } from '@/stores/uiStore';

interface ThemeContextType {
  theme: ThemeTokens;
  isDark: boolean;
}

const ThemeContext = createContext<ThemeContextType>({
  theme: tokens.light,
  isDark: false,
});

export const ThemeProvider = ({ children }: { children: React.ReactNode }) => {
  const systemScheme = useColorScheme();
  const pref = useUIStore((state) => state.theme);

  const resolved = useMemo(() => {
    return pref === 'system' ? (systemScheme === 'dark' ? 'dark' : 'light') : pref;
  }, [pref, systemScheme]);

  const theme = tokens[resolved];

  const value = useMemo(
    () => ({
      theme,
      isDark: resolved === 'dark',
    }),
    [theme, resolved]
  );

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
};

export const useTheme = () => useContext(ThemeContext).theme;
export const useThemeContext = () => useContext(ThemeContext);
