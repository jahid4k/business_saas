import { redirect } from "next/navigation";
import { auth } from "@/lib/auth";
import { Sidebar } from "@/components/layout/Sidebar";
import { Topbar } from "@/components/layout/Topbar";
import { TokenHydrator } from "@/components/shared/TokenHydrator";

export default async function AppLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const session = await auth();

  if (!session?.user) {
    redirect("/login");
  }

  return (
    <>
      {/* Hydrate in-memory access token from server session */}
      <TokenHydrator accessToken={session.user.accessToken} />

      <div className="flex h-screen overflow-hidden bg-muted/20">
        <Sidebar />

        <div className="flex flex-1 flex-col overflow-hidden">
          <Topbar />

          <main className="flex-1 overflow-y-auto">
            {children}
          </main>
        </div>
      </div>
    </>
  );
}
