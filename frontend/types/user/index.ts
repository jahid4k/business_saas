import { FuseSettingsConfigType } from '@fuse/core/FuseSettings/FuseSettings';
import { PartialDeep } from 'type-fest';

export type UserRole = string[] | string | null;
export type UserPlan = 'free' | 'starter' | 'pro' | 'business' | 'enterprise' | string;
export type UserStatus = 'active' | 'pending_email_verification' | 'invited' | 'suspended' | 'deleted' | string;

/**
 * Application-level user object used by Fuse.
 *
 * Important:
 * - `settings` is intentionally preserved because FuseSettingsProvider reads it from useUser().
 * - `loginRedirectUrl` is intentionally preserved because AuthGuardRedirect/Fuse login flow can use it.
 * - SaaS fields are additive. Existing Fuse fields are not removed.
 */
export type User = {
    id: string;
    role: UserRole;
    displayName: string;
    photoURL?: string;
    email?: string;
    shortcuts?: string[];
    settings?: PartialDeep<FuseSettingsConfigType>;
    loginRedirectUrl?: string;

    /** SaaS / multi-tenant extensions */
    organizationId?: string | null;
    organizationRole?: string | null;
    permissions?: string[];
    plan?: UserPlan;
    status?: UserStatus;
    emailVerified?: boolean;
};
