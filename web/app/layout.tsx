import { Metadata } from "next";

export const metadata: Metadata = {
  title: "IFRS 16 租赁管理系统",
  description: "零售集团 IFRS 16 租赁管理系统",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="zh-CN">
      <body>{children}</body>
    </html>
  );
}
