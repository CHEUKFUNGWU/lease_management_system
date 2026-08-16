const path = require("path");

const nextConfig = {
  reactStrictMode: true,
  webpack: (config) => {
    // M3 export libs: pptxgenjs resolves its ESM entry for "import" which
    // pulls node:fs/node:https; the prebuilt browser bundle is the same
    // library without the node shims.
    config.resolve.alias["pptxgenjs$"] = path.resolve(__dirname, "node_modules/pptxgenjs/dist/pptxgen.bundle.js");
    // The bundle keeps guarded node: requires; in the browser they are dead
    // code — scheme requests bypass resolve.alias, so stub them as externals.
    config.externals = config.externals || [];
    config.externals.push((context, request, callback) => {
      if (typeof request === "string" && request.startsWith("node:")) {
        return callback(null, "var {}");
      }
      return callback();
    });
    return config;
  },
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
