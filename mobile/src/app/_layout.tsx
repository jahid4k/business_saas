import { useEffect, useState } from 'react';
import { Stack } from 'expo-router';
import * as SplashScreen from 'expo-splash-screen';
import { useFonts } from 'expo-font';
import { Inter_400Regular, Inter_500Medium, Inter_600SemiBold, Inter_700Bold } from '@expo-google-fonts/inter';
import { ThemeProvider } from '@/theme/ThemeProvider';
import { getRefreshToken, setAccessToken, setRefreshToken } from '@/lib/secureToken';
import { useAuthStore } from '@/stores/authStore';
import { api } from '@/lib/api';
import { authApi } from '@/lib/auth';

SplashScreen.preventAutoHideAsync();

function RootLayoutNav() {
  const status = useAuthStore((state) => state.status);
  const isAuthenticated = status === 'authenticated';

  return (
    <Stack screenOptions={{ headerShown: false }}>
      <Stack.Screen name="index" />
      <Stack.Protected guard={isAuthenticated}>
        <Stack.Screen name="(dashboard)" />
        <Stack.Screen name="create-organization" />
        <Stack.Screen name="select-organization" />
      </Stack.Protected>
      <Stack.Protected guard={!isAuthenticated}>
        <Stack.Screen name="(auth)" />
      </Stack.Protected>
    </Stack>
  );
}

export default function RootLayout() {
  const [fontsLoaded] = useFonts({
    Inter_400Regular,
    Inter_500Medium,
    Inter_600SemiBold,
    Inter_700Bold,
  });

  const [authChecked, setAuthChecked] = useState(false);
  const { setStatus, setUser } = useAuthStore();

  useEffect(() => {
    async function bootstrapAuth() {
      try {
        const token = await getRefreshToken();
        if (token) {
          // Attempt to refresh
          const response = await api.post('/auth/mobile/refresh', { refresh_token: token });
          const { access_token, refresh_token: new_refresh_token } = response.data.data;
          setAccessToken(access_token);
          await setRefreshToken(new_refresh_token);
          
          // Gap fix: repopulate user
          const meResponse = await authApi.getMe();
          setUser(meResponse.data);

          setStatus('authenticated');
        } else {
          setStatus('unauthenticated');
        }
      } catch (error) {
        setStatus('unauthenticated');
      } finally {
        setAuthChecked(true);
      }
    }

    bootstrapAuth();
  }, [setStatus, setUser]);

  useEffect(() => {
    if (fontsLoaded && authChecked) {
      SplashScreen.hideAsync();
    }
  }, [fontsLoaded, authChecked]);

  if (!fontsLoaded || !authChecked) {
    return null;
  }

  return (
    <ThemeProvider>
      <RootLayoutNav />
    </ThemeProvider>
  );
}
