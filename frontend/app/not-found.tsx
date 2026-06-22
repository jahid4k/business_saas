import Link from "next/link";
import { Button } from "@/components/ui/button";

export default function NotFound() {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center text-center">
      <h1 className="mb-2 text-4xl font-bold">404</h1>
      <p className="mb-1 text-lg font-medium">Page not found</p>
      <p className="mb-8 text-sm text-muted-foreground">
        The page you&apos;re looking for doesn&apos;t exist or you don&apos;t
        have access to it.
      </p>
      <Button asChild>
        <Link href="/app">Go to dashboard</Link>
      </Button>
    </div>
  );
}
