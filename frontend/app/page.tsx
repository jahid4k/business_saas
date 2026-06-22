import { redirect } from "next/navigation";
import { auth } from "@/lib/auth";

export default async function RootPage() {
  const session = await auth();

  if (session?.user) {
    const slug = session.user.activeOrgSlug;
    redirect(slug ? `/app/${slug}/dashboard` : "/app/select-org");
  }

  redirect("/login");
}
