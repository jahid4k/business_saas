import axios from 'axios';
import { API_URL } from './constants';
import { getAccessToken, getRefreshToken, setAccessToken, setRefreshToken, clearTokens } from './secureToken';
import { useAuthStore } from '@/stores/authStore';
import { usePermissionStore } from '@/stores/permissionStore';

export const api = axios.create({
  baseURL: `${API_URL}/api/v1`,
  headers: {
    'Content-Type': 'application/json',
  },
});

api.interceptors.request.use((config) => {
  const token = getAccessToken();
  if (token && config.headers) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

let refreshPromise: Promise<string | null> | null = null;

const refreshAccessToken = async (): Promise<string | null> => {
  const refreshToken = await getRefreshToken();
  if (!refreshToken) return null;

  try {
    const response = await axios.post(`${API_URL}/api/v1/auth/mobile/refresh`, {
      refresh_token: refreshToken,
    });
    const { access_token, refresh_token: new_refresh_token } = response.data.data;
    setAccessToken(access_token);
    await setRefreshToken(new_refresh_token);
    return access_token;
  } catch (error) {
    await clearTokens();
    useAuthStore.getState().reset();
    usePermissionStore.getState().reset();
    return null;
  }
};

api.interceptors.response.use(
  (response) => response,
  async (error) => {
    const originalRequest = error.config;
    if (error.response?.status === 401 && originalRequest && !originalRequest._retry) {
      originalRequest._retry = true;
      refreshPromise ??= refreshAccessToken().finally(() => {
        refreshPromise = null;
      });
      const newToken = await refreshPromise;
      if (newToken) {
        if (originalRequest.headers) {
          originalRequest.headers.Authorization = `Bearer ${newToken}`;
        }
        return api(originalRequest);
      }
    }
    console.error(`API Error: ${error.config?.method?.toUpperCase()} ${error.config?.url} -> ${error.response?.status}`);
    console.error(`Response Body: ${JSON.stringify(error.response?.data)}`);
    return Promise.reject(error);
  }
);
