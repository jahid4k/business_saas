import React from 'react';
import { View, Text, StyleSheet, ScrollView, TouchableOpacity } from 'react-native';
import { useTheme } from '@/theme/ThemeProvider';
import { Users } from 'lucide-react-native';
import { useRouter } from 'expo-router';

export default function AppsHubScreen() {
  const theme = useTheme();
  const router = useRouter();

  return (
    <View style={[styles.container, { backgroundColor: theme.bgCanvas }]}>
      <View style={styles.header}>
        <Text style={[styles.headerTitle, { color: theme.textPrimary }]}>Apps</Text>
      </View>

      <ScrollView contentContainerStyle={styles.scrollContent}>
        <View style={styles.grid}>
          <TouchableOpacity 
            style={[styles.card, { backgroundColor: theme.bgSurface, borderColor: theme.border }]}
            onPress={() => router.push('./apps/crm')} // Relative path within apps or just '/crm' if it's absolute
          >
            <View style={[styles.iconContainer, { backgroundColor: theme.bgCanvas }]}>
              <Users size={20} color={theme.accent} />
            </View>
            <Text style={[styles.cardTitle, { color: theme.textPrimary }]}>CRM</Text>
            <Text style={[styles.cardSubtitle, { color: theme.textMuted }]}>Leads, pipeline & deals</Text>
          </TouchableOpacity>

          <TouchableOpacity 
            style={[styles.card, { backgroundColor: theme.bgSurface, borderColor: theme.border }]}
          >
            <View style={[styles.iconContainer, { backgroundColor: theme.bgCanvas }]}>
              <Users size={20} color={theme.accent} />
            </View>
            <Text style={[styles.cardTitle, { color: theme.textPrimary }]}>HRM</Text>
            <Text style={[styles.cardSubtitle, { color: theme.textMuted }]}>People, hiring & payroll</Text>
          </TouchableOpacity>
        </View>

        <View style={[styles.comingSoon, { borderColor: theme.border }]}>
          <Text style={[styles.comingSoonText, { color: theme.textMuted }]}>More modules coming soon</Text>
        </View>
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
  grid: {
    flexDirection: 'row',
    gap: 16,
    marginBottom: 24,
  },
  card: {
    flex: 1,
    padding: 16,
    borderRadius: 12,
    borderWidth: 1,
  },
  iconContainer: {
    width: 40,
    height: 40,
    borderRadius: 8,
    alignItems: 'center',
    justifyContent: 'center',
    marginBottom: 16,
  },
  cardTitle: {
    fontSize: 16,
    fontWeight: 'bold',
    marginBottom: 4,
  },
  cardSubtitle: {
    fontSize: 13,
  },
  comingSoon: {
    padding: 24,
    borderRadius: 12,
    borderWidth: 1,
    borderStyle: 'dashed',
    alignItems: 'center',
    justifyContent: 'center',
  },
  comingSoonText: {
    fontSize: 14,
    fontWeight: '500',
  }
});
