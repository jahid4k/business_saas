// src/app/layout.tsx
import type { Metadata } from "next";
import { Syne, Inter } from "next/font/google";
import { ThemeProvider } from "next-themes";
import "./globals.css";
import QueryProvider from "@/components/providers/QueryProvider";

const syne = Syne({
  variable: "--font-syne",
  subsets: ["latin"],
  display: "swap",
});

const inter = Inter({
  variable: "--font-inter",
  subsets: ["latin"],
  display: "swap",
});

export const metadata: Metadata = {
  title: "BusinessSAAS",
  description: "The all-in-one business operating system",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html
      lang="en"
      // suppressHydrationWarning is required for next-themes —
      // it modifies the class attribute during hydration
      suppressHydrationWarning
      className={`${syne.variable} ${inter.variable}`}
    >
      <body>
        <ThemeProvider
          attribute="class"
          defaultTheme="dark"
          enableSystem={false}
          disableTransitionOnChange={false}
        >
          <QueryProvider>{children}</QueryProvider>
        </ThemeProvider>
      </body>
    </html>
  );
}
