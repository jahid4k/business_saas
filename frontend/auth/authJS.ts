import NextAuth from 'next-auth';
import { createStorage } from 'unstorage';
import memoryDriver from 'unstorage/drivers/memory';
import vercelKVDriver from 'unstorage/drivers/vercel-kv';
import { UnstorageAdapter } from '@auth/unstorage-adapter';
import type { NextAuthConfig, User as NextAuthUser } from 'next-auth';
import type { Provider } from 'next-auth/providers';
import Credentials from 'next-auth/providers/credentials';
import Facebook from 'next-auth/providers/facebook';
import Google from 'next-auth/providers/google';
import { NextResponse } from 'next/server';
import { z } from 'zod';

import { User } from '@/types/user';
import UserModel from '@/types/user/models/UserModel';
import {
    authCreateAuditEvent,
    authGetDbUser,
    authGetDbUserByEmail,
    authSignInWithCredentials,
    authSignUpWithCredentials,
    authSyncOAuthUser,
    type AuthIdentity
} from './authApi';

const isProduction = process.env.NODE_ENV === 'production';

const credentialsSchema = z.object({
    formType: z.enum(['signin', 'signup']).default('signin'),
    email: z.string().trim().toLowerCase().email(),
    password: z.string().min(8).max(72),
    displayName: z.string().trim().min(2).max(120).optional(),
    organizationName: z.string().trim().min(2).max(160).optional()
});

const publicPathPrefixes = [
    '/sign-in',
    '/sign-up',
    '/sign-out',
    '/forgot-password',
    '/reset-password',
    '/auth',
    '/api/auth',
    '/assets',
    '/_next',
    '/favicon.ico'
];

function createAuthStorage() {
    if (process.env.VERCEL) {
        const url = process.env.AUTH_KV_REST_API_URL;
        const token = process.env.AUTH_KV_REST_API_TOKEN;

        if (isProduction && (!url || !token)) {
            throw new Error('Missing AUTH_KV_REST_API_URL or AUTH_KV_REST_API_TOKEN for production Auth.js storage.');
        }

        if (url && token) {
            return createStorage({
                driver: vercelKVDriver({
                    url,
                    token,
                    env: false
                })
            });
        }
    }

    if (isProduction) {
        throw new Error('Production auth storage must be persistent. Configure Vercel KV or replace this with a database adapter.');
    }

    return createStorage({ driver: memoryDriver() });
}

function normalizeRoles(role: User['role']): string[] {
    if (Array.isArray(role)) {
        return role.filter(Boolean);
    }

    if (typeof role === 'string' && role.trim()) {
        return [role];
    }

    return [];
}

function normalizeAuthenticatedRoles(role: User['role']): string[] {
    const roles = normalizeRoles(role);
    return roles.length > 0 ? roles : ['user'];
}

function createAppUser(input: Partial<User>, fallback?: Partial<User>): User {
    return UserModel({
        id: input.id ?? fallback?.id ?? '',
        role: normalizeAuthenticatedRoles(input.role ?? fallback?.role ?? null),
        displayName: input.displayName ?? fallback?.displayName ?? input.email ?? fallback?.email ?? '',
        photoURL: input.photoURL ?? fallback?.photoURL ?? '',
        email: input.email ?? fallback?.email ?? '',
        shortcuts: input.shortcuts ?? fallback?.shortcuts ?? [],
        settings: input.settings ?? fallback?.settings ?? {},
        loginRedirectUrl: input.loginRedirectUrl ?? fallback?.loginRedirectUrl ?? '/',
        organizationId: input.organizationId ?? fallback?.organizationId ?? null,
        organizationRole: input.organizationRole ?? fallback?.organizationRole ?? null,
        permissions: input.permissions ?? fallback?.permissions ?? [],
        plan: input.plan ?? fallback?.plan ?? 'free',
        status: input.status ?? fallback?.status ?? 'active',
        emailVerified: input.emailVerified ?? fallback?.emailVerified ?? false
    });
}

function toNextAuthUser(user: AuthIdentity | User): NextAuthUser & User {
    const appUser = createAppUser(user);

    return {
        ...appUser,
        name: appUser.displayName,
        image: appUser.photoURL
    };
}

