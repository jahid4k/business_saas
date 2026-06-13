import type { Metadata } from "next";
import { SessionProvider } from "next-auth/react";

// internal imports
import App from "./App";
import "./globals.css";

export const metadata: Metadata = {
  title: { default: "BusinessSAAS", template: "%s · BusinessSAAS" },
  description: "BusinessSAAS — backend-dependent business dashboard",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  // const session = await auth();
  return (
    <html lang="en">
      <body>
        <SessionProvider basePath="/auth" session={session}>
          <App>{children}</App>
        </SessionProvider>
      </body>
    </html>
  );
}
