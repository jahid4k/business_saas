// app/(app)/page.tsx
import { auth } from "@/lib/auth";
import { redirect } from "next/navigation";

export default async function AppRootPage() {
  const session = await auth();

  console.log("The is from Sesssion: ", session);

  if (!session?.user) {
    redirect("/login");
  }

  // User logged in কিন্তু কোনো active org নেই → select-org
  if (!session.user.activeOrgSlug) {
    redirect("/app/select-org");
  }

  // Active org আছে → সরাসরি dashboard
  redirect(`/app/${session.user.activeOrgSlug}/dashboard`);
}
