import React from 'react';
import { View, Text, TouchableOpacity, StyleSheet } from 'react-native';
import { useAuthStore } from '@/stores/authStore';
import { useTheme } from '@/theme/ThemeProvider';
import { ChevronDown } from 'lucide-react-native';
import { useRouter } from 'expo-router';

export function DashboardHeader({ title }: { title?: string }) {
  const theme = useTheme();
  const currentOrg = useAuthStore(state => state.currentOrg);
  const user = useAuthStore(state => state.user);
  const router = useRouter();

  const orgName = currentOrg?.name || currentOrg?.legalName || 'Workspace';
  const orgInitials = orgName.substring(0, 1).toUpperCase();
  
  const userName = user?.firstName ? `${user.firstName} ${user.lastName || ''}`.trim() : 'User';
  const userInitials = (user?.firstName?.[0] || '') + (user?.lastName?.[0] || '');

  const styles = StyleSheet.create({
    container: {
      flexDirection: 'row',
      alignItems: 'center',
      justifyContent: 'space-between',
      paddingHorizontal: 20,
      paddingTop: 60, // approximate status bar padding
      paddingBottom: 16,
      backgroundColor: theme.bgCanvas, // using bgCanvas to match the seamless background of the mock
    },
    leftContent: {
      flexDirection: 'row',
      alignItems: 'center',
      gap: 12,
    },
    orgAvatar: {
      width: 36,
      height: 36,
      borderRadius: 8,
      backgroundColor: theme.accent,
      alignItems: 'center',
      justifyContent: 'center',
    },
    orgAvatarText: {
      color: '#ffffff',
      fontWeight: 'bold',
      fontSize: 16,
    },
    orgInfo: {
      justifyContent: 'center',
    },
    orgName: {
      fontSize: 16,
      fontWeight: 'bold',
      color: theme.textPrimary,
    },
    switchRow: {
      flexDirection: 'row',
      alignItems: 'center',
      gap: 2,
    },
    switchText: {
      fontSize: 12,
      color: theme.textMuted,
    },
    userAvatar: {
      width: 36,
      height: 36,
      borderRadius: 18,
      backgroundColor: theme.bgSurface,
      borderWidth: 1,
      borderColor: theme.border,
      alignItems: 'center',
      justifyContent: 'center',
    },
    userAvatarText: {
      color: theme.accent,
      fontWeight: 'bold',
      fontSize: 12,
    }
  });

  return (
    <View style={styles.container}>
      <View style={styles.leftContent}>
        <View style={styles.orgAvatar}>
          <Text style={styles.orgAvatarText}>{orgInitials}</Text>
        </View>
        <TouchableOpacity style={styles.orgInfo} onPress={() => router.push('/select-organization')}>
          <Text style={styles.orgName}>{orgName}</Text>
          <View style={styles.switchRow}>
            <Text style={styles.switchText}>Switch organization</Text>
            <ChevronDown size={12} color={theme.textMuted} />
          </View>
        </TouchableOpacity>
      </View>
      
      <TouchableOpacity style={styles.userAvatar} onPress={() => router.push('/(dashboard)/[orgId]/settings')}>
        <Text style={styles.userAvatarText}>{userInitials || 'ME'}</Text>
      </TouchableOpacity>
    </View>
  );
}
