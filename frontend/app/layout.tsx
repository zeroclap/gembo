import type { Metadata, Viewport } from "next";
import { Geist_Mono, Space_Grotesk } from "next/font/google";
import "./globals.css";
import { cn } from "@/lib/utils";
import { Providers } from "@/components/providers";
import { AppHeader } from "@/components/app-header";
import { ServiceWorkerRegistrar } from "@/components/service-worker-registrar";
import { AuthGuard } from "@/components/auth-guard";

const spaceGrotesk = Space_Grotesk({ subsets: ["latin"], variable: "--font-sans" });

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: "Gembo - Crypto Arbitrage",
  description: "High-performance crypto arbitrage platform",
  manifest: "/manifest.webmanifest",
  appleWebApp: {
    capable: true,
    statusBarStyle: "black-translucent",
    title: "Gembo",
    startupImage: "/icons/apple-touch-icon.png",
  },
  icons: {
    apple: "/icons/apple-touch-icon.png",
  },
};

export const viewport: Viewport = {
  themeColor: [
    { media: "(prefers-color-scheme: light)", color: "#1a7a5e" },
    { media: "(prefers-color-scheme: dark)", color: "#0a0a0a" },
  ],
  width: "device-width",
  initialScale: 1,
  minimumScale: 1,
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html
      lang="en"
      suppressHydrationWarning
      className={cn("h-full antialiased", geistMono.variable, spaceGrotesk.variable, "font-sans")}
    >
      <body className="h-full flex flex-col">
        <Providers>
          <AuthGuard>
            <AppHeader />
            <ServiceWorkerRegistrar />
            <div className="flex-1 min-h-0 flex flex-col pb-14 md:pb-0">
              {children}
            </div>
          </AuthGuard>
        </Providers>
      </body>
    </html>
  );
}
