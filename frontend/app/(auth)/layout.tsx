import Link from "next/link";
import { Building2 } from "lucide-react";

export default function AuthLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-muted/30 px-4 py-12">
      {/* Logo */}
      <Link
        href="/"
        className="mb-8 flex items-center gap-2 text-lg font-semibold"
      >
        <div className="flex h-8 w-8 items-center justify-center rounded-md bg-primary">
          <Building2 className="h-5 w-5 text-primary-foreground" />
        </div>
        BusinessSAAS
      </Link>

      {/* Card */}
      <div className="w-full max-w-sm">{children}</div>

      {/* Footer */}
      <p className="mt-8 text-center text-xs text-muted-foreground">
        &copy; {new Date().getFullYear()} BusinessSAAS. All rights reserved.
      </p>
    </div>
  );
}
