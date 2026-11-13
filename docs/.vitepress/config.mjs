import { defineConfig } from "vitepress";

export default defineConfig({
  lang: "zh-CN",
  title: "DevBox",
  description: "使用 SDK 或 HTTP API 创建、连接和操作云端沙箱。",
  base: "/devspace-devbox/",
  cleanUrls: true,
  themeConfig: {
    nav: [
      { text: "快速入门", link: "/" },
      { text: "API 参考", link: "/api/" },
      { text: "SDK 参考", link: "/reference/sdk" },
      { text: "CLI 参考", link: "/cli/" },
    ],
    sidebar: [
      {
        text: "开始使用",
        items: [
          { text: "快速入门", link: "/" },
          { text: "选择模板", link: "/guide/templates" },
        ],
      },
      {
        text: "使用沙箱",
        items: [
          { text: "命令与进程", link: "/guide/commands" },
          { text: "文件操作", link: "/guide/files" },
          { text: "交互式终端与 Git", link: "/guide/terminal-and-git" },
          { text: "生命周期", link: "/guide/lifecycle" },
        ],
      },
      {
        text: "API 参考",
        items: [
          { text: "通用约定", link: "/api/" },
          { text: "沙箱", link: "/api/sandboxes" },
          { text: "模板", link: "/api/templates" },
          { text: "进程与终端", link: "/api/processes" },
          { text: "文件系统", link: "/api/filesystem" },
        ],
      },
      {
        text: "SDK 参考",
        items: [
          { text: "沙箱", link: "/reference/sdk#沙箱" },
          { text: "模板", link: "/reference/sdk#模板" },
          { text: "命令与进程", link: "/reference/sdk#命令与进程" },
          { text: "文件", link: "/reference/sdk#文件" },
          { text: "终端", link: "/reference/sdk#终端" },
          { text: "Git", link: "/reference/sdk#git" },
          { text: "配置与异常", link: "/reference/sdk#配置" },
        ],
      },
      {
        text: "CLI 参考",
        items: [
          { text: "概览", link: "/cli/" },
          { text: "安装", link: "/cli/install" },
          { text: "登录与认证", link: "/cli/authentication" },
          { text: "模板", link: "/cli/templates" },
          { text: "沙箱管理", link: "/cli/sandboxes" },
        ],
      },
      {
        text: "参考",
        items: [
          { text: "问题排查", link: "/reference/troubleshooting" },
          { text: "免责声明", link: "/reference/disclaimer" },
        ],
      },
    ],
    search: { provider: "local" },
    outline: { label: "本页内容", level: [2, 3] },
    docFooter: { prev: "上一篇", next: "下一篇" },
    sidebarMenuLabel: "目录",
    returnToTopLabel: "返回顶部",
    socialLinks: [
      { icon: "github", link: "https://github.com/huaweicloud/devspace-devbox" },
    ],
  },
});
