"use client";
// frontend/app/dashboard/layout.tsx
// Protected layout. Redirects to /login if not authenticated.

import { useEffect } from "react";
import { useRouter, usePathname } from "next/navigation";
import Link from "next/link";
import { useAuth } from "@/hooks/useAuth";

const navItems = [
  { href: "/dashboard", label: "Overview", icon: "◈" },
  { href: "/dashboard/businesses", label: "Businesses", icon: "⬡" },
  { href: "/dashboard/members", label: "Members", icon: "⬟" },
  { href: "/dashboard/tasks", label: "Tasks", icon: "◻" },
  { href: "/dashboard/roles", label: "Roles & Permissions", icon: "◆" },
  { href: "/dashboard/profile", label: "Profile", icon: "◉" },
];

export default function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const router = useRouter();
  const pathname = usePathname();
  const { isAuthenticated, user, currentBusiness, doLogout, doLogoutAll } =
    useAuth();

  useEffect(() => {
    if (!isAuthenticated) {
      router.push("/login");
    }
  }, [isAuthenticated, router]);

  if (!isAuthenticated) {
    return (
      <div className="min-h-screen bg-gray-950 flex items-center justify-center">
        <p className="text-gray-400 text-sm">Redirecting...</p>
      </div>
    );
  }

  async function handleLogout() {
    await doLogout();
    router.push("/login");
  }

  async function handleLogoutAll() {
    await doLogoutAll();
    router.push("/login");
  }

  return (
    <div className="min-h-screen bg-gray-950 flex">
      {/* Sidebar */}
      <aside className="w-60 bg-gray-900 border-r border-gray-800 flex flex-col">
        {/* Logo */}
        <div className="px-5 py-5 border-b border-gray-800">
          <h1 className="text-white font-bold text-sm tracking-wide">
            BUSINESSSAAS
          </h1>
          <p className="text-gray-500 text-xs mt-0.5">Test Interface</p>
        </div>

        {/* Business context badge */}
        {currentBusiness && (
          <div className="px-5 py-3 border-b border-gray-800">
            <p className="text-gray-500 text-xs uppercase tracking-wider mb-1">
              Workspace
            </p>
            <p className="text-white text-sm font-medium truncate">
              {currentBusiness.name}
            </p>
            <p className="text-gray-500 text-xs">{currentBusiness.slug}</p>
          </div>
        )}

        {/* Nav */}
        <nav className="flex-1 px-3 py-4 space-y-0.5">
          {navItems.map((item) => {
            const active = pathname === item.href;
            return (
              <Link
                key={item.href}
                href={item.href}
                className={`flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-colors ${
                  active
                    ? "bg-indigo-600 text-white"
                    : "text-gray-400 hover:text-white hover:bg-gray-800"
                }`}
              >
                <span className="text-base">{item.icon}</span>
                {item.label}
              </Link>
            );
          })}
        </nav>

        {/* User + logout */}
        <div className="px-4 py-4 border-t border-gray-800 space-y-2">
          {user && (
            <div className="px-1 mb-3">
              <p className="text-white text-sm font-medium">
                {user.first_name} {user.last_name}
              </p>
              <p className="text-gray-500 text-xs truncate">{user.email}</p>
            </div>
          )}
          <button
            onClick={handleLogout}
            className="w-full text-left px-3 py-2 rounded-lg text-sm text-gray-400 hover:text-white hover:bg-gray-800 transition-colors"
          >
            Sign out
          </button>
          <button
            onClick={handleLogoutAll}
            className="w-full text-left px-3 py-2 rounded-lg text-sm text-red-500 hover:text-red-400 hover:bg-gray-800 transition-colors"
          >
            Sign out all devices
          </button>
        </div>
      </aside>

      {/* Main content */}
      <main className="flex-1 overflow-auto">{children}</main>
    </div>
  );
}
