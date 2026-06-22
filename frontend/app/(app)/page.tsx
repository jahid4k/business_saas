import { redirect } from "next/navigation";
import { auth } from "@/lib/auth";

// /app — redirect to the user's active org dashboard
// If no active org in session, go to org selection page
export default async function AppRootPage() {
  const session = await auth();

  if (!session?.user) {
    redirect("/login");
  }

  if (session.user.activeOrgSlug) {
    redirect(`/app/${session.user.activeOrgSlug}/dashboard`);
  }

  // No org context yet — go to org selection
  redirect("/app/select-org");
}
