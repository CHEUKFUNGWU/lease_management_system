const nextConfig = {
  reactStrictMode: true,
  async rewrites() {
    return [
      {
        source: "/api/core/:path*",
        destination: `${process.env.NEXT_PUBLIC_API_URL}/api/:path*`,
      },
      {
        source: "/api/ai/:path*",
        destination: `${process.env.NEXT_PUBLIC_AI_URL}/api/:path*`,
      },
    ];
  },
};

module.exports = nextConfig;
