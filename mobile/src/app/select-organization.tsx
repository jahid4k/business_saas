import React, { useEffect, useState } from 'react';
import { View, Text, FlatList, TouchableOpacity, StyleSheet, ActivityIndicator } from 'react-native';
import { useOrganization } from '@/hooks/useOrganization';
import { MembershipWithRole } from '@/types';
import { useTheme } from '@/theme/ThemeProvider';
import { Link, useRouter } from 'expo-router';

export default function SelectOrgScreen() {
  const { getOrganizations, switchOrg, loading } = useOrganization();
  const [orgs, setOrgs] = useState<MembershipWithRole[]>([]);
  const [fetching, setFetching] = useState(true);
  const theme = useTheme();

  useEffect(() => {
    getOrganizations()
      .then((data) => setOrgs(data))
      .catch(console.error)
      .finally(() => setFetching(false));
  }, []);

  const styles = StyleSheet.create({
    container: { flex: 1, backgroundColor: theme.bgCanvas, padding: 24 },
    title: { fontSize: 24, fontWeight: 'bold', color: theme.textPrimary, marginBottom: 24, marginTop: 40 },
    orgItem: {
      backgroundColor: theme.bgSurface,
      padding: 16,
      borderRadius: 8,
      marginBottom: 12,
      borderWidth: 1,
      borderColor: theme.border,
    },
    orgName: { fontSize: 18, fontWeight: 'bold', color: theme.textPrimary },
    orgSlug: { fontSize: 14, color: theme.textMuted, marginTop: 4 },
    button: {
      backgroundColor: theme.accent,
      padding: 16,
      borderRadius: 8,
      alignItems: 'center',
      marginTop: 24,
    },
    buttonText: { color: '#ffffff', fontWeight: 'bold' },
    emptyText: { color: theme.textMuted, textAlign: 'center', marginTop: 40 },
  });

  return (
    <View style={styles.container}>
      <Text style={styles.title}>Select Workspace</Text>
      
      {fetching ? (
        <ActivityIndicator size="large" color={theme.accent} style={{ marginTop: 40 }} />
      ) : (
        <FlatList
          data={orgs}
          keyExtractor={(item) => item.organization.id || item.organization.publicId || Math.random().toString()}
          renderItem={({ item }) => (
            <TouchableOpacity 
              style={[styles.orgItem, loading && { opacity: 0.7 }]} 
              onPress={() => switchOrg(item.organization)}
              disabled={loading}
            >
              <Text style={styles.orgName}>{item.organization.name || item.organization.legalName}</Text>
              <Text style={styles.orgSlug}>@{item.organization.slug}</Text>
            </TouchableOpacity>
          )}
          ListEmptyComponent={<Text style={styles.emptyText}>You don't belong to any workspaces yet.</Text>}
        />
      )}

      <Link href="/create-organization" asChild>
        <TouchableOpacity style={styles.button} disabled={loading || fetching}>
          <Text style={styles.buttonText}>Create New Workspace</Text>
        </TouchableOpacity>
      </Link>
    </View>
  );
}
