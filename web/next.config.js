const nextConfig = {
  reactStrictMode: true,
  async rewrites() {
    // Server-side rewrites use Docker internal URLs (SERVER_* vars)
    // Client-side fetches use browser-accessible URLs (NEXT_PUBLIC_* vars)
    // Resolve server destinations inside the container at build time. A
    // relative NEXT_PUBLIC_* value is intentionally used by the browser and
    // must never become a self-referential rewrite destination.
    const serverApiUrl = process.env.SERVER_API_URL || "http://core-service:8080";
    const serverAiUrl = process.env.SERVER_AI_URL || "http://ai-service:8000";

    return [
      // Browser-safe same-origin aliases. The in-app browser only permits
      // the Web origin; these preserve the existing Core/AI API contracts
      // while avoiding a client-side cross-port request.
      {
        source: "/api/api/v1/:path*",
        destination: `${serverApiUrl}/api/v1/:path*`,
      },
      {
        source: "/api/ai/api/v1/:path*",
        destination: `${serverAiUrl}/api/v1/:path*`,
      },
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
