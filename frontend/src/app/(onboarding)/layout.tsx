// src/app/(onboarding)/layout.tsx
// Auth-protected but no sidebar — sits between auth and dashboard
import AuthProvider from "@/components/providers/AuthProvider";

export default function OnboardingLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div
      className="min-h-screen relative overflow-hidden"
      style={{ background: "#0a0a0a" }}
    >
      {/* Grid */}
      <div
        aria-hidden="true"
        className="fixed inset-0 pointer-events-none select-none"
        style={{
          backgroundImage: `
            linear-gradient(rgba(255,255,255,0.02) 1px, transparent 1px),
            linear-gradient(90deg, rgba(255,255,255,0.02) 1px, transparent 1px)
          `,
          backgroundSize: "56px 56px",
        }}
      />
      {/* Purple top glow */}
      <div
        aria-hidden="true"
        className="fixed pointer-events-none"
        style={{
          top: 0,
          left: "50%",
          transform: "translateX(-50%)",
          width: "100%",
          maxWidth: "900px",
          height: "400px",
          background:
            "radial-gradient(ellipse at 50% 0%, rgba(124,58,237,0.14) 0%, transparent 68%)",
        }}
      />
      <div className="relative z-10">
        <AuthProvider>{children}</AuthProvider>
      </div>
    </div>
  );
}
