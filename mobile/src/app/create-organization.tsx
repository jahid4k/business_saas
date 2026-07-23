import React from 'react';
import { View, Text, TextInput, TouchableOpacity, StyleSheet } from 'react-native';
import { useForm, Controller } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { useOrganization } from '@/hooks/useOrganization';
import { useTheme } from '@/theme/ThemeProvider';
import { Link } from 'expo-router';

const createOrgSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  slug: z.string().min(1, 'Slug is required'),
});

type CreateOrgFormData = z.infer<typeof createOrgSchema>;

export default function CreateOrgScreen() {
  const { createOrganization, switchOrg, loading } = useOrganization();
  const theme = useTheme();

  const { control, handleSubmit, formState: { errors } } = useForm<CreateOrgFormData>({
    resolver: zodResolver(createOrgSchema),
  });

  const onSubmit = async (data: CreateOrgFormData) => {
    try {
      const newOrg = await createOrganization(data);
      await switchOrg(newOrg);
    } catch (e) {
      console.error(e);
    }
  };

  const styles = StyleSheet.create({
    container: { flex: 1, backgroundColor: theme.bgCanvas, padding: 24, justifyContent: 'center' },
    title: { fontSize: 24, fontWeight: 'bold', color: theme.textPrimary, marginBottom: 24 },
    input: {
      backgroundColor: theme.bgSurface,
      borderColor: theme.border,
      borderWidth: 1,
      borderRadius: 8,
      padding: 16,
      marginBottom: 8,
      color: theme.textPrimary,
    },
    errorText: { color: theme.destructive, fontSize: 12, marginBottom: 16 },
    button: {
      backgroundColor: theme.accent,
      padding: 16,
      borderRadius: 8,
      alignItems: 'center',
      marginTop: 8,
    },
    buttonText: { color: '#ffffff', fontWeight: 'bold' },
    link: { color: theme.accentHover, marginTop: 16, textAlign: 'center' },
  });

  return (
    <View style={styles.container}>
      <Text style={styles.title}>Create Workspace</Text>
      
      <Controller
        control={control}
        name="name"
        render={({ field: { onChange, onBlur, value } }) => (
          <TextInput
            style={styles.input}
            placeholder="Workspace Name (e.g. Acme Corp)"
            placeholderTextColor={theme.textMuted}
            onBlur={onBlur}
            onChangeText={onChange}
            value={value}
          />
        )}
      />
      {errors.name && <Text style={styles.errorText}>{errors.name.message}</Text>}

      <Controller
        control={control}
        name="slug"
        render={({ field: { onChange, onBlur, value } }) => (
          <TextInput
            style={styles.input}
            placeholder="Slug (e.g. acme-corp)"
            placeholderTextColor={theme.textMuted}
            onBlur={onBlur}
            onChangeText={onChange}
            value={value}
            autoCapitalize="none"
          />
        )}
      />
      {errors.slug && <Text style={styles.errorText}>{errors.slug.message}</Text>}

      <TouchableOpacity 
        style={[styles.button, loading && { opacity: 0.7 }]} 
        onPress={handleSubmit(onSubmit)}
        disabled={loading}
      >
        <Text style={styles.buttonText}>{loading ? 'Creating...' : 'Create'}</Text>
      </TouchableOpacity>

      <Link href="/select-organization" asChild>
        <TouchableOpacity><Text style={styles.link}>Back to Select</Text></TouchableOpacity>
      </Link>
    </View>
  );
}
