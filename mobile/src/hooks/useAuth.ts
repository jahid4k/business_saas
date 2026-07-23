import { useState } from 'react';
import { authApi } from '@/lib/auth';
import { useAuthStore } from '@/stores/authStore';
import { usePermissionStore } from '@/stores/permissionStore';

export function useAuth() {
  const [loading, setLoading] = useState(false);
  const { setUser, setStatus, reset: resetAuth } = useAuthStore();
  const { reset: resetPermissions } = usePermissionStore();

  const login = async (data: any) => {
    setLoading(true);
    try {
      await authApi.login(data);
      const meResponse = await authApi.getMe();
      setUser(meResponse.data);
      setStatus('authenticated');
    } finally {
      setLoading(false);
    }
  };

  const signup = async (data: any) => {
    setLoading(true);
    try {
      return await authApi.signup(data);
    } finally {
      setLoading(false);
    }
  };

  const logout = async () => {
    setLoading(true);
    try {
      await authApi.logout();
      resetAuth();
      resetPermissions();
    } finally {
      setLoading(false);
    }
  };

  const forgotPassword = async (data: { email: string }) => {
    setLoading(true);
    try {
      console.log('Forgot password for', data.email);
      await new Promise((resolve) => setTimeout(resolve, 500));
    } finally {
      setLoading(false);
    }
  };

  const resetPassword = async (data: { password: string }) => {
    setLoading(true);
    try {
      console.log('Reset password');
      await new Promise((resolve) => setTimeout(resolve, 500));
    } finally {
      setLoading(false);
    }
  };

  return {
    login,
    signup,
    logout,
    forgotPassword,
    resetPassword,
    loading,
  };
}
