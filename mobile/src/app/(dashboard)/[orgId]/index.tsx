import React, { useEffect, useState } from 'react';
import { View, Text, StyleSheet, ScrollView, ActivityIndicator } from 'react-native';
import { useTheme } from '@/theme/ThemeProvider';
import { DashboardHeader } from '@/components/layout/DashboardHeader';
import { useAuthStore } from '@/stores/authStore';
import { ArrowUpRight, ArrowDownRight } from 'lucide-react-native';
import { dashboardApi, DashboardResponse } from '@/lib/dashboardApi';
import { useLocalSearchParams } from 'expo-router';

const MetricCard = ({ title, value, change, isPositive }: { title: string, value: string, change: string, isPositive: boolean }) => {
  const theme = useTheme();
  return (
    <View style={[styles.card, { backgroundColor: theme.bgSurface, borderColor: theme.border }]}>
      <Text style={[styles.cardTitle, { color: theme.textMuted }]}>{title}</Text>
      <Text style={[styles.cardValue, { color: theme.textPrimary }]}>{value}</Text>
      
      <View style={styles.sparklineContainer}>
        <View style={[styles.sparkline, { backgroundColor: isPositive ? theme.success : theme.destructive, opacity: 0.5, transform: [{ rotate: isPositive ? '-15deg' : '15deg' }] }]} />
      </View>

      <View style={styles.changeRow}>
        {isPositive ? (
          <ArrowUpRight size={12} color={theme.success} />
        ) : (
          <ArrowDownRight size={12} color={theme.destructive} />
        )}
        <Text style={[styles.changeText, { color: isPositive ? theme.success : theme.destructive }]}>
          {change}
        </Text>
      </View>
    </View>
  );
};

const TimelineItem = ({ title, subtitle, color }: { title: string, subtitle: string, color: string }) => {
  const theme = useTheme();
  return (
    <View style={[styles.timelineItem, { borderBottomColor: theme.border }]}>
      <View style={[styles.timelineDot, { backgroundColor: color }]} />
      <View style={styles.timelineContent}>
        <Text style={[styles.timelineTitle, { color: theme.textPrimary }]}>{title}</Text>
        <Text style={[styles.timelineSubtitle, { color: theme.textMuted }]}>{subtitle}</Text>
      </View>
    </View>
  );
}

function formatCurrency(val: number) {
  if (val >= 1000000) return `$${(val / 1000000).toFixed(2)}M`;
  if (val >= 1000) return `$${(val / 1000).toFixed(0)}K`;
  return `$${val}`;
}

function formatDateRelative(isoString: string) {
  const date = new Date(isoString);
  const now = new Date();
  const diffHours = (now.getTime() - date.getTime()) / (1000 * 60 * 60);
  
  if (diffHours < 24 && date.getDate() === now.getDate()) {
    return `Today · ${date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}`;
  }
  return `Yesterday · ${date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}`;
}

export default function DashboardHome() {
  const theme = useTheme();
  const user = useAuthStore(state => state.user);
  const { orgId } = useLocalSearchParams<{ orgId: string }>();
  const [data, setData] = useState<DashboardResponse | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (orgId) {
      dashboardApi.getMetrics(orgId)
        .then(setData)
        .catch(console.error)
        .finally(() => setLoading(false));
    }
  }, [orgId]);

  const getGreeting = () => {
    const hour = new Date().getHours();
    if (hour < 12) return 'Good morning';
    if (hour < 17) return 'Good afternoon';
    return 'Good evening';
  };

  return (
    <View style={[styles.container, { backgroundColor: theme.bgCanvas }]}>
      <DashboardHeader />
      
      {loading ? (
        <ActivityIndicator size="large" color={theme.accent} style={{ marginTop: 60 }} />
      ) : (
        <ScrollView contentContainerStyle={styles.scrollContent}>
          <Text style={[styles.greeting, { color: theme.textPrimary }]}>
            {getGreeting()}, {user?.firstName || 'User'}
          </Text>

          <View style={styles.metricsGrid}>
            <View style={styles.metricsCol}>
              <MetricCard 
                title="Pipeline Value" 
                value={formatCurrency(data?.kpis?.active_pipeline_value || 0)} 
                change="+12%" 
                isPositive={true} 
              />
              <MetricCard 
                title="Pending Approvals" 
                value={(data?.kpis?.pending_approvals || 0).toString()} 
                change="-2%" 
                isPositive={false} 
              />
            </View>
            <View style={styles.metricsCol}>
              <MetricCard 
                title="Total Headcount" 
                value={(data?.kpis?.total_headcount || 0).toString()} 
                change="+5%" 
                isPositive={true} 
              />
              <MetricCard 
                title="Active Tasks" 
                value="34" // Mocked, backend missing this KPI in standard model
                change="+22%" 
                isPositive={true} 
              />
            </View>
          </View>

          <Text style={[styles.sectionTitle, { color: theme.textPrimary }]}>Recent Activity</Text>
          <View style={styles.timeline}>
            {(data?.action_items || []).length > 0 ? (
              data!.action_items.map(item => (
                <TimelineItem 
                  key={item.id}
                  title={item.title} 
                  subtitle={formatDateRelative(item.timestamp)} 
                  color={item.type === 'stagnant_deal' ? '#f59e0b' : '#7c3aed'} 
                />
              ))
            ) : (
              <Text style={{ color: theme.textMuted, marginTop: 12 }}>No recent activity to display.</Text>
            )}
          </View>
        </ScrollView>
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1 },
  scrollContent: { padding: 24, paddingBottom: 40 },
  greeting: { fontSize: 28, fontWeight: 'bold', marginBottom: 24, letterSpacing: -0.5 },
  metricsGrid: { flexDirection: 'row', gap: 16, marginBottom: 32 },
  metricsCol: { flex: 1, gap: 16 },
  card: { padding: 16, borderRadius: 12, borderWidth: 1 },
  cardTitle: { fontSize: 13, fontWeight: '600', marginBottom: 8 },
  cardValue: { fontSize: 24, fontWeight: 'bold', marginBottom: 16, letterSpacing: -0.5 },
  sparklineContainer: { height: 20, justifyContent: 'center', marginBottom: 8 },
  sparkline: { height: 2, width: '60%', borderRadius: 1 },
  changeRow: { flexDirection: 'row', alignItems: 'center', justifyContent: 'flex-end', gap: 2 },
  changeText: { fontSize: 12, fontWeight: 'bold' },
  sectionTitle: { fontSize: 18, fontWeight: 'bold', marginBottom: 16 },
  timeline: { marginLeft: 4 },
  timelineItem: { flexDirection: 'row', paddingVertical: 16, borderBottomWidth: 1, alignItems: 'flex-start' },
  timelineDot: { width: 8, height: 8, borderRadius: 4, marginTop: 6, marginRight: 16 },
  timelineContent: { flex: 1 },
  timelineTitle: { fontSize: 15, fontWeight: '600', marginBottom: 4 },
  timelineSubtitle: { fontSize: 13 }
});
