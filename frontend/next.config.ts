import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  allowedDevOrigins: ["wro1-lmc-p20922.sparrow-artichoke.ts.net"],
  async rewrites() {
    return [
      {
        source: "/api/:path*",
        destination: "http://localhost:8080/:path*",
      },
    ];
  },
};

export default nextConfig;
