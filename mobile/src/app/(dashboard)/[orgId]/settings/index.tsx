import React from 'react';
import { View, Text, StyleSheet, ScrollView, TouchableOpacity, Switch } from 'react-native';
import { useTheme, useThemeContext } from '@/theme/ThemeProvider';
import { ChevronRight } from 'lucide-react-native';
import { useAuthStore } from '@/stores/authStore';
import { useAuth } from '@/hooks/useAuth';
import { useUIStore } from '@/stores/uiStore';

export default function ProfileScreen() {
  const theme = useTheme();
  const { isDark } = useThemeContext();
  const { setTheme } = useUIStore();
  const { user, currentOrg, membership } = useAuthStore();
  const { logout } = useAuth();

  const toggleTheme = (value: boolean) => {
    setTheme(value ? 'dark' : 'light');
  };

  const userName = user?.firstName ? `${user.firstName} ${user.lastName || ''}`.trim() : 'User';
  const userInitials = (user?.firstName?.[0] || '') + (user?.lastName?.[0] || '');
  const orgName = currentOrg?.name || 'Workspace';
  const currentRole = membership?.role;
  const roleDisplay = currentRole ? currentRole.charAt(0).toUpperCase() + currentRole.slice(1) : 'Member';

  return (
    <View style={[styles.container, { backgroundColor: theme.bgCanvas }]}>
      <View style={styles.header}>
        <Text style={[styles.headerTitle, { color: theme.textPrimary }]}>Profile</Text>
      </View>

      <ScrollView contentContainerStyle={styles.scrollContent}>
        <View style={[styles.profileCard, { backgroundColor: theme.bgSurface, borderColor: theme.border }]}>
          <View style={[styles.avatar, { backgroundColor: theme.accent + '20' }]}>
            <Text style={[styles.avatarText, { color: theme.accent }]}>{userInitials || 'ME'}</Text>
          </View>
          <View style={styles.profileInfo}>
            <Text style={[styles.profileName, { color: theme.textPrimary }]}>{userName}</Text>
            <Text style={[styles.profileSubtitle, { color: theme.textMuted }]}>
              {orgName} · {roleDisplay}
            </Text>
          </View>
        </View>

        <View style={styles.menuGroup}>
          <TouchableOpacity style={[styles.menuItem, { backgroundColor: theme.bgSurface, borderColor: theme.border, borderBottomWidth: 0, borderTopLeftRadius: 12, borderTopRightRadius: 12 }]}>
            <Text style={[styles.menuText, { color: theme.textPrimary }]}>Account</Text>
            <ChevronRight size={20} color={theme.textMuted} />
          </TouchableOpacity>
          <View style={[styles.divider, { backgroundColor: theme.border }]} />
          
          <TouchableOpacity style={[styles.menuItem, { backgroundColor: theme.bgSurface, borderColor: theme.border, borderTopWidth: 0, borderBottomWidth: 0 }]}>
            <Text style={[styles.menuText, { color: theme.textPrimary }]}>Security</Text>
            <ChevronRight size={20} color={theme.textMuted} />
          </TouchableOpacity>
          <View style={[styles.divider, { backgroundColor: theme.border }]} />
          
          <TouchableOpacity style={[styles.menuItem, { backgroundColor: theme.bgSurface, borderColor: theme.border, borderTopWidth: 0, borderBottomWidth: 0 }]}>
            <Text style={[styles.menuText, { color: theme.textPrimary }]}>Members</Text>
            <ChevronRight size={20} color={theme.textMuted} />
          </TouchableOpacity>
          <View style={[styles.divider, { backgroundColor: theme.border }]} />
          
          <View style={[styles.menuItem, { backgroundColor: theme.bgSurface, borderColor: theme.border, borderTopWidth: 0, borderBottomLeftRadius: 12, borderBottomRightRadius: 12 }]}>
            <Text style={[styles.menuText, { color: theme.textPrimary }]}>Theme</Text>
            <Switch 
              value={isDark} 
              onValueChange={toggleTheme}
              trackColor={{ false: theme.border, true: theme.accent }}
            />
          </View>
        </View>

        <TouchableOpacity 
          style={[styles.menuItem, { backgroundColor: theme.bgSurface, borderColor: theme.border, borderRadius: 12, marginTop: 24, justifyContent: 'flex-start' }]}
          onPress={() => logout()}
        >
          <Text style={[styles.menuText, { color: theme.destructive }]}>Log out</Text>
        </TouchableOpacity>
      </ScrollView>
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1 },
  header: {
    paddingHorizontal: 24,
    paddingTop: 60,
    paddingBottom: 24,
  },
  headerTitle: {
    fontSize: 28,
    fontWeight: 'bold',
  },
  scrollContent: {
    paddingHorizontal: 24,
    paddingBottom: 40,
  },
  profileCard: {
    flexDirection: 'row',
    alignItems: 'center',
    padding: 16,
    borderRadius: 12,
    borderWidth: 1,
    marginBottom: 24,
  },
  avatar: {
    width: 48,
    height: 48,
    borderRadius: 24,
    alignItems: 'center',
    justifyContent: 'center',
    marginRight: 16,
  },
  avatarText: {
    fontSize: 16,
    fontWeight: 'bold',
  },
  profileInfo: {
    flex: 1,
  },
  profileName: {
    fontSize: 16,
    fontWeight: 'bold',
    marginBottom: 4,
  },
  profileSubtitle: {
    fontSize: 14,
  },
  menuGroup: {
    borderRadius: 12,
    borderWidth: 1,
    borderColor: 'transparent', // The items will have the borders
  },
  menuItem: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: 16,
    borderWidth: 1,
  },
  menuText: {
    fontSize: 16,
    fontWeight: '500',
  },
  divider: {
    height: 1,
    width: '100%',
  }
});
