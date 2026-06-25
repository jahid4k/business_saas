// src/app/(dashboard)/[orgId]/layout.tsx
// Server component — unwraps params and passes orgId to the client shell
import OrgProvider from "@/components/providers/OrgProvider";

interface Props {
  children: React.ReactNode;
  params: Promise<{ orgId: string }>;
}

export default async function OrgLayout({ children, params }: Props) {
  const { orgId } = await params;
  return <OrgProvider orgId={orgId}>{children}</OrgProvider>;
}
