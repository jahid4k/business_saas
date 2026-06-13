import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Required for the production Docker stage
  output: "standalone",

  env: {
    BACKEND_INTERNAL_URL:
      process.env.BACKEND_INTERNAL_URL ?? "http://localhost:8080",
  },

  images: {
    remotePatterns: [],
  },
};

export default nextConfig;
