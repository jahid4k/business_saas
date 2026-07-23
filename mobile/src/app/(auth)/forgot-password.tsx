import React from 'react';
import { View, Text, TextInput, TouchableOpacity, StyleSheet } from 'react-native';
import { useForm, Controller } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { useAuth } from '@/hooks/useAuth';
import { useTheme } from '@/theme/ThemeProvider';
import { Link, useRouter } from 'expo-router';

const forgotSchema = z.object({
  email: z.string().email('Invalid email address'),
});

type ForgotFormData = z.infer<typeof forgotSchema>;

export default function ForgotPasswordScreen() {
  const { forgotPassword, loading } = useAuth();
  const theme = useTheme();
  const router = useRouter();

  const { control, handleSubmit, formState: { errors } } = useForm<ForgotFormData>({
    resolver: zodResolver(forgotSchema),
  });

  const onSubmit = async (data: ForgotFormData) => {
    try {
      await forgotPassword(data);
      router.push('/login');
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
      <Text style={styles.title}>Reset Password</Text>
      
      <Controller
        control={control}
        name="email"
        render={({ field: { onChange, onBlur, value } }) => (
          <TextInput
            style={styles.input}
            placeholder="Email"
            placeholderTextColor={theme.textMuted}
            onBlur={onBlur}
            onChangeText={onChange}
            value={value}
            autoCapitalize="none"
            keyboardType="email-address"
          />
        )}
      />
      {errors.email && <Text style={styles.errorText}>{errors.email.message}</Text>}

      <TouchableOpacity 
        style={[styles.button, loading && { opacity: 0.7 }]} 
        onPress={handleSubmit(onSubmit)}
        disabled={loading}
      >
        <Text style={styles.buttonText}>{loading ? 'Sending link...' : 'Send Reset Link'}</Text>
      </TouchableOpacity>

      <Link href="/login" asChild>
        <TouchableOpacity><Text style={styles.link}>Back to Login</Text></TouchableOpacity>
      </Link>
    </View>
  );
}
