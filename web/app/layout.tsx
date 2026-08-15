import { Metadata } from "next";
import { Inter } from "next/font/google";
import { AntdRegistry } from "@ant-design/nextjs-registry";
import { AuthProvider } from "./context/AuthContext";
import { LanguageProvider } from "./context/LanguageContext";
import ThemeProvider from "./components/ThemeProvider";
import "./globals.css";

// 自托管 Inter（STY-004）：CSS @import 渲染阻塞且依赖 Google 运行时；
// next/font 构建期内联字体文件，display: swap 避免首屏字体跳动。
// 中文字形不走 Inter，回退栈在 globals.css / tokens.ts / tailwind 保留。
const inter = Inter({
  subsets: ["latin"],
  weight: ["400", "500", "600"],
  display: "swap",
  variable: "--font-inter",
});

export const metadata: Metadata = {
  title: "零售经营分析工作站",
  description: "线下零售经营分析工作站：经营脉搏、门店 360、租金谈判测算与承租合同分析",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="zh-CN" className={inter.variable}>
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
