import React from 'react';
import { View, Text, StyleSheet, ScrollView } from 'react-native';
import { useTheme } from '@/theme/ThemeProvider';
import { Bell } from 'lucide-react-native';

const NotificationItem = ({ title, time, color, isUnread }: { title: string, time: string, color: string, isUnread: boolean }) => {
  const theme = useTheme();
  return (
    <View style={[styles.notificationItem, { borderBottomColor: theme.border }]}>
      <View style={[styles.iconContainer, { backgroundColor: color + '20' }]}>
        <Bell size={16} color={color} />
      </View>
      <View style={styles.notificationContent}>
        <Text style={[styles.notificationTitle, { color: theme.textPrimary }]}>{title}</Text>
        <Text style={[styles.notificationTime, { color: theme.textMuted }]}>{time}</Text>
      </View>
      {isUnread && (
        <View style={[styles.unreadDot, { backgroundColor: theme.accent }]} />
      )}
    </View>
  );
};

export default function AlertsScreen() {
  const theme = useTheme();

  return (
    <View style={[styles.container, { backgroundColor: theme.bgCanvas }]}>
      <View style={styles.header}>
        <Text style={[styles.headerTitle, { color: theme.textPrimary }]}>Notifications</Text>
      </View>

      <ScrollView contentContainerStyle={styles.scrollContent}>
        <Text style={[styles.sectionTitle, { color: theme.textMuted }]}>TODAY</Text>
        
        <NotificationItem 
          title="New lead assigned: Grace Lin"
          time="10m ago"
          color="#7c3aed"
          isUnread={true}
        />
        <NotificationItem 
          title="Task due today: Draft contract — Vantage Systems"
          time="1h ago"
          color="#f59e0b"
          isUnread={true}
        />
        <NotificationItem 
          title="Elena Voss moved to Qualified"
          time="3h ago"
          color="#7c3aed"
          isUnread={false}
        />

        <Text style={[styles.sectionTitle, { color: theme.textMuted, marginTop: 24 }]}>EARLIER</Text>
        
        <NotificationItem 
          title="Priya Shah invited you to Northwind Robotics"
          time="Yesterday"
          color="#64748b"
          isUnread={false}
        />
        <NotificationItem 
          title="Payroll run completed for June"
          time="2d ago"
          color="#10b981"
          isUnread={false}
        />
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
  sectionTitle: {
    fontSize: 12,
    fontWeight: 'bold',
    letterSpacing: 1,
    marginBottom: 16,
  },
  notificationItem: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingVertical: 16,
    borderBottomWidth: 1,
  },
  iconContainer: {
    width: 36,
    height: 36,
    borderRadius: 18,
    alignItems: 'center',
    justifyContent: 'center',
    marginRight: 16,
  },
  notificationContent: {
    flex: 1,
    paddingRight: 16,
  },
  notificationTitle: {
    fontSize: 15,
    fontWeight: '500',
    marginBottom: 4,
  },
  notificationTime: {
    fontSize: 13,
  },
  unreadDot: {
    width: 8,
    height: 8,
    borderRadius: 4,
  }
});