function isPublicPath(pathname: string) {
    return publicPathPrefixes.some((prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`));
}

function hasAnyRole(role: User['role'] | undefined, allowedRoles: string[]) {
    return normalizeRoles(role ?? null).some((currentRole) => allowedRoles.includes(currentRole));
}

function hasPermission(permissions: unknown, permission: string) {
    return Array.isArray(permissions) && permissions.includes(permission);
}

function getErrorStatus(error: unknown) {
    if (typeof error === 'object' && error !== null && 'response' in error) {
        const response = (error as { response?: Response }).response;
        return response?.status;
    }

    if (typeof error === 'object' && error !== null && 'status' in error) {
        return (error as { status?: number }).status;
    }

    return undefined;
}

async function getFreshAppUser(tokenUser: Partial<User>) {
    const userId = tokenUser.id;
    const email = tokenUser.email;

    if (userId) {
        const response = await authGetDbUser(userId);
        return (await response.json()) as User;
    }

    if (email) {
        const response = await authGetDbUserByEmail(email);
        return (await response.json()) as User;
    }

    return null;
}

async function safeAudit(payload: Parameters<typeof authCreateAuditEvent>[0]) {
    try {
        await authCreateAuditEvent(payload);
    } catch (error) {
        if (!isProduction) {
            console.warn('Audit event failed:', error);
        }
    }
}

const storage = createAuthStorage();

export const providers: Provider[] = [
    Credentials({
        credentials: {
            email: { label: 'Email', type: 'email' },
            password: { label: 'Password', type: 'password' },
            formType: { label: 'Form Type', type: 'text' },
            displayName: { label: 'Display Name', type: 'text' },
            organizationName: { label: 'Organization Name', type: 'text' }
        },
        async authorize(rawCredentials) {
            const parsed = credentialsSchema.safeParse(rawCredentials);

            if (!parsed.success) {
                return null;
            }

            try {
                const { formType, email, password, displayName, organizationName } = parsed.data;

                const authUser =
                    formType === 'signup'
                        ? await authSignUpWithCredentials({
                            email,
                            password,
                            displayName: displayName ?? email.split('@')[0],
                            organizationName
                        })
                        : await authSignInWithCredentials({ email, password });

                if (!authUser?.id || !authUser.email) {
                    return null;
                }

                if (authUser.status && !['active', 'pending_email_verification', 'invited'].includes(authUser.status)) {
                    return null;
                }

                return toNextAuthUser(authUser);
            } catch (error) {
                const status = getErrorStatus(error);

                if ([400, 401, 403, 404, 409, 422, 423, 429].includes(status ?? 0)) {
                    return null;
                }

                throw error;
            }
        }
    }),
    Google,
    Facebook
];

const config = {
    theme: { logo: '/assets/images/logo/logo.svg' },
    adapter: UnstorageAdapter(storage),
    pages: {
        signIn: '/sign-in',
        error: '/sign-in'
    },
    providers,
    basePath: '/auth',
    trustHost: true,
    secret: process.env.AUTH_SECRET,
    callbacks: {
        authorized({ auth, request }) {
            const { pathname, search } = request.nextUrl;

            if (isPublicPath(pathname)) {
                return true;
            }

            if (!auth?.user) {
                const signInUrl = new URL('/sign-in', request.nextUrl.origin);
                signInUrl.searchParams.set('callbackUrl', `${pathname}${search}`);
                return NextResponse.redirect(signInUrl);
            }

            if (pathname.startsWith('/admin')) {
                return hasAnyRole(auth.user.role, ['owner', 'admin']);
            }

            if (pathname.startsWith('/billing')) {
                return hasAnyRole(auth.user.role, ['owner', 'admin']) || hasPermission(auth.user.permissions, 'billing.manage');
            }

            return true;
        },
        async signIn({ user, account, profile }) {
            if (account?.provider && account.provider !== 'credentials' && !user.email) {
                return false;
            }

            if (account?.provider && account.provider !== 'credentials') {
                try {
                    const syncedUser = await authSyncOAuthUser({
                        provider: account.provider,
                        providerAccountId: account.providerAccountId,
                        email: user.email as string,
                        displayName: user.name,
                        photoURL: user.image,
                        emailVerified: Boolean((profile as { email_verified?: boolean })?.email_verified)
                    });

                    Object.assign(user, toNextAuthUser(syncedUser));
                } catch (error) {
                    if (!isProduction) {
                        console.error('OAuth user sync failed:', error);
                    }

                    return false;
                }
            }

            return true;
        },
        async jwt({ token, trigger, account, user }) {
            if (user) {
                const appUser = createAppUser({
                    id: user.id ?? token.sub ?? '',
                    email: user.email ?? '',
                    displayName: user.displayName ?? user.name ?? user.email ?? '',
                    photoURL: user.photoURL ?? user.image ?? '',
                    role: user.role ?? ['user'],
                    shortcuts: user.shortcuts ?? [],
                    settings: user.settings ?? {},
                    loginRedirectUrl: user.loginRedirectUrl ?? '/',
                    organizationId: user.organizationId ?? null,
                    organizationRole: user.organizationRole ?? null,
                    permissions: user.permissions ?? [],
                    plan: user.plan ?? 'free',
                    status: user.status ?? 'active',
                    emailVerified: user.emailVerified ?? false
                });

                token.sub = appUser.id;
                token.name = appUser.displayName;
                token.email = appUser.email;
                token.picture = appUser.photoURL;
                token.appUser = appUser;
            }

            if (trigger === 'update' && token.appUser) {
                try {
                    const freshUser = await getFreshAppUser(token.appUser);

                    if (freshUser) {
                        const appUser = createAppUser(freshUser, token.appUser);

                        token.sub = appUser.id;
                        token.name = appUser.displayName;
                        token.email = appUser.email;
                        token.picture = appUser.photoURL;
                        token.appUser = appUser;
                    }
                } catch (error) {
                    if (!isProduction) {
                        console.warn('Failed to refresh session user:', error);
                    }
                }
            }

            if (account?.provider) {
                token.provider = account.provider;
            }

            if (process.env.AUTH_EXPOSE_PROVIDER_ACCESS_TOKEN === 'true' && account?.access_token) {
                token.accessToken = account.access_token;
            }

            return token;
        },
        session({ session, token }) {
            const appUser = createAppUser((token.appUser ?? {}) as Partial<User>, {
                id: String(token.sub ?? ''),
                displayName: token.name ?? '',
                email: token.email ?? '',
                photoURL: token.picture ?? '',
                role: ['user']
            });

            session.user = {
                ...session.user,
                id: appUser.id,
                name: appUser.displayName,
                email: appUser.email,
                image: appUser.photoURL,
                role: appUser.role,
                organizationId: appUser.organizationId ?? null,
                organizationRole: appUser.organizationRole ?? null,
                permissions: appUser.permissions ?? [],
                plan: appUser.plan ?? 'free',
                status: appUser.status ?? 'active',
                emailVerified: appUser.emailVerified ?? false,
                loginRedirectUrl: appUser.loginRedirectUrl ?? '/'
            };

            /**
             * This is the key Fuse compatibility point.
             * useUser() reads `data?.db`, and FuseSettingsProvider reads `user?.settings` from it.
             */
            session.db = appUser;

            if (token.accessToken && typeof token.accessToken === 'string') {
                session.accessToken = token.accessToken;
            }

            return session;
        },
        redirect({ url, baseUrl }) {
            if (url.startsWith('/')) {
                return `${baseUrl}${url}`;
            }

            if (new URL(url).origin === baseUrl) {
                return url;
            }

            return baseUrl;
        }
    },
    events: {
        async signIn(message) {
            await safeAudit({
                action: 'auth.sign_in',
                userId: message.user?.id,
                email: message.user?.email,
                provider: message.account?.provider,
                organizationId: message.user?.organizationId,
                metadata: { isNewUser: message.isNewUser ?? false }
            });
        },
        async signOut(message) {
            await safeAudit({
                action: 'auth.sign_out',
                userId: 'token' in message ? message.token?.sub : undefined,
                email: 'token' in message ? (message.token?.email as string | undefined) : undefined,
                organizationId: 'token' in message ? message.token?.appUser?.organizationId : undefined
            });
        },
        async createUser(message) {
            await safeAudit({
                action: 'auth.create_user',
                userId: message.user?.id,
                email: message.user?.email,
                organizationId: message.user?.organizationId
            });
        },
        async linkAccount(message) {
            await safeAudit({
                action: 'auth.link_account',
                userId: message.user?.id,
                email: message.user?.email,
                provider: message.account?.provider,
                organizationId: message.user?.organizationId
            });
        },
        async updateUser(message) {
            await safeAudit({
                action: 'auth.update_user',
                userId: message.user?.id,
                email: message.user?.email,
                organizationId: message.user?.organizationId
            });
        }
    },
    logger: {
        error(code, ...message) {
            console.error('[auth:error]', code, ...message);
        },
        warn(code, ...message) {
            console.warn('[auth:warn]', code, ...message);
        },
        debug(code, ...message) {
            if (!isProduction) {
                console.debug('[auth:debug]', code, ...message);
            }
        }
    },
    session: {
        strategy: 'jwt',
        maxAge: Number(process.env.AUTH_SESSION_MAX_AGE_SECONDS ?? 7 * 24 * 60 * 60),
        updateAge: Number(process.env.AUTH_SESSION_UPDATE_AGE_SECONDS ?? 24 * 60 * 60)
    },
    debug: !isProduction
} satisfies NextAuthConfig;

export type AuthJsProvider = {
    id: string;
    name: string;
    style?: {
        text?: string;
        bg?: string;
    };
};

export const authJsProviderMap: AuthJsProvider[] = providers
    .map((provider) => {
        const providerData = typeof provider === 'function' ? provider() : provider;

        return {
            id: providerData.id,
            name: providerData.name,
            style: {
                text: (providerData as { style?: { text: string } }).style?.text,
                bg: (providerData as { style?: { bg: string } }).style?.bg
            }
        };
    })
    .filter((provider) => provider.id !== 'credentials');

export const { handlers, auth, signIn, signOut } = NextAuth(config);
