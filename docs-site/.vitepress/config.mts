import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'HotPlex',
  description: 'The Strategic Bridge for AI Agent Engineering - Stateful, Secure, and High-Performance.',
  lang: 'en-US',
  base: '/hotplex/',

  head: [
    ['link', { rel: 'icon', href: '/hotplex/favicon.ico' }],
    ['meta', { name: 'theme-color', content: '#00ADD8' }],
  ],

  themeConfig: {
    logo: '/logo.svg',
    siteTitle: 'HotPlex',

    nav: [
      { text: 'Home', link: '/' },
      { text: 'Guide', link: '/guide/getting-started' },
      { text: 'SDKs', link: '/sdks/go-sdk' },
      { text: 'Reference', link: '/reference/api' },
      { text: 'Plan', link: '/plan/technical-plan' },
      { text: 'Migration', link: '/migration/v0.9.0' },
      { text: 'GitHub', link: 'https://github.com/hrygo/hotplex' }
    ],

    sidebar: {
      '/migration/': [
        {
          text: 'Migration Guides',
          items: [
            { text: 'v0.9.0 (Current)', link: '/migration/v0.9.0' },
            { text: 'v0.8.0', link: '/migration/v0.8.0' }
          ]
        }
      ],
      '/guide/': [
        {
          text: 'Introduction',
          items: [
            { text: 'Getting Started', link: '/guide/getting-started' },
            { text: 'Quick Start', link: '/guide/quick-start' },
          ]
        },
        {
          text: 'Core Concepts',
          items: [
            { text: 'Architecture', link: '/guide/architecture' },
            { text: 'Security & Isolation', link: '/guide/security' }
          ]
        },
        {
          text: 'Connectivity',
          items: [
            { text: 'WebSocket Protocol', link: '/guide/websocket' },
            { text: 'OpenCode HTTP/SSE', link: '/guide/opencode-http' },
            { text: 'ChatApps Overview', link: '/guide/chatapps' },
            { text: '└─ DingTalk Deep Dive', link: '/guide/chatapps-dingtalk' },
            { text: '└─ Slack Deep Dive', link: '/guide/chatapps-slack' },
            { text: '└─ Slack Gap Analysis', link: '/guide/slack-gap-analysis' }
          ]
        },
        {
          text: 'Advanced Features',
          items: [
            { text: 'Event Hooks System', link: '/guide/hooks' },
            { text: 'Observability (OTel/Prom)', link: '/guide/observability' },
            { text: 'Docker Execution', link: '/guide/docker' }
          ]
        },
        {
          text: 'Operations',
          items: [
            { text: 'Production Deployment', link: '/guide/deployment' },
            { text: 'Benchmark & Performance', link: '/guide/performance' }
          ]
        }
      ],
      '/sdks/': [
        {
          text: 'SDKs',
          items: [
            { text: 'Go SDK', link: '/sdks/go-sdk' },
            { text: 'Python SDK', link: '/sdks/python-sdk' },
            { text: 'TypeScript SDK', link: '/sdks/typescript-sdk' }
          ]
        }
      ],
      '/reference/': [
        {
          text: 'Reference',
          items: [
            { text: 'API Reference', link: '/reference/api' }
          ]
        }
      ],
      '/plan/': [
        {
          text: 'Technical Plan',
          items: [
            { text: 'Development Plan', link: '/plan/technical-plan' }
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
