const nextConfig = {
  reactStrictMode: true,
  async rewrites() {
    // Server-side rewrites use Docker internal URLs (SERVER_* vars)
    // Client-side fetches use browser-accessible URLs (NEXT_PUBLIC_* vars)
    const serverApiUrl = process.env.SERVER_API_URL || process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
    const serverAiUrl = process.env.SERVER_AI_URL || process.env.NEXT_PUBLIC_AI_URL || "http://localhost:8081";

    return [
      {
        source: "/api/core/:path*",
        destination: `${serverApiUrl}/api/:path*`,
      },
      {
        source: "/api/ai/:path*",
        destination: `${serverAiUrl}/api/v1/:path*`,
      },
    ];
  },
};

module.exports = nextConfig;