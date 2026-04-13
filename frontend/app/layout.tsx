import type { Metadata } from "next";
import { Geist_Mono, Space_Grotesk } from "next/font/google";
import "./globals.css";
import { cn } from "@/lib/utils";
import { Providers } from "@/components/providers";
import { AppHeader } from "@/components/app-header";

const spaceGrotesk = Space_Grotesk({ subsets: ["latin"], variable: "--font-sans" });

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: "Gembo - Crypto Arbitrage",
  description: "High-performance crypto arbitrage platform",
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
          <AppHeader />
          <div className="flex-1 min-h-0 flex flex-col pb-14 md:pb-0">
            {children}
          </div>
        </Providers>
      </body>
    </html>
  );
}
