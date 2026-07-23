import React from 'react';
import { View, Text, TextInput, TouchableOpacity, StyleSheet, ScrollView } from 'react-native';
import { useForm, Controller } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { useAuth } from '@/hooks/useAuth';
import { useTheme } from '@/theme/ThemeProvider';
import { Link, useRouter } from 'expo-router';

const signupSchema = z.object({
  email: z.string().email('Invalid email address'),
  password: z.string().min(8, 'Password must be at least 8 characters'),
  first_name: z.string().min(1, 'First name is required').optional(),
  last_name: z.string().min(1, 'Last name is required').optional(),
  displayName: z.string().min(1, 'Display name is required').optional(),
});

type SignupFormData = z.infer<typeof signupSchema>;

export default function SignupScreen() {
  const { signup, loading } = useAuth();
  const theme = useTheme();
  const router = useRouter();

  const { control, handleSubmit, formState: { errors } } = useForm<SignupFormData>({
    resolver: zodResolver(signupSchema),
  });

  const onSubmit = async (data: SignupFormData) => {
    try {
      await signup(data);
      // Signup does NOT auto-login. Route to login.
      router.push('/login');
    } catch (e) {
      console.error(e);
    }
  };

  const styles = StyleSheet.create({
    container: { flexGrow: 1, backgroundColor: theme.bgCanvas, padding: 24, justifyContent: 'center' },
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
    <ScrollView contentContainerStyle={styles.container}>
      <Text style={styles.title}>Create Account</Text>
      
      <Controller control={control} name="first_name" render={({ field: { onChange, onBlur, value } }) => (
        <TextInput style={styles.input} placeholder="First Name" placeholderTextColor={theme.textMuted} onBlur={onBlur} onChangeText={onChange} value={value} />
      )} />
      {errors.first_name && <Text style={styles.errorText}>{errors.first_name.message}</Text>}

      <Controller control={control} name="last_name" render={({ field: { onChange, onBlur, value } }) => (
        <TextInput style={styles.input} placeholder="Last Name" placeholderTextColor={theme.textMuted} onBlur={onBlur} onChangeText={onChange} value={value} />
      )} />
      {errors.last_name && <Text style={styles.errorText}>{errors.last_name.message}</Text>}

      <Controller control={control} name="displayName" render={({ field: { onChange, onBlur, value } }) => (
        <TextInput style={styles.input} placeholder="Display Name" placeholderTextColor={theme.textMuted} onBlur={onBlur} onChangeText={onChange} value={value} />
      )} />
      {errors.displayName && <Text style={styles.errorText}>{errors.displayName.message}</Text>}

      <Controller control={control} name="email" render={({ field: { onChange, onBlur, value } }) => (
        <TextInput style={styles.input} placeholder="Email" placeholderTextColor={theme.textMuted} onBlur={onBlur} onChangeText={onChange} value={value} autoCapitalize="none" keyboardType="email-address" />
      )} />
      {errors.email && <Text style={styles.errorText}>{errors.email.message}</Text>}

      <Controller control={control} name="password" render={({ field: { onChange, onBlur, value } }) => (
        <TextInput style={styles.input} placeholder="Password" placeholderTextColor={theme.textMuted} onBlur={onBlur} onChangeText={onChange} value={value} secureTextEntry />
      )} />
      {errors.password && <Text style={styles.errorText}>{errors.password.message}</Text>}

      <TouchableOpacity style={[styles.button, loading && { opacity: 0.7 }]} onPress={handleSubmit(onSubmit)} disabled={loading}>
        <Text style={styles.buttonText}>{loading ? 'Creating account...' : 'Sign Up'}</Text>
      </TouchableOpacity>

      <Link href="/login" asChild>
        <TouchableOpacity><Text style={styles.link}>Already have an account? Login</Text></TouchableOpacity>
      </Link>
    </ScrollView>
  );
}
