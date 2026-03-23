import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  async rewrites() {
    return [
      {
        source: '/api/:path*',
        // Using 127.0.0.1 is safer than localhost for Node.js -> Docker routing
        destination: 'http://127.0.0.1:8080/api/:path*', 
      },
    ]
  },
};

export default nextConfig;