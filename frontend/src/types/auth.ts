// src/types/auth.ts
// ─── Exact match to backend user/model.go SafeUser JSON tags ─────────

export interface ClientTokenPair {
  access_token: string;
  expires_in: number; // seconds
}

export interface SafeUser {
  id: string;
  publicId?: string;
  email?: string;
  username?: string;
  displayName: string;
  firstName?: string;
  lastName?: string;
  fullName?: string;
  photoURL?: string;
  coverPhotoURL?: string;
  phone?: string;
  phoneVerified: boolean;
  emailVerified: boolean;
  emailVerifiedAt?: string;
  country?: string;
  timezone: string;
  locale: string;
  language: string;
  currency: string;
  status: "active" | "suspended" | "deleted" | "pending_verification";
  accountType: string;
  loginRedirectUrl: string;
  shortcuts: string[];
  settings: Record<string, unknown>;
  preferences: Record<string, unknown>;
  onboarding: Record<string, unknown>;
  featureFlags: Record<string, unknown>;
  twoFactorEnabled: boolean;
  lastLoginAt?: string;
  lastActivityAt?: string;
  createdAt: string;
  updatedAt: string;
}

// Note: signup body uses snake_case (matches backend SignupRequest JSON tags)
export interface SignupRequest {
  email: string;
  password: string;
  first_name: string;
  last_name: string;
  displayName?: string;
}

// One stored avatar (see backend/internal/user/avatar.go UserAvatar.ToResponse).
// `url` is server-relative, same convention as SafeUser.photoURL — resolve it
// with resolveAssetUrl before rendering in an <img>.
export interface UserAvatar {
  id: string;
  fileSize: number;
  width: number;
  height: number;
  isActive: boolean;
  createdAt: string;
  url: string;
}
