import { Stack } from 'expo-router';
import React from 'react';

export default function AppsLayout() {
  return (
    <Stack screenOptions={{ headerShown: false }}>
      <Stack.Screen name="index" />
      <Stack.Screen name="crm/index" />
    </Stack>
  );
}
