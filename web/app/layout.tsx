import { Metadata } from "next";
import { AntdRegistry } from "@ant-design/nextjs-registry";
import { AuthProvider } from "./context/AuthContext";
import { LanguageProvider } from "./context/LanguageContext";
import ThemeProvider from "./components/ThemeProvider";
import "./globals.css";

export const metadata: Metadata = {
  title: "租赁管理系统",
  description: "零售集团租赁管理系统",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="zh-CN">
      <body>
        <AntdRegistry>
          <ThemeProvider>
            <AuthProvider>
              <LanguageProvider>{children}</LanguageProvider>
            </AuthProvider>
          </ThemeProvider>
        </AntdRegistry>
      </body>
    </html>
  );
}
