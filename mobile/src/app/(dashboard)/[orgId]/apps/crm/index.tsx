import React, { useEffect, useState } from 'react';
import { View, Text, StyleSheet, ScrollView, TouchableOpacity, TextInput, ActivityIndicator } from 'react-native';
import { useTheme } from '@/theme/ThemeProvider';
import { ChevronLeft } from 'lucide-react-native';
import { useRouter, useLocalSearchParams } from 'expo-router';
import { crmApi, Lead, LeadStatus } from '@/lib/crmApi';

const tabs = ['Leads', 'Pipeline', 'Reports', 'Agenda', 'Setup'];
const filters = ['All', 'New', 'Contacted', 'Qualified', 'Converted'];

function getInitials(first: string, last: string) {
  return `${first?.[0] || ''}${last?.[0] || ''}`.toUpperCase();
}

function formatRelativeTime(dateString: string) {
  const diffHours = (new Date().getTime() - new Date(dateString).getTime()) / (1000 * 60 * 60);
  if (diffHours < 24) return `${Math.floor(diffHours)}h ago`;
  return `${Math.floor(diffHours / 24)}d ago`;
}

function formatCurrency(val?: number) {
  if (!val) return '';
  if (val >= 1000) return `$${(val / 1000).toFixed(0)}K`;
  return `$${val}`;
}

const StatusPill = ({ status }: { status: LeadStatus }) => {
  const theme = useTheme();
  let bgColor = theme.bgSurface;
  let textColor = theme.textMuted;

  switch(status) {
    case 'new': bgColor = theme.accent + '20'; textColor = theme.accent; break;
    case 'contacted': bgColor = '#fef3c7'; textColor = '#b45309'; break;
    case 'qualified': bgColor = '#d1fae5'; textColor = '#047857'; break;
    case 'converted': bgColor = '#d1fae5'; textColor = '#047857'; break;
  }

  return (
    <View style={[styles.statusPill, { backgroundColor: bgColor }]}>
      <Text style={[styles.statusPillText, { color: textColor }]}>
        {status.charAt(0).toUpperCase() + status.slice(1)}
      </Text>
    </View>
  );
};

