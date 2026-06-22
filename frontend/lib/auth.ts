// lib/auth.ts
// next-auth v5 configuration.
//
// Strategy (ADR-0006):
//   - Credentials provider calls Go backend POST /auth/login
//   - On success, Go returns { access_token, user }
//   - We store access_token in next-auth JWT so middleware can read it
//   - access_token is also hydrated into the in-memory store on session load
//   - The httpOnly refresh cookie is set by Go — browser sends it on /auth/refresh automatically

import NextAuth, { type DefaultSession, type NextAuthConfig } from "next-auth";
import Credentials from "next-auth/providers/credentials";

// ----------------------------------------------------------
// Extend next-auth types to include our custom fields
// ----------------------------------------------------------

declare module "next-auth" {
  interface Session {
    user: {
      id: string;
      email: string;
      name: string;
      accessToken: string; // Go backend JWT
      activeOrgId: string | null;
      activeOrgSlug: string | null;
      activeRole: string | null;
    } & DefaultSession["user"];
    error?: "RefreshFailed";
  }

  interface User {
    id: string;
    email: string;
    name: string;
    accessToken: string;
    activeOrgId: string | null;
    activeOrgSlug: string | null;
    activeRole: string | null;
  }
}

declare module "next-auth/jwt" {
  interface JWT {
    accessToken: string;
    userId: string;
    activeOrgId: string | null;
    activeOrgSlug: string | null;
    activeRole: string | null;
    error?: "RefreshFailed";
  }
}

// ----------------------------------------------------------
// Auth config
// ----------------------------------------------------------

const backendUrl =
  process.env.BACKEND_INTERNAL_URL || "http://localhost:8080";

export const authConfig: NextAuthConfig = {
  providers: [
    Credentials({
      id: "credentials",
      name: "Email and password",
      credentials: {
        email: { label: "Email", type: "email" },
        password: { label: "Password", type: "password" },
      },
      async authorize(credentials) {
        if (!credentials?.email || !credentials?.password) return null;

        try {
          const res = await fetch(`${backendUrl}/api/v1/auth/login`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
              email: credentials.email,
              password: credentials.password,
            }),
            // Note: httpOnly cookie set by Go backend flows through automatically
            // because this runs server-side and the response sets Set-Cookie
          });

          if (!res.ok) return null;

          const body = await res.json();
          const data = body?.data;

          if (!data?.access_token || !data?.user) return null;

          return {
            id: data.user.id,
            email: data.user.email,
            name: data.user.name ?? data.user.email,
            accessToken: data.access_token,
            activeOrgId: null,
            activeOrgSlug: null,
            activeRole: null,
          };
        } catch (err) {
          console.error("[auth] Login failed:", err);
          return null;
        }
      },
    }),
  ],

  pages: {
    signIn: "/login",
    error: "/login",
  },

  session: {
    strategy: "jwt",
    maxAge: 7 * 24 * 60 * 60, // 7 days (matches refresh token TTL)
  },

  callbacks: {
    async jwt({ token, user, trigger, session: updateSession }) {
      // Initial sign-in — user object is present
      if (user) {
        token.userId = user.id;
        token.accessToken = user.accessToken;
        token.activeOrgId = user.activeOrgId;
        token.activeOrgSlug = user.activeOrgSlug;
        token.activeRole = user.activeRole;
      }

      // Session update triggered by useSession().update()
      // Used when user switches org — we update the org context in the token
      if (trigger === "update" && updateSession) {
        if (updateSession.accessToken)
          token.accessToken = updateSession.accessToken;
        if (updateSession.activeOrgId !== undefined)
          token.activeOrgId = updateSession.activeOrgId;
        if (updateSession.activeOrgSlug !== undefined)
          token.activeOrgSlug = updateSession.activeOrgSlug;
        if (updateSession.activeRole !== undefined)
          token.activeRole = updateSession.activeRole;
      }

      return token;
    },

    async session({ session, token }) {
      session.user.id = token.userId;
      session.user.accessToken = token.accessToken;
      session.user.activeOrgId = token.activeOrgId;
      session.user.activeOrgSlug = token.activeOrgSlug;
      session.user.activeRole = token.activeRole;
      if (token.error) session.error = token.error;
      return session;
    },
  },

  trustHost: true,
  secret: process.env.NEXTAUTH_SECRET,
};

export const { handlers, auth, signIn, signOut } = NextAuth(authConfig);
