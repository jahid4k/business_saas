import { User } from '@auth/user';
import UserModel from '@auth/user/models/UserModel';
import { PartialDeep } from 'type-fest';
import api from '@/utils/api';

export type AuthIdentity = User & {
    id: string;
    email: string;
    displayName: string;
    role: NonNullable<User['role']>;
};

export type CredentialsSignInPayload = {
    email: string;
    password: string;
};

export type CredentialsSignUpPayload = {
    displayName: string;
    email: string;
    password: string;
    organizationName?: string;
};

export type OAuthSyncPayload = {
    provider: string;
    providerAccountId?: string;
    email: string;
    displayName?: string | null;
    photoURL?: string | null;
    emailVerified?: boolean;
};

export type AuditEventPayload = {
    action: 'auth.sign_in' | 'auth.sign_out' | 'auth.create_user' | 'auth.link_account' | 'auth.update_user';
    userId?: string | null;
    email?: string | null;
    provider?: string | null;
    organizationId?: string | null;
    metadata?: Record<string, unknown>;
};

const IDENTITY_API_PREFIX = 'identity';

/**
 * Credentials sign-in must be validated by your real backend.
 * Backend duties: email normalization, password hash verification,
 * account status check, tenant membership lookup, and audit logging.
 */
export async function authSignInWithCredentials(payload: CredentialsSignInPayload): Promise<AuthIdentity> {
    return api
        .post(`${IDENTITY_API_PREFIX}/sign-in`, {
            json: payload
        })
        .json<AuthIdentity>();
}

/**
 * Signup should create a normal SaaS user, not a global admin.
 * Backend duties: duplicate email check, password hashing, optional organization creation,
 * email verification flow, default plan assignment, and safe default role.
 */
export async function authSignUpWithCredentials(payload: CredentialsSignUpPayload): Promise<AuthIdentity> {
    return api
        .post(`${IDENTITY_API_PREFIX}/sign-up`, {
            json: payload
        })
        .json<AuthIdentity>();
}

/**
 * OAuth sync creates or updates the app-level user after Google/Facebook login.
 * The backend must return the full Fuse user shape, including settings/shortcuts/loginRedirectUrl.
 */
export async function authSyncOAuthUser(payload: OAuthSyncPayload): Promise<AuthIdentity> {
    return api
        .post(`${IDENTITY_API_PREFIX}/oauth/sync`, {
            json: payload
        })
        .json<AuthIdentity>();
}

/**
 * Get user by id.
 */
export async function authGetDbUser(userId: string): Promise<Response> {
    return api.get(`${IDENTITY_API_PREFIX}/users/${encodeURIComponent(userId)}`);
}

/**
 * Get user by email.
 */
export async function authGetDbUserByEmail(email: string): Promise<Response> {
    return api.get(`${IDENTITY_API_PREFIX}/users/by-email/${encodeURIComponent(email)}`);
}

/**
 * Update user.
 * Must preserve user.settings because FuseSettingsProvider depends on it.
 */
export function authUpdateDbUser(user: PartialDeep<User>) {
    if (!user.id) {
        throw new Error('Cannot update user without an id.');
    }

    return api.put(`${IDENTITY_API_PREFIX}/users/${encodeURIComponent(String(user.id))}`, {
        body: JSON.stringify(UserModel(user))
    });
}

/**
 * Create user.
 * Kept for backward compatibility. Prefer authSignUpWithCredentials or authSyncOAuthUser.
 */
export async function authCreateDbUser(user: PartialDeep<User>) {
    return api.post(`${IDENTITY_API_PREFIX}/users`, {
        body: JSON.stringify(UserModel(user))
    });
}

/**
 * Non-blocking audit logging helper.
 */
export async function authCreateAuditEvent(payload: AuditEventPayload): Promise<void> {
    await api.post(`${IDENTITY_API_PREFIX}/audit-events`, {
        json: payload
    });
}