export default function CRMScreen() {
  const theme = useTheme();
  const router = useRouter();
  const { orgId } = useLocalSearchParams<{ orgId: string }>();
  
  const [activeTab, setActiveTab] = useState('Leads');
  const [activeFilter, setActiveFilter] = useState('All');
  const [leads, setLeads] = useState<Lead[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (orgId) {
      crmApi.listLeads(orgId)
        .then(res => setLeads(res.leads || []))
        .catch(console.error)
        .finally(() => setLoading(false));
    }
  }, [orgId]);

  const filteredLeads = leads.filter(lead => {
    if (activeFilter === 'All') return true;
    return lead.status.toLowerCase() === activeFilter.toLowerCase();
  });

  return (
    <View style={[styles.container, { backgroundColor: theme.bgCanvas }]}>
      <View style={styles.header}>
        <TouchableOpacity style={styles.backBtn} onPress={() => router.back()}>
          <ChevronLeft size={24} color={theme.textPrimary} />
        </TouchableOpacity>
        <Text style={[styles.headerTitle, { color: theme.textPrimary }]}>CRM</Text>
      </View>

      <View style={styles.tabsContainer}>
        <ScrollView horizontal showsHorizontalScrollIndicator={false} contentContainerStyle={styles.tabsScroll}>
          {tabs.map(tab => {
            const isActive = tab === activeTab;
            return (
              <TouchableOpacity 
                key={tab} 
                style={[styles.tab, isActive ? { backgroundColor: theme.accent, borderColor: theme.accent } : { borderColor: theme.border, backgroundColor: theme.bgCanvas }]}
                onPress={() => setActiveTab(tab)}
              >
                <Text style={[styles.tabText, { color: isActive ? '#fff' : theme.textPrimary }]}>{tab}</Text>
              </TouchableOpacity>
            )
          })}
        </ScrollView>
      </View>

      {activeTab === 'Leads' && (
        <>
          <View style={styles.searchContainer}>
            <TextInput 
              style={[styles.searchInput, { backgroundColor: theme.bgSurface, borderColor: theme.border, color: theme.textPrimary }]} 
              placeholder="Search leads"
              placeholderTextColor={theme.textMuted}
            />
          </View>

          <View style={styles.filtersContainer}>
            <ScrollView horizontal showsHorizontalScrollIndicator={false} contentContainerStyle={styles.filtersScroll}>
              {filters.map(filter => {
                const isActive = filter === activeFilter;
                return (
                  <TouchableOpacity 
                    key={filter} 
                    style={[styles.filterChip, isActive ? { backgroundColor: theme.accent, borderColor: theme.accent } : { borderColor: theme.border, backgroundColor: theme.bgCanvas }]}
                    onPress={() => setActiveFilter(filter)}
                  >
                    <Text style={[styles.filterText, { color: isActive ? '#fff' : theme.textPrimary }]}>{filter}</Text>
                  </TouchableOpacity>
                )
              })}
            </ScrollView>
          </View>

          {loading ? (
            <ActivityIndicator size="large" color={theme.accent} style={{ marginTop: 40 }} />
          ) : (
            <ScrollView contentContainerStyle={styles.listContent}>
              {filteredLeads.map(lead => (
                <View key={lead.id} style={[styles.leadItem, { borderBottomColor: theme.border }]}>
                  <View style={[styles.avatar, { backgroundColor: theme.accent + '20' }]}>
                    <Text style={[styles.avatarText, { color: theme.accent }]}>{getInitials(lead.firstName, lead.lastName)}</Text>
                  </View>
                  <View style={styles.leadInfo}>
                    <Text style={[styles.leadName, { color: theme.textPrimary }]}>{lead.firstName} {lead.lastName}</Text>
                    {lead.companyName && <Text style={[styles.leadCompany, { color: theme.textMuted }]}>{lead.companyName}</Text>}
                    <Text style={[styles.leadMeta, { color: theme.textMuted }]}>
                      {lead.source || 'Direct'} · {formatRelativeTime(lead.createdAt)}
                      {lead.estimatedValue ? ` · ${formatCurrency(lead.estimatedValue)}` : ''}
                    </Text>
                  </View>
                  <StatusPill status={lead.status} />
                </View>
              ))}
            </ScrollView>
          )}
        </>
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1 },
  header: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingHorizontal: 24,
    paddingTop: 60,
    paddingBottom: 16,
    gap: 8,
  },
  backBtn: {
    padding: 4,
    marginLeft: -4,
  },
  headerTitle: {
    fontSize: 24,
    fontWeight: 'bold',
  },
  tabsContainer: {
    marginBottom: 16,
  },
  tabsScroll: {
    paddingHorizontal: 24,
    gap: 8,
  },
  tab: {
    paddingHorizontal: 16,
    paddingVertical: 8,
    borderRadius: 20,
    borderWidth: 1,
  },
  tabText: {
    fontSize: 14,
    fontWeight: '500',
  },
  searchContainer: {
    paddingHorizontal: 24,
    marginBottom: 16,
  },
  searchInput: {
    height: 44,
    borderRadius: 8,
    borderWidth: 1,
    paddingHorizontal: 16,
    fontSize: 15,
  },
  filtersContainer: {
    marginBottom: 16,
  },
  filtersScroll: {
    paddingHorizontal: 24,
    gap: 8,
  },
  filterChip: {
    paddingHorizontal: 14,
    paddingVertical: 6,
    borderRadius: 16,
    borderWidth: 1,
  },
  filterText: {
    fontSize: 13,
    fontWeight: '500',
  },
  listContent: {
    paddingHorizontal: 24,
    paddingBottom: 40,
  },
  leadItem: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingVertical: 16,
    borderBottomWidth: 1,
  },
  avatar: {
    width: 40,
    height: 40,
    borderRadius: 20,
    alignItems: 'center',
    justifyContent: 'center',
    marginRight: 16,
  },
  avatarText: {
    fontWeight: 'bold',
    fontSize: 14,
  },
  leadInfo: {
    flex: 1,
    justifyContent: 'center',
  },
  leadName: {
    fontSize: 16,
    fontWeight: '600',
    marginBottom: 2,
  },
  leadCompany: {
    fontSize: 14,
    marginBottom: 2,
  },
  leadMeta: {
    fontSize: 13,
  },
  statusPill: {
    paddingHorizontal: 8,
    paddingVertical: 4,
    borderRadius: 4,
  },
  statusPillText: {
    fontSize: 11,
    fontWeight: 'bold',
  }
});
