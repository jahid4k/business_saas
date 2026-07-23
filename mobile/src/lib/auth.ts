import { api } from './api';
import { clearTokens, getRefreshToken, setAccessToken, setRefreshToken } from './secureToken';

export const authApi = {
  signup: async (data: any) => {
    const response = await api.post('/auth/mobile/signup', data);
    return response.data.data;
  },
  login: async (data: any) => {
    const response = await api.post('/auth/mobile/login', data);
    const { access_token, refresh_token } = response.data.data;
    setAccessToken(access_token);
    await setRefreshToken(refresh_token);
    return response.data.data;
  },
  logout: async () => {
    const refreshToken = await getRefreshToken();
    if (refreshToken) {
      try {
        await api.post('/auth/mobile/logout', { refresh_token: refreshToken });
      } catch (e) {
        // Ignore errors during logout
      }
    }
    await clearTokens();
  },
  getMe: async () => {
    const response = await api.get('/auth/me');
    return response.data.data.user;
  }
};
