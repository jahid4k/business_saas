import React from 'react';
import { Redirect } from 'expo-router';
import { useAuthStore } from '@/stores/authStore';

export default function Index() {
  const status = useAuthStore((state) => state.status);
  const currentOrg = useAuthStore((state) => state.currentOrg);

  if (status === 'authenticated') {
    if (currentOrg) {
      return <Redirect href={`/(dashboard)/${currentOrg.id || (currentOrg as any).publicId}`} />;
    }
    return <Redirect href="/select-organization" />;
  }

  return <Redirect href="/login" />;
}
