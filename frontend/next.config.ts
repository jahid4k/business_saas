import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  images: {
    remotePatterns: [
      {
        protocol: "http",
        hostname: "http://localhost:8080/**",
        pathname: "/uploads/**",
      },
    ],
  },
};

export default nextConfig;
