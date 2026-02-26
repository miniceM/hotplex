import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'HotPlex',
  description: 'The Strategic Bridge for AI Agent Engineering - Stateful, Secure, and High-Performance.',
  lang: 'en-US',
  base: '/hotplex/',

  head: [
    ['link', { rel: 'icon', href: '/hotplex/favicon.ico' }],
    ['meta', { name: 'theme-color', content: '#00ADD8' }],
    ['meta', { name: 'google', content: 'notranslate' }],
  ],

  themeConfig: {
    logo: '/logo.svg',
    siteTitle: 'HotPlex',

    nav: [
      { text: 'Home', link: '/' },
      {
        text: 'Guide',
        items: [
          { text: 'Essentials', link: '/guide/getting-started' },
          { text: 'ChatApps', link: '/guide/chatapps' },
          { text: 'Providers', link: '/providers/claude' },
        ]
      },
      { text: 'SDKs', link: '/sdks/go-sdk' },
      { text: 'GitHub', link: 'https://github.com/hrygo/hotplex' }
    ],

    sidebar: {
      '/': [
        {
          text: 'Essentials',
          collapsed: false,
          items: [
            { text: 'Getting Started', link: '/guide/getting-started' },
            { text: 'Quick Start', link: '/guide/quick-start' },
            { text: 'Architecture', link: '/guide/architecture' },
            { text: 'Security', link: '/guide/security' },
            { text: 'Hooks System', link: '/guide/hooks' },
          ]
        },
        {
          text: 'AI Providers',
          collapsed: false,
          items: [
            { text: 'Claude Code', link: '/providers/claude' },
            { text: 'OpenCode', link: '/providers/opencode' },
          ]
        },
        {
          text: 'Connectivity',
          collapsed: false,
          items: [
            { text: 'WebSocket', link: '/guide/websocket' },
            { text: 'OpenCode HTTP/SSE', link: '/guide/opencode-http' },
          ]
        },
        {
          text: 'ChatApps Ecosystem',
          collapsed: true,
          items: [
            { text: 'Overview', link: '/guide/chatapps' },
            { text: 'Slack Integration', link: '/guide/chatapps-slack' },
            { text: 'Feishu / Lark', link: '/guide/chatapps-feishu' },
            { text: 'DingTalk', link: '/guide/chatapps-dingtalk' },
            { text: 'Gap Analysis', link: '/guide/slack-gap-analysis' },
          ]
        },
        {
          text: 'SDK Reference',
          collapsed: true,
          items: [
            { text: 'Go SDK', link: '/sdks/go-sdk' },
            { text: 'Python SDK', link: '/sdks/python-sdk' },
            { text: 'TypeScript SDK', link: '/sdks/typescript-sdk' },
            { text: 'API Reference', link: '/reference/api' },
          ]
        },
        {
          text: 'Engineering & Operations',
          collapsed: true,
          items: [
            { text: 'Observability', link: '/guide/observability' },
            { text: 'Docker Execution', link: '/guide/docker' },
            { text: 'Production Guide', link: '/guide/deployment' },
            { text: 'Performance', link: '/guide/performance' },
          ]
        },
        {
          text: 'Meta & Analysis',
          collapsed: true,
          items: [
            { text: 'Technical Plan', link: '/plan/technical-plan' },
          ]
        }
      ]
    },


    socialLinks: [
      { icon: 'github', link: 'https://github.com/hrygo/hotplex' }
    ],

    footer: {
      message: 'Released under the MIT License.',
      copyright: 'Copyright © 2026 HotPlex Team'
    },

    search: {
      provider: 'local'
    },

    editLink: {
      pattern: 'https://github.com/hrygo/hotplex/edit/main/docs-site/:path',
      text: 'Edit this page on GitHub'
    },

    lastUpdated: {
      text: 'Last updated',
      formatOptions: {
        dateStyle: 'medium',
        timeStyle: 'short'
      }
    }
  }
})
