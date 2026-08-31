import { Metadata } from "next";
import { cookies } from "next/headers";
import { AntdRegistry } from "@ant-design/nextjs-registry";
import { AuthProvider } from "./context/AuthContext";
import { LanguageProvider } from "./context/LanguageContext";
import ThemeProvider from "./components/ThemeProvider";
import { THEME_COOKIE, parseTheme, type AppTheme } from "./lib/theme-cookie";
import "./globals.css";

// Typography is intentionally local-first: the interface uses the operating
// system's SF Pro / Helvetica stack and falls back to installed CJK fonts.
export const metadata: Metadata = {
  title: "零售经营分析工作站",
  description: "线下零售经营分析工作站：经营脉搏、门店 360、租金谈判测算与承租合同分析",
};

/**
 * DARK-003: on a first visit there is no cookie and the server cannot read the
 * OS preference, so the document would always arrive light. This runs before
 * first paint, writes the cookie from `prefers-color-scheme`, and reloads once
 * so the server can render the right theme — antd's styles are generated
 * server-side and must agree with the markup, which is the whole reason the
 * theme is a server concern here.
 *
 * The `sessionStorage` guard makes it strictly one reload per session: if the
 * cookie somehow does not stick (blocked cookies), the page stays light rather
 * than reloading forever.
 */
const THEME_BOOTSTRAP = `
(function () {
  try {
    if (document.cookie.indexOf('${THEME_COOKIE}=') !== -1) return;
    if (sessionStorage.getItem('${THEME_COOKIE}-probed')) return;
    sessionStorage.setItem('${THEME_COOKIE}-probed', '1');
    if (!window.matchMedia || !window.matchMedia('(prefers-color-scheme: dark)').matches) return;
    document.cookie = '${THEME_COOKIE}=dark; path=/; max-age=31536000; samesite=lax';
    window.location.reload();
  } catch (e) {}
})();
`;

export default async function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const cookieStore = await cookies();
  const theme: AppTheme = parseTheme(cookieStore.get(THEME_COOKIE)?.value);

  return (
    <html lang="zh-CN" data-theme={theme}>
      <head>
        <script dangerouslySetInnerHTML={{ __html: THEME_BOOTSTRAP }} />
      </head>
      <body>
        <AntdRegistry>
          <ThemeProvider initialTheme={theme}>
            <AuthProvider>
              <LanguageProvider>{children}</LanguageProvider>
            </AuthProvider>
          </ThemeProvider>
        </AntdRegistry>
      </body>
    </html>
  );
}
