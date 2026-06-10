// Type definitions and content factories for the Help page.
// Each locale exports a function that takes the current API base URL and host
// (e.g. "https://example.com" and "example.com") and returns a structured
// content tree rendered by HelpBlock.vue.

export interface HelpChrome {
  title: string
  tagline: string
  toc: string
  backHome: string
  backDashboard: string
  backToTop: string
  intro: string
  copy: string
  copied: string
}

export type UlItem = string | { html: string; children?: Block[] }

export type Block =
  | { t: 'p'; html: string }
  | { t: 'h3'; text: string }
  | { t: 'h4'; text: string }
  | { t: 'ul'; items: UlItem[] }
  | { t: 'ol'; items: string[] }
  | { t: 'steps'; items: string[] }
  | { t: 'code'; lang: string; code: string }
  | { t: 'callout'; variant: 'info' | 'warning' | 'tip'; html: string }
  | { t: 'table'; head?: string[]; rows: string[][] }
  | { t: 'faq'; items: Array<{ q: string; blocks: Block[] }> }

export interface Section {
  id: string
  title: string
  blocks: Block[]
}

export interface HelpContent {
  chrome: HelpChrome
  sections: Section[]
}

export type HelpFactory = (base: string, host: string) => HelpContent

// ---------------------------------------------------------------------------
// Chinese content
// ---------------------------------------------------------------------------

export const zh: HelpFactory = (base) => ({
  chrome: {
    title: '使用帮助',
    tagline: '零基础上手指南',
    toc: '目录',
    backHome: '返回首页',
    backDashboard: '返回工作台',
    backToTop: '回到顶部',
    intro: '本教程面向零基础用户，手把手教你安装配置 Claude Code（终端版 + VS Code 插件）、OpenClaw、Opencode，并接入我们的 API 中转服务，支持 openai、anthropic 协议。',
    copy: '复制',
    copied: '已复制'
  },
  sections: [
    {
      id: 'quick-start',
      title: '快速开始',
      blocks: [
        { t: 'steps', items: [
          `打开 <a href="${base}" target="_blank" rel="noopener noreferrer">${base}</a>（如在内地需开启香港等地区代理）`,
          '注册账号',
          `进入<a href="${base}/purchase" target="_blank" rel="noopener noreferrer">充值/订阅</a>界面，购买余额或者订阅套餐`,
          '生成 API 密钥 → 选择对应购买套餐的订阅（日卡 / 周卡 / 月卡）',
          '选择对应的客户端工具按照下方文档配置即可'
        ]}
      ]
    },
    {
      id: 'prepare',
      title: '1. 准备工作',
      blocks: [
        { t: 'p', html: '你需要一台电脑（macOS、Windows 或 Linux 均可），以及稳定的网络连接。' },
        { t: 'h3', text: '1.1 安装 Node.js' },
        { t: 'p', html: 'Claude Code 和 OpenClaw 都依赖 Node.js 22 或更高版本。' },
        { t: 'h4', text: 'macOS / Linux' },
        { t: 'p', html: '打开终端，粘贴以下命令并回车：' },
        { t: 'code', lang: 'bash', code: 'curl -fsSL https://fnm.vercel.app/install | bash' },
        { t: 'p', html: '安装完成后，关闭终端重新打开，然后运行：' },
        { t: 'code', lang: 'bash', code: 'fnm install 22\nfnm use 22\nnode -v' },
        { t: 'p', html: '看到类似 <code>v22.x.x</code> 的输出就说明安装成功。' },
        { t: 'h4', text: 'Windows' },
        { t: 'ul', items: [
          '访问 Node.js 官网，下载 LTS 版本（22.x）',
          '双击安装包，一路点「下一步」即可',
          '安装完成后，打开 PowerShell，输入 <code>node -v</code> 确认版本'
        ]},
        { t: 'callout', variant: 'info', html: '<strong>建议：</strong> Windows 用户强烈建议安装 WSL2，后续操作在 WSL2 的 Ubuntu 终端中进行体验更好。' },
        { t: 'code', lang: 'powershell', code: 'wsl --install' },
        { t: 'p', html: '安装后重启电脑即可使用。' }
      ]
    },
    {
      id: 'install-cc',
      title: '2. 安装 Claude Code（终端版）',
      blocks: [
        { t: 'p', html: 'Claude Code 是 Anthropic 官方的 AI 编程助手，可以在终端中直接使用。' },
        { t: 'callout', variant: 'warning', html: '由于 Claude 官方封锁了中国及香港用户，所以<strong>下载安装时需要翻墙至美国或日本</strong>。安装后即可不用翻墙直连我们的中转服务。' },
        { t: 'h3', text: 'macOS / Linux / WSL2' },
        { t: 'code', lang: 'bash', code: 'curl -fsSL https://claude.ai/install.sh | bash' },
        { t: 'h3', text: 'Windows PowerShell' },
        { t: 'code', lang: 'powershell', code: 'irm https://claude.ai/install.ps1 | iex' },
        { t: 'p', html: '<small>⚠️ Windows 用户需要先安装 Git for Windows。</small>' },
        { t: 'p', html: '安装完成后验证：' },
        { t: 'code', lang: 'bash', code: 'claude --version' },
        { t: 'p', html: '看到版本号输出即表示安装成功。Windows 若报错可尝试添加环境变量。' }
      ]
    },
    {
      id: 'config-cc',
      title: '3. 配置 Claude Code 使用中转服务',
      blocks: [
        { t: 'p', html: '这是最关键的一步——让 Claude Code 连接到我们的中转服务，而不是 Anthropic 官方 API。' },
        { t: 'h3', text: '方法一：环境变量配置（推荐）' },
        { t: 'h4', text: 'macOS / Linux / WSL2' },
        { t: 'p', html: '编辑你的 shell 配置文件：' },
        { t: 'code', lang: 'bash', code: '# 如果你用的是 zsh（macOS 默认）\nnano ~/.zshrc\n\n# 如果你用的是 bash\nnano ~/.bashrc' },
        { t: 'p', html: '在文件末尾添加：' },
        { t: 'code', lang: 'bash', code: `export ANTHROPIC_BASE_URL="${base}"\nexport ANTHROPIC_AUTH_TOKEN="sk-你的API Key"` },
        { t: 'p', html: '保存退出后执行：' },
        { t: 'code', lang: 'bash', code: 'source ~/.zshrc  # 或 source ~/.bashrc' },
        { t: 'h4', text: 'Windows PowerShell' },
        { t: 'code', lang: 'powershell', code: `$env:ANTHROPIC_BASE_URL = "${base}"\n$env:ANTHROPIC_AUTH_TOKEN = "sk-你的API Key"` },
        { t: 'p', html: '设置后重新打开 PowerShell 生效。' },
        { t: 'h4', text: 'Windows（通过 setx 命令）' },
        { t: 'code', lang: 'powershell', code: `setx ANTHROPIC_BASE_URL "${base}"\nsetx ANTHROPIC_AUTH_TOKEN "你的 key"` },
        { t: 'h3', text: '方法二：Claude Code 设置文件' },
        { t: 'p', html: '在终端中运行 <code>claude</code> 进入交互模式，输入 <code>/config</code> 打开设置，或者直接编辑 <code>~/.claude/settings.json</code>：' },
        { t: 'code', lang: 'json', code: `{\n  "env": {\n    "ANTHROPIC_BASE_URL": "${base}",\n    "ANTHROPIC_AUTH_TOKEN": "sk-你的API Key"\n  }\n}` },
        { t: 'p', html: '验证配置：' },
        { t: 'code', lang: 'bash', code: 'cd 任意一个项目目录\nclaude\n/model opus' },
        { t: 'h3', text: '方法三：使用 CC Switch 图形化工具（推荐新手）' },
        { t: 'p', html: 'CC Switch 是一款跨平台桌面工具，提供图形界面来管理 Claude Code、Codex、Gemini CLI 的 API 配置，支持一键切换、速度测试、MCP 管理等功能。' },
        { t: 'p', html: 'GitHub 地址：<a href="https://github.com/farion1231/cc-switch" target="_blank" rel="noopener noreferrer">https://github.com/farion1231/cc-switch</a>' },
        { t: 'h4', text: '第一步：下载安装' },
        { t: 'ul', items: [
          '<strong>Windows：</strong>下载 <code>CC-Switch-v{版本号}-Windows.msi</code>（或 <code>.zip</code> 便携版）',
          { html: '<strong>macOS：</strong>推荐用 Homebrew 安装', children: [
            { t: 'code', lang: 'bash', code: 'brew tap farion1231/ccswitch\nbrew install --cask cc-switch' },
            { t: 'p', html: '或下载 <code>CC-Switch-v{版本号}-macOS.zip</code> 手动安装' }
          ]},
          '<strong>Linux：</strong>下载 <code>.deb</code>（Ubuntu/Debian）、<code>.rpm</code>（Fedora）或 <code>.AppImage</code>（通用）'
        ]},
        { t: 'callout', variant: 'warning', html: 'macOS 首次打开可能提示「无法验证开发者」，请前往「系统设置」→「隐私与安全性」→ 点击「仍要打开」。' },
        { t: 'h4', text: '第二步：添加中转服务配置' },
        { t: 'ul', items: [
          '打开 CC Switch',
          '点击「Add Provider」（添加提供商）',
          '选择「Custom」（自定义）',
          { html: '填写信息：', children: [{ t: 'ul', items: [
            '名称：随便起，比如 <code>ccvibe 中转</code>',
            `Base URL：<code>${base}</code>`,
            'API Key：粘贴你在控制台获取的 API Key'
          ]}]},
          '点击保存'
        ]},
        { t: 'h4', text: '第三步：启用配置' },
        { t: 'ul', items: [
          '选中刚才创建的中转配置',
          '点击「Enable」（启用）',
          '重启终端或 Claude Code 即可生效'
        ]},
        { t: 'p', html: '也可以通过系统托盘图标快速切换——右键托盘图标，直接选择要使用的配置。' },
        { t: 'p', html: '验证配置：' },
        { t: 'code', lang: 'bash', code: 'cd 任意一个项目目录\nclaude\n/model opus' },
        { t: 'p', html: '进入 Claude Code 后，随便问一个问题（比如「你好，你是什么模型？」）。如果能正常回复，说明配置成功。' }
      ]
    },
    {
      id: 'vscode',
      title: '4. 安装 VS Code + Claude Code 插件',
      blocks: [
        { t: 'h3', text: '4.1 安装 VS Code' },
        { t: 'ul', items: [
          '访问 VS Code 官网',
          '下载对应系统的安装包',
          '安装并打开'
        ]},
        { t: 'h3', text: '4.2 安装 Claude Code 插件' },
        { t: 'ul', items: [
          '打开 VS Code',
          '按 <kbd>Cmd+Shift+X</kbd>（macOS）或 <kbd>Ctrl+Shift+X</kbd>（Windows/Linux）打开扩展面板',
          '搜索 <code>Claude Code</code>',
          '找到 Anthropic 官方发布的插件，点击「安装」'
        ]}
      ]
    },
    {
      id: 'config-vscode',
      title: '5. 配置 VS Code 中的 Claude Code 插件',
      blocks: [
        { t: 'h3', text: '5.1 打开 VS Code 设置' },
        { t: 'p', html: '按 <kbd>Cmd+,</kbd>（macOS）或 <kbd>Ctrl+,</kbd>（Windows/Linux）打开设置。' },
        { t: 'h3', text: '5.2 配置环境变量' },
        { t: 'p', html: '在 VS Code 的 <code>settings.json</code> 中添加（按 <kbd>Cmd+Shift+P</kbd> → 输入 <em>Open User Settings (JSON)</em>）：' },
        { t: 'code', lang: 'json', code: `{\n  "claude-code.env": {\n    "ANTHROPIC_BASE_URL": "${base}",\n    "ANTHROPIC_API_KEY": "sk-你的API Key"\n  },\n  "claudeCode.environmentVariables": [\n    { "name": "ANTHROPIC_BASE_URL", "value": "${base}" },\n    { "name": "ANTHROPIC_AUTH_TOKEN", "value": "sk-你的API Key" }\n  ]\n}` },
        { t: 'callout', variant: 'tip', html: '💡 如果你已经在第 3 步通过环境变量或 <code>~/.claude/settings.json</code> 配置过，VS Code 插件会自动继承，这一步可以跳过。' },
        { t: 'h3', text: '5.3 使用插件' },
        { t: 'ul', items: [
          '按 <kbd>Cmd+Shift+P</kbd>（macOS）或 <kbd>Ctrl+Shift+P</kbd>（Windows/Linux）',
          '输入 <code>Claude Code</code>',
          '选择 <code>Claude Code: Open in New Tab</code>',
          '开始对话'
        ]},
        { t: 'p', html: '如果打开还是提示需要登录请参考<a href="https://www.cnblogs.com/wuhaoliu/p/19537431" target="_blank" rel="noopener noreferrer">这个链接</a>，把 URL 和 API Key 替换成中转服务的即可。' }
      ]
    },
    {
      id: 'openclaw',
      title: '6. 安装配置 OpenClaw',
      blocks: [
        { t: 'p', html: 'OpenClaw 是一个开源的 AI 助手框架，可以把 Claude 接入 Telegram、微信、Discord 等各种平台。' },
        { t: 'h3', text: '6.1 安装 OpenClaw' },
        { t: 'h4', text: 'macOS / Linux / WSL2' },
        { t: 'code', lang: 'bash', code: 'curl -fsSL https://openclaw.ai/install.sh | bash' },
        { t: 'h4', text: 'Windows PowerShell' },
        { t: 'code', lang: 'powershell', code: 'iwr -useb https://openclaw.ai/install.ps1 | iex' },
        { t: 'p', html: '安装脚本会自动检测 Node.js 环境并启动配置向导。' },
        { t: 'h3', text: '6.2 配置向导' },
        { t: 'ul', items: [
          '选择 AI 提供商 → 选择 <strong>Anthropic</strong>',
          '输入 API Key → 粘贴你在控制台获取的 API Key',
          `设置 Base URL → 输入 <code>${base}/v1</code> 或 <code>${base}</code>`,
          '选择模型 → 推荐 <code>claude-sonnet-4-20250514</code>（性价比最高）',
          'Gateway 密码 → 设置一个密码，用于保护 OpenClaw 控制面板'
        ]},
        { t: 'p', html: '如果错过了配置向导，可以重新运行：' },
        { t: 'code', lang: 'bash', code: 'openclaw configure' },
        { t: 'h3', text: '6.3 手动配置（推荐）' },
        { t: 'p', html: '编辑 <code>~/.openclaw/openclaw.json</code>，建议配置 <code>"maxTokens": 65536</code>：' },
        { t: 'code', lang: 'json', code: `{\n  "agents": {\n    "defaults": {\n      "model": "ccvibe/claude-opus-4-6",\n      "models": {\n        "ccvibe/claude-opus-4-6": {}\n      }\n    }\n  },\n  "providers": {\n    "ccvibe": {\n      "baseUrl": "${base}/v1",\n      "authHeader": true,\n      "auth": "api-key",\n      "apiKey": "sk-替换成你的key",\n      "api": "anthropic-messages",\n      "models": [\n        {\n          "id": "claude-opus-4-6",\n          "name": "claude-opus-4-6",\n          "reasoning": false,\n          "contextWindow": 1000000,\n          "maxTokens": 65536\n        }\n      ]\n    }\n  }\n}` },
        { t: 'h3', text: '6.4 重启 OpenClaw' },
        { t: 'code', lang: 'bash', code: 'openclaw gateway restart' },
        { t: 'p', html: '然后访问控制面板：' },
        { t: 'code', lang: 'bash', code: 'openclaw dashboard' },
        { t: 'h3', text: '6.5 验证' },
        { t: 'code', lang: 'bash', code: 'openclaw status\nopenclaw doctor' },
        { t: 'p', html: '如果请求有问题，可以检查 <code>~/.openclaw/agents/main/agent/models.json</code> 是否有错误的缓存配置。如果一切正常，你会看到绿色的状态信息。' },
        { t: 'h3', text: '6.6 腾讯云 OpenClaw 配置' },
        { t: 'code', lang: 'json', code: `{\n  "provider": "anthropic",\n  "base_url": "${base}",\n  "api": "anthropic-messages",\n  "api_key": "your-api-key-here",\n  "model": {\n    "id": "claude-opus-4-6",\n    "name": "Claude Opus 4.6"\n  }\n}` },
        { t: 'p', html: '替换 <code>api_key</code>，重启龙虾即可使用。参考文档：<a href="https://cloud.tencent.com/developer/article/2625144" target="_blank" rel="noopener noreferrer">Anthropic Claude 配置</a>' }
      ]
    },
    {
      id: 'opencode',
      title: '7. Open Code',
      blocks: [
        { t: 'code', lang: 'json', code: `{\n  "$schema": "https://opencode.ai/config.json",\n  "provider": {\n    "anthropic": {\n      "options": {\n        "baseURL": "${base}/v1",\n        "apiKey": "sk-你的api key"\n      }\n    }\n  },\n  "model": "anthropic/claude-opus-4-6",\n  "small_model": "anthropic/claude-haiku-4-5"\n}` },
        { t: 'ul', items: [
          '<code>opencode.json</code> 一般在 <code>~/.config/opencode/opencode.json</code>，最新版本可能是 <code>opencode.jsonc</code>，请先确认',
          '请务必使用 anthropic 协议，不要用 openai 协议，否则可能调用报错'
        ]},
        { t: 'p', html: '启动：<code>opencode</code>' },
        { t: 'p', html: '参考文档：<a href="https://opencode.ai/docs/zh-cn/providers/" target="_blank" rel="noopener noreferrer">opencode.ai/docs</a>' }
      ]
    },
    {
      id: 'hermes',
      title: '8. Hermes Agent',
      blocks: [
        { t: 'p', html: 'Hermes Agent 是 <a href="https://github.com/NousResearch" target="_blank" rel="noopener noreferrer">Nous Research</a> 开源的「自进化」AI 智能体框架，能<strong>自动从任务中提炼技能、越用越强</strong>，让模型自主规划并调用工具完成多步任务。' },
        { t: 'p', html: '接入「中转」的本质就是把 Hermes <strong>指向一个 OpenAI 兼容的模型端点</strong>（base_url 必须以 <code>/v1</code> 结尾），不涉及网络代理。接入本服务时：端点填 <code>本服务地址/v1</code>、Key 填你的 <code>sk-...</code>、模型填本平台支持的模型。下面三种方法任选其一。' },
        { t: 'h4', text: '方法一：hermes model 交互式配置（最推荐）' },
        { t: 'steps', items: [
          '终端运行 <code>hermes model</code>',
          '在 provider 列表里选 <code>Custom endpoint (self-hosted / VLLM / etc.)</code>',
          `填写 Base URL <code>${base}/v1</code>、API Key（输入无回显，粘贴后直接回车）、模型名称（如 <code>gpt-5.5</code> / <code>claude-opus-4-8</code>）`,
          '保存后发一条消息验证回复正常；之后可随时用 <code>hermes model</code> 在不同配置间切换'
        ]},
        { t: 'h4', text: '方法二：编辑 ~/.hermes/config.yaml（长期固定）' },
        { t: 'code', lang: 'yaml', code: `# ~/.hermes/config.yaml\nmodel:\n  provider: custom\n  base_url: ${base}/v1\n  model: gpt-5.5\n  # api_key 留空则自动读取 .env 里的 OPENAI_API_KEY` },
        { t: 'p', html: '密钥务必放到 <code>~/.hermes/.env</code>（不要写进 config.yaml；Hermes 会自动加载并读取其中的 <code>OPENAI_API_KEY</code>，文件权限设为 0600）：' },
        { t: 'code', lang: 'bash', code: `echo 'OPENAI_API_KEY=sk-你的api key' >> ~/.hermes/.env` },
        { t: 'h4', text: '方法三：OPENAI_BASE_URL 环境变量（旧版兼容，不推荐）' },
        { t: 'p', html: '新版 Hermes 主模型已改为以 <code>config.yaml</code> 的 <code>model.base_url</code> 为准；<code>OPENAI_BASE_URL</code> 仅作为旧版回退仍被识别。若你在老环境里临时试用可用下面方式，正式配置请优先用方法一或方法二。' },
        { t: 'code', lang: 'bash', code: `# 持久化：写入 ~/.hermes/.env\nOPENAI_API_KEY=sk-你的api key\nOPENAI_BASE_URL=${base}/v1\n\n# 临时（单次会话）\nexport OPENAI_BASE_URL="${base}/v1" && hermes` },
        { t: 'callout', variant: 'tip', html: '💡 <strong>base_url 必须以 <code>/v1</code> 结尾</strong>（OpenAI 兼容标准路径，缺了常报 404）；API Key 只放 <code>.env</code>，切勿提交到公开仓库；若用 Anthropic 系模型，按对应工具改用 anthropic 协议地址。' },
        { t: 'ul', items: [
          '连接超时 → 用 <code>curl -v 本服务地址/v1/models</code> 测连通性',
          '回复乱码 / 格式异常 → 确认上游已开启 OpenAI 兼容',
          'API Key 报错 → 检查 <code>~/.hermes/.env</code> 路径与密钥前后空格',
          '请求 404 → base_url 缺少 <code>/v1</code> 后缀'
        ]},
        { t: 'p', html: '参考文档（官方）：<a href="https://hermes-agent.nousresearch.com/docs/user-guide/configuration" target="_blank" rel="noopener noreferrer">Configuration</a> · <a href="https://hermes-agent.nousresearch.com/docs/integrations/providers" target="_blank" rel="noopener noreferrer">AI Providers</a> · <a href="https://github.com/NousResearch/hermes-agent" target="_blank" rel="noopener noreferrer">GitHub</a>' }
      ]
    },
    {
      id: 'cursor',
      title: '9. Cursor',
      blocks: [
        { t: 'p', html: '参考文档：<a href="https://gitcode.csdn.net/69b92f9f0a2f6a37c5981c5e.html" target="_blank" rel="noopener noreferrer">gitcode.csdn.net</a>' }
      ]
    },
    {
      id: 'pricing',
      title: '10. 计费说明',
      blocks: [
        { t: 'h3', text: '10.1 计费方式' },
        { t: 'p', html: '我们的中转服务采用与 Anthropic 官方<strong>完全一致的 1:1 计价</strong>，按 Token 用量计费，不额外加价。' },
        { t: 'h3', text: '10.2 Claude 模型价格参考' },
        { t: 'table', head: ['模型', '基础输入 Token', '5分钟缓存写入', '1小时缓存写入', '缓存命中与刷新', '输出 Token'], rows: [
          ['Claude Fable 5', '$10 / MTok', '$12.50 / MTok', '$20 / MTok', '$1 / MTok', '$50 / MTok'],
          ['Claude Opus 4.8', '$5 / MTok', '$6.25 / MTok', '$10 / MTok', '$0.50 / MTok', '$25 / MTok'],
          ['Claude Opus 4.7', '$5 / MTok', '$6.25 / MTok', '$10 / MTok', '$0.50 / MTok', '$25 / MTok'],
          ['Claude Opus 4.6', '$5 / MTok', '$6.25 / MTok', '$10 / MTok', '$0.50 / MTok', '$25 / MTok'],
          ['Claude Opus 4.5', '$5 / MTok', '$6.25 / MTok', '$10 / MTok', '$0.50 / MTok', '$25 / MTok'],
          ['Claude Opus 4.1', '$15 / MTok', '$18.75 / MTok', '$30 / MTok', '$1.50 / MTok', '$75 / MTok'],
          ['Claude Opus 4（已弃用）', '$15 / MTok', '$18.75 / MTok', '$30 / MTok', '$1.50 / MTok', '$75 / MTok'],
          ['Claude Sonnet 4.6', '$3 / MTok', '$3.75 / MTok', '$6 / MTok', '$0.30 / MTok', '$15 / MTok'],
          ['Claude Sonnet 4.5', '$3 / MTok', '$3.75 / MTok', '$6 / MTok', '$0.30 / MTok', '$15 / MTok'],
          ['Claude Sonnet 4（已弃用）', '$3 / MTok', '$3.75 / MTok', '$6 / MTok', '$0.30 / MTok', '$15 / MTok'],
          ['Claude Haiku 4.5', '$1 / MTok', '$1.25 / MTok', '$2 / MTok', '$0.10 / MTok', '$5 / MTok'],
          ['Claude Haiku 3.5（已退役，Bedrock 与 Vertex AI 除外）', '$0.80 / MTok', '$1 / MTok', '$1.60 / MTok', '$0.08 / MTok', '$4 / MTok']
        ]},
        { t: 'p', html: '工具使用需要写模型全称，参考以下列表（区分大小写）：' },
        { t: 'table', rows: [
          ['<code>claude-fable-5</code>'],
          ['<code>claude-opus-4-8</code>'],
          ['<code>claude-opus-4-8-thinking</code>'],
          ['<code>claude-opus-4-7</code>'],
          ['<code>claude-opus-4-7-thinking</code>'],
          ['<code>claude-opus-4-6</code>'],
          ['<code>claude-opus-4-6-20260130</code>'],
          ['<code>claude-opus-4-6-thinking</code>'],
          ['<code>claude-opus-4-5-20251101</code>'],
          ['<code>claude-sonnet-4-6</code>'],
          ['<code>claude-sonnet-4-5</code>'],
          ['<code>claude-sonnet-4-5-20250929</code>'],
          ['<code>claude-sonnet-4-20250514</code>'],
          ['<code>claude-haiku-4-5-20251001</code>']
        ]},
        { t: 'callout', variant: 'warning', html: '不要使用低价的 haiku 模型，可能会遇到官方限流，建议使用 sonnet 或 opus 系列模型。' },
        { t: 'callout', variant: 'tip', html: '💡 <strong>Prompt Caching</strong>（提示缓存）可以大幅降低重复内容的费用，缓存命中的输入 Token 价格降低 90%。我们的中转服务完整支持 Prompt Caching。' },
        { t: 'h3', text: '10.3 Claude Code 日均费用参考' },
        { t: 'p', html: '根据 Anthropic 官方数据：' },
        { t: 'ul', items: [
          '平均每位开发者每天约 <strong>$6</strong>',
          '90% 的用户日均费用低于 <strong>$12</strong>',
          '使用 Sonnet 模型月均约 <strong>$100–200</strong>'
        ]},
        { t: 'h3', text: '10.4 省钱技巧' },
        { t: 'ul', items: [
          '使用 Sonnet 模型处理日常编码任务（性价比最高）',
          '只在复杂架构设计时切换到 Opus',
          '切换任务时用 <code>/clear</code> 清空上下文，避免无用 Token 消耗',
          '用 <code>/cost</code> 命令随时查看当前会话的费用',
          '用 <code>/compact</code> 命令压缩对话历史，减少上下文长度'
        ]}
      ]
    },
    {
      id: 'codex',
      title: '11. CodeX 使用教程',
      blocks: [
        { t: 'p', html: '支持的模型清单：' },
        { t: 'code', lang: 'bash', code: 'gpt-5.4\ngpt-5.4-mini\ngpt-5.5\ngpt-image-2\ncodex-auto-review' },
        { t: 'callout', variant: 'warning', html: 'Codex 用户创建秘钥的时候，分组一定要选择 <strong>Codex</strong> 的分组。' },
        { t: 'h3', text: '在 Codex 中使用' },
        { t: 'code', lang: 'bash', code: 'vi ~/.codex/config.toml' },
        { t: 'code', lang: 'toml', code: `model_provider = "OpenAI"\nmodel = "gpt-5.5"\nreview_model = "gpt-5.5"\nmodel_reasoning_effort = "xhigh"\ndisable_response_storage = true\nnetwork_access = "enabled"\nmodel_context_window = 200000\nmodel_auto_compact_token_limit = 160000\n\n[model_providers.OpenAI]\nname = "OpenAI"\nbase_url = "${base}/v1"\nwire_api = "responses"\nrequires_openai_auth = true\nrequest_max_retries = 4\nstream_max_retries = 5` },
        { t: 'code', lang: 'bash', code: 'vi ~/.codex/auth.json' },
        { t: 'code', lang: 'json', code: '{\n  "OPENAI_API_KEY": "你的API_KEY秘钥"\n}' },
        { t: 'h3', text: '其它工具使用参考（支持以下标准 openai 接口协议）' },
        { t: 'p', html: '请求示例（需要开启流式返回：<code>stream:true</code>）：' },
        { t: 'code', lang: 'bash', code: `curl ${base}/v1/chat/completions \\\n  -H "Content-Type: application/json" \\\n  -H "Authorization: Bearer sk-xxxx" \\\n  -d '{\n    "model": "gpt-5.5",\n    "messages": [\n      { "role": "user", "content": "你是谁？" }\n    ],\n    "stream": true\n  }'` },
        { t: 'code', lang: 'bash', code: `curl ${base}/v1/messages \\\n  -H "Content-Type: application/json" \\\n  -H "Authorization: Bearer sk-xxxx" \\\n  -d '{\n    "model": "gpt-5.5",\n    "messages": [\n      { "role": "user", "content": "你是谁？" }\n    ],\n    "stream": true\n  }'` },
        { t: 'code', lang: 'bash', code: `curl --request POST \\\n  --url ${base}/v1/responses \\\n  --header 'Authorization: Bearer sk-xxxx' \\\n  --header 'Content-Type: application/json' \\\n  --data '{\n    "model": "gpt-5.5",\n    "input": [\n      {\n        "role": "user",\n        "content": [\n          {"type": "input_text", "text": "Hello, what can you do?"}\n        ]\n      }\n    ],\n    "stream": true,\n    "reasoning": {"effort": "high"}\n  }'` }
      ]
    },
    {
      id: 'image-gen',
      title: '12. 图像生成（gpt-image-2）',
      blocks: [
        { t: 'h3', text: '注意事项' },
        { t: 'callout', variant: 'warning', html: 'image-2 生图不能用于非法目的，会触发 GPT 的风控。我们这边也有自己的风控系统，部分内容会被打回要求修改提示词，我们也会定时检查各渠道的风控情况。' },
        { t: 'p', html: 'GPT 偶尔会异常触发风控（比如某些看起来无害的提示词被拦），并不是我们系统的问题。' },
        { t: 'h3', text: '超时设置（重要）' },
        { t: 'callout', variant: 'warning', html: '生图是<strong>同步阻塞</strong>请求：高分辨率（2K/4K）、图改图、<code>n&gt;1</code> 批量时，单次可能耗时数分钟。<strong>客户端超时建议设到 600s</strong>，否则会在图片就绪前被客户端主动断开，表现为请求超时但渠道其实仍在出图。' },
        { t: 'ul', items: [
          'curl：加 <code>--max-time 600</code>',
          'Python requests：<code>timeout=600</code>（下方示例已按 600 给出）',
          'OpenAI SDK：<code>OpenAI(base_url=..., api_key=..., timeout=600)</code>'
        ]},
        { t: 'h4', text: 'Codex 配置' },
        { t: 'p', html: '用 Codex CLI 走本网关时，把 SSE 空闲超时从默认 <code>300000</code>ms（5 分钟）调大到 <code>600000</code>ms（10 分钟），写在 <code>~/.codex/config.toml</code> 对应的 <code>[model_providers.OpenAI]</code> 段（其余字段见上方「11. CodeX 使用教程」）：' },
        { t: 'code', lang: 'toml', code: `[model_providers.OpenAI]\nname = "OpenAI"\nbase_url = "${base}/v1"\nwire_api = "responses"\nrequires_openai_auth = true\nstream_idle_timeout_ms = 600000   # 长任务/生图：SSE 空闲超时调到 600s（默认 300000=5 分钟）` },
        { t: 'p', html: '若服务前面还有 nginx 反向代理，默认 60s 超时会把长任务 / compact 请求提前掐断；在对应的 <code>server</code> 或 <code>location</code> 段把超时同步调到 600s：' },
        { t: 'code', lang: 'nginx', code: 'proxy_read_timeout 600s;\nproxy_send_timeout 600s;\nsend_timeout 600s;' },
        { t: 'h3', text: '接口示例' },
        { t: 'p', html: '图片接口支持通过 <code>size</code> 自动推断输出档位：<code>1024x1024</code> / <code>1k</code> 为原图，<code>2048x2048</code> / <code>2k</code> 为 2K，<code>3840x2160</code> / <code>4k</code> 为 4K；也可显式传 <code>upscale</code> 覆盖。' },
        { t: 'h4', text: 'Curl 文生图' },
        { t: 'code', lang: 'bash', code: `curl ${base}/images/generations \\\n  -H "Authorization: Bearer \${YOUR_API_KEY}" \\\n  -H "Content-Type: application/json" \\\n  -d '{\n    "model": "gpt-image-2",\n    "prompt": "A cute orange cat playing with yarn, studio ghibli style",\n    "n": 1,\n    "size": "1024x1024"\n  }'` },
        { t: 'p', html: '图片质量参数（可选）：<code>quality</code>：<code>low</code> / <code>medium</code> / <code>high</code> / <code>auto</code>' },
        { t: 'h4', text: 'Curl 图改图' },
        { t: 'code', lang: 'bash', code: `curl ${base}/images/edits \\\n  -H "Authorization: Bearer \${YOUR_API_KEY}" \\\n  -F model="gpt-image-2" \\\n  -F prompt="Restyle this image as a watercolor painting, soft pastel palette" \\\n  -F n=1 \\\n  -F size="1024x1024" \\\n  -F image="@cat.png"` },
        { t: 'p', html: '图改图使用 OpenAI 兼容 <code>multipart/form-data</code>；单次最多 4 张，单张最大 20MB。' },
        { t: 'h4', text: 'Python（OpenAI SDK）' },
        { t: 'code', lang: 'python', code: `from openai import OpenAI\n\nclient = OpenAI(\n    base_url="${base}",\n    api_key="\${YOUR_API_KEY}",\n)\n\nresp = client.images.generate(\n    model="gpt-image-2",\n    prompt="A cute orange cat playing with yarn",\n    n=1,\n    size="1024x1024",\n)\nprint(resp.data[0].url)` },
        { t: 'h4', text: 'Python（requests 文生图）' },
        { t: 'code', lang: 'python', code: `import requests\n\nAPI_KEY = "\${YOUR_API_KEY}"\nBASE_URL = "${base}"\n\nresp = requests.post(\n    f"{BASE_URL}/images/generations",\n    headers={\n        "Authorization": f"Bearer {API_KEY}",\n        "Content-Type": "application/json",\n    },\n    json={\n        "model": "gpt-image-2",\n        "prompt": "A cute orange cat playing with yarn",\n        "n": 1,\n        "size": "1024x1024",\n    },\n    timeout=600,\n)\nresp.raise_for_status()\ndata = resp.json()\nprint(data["data"][0]["url"])` },
        { t: 'h4', text: 'Python（requests 图改图）' },
        { t: 'code', lang: 'python', code: `import requests\n\nAPI_KEY = "\${YOUR_API_KEY}"\nBASE_URL = "${base}"\n\nresp = requests.post(\n    f"{BASE_URL}/images/edits",\n    headers={\n        "Authorization": f"Bearer {API_KEY}",\n    },\n    data={\n        "model": "gpt-image-2",\n        "prompt": "Turn this image into a watercolor painting",\n        "n": "1",\n        "size": "1024x1024",\n    },\n    files={\n        "image": open("cat.png", "rb"),\n    },\n    timeout=600,\n)\nresp.raise_for_status()\nitem = resp.json()["data"][0]\nprint(item["url"])` },
        { t: 'p', html: '图改图上传字段使用 <code>image</code> / <code>image[]</code>；返回图片访问 url。' },
        { t: 'h3', text: '尺寸说明' },
        { t: 'h4', text: '一、OpenAI 官方常见支持尺寸' },
        { t: 'p', html: '目前官方最稳定的是这些（兼容性最好的三组）：' },
        { t: 'table', head: ['比例', '尺寸'], rows: [
          ['1:1（正方形）', '<code>1024x1024</code>'],
          ['3:2（横向）', '<code>1536x1024</code>'],
          ['2:3（竖向）', '<code>1024x1536</code>']
        ]},
        { t: 'h4', text: '二、gpt-image-2 新增支持' },
        { t: 'p', html: '相比旧模型，现在很多渠道已经支持自定义 <code>"size": "宽x高"</code>，例如：' },
        { t: 'code', lang: 'json', code: '"size": "1920x1080"\n"size": "2048x2048"\n"size": "3840x2160"' },
        { t: 'h4', text: '三、理论支持范围' },
        { t: 'p', html: '宽高要求（多数兼容实现）：' },
        { t: 'ul', items: [
          '必须是整数',
          '建议 64 的倍数',
          '部分渠道要求 32 的倍数'
        ]},
        { t: 'p', html: '✅ 合法：<code>1024x1024</code>、<code>1536x1024</code>、<code>1920x1080</code>、<code>2048x1152</code>、<code>3840x2160</code>' },
        { t: 'p', html: '❌ 可能失败：<code>1001x777</code>、<code>1919x1079</code>' },
        { t: 'h4', text: '四、常见比例推荐' },
        { t: 'table', head: ['比例', '尺寸', '适用场景'], rows: [
          ['1:1', '<code>1024x1024</code> / <code>1536x1536</code> / <code>2048x2048</code>', 'Logo、商品图、头像'],
          ['16:9（横）', '<code>1280x720</code> / <code>1920x1080</code> / <code>2560x1440</code> / <code>3840x2160</code>', 'PPT、Banner、视频封面'],
          ['9:16（竖）', '<code>1080x1920</code> / <code>1440x2560</code> / <code>2160x3840</code>', '手机壁纸、小红书、抖音'],
          ['4:3', '<code>1600x1200</code> / <code>2048x1536</code>', '传统照片'],
          ['3:2', '<code>1536x1024</code> / <code>1920x1280</code>', '相机比例']
        ]}
      ]
    },
    {
      id: 'video-gen',
      title: '13. 视频生成（Sora/Seedance 2.0）',
      blocks: [
        { t: 'p', html: '视频生成是<strong>异步任务接口</strong>：创建任务 → 轮询状态 → 下载视频。需使用在<strong>视频分组</strong>下创建的 API Key。支持 Sora（<strong>按秒计费</strong>）与 Seedance 2.0（<strong>按次计费</strong>）两类模型。' },
        { t: 'h3', text: '支持的模型' },
        { t: 'table', head: ['模型', '分辨率', '计费方式'], rows: [
          ['<code>sora-v3-fast</code>', '480p', '按秒'],
          ['<code>sora-v3-pro</code>', '720p', '按秒'],
          ['<code>seedance-2.0-fast-pass</code>', '720p', '按次（固定单价，时长不影响费用）'],
          ['<code>seedance-2.0-pass</code>', '720p', '按次（固定单价，时长不影响费用）']
        ]},
        { t: 'h3', text: '接口流程' },
        { t: 'ul', items: [
          '① <code>POST /v1/videos</code> 创建任务，返回 <code>id</code>（task_id）',
          '② <code>GET /v1/videos/{task_id}</code> 轮询状态，直到 <code>status=completed</code>（建议每 5 秒一次）',
          '③ <code>GET /v1/videos/{task_id}/content</code> 下载视频（mp4）'
        ]},
        { t: 'h4', text: '1) 创建任务' },
        { t: 'code', lang: 'bash', code: `curl ${base}/videos \\\n  -H "Authorization: Bearer \${YOUR_API_KEY}" \\\n  -H "Content-Type: application/json" \\\n  -d '{\n    "model": "sora-v3-fast",\n    "prompt": "雨夜霓虹街道，镜头缓慢推进，电影感光影",\n    "aspect_ratio": "16:9",\n    "resolution": "480p",\n    "seconds": "5"\n  }'` },
        { t: 'p', html: '返回（任务已入队）：' },
        { t: 'code', lang: 'json', code: '{\n  "id": "task_xxx",\n  "object": "video",\n  "model": "sora-v3-fast",\n  "status": "queued",\n  "progress": 0,\n  "created_at": 1779560000\n}' },
        { t: 'p', html: 'Seedance 2.0（按次计费，<code>duration</code> 支持 4/5/10/15）：' },
        { t: 'code', lang: 'bash', code: `curl ${base}/videos \\\n  -H "Authorization: Bearer \${YOUR_API_KEY}" \\\n  -H "Content-Type: application/json" \\\n  -d '{\n    "model": "seedance-2.0-pass",\n    "prompt": "黄昏海边，海浪缓慢拍打礁石，镜头缓慢拉升，电影感",\n    "ratio": "16:9",\n    "resolution": "720p",\n    "duration": 15\n  }'` },
        { t: 'h4', text: '2) 轮询状态' },
        { t: 'code', lang: 'bash', code: `curl ${base}/videos/task_xxx \\\n  -H "Authorization: Bearer \${YOUR_API_KEY}"` },
        { t: 'p', html: '完成（<code>status=completed</code>）返回：' },
        { t: 'code', lang: 'json', code: '{\n  "id": "task_xxx",\n  "status": "completed",\n  "progress": 100,\n  "seconds": "5",\n  "size": "854x480",\n  "video_url": ".../v1/videos/task_xxx/content"\n}' },
        { t: 'p', html: '状态值：<code>queued</code> / <code>in_progress</code> / <code>completed</code> / <code>failed</code>。' },
        { t: 'h4', text: '3) 下载视频' },
        { t: 'code', lang: 'bash', code: `curl ${base}/videos/task_xxx/content \\\n  -H "Authorization: Bearer \${YOUR_API_KEY}" \\\n  -o out.mp4` },
        { t: 'h3', text: '参数说明' },
        { t: 'table', head: ['参数', '必填', '说明'], rows: [
          ['<code>model</code>', '是', 'sora-v3-fast / sora-v3-pro / seedance-2.0-fast-pass / seedance-2.0-pass'],
          ['<code>prompt</code>', '是', '提示词'],
          ['<code>resolution</code>', '否', '480p / 720p / 1080p（Sora 按秒时 ≥1080 走高清档；Seedance 用 720p）'],
          ['<code>aspect_ratio</code> / <code>ratio</code>', '否', '画面比例，二者等价（16:9 / 9:16 / 1:1 等）'],
          ['<code>seconds</code> / <code>duration</code>', '否', '时长，二者等价（Sora 5/10/15；Seedance 4/5/10/15）。按次计费时时长不影响费用'],
          ['<code>image_url</code>', '否', 'Sora 参考图（图生视频）'],
          ['<code>first_image</code> / <code>last_image</code>', '否', 'Seedance 首尾帧，须成对，且不能与参考图/视频同用'],
          ['<code>referenceImages</code>', '否', 'Seedance 参考图数组（最多 4）'],
          ['<code>referenceVideos</code>', '否', 'Seedance 参考视频数组（最多 3）']
        ]},
        { t: 'h4', text: '失败返回示例' },
        { t: 'code', lang: 'json', code: '// 403 余额不足\n{ "code": "INSUFFICIENT_BALANCE", "message": "Insufficient account balance" }\n\n// 403 分组未开视频\n{ "error": { "type": "permission_error", "message": "Video generation is not enabled for this group" } }\n\n// 503 无可用账号\n{ "error": { "type": "api_error", "message": "No available compatible accounts" } }' },
        { t: 'callout', variant: 'tip', html: '💡 计费在<strong>创建成功</strong>时扣一次（按 request_id 幂等），失败的创建（鉴权/余额/上游错误）不计费，轮询与下载不计费。<strong>按秒</strong>（Sora）= 时长 × 每秒价（按分辨率档）× 倍率；<strong>按次</strong>（Seedance 2.0）= 固定单价 × 倍率，与时长/分辨率无关。' },
        { t: 'callout', variant: 'warning', html: '生成通常需要几分钟。优先用 <code>GET /v1/videos/{task_id}/content</code> 下载（自动路由到创建任务的同一账号）；部分中转该端点不支持 API Key 下载（返回 401），此时改用完成响应里的 <code>video_url</code> 直链下载。' }
      ]
    },
    {
      id: 'faq',
      title: '14. 常见问题',
      blocks: [
        { t: 'h3', text: '安装相关' },
        { t: 'faq', items: [
          { q: '安装时提示 <code>command not found: claude</code> 或 \'claude\' 不是内部或外部命令？', blocks: [
            { t: 'p', html: '这是最常见的问题，说明 Claude Code 的安装路径没有加入系统 PATH。' },
            { t: 'p', html: '<strong>macOS / Linux：</strong>' },
            { t: 'code', lang: 'bash', code: "# 检查 PATH 中是否包含安装目录\necho $PATH | tr ':' '\\n' | grep local/bin\n\n# 如果没有输出，添加到 shell 配置\n# zsh 用户（macOS 默认）\necho 'export PATH=\"$HOME/.local/bin:$PATH\"' >> ~/.zshrc\nsource ~/.zshrc\n\n# bash 用户（Linux 默认）\necho 'export PATH=\"$HOME/.local/bin:$PATH\"' >> ~/.bashrc\nsource ~/.bashrc" },
            { t: 'p', html: '<strong>Windows PowerShell：</strong>' },
            { t: 'code', lang: 'powershell', code: "# 检查 PATH\n$env:PATH -split ';' | Select-String 'local\\\\bin'\n\n# 如果没有输出，添加到 PATH\n$currentPath = [Environment]::GetEnvironmentVariable('PATH', 'User')\n[Environment]::SetEnvironmentVariable('PATH', \"$currentPath;$env:USERPROFILE\\.local\\bin\", 'User')" },
            { t: 'p', html: '添加后重新打开终端即可。' }
          ]},
          { q: '安装脚本报错 <code>syntax error near unexpected token \'&lt;\'</code>？', blocks: [
            { t: 'p', html: '说明安装脚本下载到的是 HTML 页面，通常是网络问题。' },
            { t: 'ul', items: [
              '检查能否访问 Google Cloud Storage：<code>curl -sI https://storage.googleapis.com</code>',
              { html: '使用替代安装方式：', children: [
                { t: 'code', lang: 'bash', code: '# macOS\nbrew install --cask claude-code\n\n# Windows\nwinget install Anthropic.ClaudeCode' }
              ]},
              '如果提示 <em>App unavailable in region</em>，请使用代理网络'
            ]}
          ]},
          { q: '安装时提示 <code>curl: (56) Failure writing output to destination</code>？', blocks: [
            { t: 'p', html: '下载过程中网络连接中断了。' },
            { t: 'ul', items: [
              '检查网络稳定性，重试安装命令',
              { html: '或者先下载脚本再执行：', children: [
                { t: 'code', lang: 'bash', code: 'curl -fsSL https://claude.ai/install.sh -o install.sh\nbash install.sh' }
              ]}
            ]}
          ]},
          { q: '安装时报 TLS / SSL 错误？', blocks: [
            { t: 'p', html: '常见错误：<code>TLS connect error</code>、<code>SSL/TLS secure channel</code>、<code>unable to get local issuer certificate</code>。' },
            { t: 'code', lang: 'bash', code: '# Ubuntu/Debian 更新证书\nsudo apt-get update && sudo apt-get install ca-certificates\n\n# macOS 更新证书\nbrew install ca-certificates' },
            { t: 'p', html: 'Windows 用户在 PowerShell 中先执行：' },
            { t: 'code', lang: 'powershell', code: '[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12' },
            { t: 'p', html: '然后重新运行安装命令。' }
          ]},
          { q: 'Linux 服务器安装时被 Killed？', blocks: [
            { t: 'p', html: '系统内存不足，被 OOM Killer 终止。Claude Code 安装至少需要 4GB 内存。添加 Swap 空间：' },
            { t: 'code', lang: 'bash', code: 'sudo fallocate -l 2G /swapfile\nsudo chmod 600 /swapfile\nsudo mkswap /swapfile\nsudo swapon /swapfile' },
            { t: 'p', html: '然后重新安装。' }
          ]},
          { q: 'Windows 提示 <code>Claude Code on Windows requires git-bash</code>？', blocks: [
            { t: 'ul', items: [
              '下载安装 Git for Windows',
              '安装时勾选 <em>Add to PATH</em>',
              '重启终端'
            ]},
            { t: 'p', html: '如果已安装但仍报错，在 <code>~/.claude/settings.json</code> 中指定路径：' },
            { t: 'code', lang: 'json', code: '{\n  "env": {\n    "CLAUDE_CODE_GIT_BASH_PATH": "C:\\\\Program Files\\\\Git\\\\bin\\\\bash.exe"\n  }\n}' }
          ]},
          { q: 'Windows 上 <code>irm</code> 或 <code>&amp;&amp;</code> 不识别？', blocks: [
            { t: 'ul', items: [
              '<code>irm is not recognized</code> → 你在 CMD 里，不是 PowerShell，请打开 PowerShell 再执行',
              { html: '<code>&amp;&amp; is not valid</code> → 你在 PowerShell 里用了 CMD 的命令。用 PowerShell 版本：', children: [
                { t: 'code', lang: 'powershell', code: 'irm https://claude.ai/install.ps1 | iex' }
              ]}
            ]}
          ]}
        ]},
        { t: 'h3', text: '连接与认证相关' },
        { t: 'faq', items: [
          { q: '报错 401 Unauthorized 或 Authentication failed？', blocks: [
            { t: 'p', html: 'API Key 认证失败。排查步骤：' },
            { t: 'ul', items: [
              '确认 API Key 复制完整，前后没有多余空格',
              { html: '确认环境变量设置正确：', children: [
                { t: 'code', lang: 'bash', code: 'echo $ANTHROPIC_API_KEY\necho $ANTHROPIC_BASE_URL' }
              ]},
              `确认 Base URL 是 <code>${base}</code>（注意 https，不要漏 s）`,
              '登录控制台检查账户余额是否充足',
              '检查 API Key 是否已过期或被禁用'
            ]}
          ]},
          { q: '报错 Connection timeout 或 ECONNREFUSED？', blocks: [
            { t: 'p', html: '无法连接到中转服务。排查步骤：' },
            { t: 'ul', items: [
              `检查网络是否正常：<code>curl -sI ${base}</code>`,
              '如果在公司网络，可能需要设置代理：<code>export HTTPS_PROXY=http://你的代理地址:端口</code>',
              '检查防火墙是否拦截了 HTTPS 请求'
            ]}
          ]},
          { q: '报错 429 Too Many Requests 或 Rate limit exceeded？', blocks: [
            { t: 'ul', items: [
              '等待几秒后重试，Claude Code 会自动重试',
              '减少并发使用的 Claude Code 实例数量',
              '如果频繁遇到，联系客服提升速率限制'
            ]}
          ]},
          { q: '报错 overloaded_error 或 503 Service Unavailable？', blocks: [
            { t: 'ul', items: [
              '等待 1–2 分钟后重试',
              '这是 Anthropic 服务端的临时问题，通常很快恢复',
              '如果持续出现，在 Claude Code 中输入 <code>/model</code> 切换其他模型'
            ]}
          ]}
        ]},
        { t: 'h3', text: 'Claude Code 使用相关' },
        { t: 'faq', items: [
          { q: '如何查看当前会话花了多少钱？', blocks: [
            { t: 'p', html: '在 Claude Code 中输入 <code>/cost</code>，会显示当前会话的 Token 用量和费用。' }
          ]},
          { q: '如何切换模型？', blocks: [
            { t: 'p', html: '输入 <code>/model</code> 后选择想用的模型。推荐日常用 Sonnet（便宜且够用），复杂任务用 Opus。' }
          ]},
          { q: '对话太长导致响应变慢或报错？', blocks: [
            { t: 'p', html: '上下文窗口快满了。' },
            { t: 'ul', items: [
              '输入 <code>/compact</code> 压缩对话历史',
              '输入 <code>/clear</code> 清空对话，开始新会话',
              '切换任务前养成 <code>/clear</code> 的习惯'
            ]}
          ]},
          { q: 'Claude Code 可以做什么？', blocks: [
            { t: 'ul', items: [
              '阅读和理解整个代码库',
              '编写、修改、删除代码文件',
              '执行终端命令（编译、测试、Git 操作等）',
              '创建 Git commit、分支、Pull Request',
              '修复 Bug、写测试、重构代码',
              '解释代码逻辑、回答技术问题'
            ]}
          ]},
          { q: 'Claude Code 修改了我的文件，怎么撤销？', blocks: [
            { t: 'ul', items: [
              '按 <kbd>Ctrl+Z</kbd>（在 Claude Code 交互界面中）撤销上一步操作',
              { html: '如果已退出，用 Git 恢复：', children: [
                { t: 'code', lang: 'bash', code: 'git diff           # 查看改动\ngit checkout .     # 撤销所有未提交的改动' }
              ]}
            ]},
            { t: 'callout', variant: 'tip', html: '💡 建议在让 Claude Code 做大改动前先 <code>git commit</code>，这样随时可以回退。' }
          ]},
          { q: '如何让 Claude Code 不要自动执行命令？', blocks: [
            { t: 'p', html: '在 <code>~/.claude/settings.json</code> 中设置：' },
            { t: 'code', lang: 'json', code: '{\n  "permissions": {\n    "allow": [],\n    "deny": ["Bash(*)", "Computer(*)"]\n  }\n}' },
            { t: 'p', html: '这样每次执行命令都会先征求你的同意。' }
          ]}
        ]},
        { t: 'h3', text: 'VS Code 插件相关' },
        { t: 'faq', items: [
          { q: 'VS Code 插件安装后找不到？', blocks: [
            { t: 'ul', items: [
              '按 <kbd>Cmd+Shift+P</kbd> / <kbd>Ctrl+Shift+P</kbd>',
              '输入 <code>Claude Code</code>',
              '选择 <code>Claude Code: Open in New Tab</code>'
            ]},
            { t: 'p', html: '如果搜索不到，确认插件已安装：按 <kbd>Cmd+Shift+X</kbd> 打开扩展面板，搜索 "Claude Code" 确认状态为「已安装」。' }
          ]},
          { q: 'VS Code 插件提示连接失败？', blocks: [
            { t: 'ul', items: [
              '先确认终端版 Claude Code 能正常使用（排除 API Key 和网络问题）',
              '检查 VS Code 的 <code>settings.json</code> 中环境变量是否正确',
              '重启 VS Code',
              '如果仍然不行，卸载插件重新安装'
            ]}
          ]}
        ]},
        { t: 'h3', text: 'OpenClaw 相关' },
        { t: 'faq', items: [
          { q: '<code>openclaw</code> 命令找不到？', blocks: [
            { t: 'code', lang: 'bash', code: '# 检查 npm 全局安装路径\nnpm prefix -g\n\n# 确保该路径的 bin 目录在 PATH 中\nexport PATH="$(npm prefix -g)/bin:$PATH"' },
            { t: 'p', html: '添加到 <code>~/.zshrc</code> 或 <code>~/.bashrc</code> 中使其永久生效。' }
          ]},
          { q: 'OpenClaw 启动失败？', blocks: [
            { t: 'code', lang: 'bash', code: '# 运行诊断\nopenclaw doctor\n\n# 查看详细日志\nopenclaw gateway logs\n\n# 重启 Gateway\nopenclaw gateway restart' }
          ]},
          { q: 'OpenClaw 如何配置使用我们的中转服务？', blocks: [
            { t: 'p', html: '在配置向导中选择 Anthropic 提供商，输入 API Key 和 Base URL，或直接编辑配置：' },
            { t: 'code', lang: 'bash', code: 'openclaw configure' },
            { t: 'p', html: `选择 Anthropic → 输入 API Key → 输入 Base URL <code>${base}</code>` }
          ]}
        ]},
        { t: 'h3', text: '更新与维护' },
        { t: 'faq', items: [
          { q: '如何更新 Claude Code？', blocks: [
            { t: 'code', lang: 'bash', code: '# 原生安装会自动更新，也可以手动触发\nclaude update' },
            { t: 'p', html: '如果是 Homebrew 安装：' },
            { t: 'code', lang: 'bash', code: 'brew upgrade claude-code' }
          ]},
          { q: '如何更新 OpenClaw？', blocks: [
            { t: 'code', lang: 'bash', code: 'openclaw update' }
          ]},
          { q: '如何完全卸载 Claude Code？', blocks: [
            { t: 'code', lang: 'bash', code: '# 原生安装\nclaude uninstall\n\n# npm 安装\nnpm uninstall -g @anthropic-ai/claude-code\n\n# Homebrew 安装\nbrew uninstall --cask claude-code' }
          ]},
          { q: '如何完全卸载 OpenClaw？', blocks: [
            { t: 'code', lang: 'bash', code: 'npm uninstall -g openclaw\nrm -rf ~/.openclaw' }
          ]}
        ]}
      ]
    },
    {
      id: 'reference',
      title: '快速参考',
      blocks: [
        { t: 'table', head: ['项目', '值'], rows: [
          ['中转服务地址', `<code>${base}</code>`],
          ['环境变量 - Base URL', `<code>ANTHROPIC_BASE_URL=${base}</code>`],
          ['环境变量 - API Key', '<code>ANTHROPIC_API_KEY=sk-你的Key</code>'],
          ['推荐模型', '<code>claude-sonnet-4-20250514</code>'],
          ['OpenClaw 文档', '<a href="https://docs.openclaw.ai" target="_blank" rel="noopener noreferrer">docs.openclaw.ai</a>'],
          ['Claude Code 文档', '<a href="https://code.claude.com/docs" target="_blank" rel="noopener noreferrer">code.claude.com/docs</a>']
        ]}
      ]
    }
  ]
})

// ---------------------------------------------------------------------------
// English content
// ---------------------------------------------------------------------------

export const en: HelpFactory = (base) => ({
  chrome: {
    title: 'Help',
    tagline: 'Beginner-friendly setup guide',
    toc: 'Contents',
    backHome: 'Back to Home',
    backDashboard: 'Back to Dashboard',
    backToTop: 'Back to Top',
    intro: 'A step-by-step guide for installing and configuring Claude Code (terminal + VS Code extension), OpenClaw, and Opencode against our API gateway. Both OpenAI and Anthropic protocols are supported.',
    copy: 'Copy',
    copied: 'Copied'
  },
  sections: [
    {
      id: 'quick-start',
      title: 'Quick Start',
      blocks: [
        { t: 'steps', items: [
          `Open <a href="${base}" target="_blank" rel="noopener noreferrer">${base}</a> (use a Hong Kong / overseas proxy if you are in mainland China)`,
          'Register an account',
          `Open the <a href="${base}/purchase" target="_blank" rel="noopener noreferrer">Recharge / Subscription</a> page to top up balance or buy a subscription plan`,
          'Create an API key → pick the subscription group matching your plan (daily / weekly / monthly)',
          'Pick your client tool and follow the instructions below'
        ]}
      ]
    },
    {
      id: 'prepare',
      title: '1. Prerequisites',
      blocks: [
        { t: 'p', html: 'You need a computer (macOS, Windows, or Linux) and a stable internet connection.' },
        { t: 'h3', text: '1.1 Install Node.js' },
        { t: 'p', html: 'Both Claude Code and OpenClaw require Node.js 22 or newer.' },
        { t: 'h4', text: 'macOS / Linux' },
        { t: 'p', html: 'Open Terminal and paste:' },
        { t: 'code', lang: 'bash', code: 'curl -fsSL https://fnm.vercel.app/install | bash' },
        { t: 'p', html: 'Close and reopen the terminal, then run:' },
        { t: 'code', lang: 'bash', code: 'fnm install 22\nfnm use 22\nnode -v' },
        { t: 'p', html: 'Output like <code>v22.x.x</code> means it works.' },
        { t: 'h4', text: 'Windows' },
        { t: 'ul', items: [
          'Visit the Node.js website and download the LTS release (22.x)',
          'Double-click the installer and accept the defaults',
          'Open PowerShell and run <code>node -v</code> to confirm'
        ]},
        { t: 'callout', variant: 'info', html: '<strong>Tip:</strong> Windows users are strongly encouraged to install WSL2 and run subsequent commands inside Ubuntu for a better experience.' },
        { t: 'code', lang: 'powershell', code: 'wsl --install' },
        { t: 'p', html: 'Reboot after install.' }
      ]
    },
    {
      id: 'install-cc',
      title: '2. Install Claude Code (terminal)',
      blocks: [
        { t: 'p', html: 'Claude Code is Anthropic\'s official AI coding assistant that runs in your terminal.' },
        { t: 'callout', variant: 'warning', html: 'Anthropic blocks China and Hong Kong IPs, so <strong>you need a VPN to a region like the US or Japan during install</strong>. Once installed, you can drop the VPN and connect directly to our gateway.' },
        { t: 'h3', text: 'macOS / Linux / WSL2' },
        { t: 'code', lang: 'bash', code: 'curl -fsSL https://claude.ai/install.sh | bash' },
        { t: 'h3', text: 'Windows PowerShell' },
        { t: 'code', lang: 'powershell', code: 'irm https://claude.ai/install.ps1 | iex' },
        { t: 'p', html: '<small>⚠️ Windows users must install Git for Windows first.</small>' },
        { t: 'p', html: 'Verify the install:' },
        { t: 'code', lang: 'bash', code: 'claude --version' },
        { t: 'p', html: 'A version string means success. On Windows, add the install directory to PATH if you get an error.' }
      ]
    },
    {
      id: 'config-cc',
      title: '3. Point Claude Code at our gateway',
      blocks: [
        { t: 'p', html: 'This is the key step — connect Claude Code to our gateway instead of Anthropic\'s official API.' },
        { t: 'h3', text: 'Option 1: Environment variables (recommended)' },
        { t: 'h4', text: 'macOS / Linux / WSL2' },
        { t: 'p', html: 'Edit your shell config:' },
        { t: 'code', lang: 'bash', code: '# zsh (macOS default)\nnano ~/.zshrc\n\n# bash\nnano ~/.bashrc' },
        { t: 'p', html: 'Append:' },
        { t: 'code', lang: 'bash', code: `export ANTHROPIC_BASE_URL="${base}"\nexport ANTHROPIC_AUTH_TOKEN="sk-your-api-key"` },
        { t: 'p', html: 'Reload:' },
        { t: 'code', lang: 'bash', code: 'source ~/.zshrc  # or source ~/.bashrc' },
        { t: 'h4', text: 'Windows PowerShell' },
        { t: 'code', lang: 'powershell', code: `$env:ANTHROPIC_BASE_URL = "${base}"\n$env:ANTHROPIC_AUTH_TOKEN = "sk-your-api-key"` },
        { t: 'p', html: 'Reopen PowerShell for it to take effect.' },
        { t: 'h4', text: 'Windows (via setx)' },
        { t: 'code', lang: 'powershell', code: `setx ANTHROPIC_BASE_URL "${base}"\nsetx ANTHROPIC_AUTH_TOKEN "your-key"` },
        { t: 'h3', text: 'Option 2: Claude Code settings file' },
        { t: 'p', html: 'Run <code>claude</code> and type <code>/config</code>, or edit <code>~/.claude/settings.json</code> directly:' },
        { t: 'code', lang: 'json', code: `{\n  "env": {\n    "ANTHROPIC_BASE_URL": "${base}",\n    "ANTHROPIC_AUTH_TOKEN": "sk-your-api-key"\n  }\n}` },
        { t: 'p', html: 'Verify:' },
        { t: 'code', lang: 'bash', code: 'cd any-project-dir\nclaude\n/model opus' },
        { t: 'h3', text: 'Option 3: CC Switch GUI (great for beginners)' },
        { t: 'p', html: 'CC Switch is a cross-platform desktop app for managing API configs across Claude Code, Codex, and Gemini CLI — one-click switching, speed tests, MCP management, no manual config files.' },
        { t: 'p', html: 'GitHub: <a href="https://github.com/farion1231/cc-switch" target="_blank" rel="noopener noreferrer">https://github.com/farion1231/cc-switch</a>' },
        { t: 'h4', text: 'Step 1: Download and install' },
        { t: 'ul', items: [
          '<strong>Windows:</strong> download <code>CC-Switch-v{version}-Windows.msi</code> (or the portable <code>.zip</code>)',
          { html: '<strong>macOS:</strong> Homebrew is recommended', children: [
            { t: 'code', lang: 'bash', code: 'brew tap farion1231/ccswitch\nbrew install --cask cc-switch' },
            { t: 'p', html: 'Or grab <code>CC-Switch-v{version}-macOS.zip</code> and install manually.' }
          ]},
          '<strong>Linux:</strong> download <code>.deb</code> (Ubuntu/Debian), <code>.rpm</code> (Fedora), or <code>.AppImage</code> (generic)'
        ]},
        { t: 'callout', variant: 'warning', html: 'On macOS the first launch may show "Cannot verify developer". Go to System Settings → Privacy & Security → "Open Anyway".' },
        { t: 'h4', text: 'Step 2: Add the gateway provider' },
        { t: 'ul', items: [
          'Open CC Switch',
          'Click "Add Provider"',
          'Pick "Custom"',
          { html: 'Fill in:', children: [{ t: 'ul', items: [
            'Name: anything you like, e.g. <code>gateway</code>',
            `Base URL: <code>${base}</code>`,
            'API Key: paste the key from the dashboard'
          ]}]},
          'Save'
        ]},
        { t: 'h4', text: 'Step 3: Enable it' },
        { t: 'ul', items: [
          'Select the provider you just created',
          'Click "Enable"',
          'Restart your terminal or Claude Code'
        ]},
        { t: 'p', html: 'You can also right-click the tray icon to switch quickly.' },
        { t: 'p', html: 'Verify:' },
        { t: 'code', lang: 'bash', code: 'cd any-project-dir\nclaude\n/model opus' },
        { t: 'p', html: 'Inside Claude Code, ask anything (e.g. "hello, which model are you?"). A reply means the config is working.' }
      ]
    },
    {
      id: 'vscode',
      title: '4. VS Code + Claude Code extension',
      blocks: [
        { t: 'h3', text: '4.1 Install VS Code' },
        { t: 'ul', items: [
          'Visit the VS Code website',
          'Download the installer for your OS',
          'Install and launch'
        ]},
        { t: 'h3', text: '4.2 Install the Claude Code extension' },
        { t: 'ul', items: [
          'Open VS Code',
          'Press <kbd>Cmd+Shift+X</kbd> (macOS) or <kbd>Ctrl+Shift+X</kbd> (Windows/Linux)',
          'Search for <code>Claude Code</code>',
          'Pick the official Anthropic extension and click "Install"'
        ]}
      ]
    },
    {
      id: 'config-vscode',
      title: '5. Configure the VS Code extension',
      blocks: [
        { t: 'h3', text: '5.1 Open settings' },
        { t: 'p', html: 'Press <kbd>Cmd+,</kbd> (macOS) or <kbd>Ctrl+,</kbd> (Windows/Linux).' },
        { t: 'h3', text: '5.2 Environment variables' },
        { t: 'p', html: 'Add to <code>settings.json</code> (<kbd>Cmd+Shift+P</kbd> → <em>Open User Settings (JSON)</em>):' },
        { t: 'code', lang: 'json', code: `{\n  "claude-code.env": {\n    "ANTHROPIC_BASE_URL": "${base}",\n    "ANTHROPIC_API_KEY": "sk-your-api-key"\n  },\n  "claudeCode.environmentVariables": [\n    { "name": "ANTHROPIC_BASE_URL", "value": "${base}" },\n    { "name": "ANTHROPIC_AUTH_TOKEN", "value": "sk-your-api-key" }\n  ]\n}` },
        { t: 'callout', variant: 'tip', html: '💡 If you already configured environment variables or <code>~/.claude/settings.json</code> in step 3, the VS Code extension inherits them — you can skip this.' },
        { t: 'h3', text: '5.3 Using the extension' },
        { t: 'ul', items: [
          'Press <kbd>Cmd+Shift+P</kbd> (macOS) or <kbd>Ctrl+Shift+P</kbd> (Windows/Linux)',
          'Type <code>Claude Code</code>',
          'Pick <code>Claude Code: Open in New Tab</code>',
          'Start chatting'
        ]},
        { t: 'p', html: 'If you are still prompted to sign in, see <a href="https://www.cnblogs.com/wuhaoliu/p/19537431" target="_blank" rel="noopener noreferrer">this guide</a> and swap the URL and key for the gateway\'s.' }
      ]
    },
    {
      id: 'openclaw',
      title: '6. Install and configure OpenClaw',
      blocks: [
        { t: 'p', html: 'OpenClaw is an open-source AI assistant framework that brings Claude into Telegram, WeChat, Discord, and other platforms.' },
        { t: 'h3', text: '6.1 Install OpenClaw' },
        { t: 'h4', text: 'macOS / Linux / WSL2' },
        { t: 'code', lang: 'bash', code: 'curl -fsSL https://openclaw.ai/install.sh | bash' },
        { t: 'h4', text: 'Windows PowerShell' },
        { t: 'code', lang: 'powershell', code: 'iwr -useb https://openclaw.ai/install.ps1 | iex' },
        { t: 'p', html: 'The installer auto-detects Node.js and launches the onboarding wizard.' },
        { t: 'h3', text: '6.2 Onboarding wizard' },
        { t: 'ul', items: [
          'Provider → <strong>Anthropic</strong>',
          'API Key → paste the key from the dashboard',
          `Base URL → <code>${base}/v1</code> or <code>${base}</code>`,
          'Model → recommend <code>claude-sonnet-4-20250514</code> (best price/quality)',
          'Gateway password → pick a password protecting the OpenClaw dashboard'
        ]},
        { t: 'p', html: 'Re-run the wizard anytime:' },
        { t: 'code', lang: 'bash', code: 'openclaw configure' },
        { t: 'h3', text: '6.3 Manual config (recommended)' },
        { t: 'p', html: 'Edit <code>~/.openclaw/openclaw.json</code>. Recommended: <code>"maxTokens": 65536</code>.' },
        { t: 'code', lang: 'json', code: `{\n  "agents": {\n    "defaults": {\n      "model": "ccvibe/claude-opus-4-6",\n      "models": {\n        "ccvibe/claude-opus-4-6": {}\n      }\n    }\n  },\n  "providers": {\n    "ccvibe": {\n      "baseUrl": "${base}/v1",\n      "authHeader": true,\n      "auth": "api-key",\n      "apiKey": "sk-replace-with-your-key",\n      "api": "anthropic-messages",\n      "models": [\n        {\n          "id": "claude-opus-4-6",\n          "name": "claude-opus-4-6",\n          "reasoning": false,\n          "contextWindow": 1000000,\n          "maxTokens": 65536\n        }\n      ]\n    }\n  }\n}` },
        { t: 'h3', text: '6.4 Restart OpenClaw' },
        { t: 'code', lang: 'bash', code: 'openclaw gateway restart' },
        { t: 'p', html: 'Then open the dashboard:' },
        { t: 'code', lang: 'bash', code: 'openclaw dashboard' },
        { t: 'h3', text: '6.5 Verify' },
        { t: 'code', lang: 'bash', code: 'openclaw status\nopenclaw doctor' },
        { t: 'p', html: 'If requests misbehave, check <code>~/.openclaw/agents/main/agent/models.json</code> for stale cached config. Green status means you are good.' },
        { t: 'h3', text: '6.6 Tencent Cloud OpenClaw' },
        { t: 'code', lang: 'json', code: `{\n  "provider": "anthropic",\n  "base_url": "${base}",\n  "api": "anthropic-messages",\n  "api_key": "your-api-key-here",\n  "model": {\n    "id": "claude-opus-4-6",\n    "name": "Claude Opus 4.6"\n  }\n}` },
        { t: 'p', html: 'Replace <code>api_key</code> and restart. Reference: <a href="https://cloud.tencent.com/developer/article/2625144" target="_blank" rel="noopener noreferrer">Anthropic Claude config</a>.' }
      ]
    },
    {
      id: 'opencode',
      title: '7. Open Code',
      blocks: [
        { t: 'code', lang: 'json', code: `{\n  "$schema": "https://opencode.ai/config.json",\n  "provider": {\n    "anthropic": {\n      "options": {\n        "baseURL": "${base}/v1",\n        "apiKey": "sk-your-api-key"\n      }\n    }\n  },\n  "model": "anthropic/claude-opus-4-6",\n  "small_model": "anthropic/claude-haiku-4-5"\n}` },
        { t: 'ul', items: [
          '<code>opencode.json</code> usually lives at <code>~/.config/opencode/opencode.json</code>. Newer versions may use <code>opencode.jsonc</code> — check first.',
          'Use the <strong>anthropic</strong> protocol, not openai, otherwise requests may fail.'
        ]},
        { t: 'p', html: 'Launch: <code>opencode</code>' },
        { t: 'p', html: 'Reference: <a href="https://opencode.ai/docs/zh-cn/providers/" target="_blank" rel="noopener noreferrer">opencode.ai/docs</a>' }
      ]
    },
    {
      id: 'hermes',
      title: '8. Hermes Agent',
      blocks: [
        { t: 'p', html: 'Hermes Agent is a <strong>self-evolving</strong> AI agent framework open-sourced by <a href="https://github.com/NousResearch" target="_blank" rel="noopener noreferrer">Nous Research</a> that <strong>distills skills from tasks and improves with use</strong>, letting the model plan autonomously and call tools across multi-step tasks.' },
        { t: 'p', html: 'Setting a "relay" simply means <strong>pointing Hermes at an OpenAI-compatible model endpoint</strong> (the base URL must end with <code>/v1</code>) — no network proxy involved. For this gateway: endpoint = <code>&lt;gateway&gt;/v1</code>, key = your <code>sk-...</code>, model = any model we support. Pick one of the three methods below.' },
        { t: 'h4', text: 'Method 1: hermes model interactive config (recommended)' },
        { t: 'steps', items: [
          'Run <code>hermes model</code> in a terminal',
          'In the provider list, choose <code>Custom endpoint (self-hosted / VLLM / etc.)</code>',
          `Enter Base URL <code>${base}/v1</code>, API Key (no echo — paste then press Enter), and model name (e.g. <code>gpt-5.5</code> / <code>claude-opus-4-8</code>)`,
          'Send a message to verify; switch configs anytime with <code>hermes model</code>'
        ]},
        { t: 'h4', text: 'Method 2: edit ~/.hermes/config.yaml (persistent)' },
        { t: 'code', lang: 'yaml', code: `# ~/.hermes/config.yaml\nmodel:\n  provider: custom\n  base_url: ${base}/v1\n  model: gpt-5.5\n  # leave api_key empty to fall back to OPENAI_API_KEY in .env` },
        { t: 'p', html: 'Put the key in <code>~/.hermes/.env</code> (not in config.yaml; Hermes auto-loads it and reads <code>OPENAI_API_KEY</code>, with file mode 0600):' },
        { t: 'code', lang: 'bash', code: `echo 'OPENAI_API_KEY=sk-your-api-key' >> ~/.hermes/.env` },
        { t: 'h4', text: 'Method 3: OPENAI_BASE_URL env var (legacy fallback, not recommended)' },
        { t: 'p', html: 'Recent Hermes resolves the main model from <code>model.base_url</code> in <code>config.yaml</code>; <code>OPENAI_BASE_URL</code> is only kept as a legacy fallback. Use it for quick tests on older setups — for real config prefer Method 1 or 2.' },
        { t: 'code', lang: 'bash', code: `# Persistent: append to ~/.hermes/.env\nOPENAI_API_KEY=sk-your-api-key\nOPENAI_BASE_URL=${base}/v1\n\n# Temporary (one session)\nexport OPENAI_BASE_URL="${base}/v1" && hermes` },
        { t: 'callout', variant: 'tip', html: '💡 <strong>base_url must end with <code>/v1</code></strong> (the OpenAI-compatible path; missing it usually returns 404). Keep the API key in <code>.env</code> only — never commit it. For Anthropic-family models, use the anthropic-protocol endpoint as your tool requires.' },
        { t: 'ul', items: [
          'Connection timeout → test with <code>curl -v &lt;gateway&gt;/v1/models</code>',
          'Garbled / malformed replies → confirm the upstream has OpenAI compatibility enabled',
          'API key errors → check the <code>~/.hermes/.env</code> path and stray spaces around the key',
          '404 on requests → base_url is missing the <code>/v1</code> suffix'
        ]},
        { t: 'p', html: 'Official docs: <a href="https://hermes-agent.nousresearch.com/docs/user-guide/configuration" target="_blank" rel="noopener noreferrer">Configuration</a> · <a href="https://hermes-agent.nousresearch.com/docs/integrations/providers" target="_blank" rel="noopener noreferrer">AI Providers</a> · <a href="https://github.com/NousResearch/hermes-agent" target="_blank" rel="noopener noreferrer">GitHub</a>' }
      ]
    },
    {
      id: 'cursor',
      title: '9. Cursor',
      blocks: [
        { t: 'p', html: 'Reference: <a href="https://gitcode.csdn.net/69b92f9f0a2f6a37c5981c5e.html" target="_blank" rel="noopener noreferrer">gitcode.csdn.net</a>' }
      ]
    },
    {
      id: 'pricing',
      title: '10. Pricing',
      blocks: [
        { t: 'h3', text: '10.1 Pricing model' },
        { t: 'p', html: 'Our gateway uses <strong>1:1 pricing matching Anthropic\'s official rates</strong> — pay per token, no markup.' },
        { t: 'h3', text: '10.2 Claude model prices' },
        { t: 'table', head: ['Model', 'Base Input Tokens', '5m Cache Writes', '1h Cache Writes', 'Cache Hits & Refreshes', 'Output Tokens'], rows: [
          ['Claude Fable 5', '$10 / MTok', '$12.50 / MTok', '$20 / MTok', '$1 / MTok', '$50 / MTok'],
          ['Claude Opus 4.8', '$5 / MTok', '$6.25 / MTok', '$10 / MTok', '$0.50 / MTok', '$25 / MTok'],
          ['Claude Opus 4.7', '$5 / MTok', '$6.25 / MTok', '$10 / MTok', '$0.50 / MTok', '$25 / MTok'],
          ['Claude Opus 4.6', '$5 / MTok', '$6.25 / MTok', '$10 / MTok', '$0.50 / MTok', '$25 / MTok'],
          ['Claude Opus 4.5', '$5 / MTok', '$6.25 / MTok', '$10 / MTok', '$0.50 / MTok', '$25 / MTok'],
          ['Claude Opus 4.1', '$15 / MTok', '$18.75 / MTok', '$30 / MTok', '$1.50 / MTok', '$75 / MTok'],
          ['Claude Opus 4 (deprecated)', '$15 / MTok', '$18.75 / MTok', '$30 / MTok', '$1.50 / MTok', '$75 / MTok'],
          ['Claude Sonnet 4.6', '$3 / MTok', '$3.75 / MTok', '$6 / MTok', '$0.30 / MTok', '$15 / MTok'],
          ['Claude Sonnet 4.5', '$3 / MTok', '$3.75 / MTok', '$6 / MTok', '$0.30 / MTok', '$15 / MTok'],
          ['Claude Sonnet 4 (deprecated)', '$3 / MTok', '$3.75 / MTok', '$6 / MTok', '$0.30 / MTok', '$15 / MTok'],
          ['Claude Haiku 4.5', '$1 / MTok', '$1.25 / MTok', '$2 / MTok', '$0.10 / MTok', '$5 / MTok'],
          ['Claude Haiku 3.5 (retired, except on Bedrock and Vertex AI)', '$0.80 / MTok', '$1 / MTok', '$1.60 / MTok', '$0.08 / MTok', '$4 / MTok']
        ]},
        { t: 'p', html: 'Tool integrations need the full model name — copy from the table below (case-sensitive):' },
        { t: 'table', rows: [
          ['<code>claude-fable-5</code>'],
          ['<code>claude-opus-4-8</code>'],
          ['<code>claude-opus-4-8-thinking</code>'],
          ['<code>claude-opus-4-7</code>'],
          ['<code>claude-opus-4-7-thinking</code>'],
          ['<code>claude-opus-4-6</code>'],
          ['<code>claude-opus-4-6-20260130</code>'],
          ['<code>claude-opus-4-6-thinking</code>'],
          ['<code>claude-opus-4-5-20251101</code>'],
          ['<code>claude-sonnet-4-6</code>'],
          ['<code>claude-sonnet-4-5</code>'],
          ['<code>claude-sonnet-4-5-20250929</code>'],
          ['<code>claude-sonnet-4-20250514</code>'],
          ['<code>claude-haiku-4-5-20251001</code>']
        ]},
        { t: 'callout', variant: 'warning', html: 'Avoid the cheap haiku model — you may hit upstream rate limits. Prefer sonnet or opus.' },
        { t: 'callout', variant: 'tip', html: '💡 <strong>Prompt Caching</strong> cuts cost dramatically for repeated context — cache hits are billed at 10% of the normal input price. Our gateway fully supports it.' },
        { t: 'h3', text: '10.3 Average Claude Code spend' },
        { t: 'p', html: 'Per Anthropic\'s data:' },
        { t: 'ul', items: [
          'Average developer: ~<strong>$6/day</strong>',
          '90th percentile users stay below <strong>$12/day</strong>',
          'Sonnet-only users average <strong>$100–200/month</strong>'
        ]},
        { t: 'h3', text: '10.4 Cost-saving tips' },
        { t: 'ul', items: [
          'Use Sonnet for day-to-day coding (best value)',
          'Switch to Opus only for hard architecture decisions',
          'Run <code>/clear</code> when switching tasks to drop unused context',
          'Use <code>/cost</code> to see the current session\'s spend',
          'Use <code>/compact</code> to compress conversation history'
        ]}
      ]
    },
    {
      id: 'codex',
      title: '11. Codex setup',
      blocks: [
        { t: 'p', html: 'Supported models:' },
        { t: 'code', lang: 'bash', code: 'gpt-5.4\ngpt-5.4-mini\ngpt-5.5\ngpt-image-2\ncodex-auto-review' },
        { t: 'callout', variant: 'warning', html: 'When creating an API key, Codex users must pick the <strong>Codex</strong> group.' },
        { t: 'h3', text: 'Codex CLI' },
        { t: 'code', lang: 'bash', code: 'vi ~/.codex/config.toml' },
        { t: 'code', lang: 'toml', code: `model_provider = "OpenAI"\nmodel = "gpt-5.5"\nreview_model = "gpt-5.5"\nmodel_reasoning_effort = "xhigh"\ndisable_response_storage = true\nnetwork_access = "enabled"\nmodel_context_window = 200000\nmodel_auto_compact_token_limit = 160000\n\n[model_providers.OpenAI]\nname = "OpenAI"\nbase_url = "${base}/v1"\nwire_api = "responses"\nrequires_openai_auth = true\nrequest_max_retries = 4\nstream_max_retries = 5` },
        { t: 'code', lang: 'bash', code: 'vi ~/.codex/auth.json' },
        { t: 'code', lang: 'json', code: '{\n  "OPENAI_API_KEY": "your-api-key"\n}' },
        { t: 'h3', text: 'Other tools (standard OpenAI protocol)' },
        { t: 'p', html: 'Request example (must enable streaming: <code>stream:true</code>):' },
        { t: 'code', lang: 'bash', code: `curl ${base}/v1/chat/completions \\\n  -H "Content-Type: application/json" \\\n  -H "Authorization: Bearer sk-xxxx" \\\n  -d '{\n    "model": "gpt-5.5",\n    "messages": [\n      { "role": "user", "content": "Who are you?" }\n    ],\n    "stream": true\n  }'` },
        { t: 'code', lang: 'bash', code: `curl ${base}/v1/messages \\\n  -H "Content-Type: application/json" \\\n  -H "Authorization: Bearer sk-xxxx" \\\n  -d '{\n    "model": "gpt-5.5",\n    "messages": [\n      { "role": "user", "content": "Who are you?" }\n    ],\n    "stream": true\n  }'` },
        { t: 'code', lang: 'bash', code: `curl --request POST \\\n  --url ${base}/v1/responses \\\n  --header 'Authorization: Bearer sk-xxxx' \\\n  --header 'Content-Type: application/json' \\\n  --data '{\n    "model": "gpt-5.5",\n    "input": [\n      {\n        "role": "user",\n        "content": [\n          {"type": "input_text", "text": "Hello, what can you do?"}\n        ]\n      }\n    ],\n    "stream": true,\n    "reasoning": {"effort": "high"}\n  }'` }
      ]
    },
    {
      id: 'image-gen',
      title: '12. Image generation (gpt-image-2)',
      blocks: [
        { t: 'h3', text: 'Caveats' },
        { t: 'callout', variant: 'warning', html: 'Do not use image-2 for unlawful content — it will trip GPT\'s safety filters. We also run our own moderation; some prompts will be rejected with a request to rephrase. We sweep upstream channels regularly for moderation issues.' },
        { t: 'p', html: 'GPT occasionally flags harmless prompts as a false positive — that is upstream behaviour, not our gateway.' },
        { t: 'h3', text: 'Timeout (important)' },
        { t: 'callout', variant: 'warning', html: 'Image generation is a <strong>synchronous, blocking</strong> request: high resolutions (2K/4K), image edits, and <code>n&gt;1</code> batches can each take several minutes. <strong>Set the client timeout to 600s</strong>, otherwise the client disconnects before the image is ready — it looks like a timeout while the upstream is still rendering.' },
        { t: 'ul', items: [
          'curl: add <code>--max-time 600</code>',
          'Python requests: <code>timeout=600</code> (examples below already use 600)',
          'OpenAI SDK: <code>OpenAI(base_url=..., api_key=..., timeout=600)</code>'
        ]},
        { t: 'h4', text: 'Codex config' },
        { t: 'p', html: 'When driving this gateway through Codex CLI, raise the SSE idle timeout from the default <code>300000</code>ms (5 min) to <code>600000</code>ms (10 min) in the <code>[model_providers.OpenAI]</code> block of <code>~/.codex/config.toml</code> (other fields are in "11. Codex setup"):' },
        { t: 'code', lang: 'toml', code: `[model_providers.OpenAI]\nname = "OpenAI"\nbase_url = "${base}/v1"\nwire_api = "responses"\nrequires_openai_auth = true\nstream_idle_timeout_ms = 600000   # long tasks / image gen: SSE idle timeout 600s (default 300000 = 5 min)` },
        { t: 'p', html: 'If an nginx reverse proxy sits in front of the gateway, its default 60s timeout cuts off long tasks / compact requests early; raise the timeouts to 600s in the matching <code>server</code> or <code>location</code> block:' },
        { t: 'code', lang: 'nginx', code: 'proxy_read_timeout 600s;\nproxy_send_timeout 600s;\nsend_timeout 600s;' },
        { t: 'h3', text: 'API examples' },
        { t: 'p', html: 'The image endpoint infers the output tier from <code>size</code>: <code>1024x1024</code> / <code>1k</code> = native, <code>2048x2048</code> / <code>2k</code> = 2K, <code>3840x2160</code> / <code>4k</code> = 4K. Pass <code>upscale</code> explicitly to override.' },
        { t: 'h4', text: 'Curl — text to image' },
        { t: 'code', lang: 'bash', code: `curl ${base}/images/generations \\\n  -H "Authorization: Bearer \${YOUR_API_KEY}" \\\n  -H "Content-Type: application/json" \\\n  -d '{\n    "model": "gpt-image-2",\n    "prompt": "A cute orange cat playing with yarn, studio ghibli style",\n    "n": 1,\n    "size": "1024x1024"\n  }'` },
        { t: 'p', html: 'Optional quality parameter: <code>quality</code> = <code>low</code> / <code>medium</code> / <code>high</code> / <code>auto</code>' },
        { t: 'h4', text: 'Curl — image to image' },
        { t: 'code', lang: 'bash', code: `curl ${base}/images/edits \\\n  -H "Authorization: Bearer \${YOUR_API_KEY}" \\\n  -F model="gpt-image-2" \\\n  -F prompt="Restyle this image as a watercolor painting, soft pastel palette" \\\n  -F n=1 \\\n  -F size="1024x1024" \\\n  -F image="@cat.png"` },
        { t: 'p', html: 'Image edits use OpenAI-compatible <code>multipart/form-data</code>. Up to 4 images per call, each ≤ 20 MB.' },
        { t: 'h4', text: 'Python (OpenAI SDK)' },
        { t: 'code', lang: 'python', code: `from openai import OpenAI\n\nclient = OpenAI(\n    base_url="${base}",\n    api_key="\${YOUR_API_KEY}",\n)\n\nresp = client.images.generate(\n    model="gpt-image-2",\n    prompt="A cute orange cat playing with yarn",\n    n=1,\n    size="1024x1024",\n)\nprint(resp.data[0].url)` },
        { t: 'h4', text: 'Python (requests — text to image)' },
        { t: 'code', lang: 'python', code: `import requests\n\nAPI_KEY = "\${YOUR_API_KEY}"\nBASE_URL = "${base}"\n\nresp = requests.post(\n    f"{BASE_URL}/images/generations",\n    headers={\n        "Authorization": f"Bearer {API_KEY}",\n        "Content-Type": "application/json",\n    },\n    json={\n        "model": "gpt-image-2",\n        "prompt": "A cute orange cat playing with yarn",\n        "n": 1,\n        "size": "1024x1024",\n    },\n    timeout=600,\n)\nresp.raise_for_status()\ndata = resp.json()\nprint(data["data"][0]["url"])` },
        { t: 'h4', text: 'Python (requests — image to image)' },
        { t: 'code', lang: 'python', code: `import requests\n\nAPI_KEY = "\${YOUR_API_KEY}"\nBASE_URL = "${base}"\n\nresp = requests.post(\n    f"{BASE_URL}/images/edits",\n    headers={\n        "Authorization": f"Bearer {API_KEY}",\n    },\n    data={\n        "model": "gpt-image-2",\n        "prompt": "Turn this image into a watercolor painting",\n        "n": "1",\n        "size": "1024x1024",\n    },\n    files={\n        "image": open("cat.png", "rb"),\n    },\n    timeout=600,\n)\nresp.raise_for_status()\nitem = resp.json()["data"][0]\nprint(item["url"])` },
        { t: 'p', html: 'Upload field is <code>image</code> or <code>image[]</code>. Response contains the image URL.' },
        { t: 'h3', text: 'Size reference' },
        { t: 'h4', text: '1. OpenAI official supported sizes' },
        { t: 'p', html: 'The most reliable trio:' },
        { t: 'table', head: ['Ratio', 'Size'], rows: [
          ['1:1 (square)', '<code>1024x1024</code>'],
          ['3:2 (landscape)', '<code>1536x1024</code>'],
          ['2:3 (portrait)', '<code>1024x1536</code>']
        ]},
        { t: 'h4', text: '2. gpt-image-2 additions' },
        { t: 'p', html: 'Compared with the older model, many channels now accept arbitrary <code>"size": "WxH"</code>, e.g.:' },
        { t: 'code', lang: 'json', code: '"size": "1920x1080"\n"size": "2048x2048"\n"size": "3840x2160"' },
        { t: 'h4', text: '3. Theoretical range' },
        { t: 'p', html: 'Width / height rules across most implementations:' },
        { t: 'ul', items: [
          'Must be integers',
          'Multiples of 64 recommended',
          'Some channels require multiples of 32'
        ]},
        { t: 'p', html: '✅ Valid: <code>1024x1024</code>, <code>1536x1024</code>, <code>1920x1080</code>, <code>2048x1152</code>, <code>3840x2160</code>' },
        { t: 'p', html: '❌ May fail: <code>1001x777</code>, <code>1919x1079</code>' },
        { t: 'h4', text: '4. Common ratios' },
        { t: 'table', head: ['Ratio', 'Sizes', 'Use case'], rows: [
          ['1:1', '<code>1024x1024</code> / <code>1536x1536</code> / <code>2048x2048</code>', 'Logos, product shots, avatars'],
          ['16:9 (landscape)', '<code>1280x720</code> / <code>1920x1080</code> / <code>2560x1440</code> / <code>3840x2160</code>', 'Slides, banners, video covers'],
          ['9:16 (portrait)', '<code>1080x1920</code> / <code>1440x2560</code> / <code>2160x3840</code>', 'Phone wallpapers, social shorts'],
          ['4:3', '<code>1600x1200</code> / <code>2048x1536</code>', 'Classic photo'],
          ['3:2', '<code>1536x1024</code> / <code>1920x1280</code>', 'Camera aspect ratio']
        ]}
      ]
    },
    {
      id: 'video-gen',
      title: '13. Video generation (Sora / Seedance 2.0)',
      blocks: [
        { t: 'p', html: 'Video generation is an <strong>async job API</strong>: create a job → poll status → download the video. Use an API key created under a <strong>video-enabled group</strong>. Supports Sora (<strong>per-second billing</strong>) and Seedance 2.0 (<strong>per-request billing</strong>).' },
        { t: 'h3', text: 'Supported models' },
        { t: 'table', head: ['Model', 'Resolution', 'Billing'], rows: [
          ['<code>sora-v3-fast</code>', '480p', 'per second'],
          ['<code>sora-v3-pro</code>', '720p', 'per second'],
          ['<code>seedance-2.0-fast-pass</code>', '720p', 'per request (flat; duration does not affect cost)'],
          ['<code>seedance-2.0-pass</code>', '720p', 'per request (flat; duration does not affect cost)']
        ]},
        { t: 'h3', text: 'Flow' },
        { t: 'ul', items: [
          '① <code>POST /v1/videos</code> — create a job, returns <code>id</code> (task_id)',
          '② <code>GET /v1/videos/{task_id}</code> — poll until <code>status=completed</code> (every ~5s)',
          '③ <code>GET /v1/videos/{task_id}/content</code> — download the video (mp4)'
        ]},
        { t: 'h4', text: '1) Create a job' },
        { t: 'code', lang: 'bash', code: `curl ${base}/videos \\\n  -H "Authorization: Bearer \${YOUR_API_KEY}" \\\n  -H "Content-Type: application/json" \\\n  -d '{\n    "model": "sora-v3-fast",\n    "prompt": "A neon-lit rainy street, slow dolly-in, cinematic",\n    "aspect_ratio": "16:9",\n    "resolution": "480p",\n    "seconds": "5"\n  }'` },
        { t: 'p', html: 'Response (job queued):' },
        { t: 'code', lang: 'json', code: '{\n  "id": "task_xxx",\n  "object": "video",\n  "model": "sora-v3-fast",\n  "status": "queued",\n  "progress": 0,\n  "created_at": 1779560000\n}' },
        { t: 'p', html: 'Seedance 2.0 (per-request, <code>duration</code> in 4/5/10/15):' },
        { t: 'code', lang: 'bash', code: `curl ${base}/videos \\\n  -H "Authorization: Bearer \${YOUR_API_KEY}" \\\n  -H "Content-Type: application/json" \\\n  -d '{\n    "model": "seedance-2.0-pass",\n    "prompt": "Seaside at dusk, slow crane-up, cinematic",\n    "ratio": "16:9",\n    "resolution": "720p",\n    "duration": 15\n  }'` },
        { t: 'h4', text: '2) Poll status' },
        { t: 'code', lang: 'bash', code: `curl ${base}/videos/task_xxx \\\n  -H "Authorization: Bearer \${YOUR_API_KEY}"` },
        { t: 'p', html: 'Completed (<code>status=completed</code>) response:' },
        { t: 'code', lang: 'json', code: '{\n  "id": "task_xxx",\n  "status": "completed",\n  "progress": 100,\n  "seconds": "5",\n  "size": "854x480",\n  "video_url": ".../v1/videos/task_xxx/content"\n}' },
        { t: 'p', html: 'Status values: <code>queued</code> / <code>in_progress</code> / <code>completed</code> / <code>failed</code>.' },
        { t: 'h4', text: '3) Download' },
        { t: 'code', lang: 'bash', code: `curl ${base}/videos/task_xxx/content \\\n  -H "Authorization: Bearer \${YOUR_API_KEY}" \\\n  -o out.mp4` },
        { t: 'h3', text: 'Parameters' },
        { t: 'table', head: ['Param', 'Required', 'Notes'], rows: [
          ['<code>model</code>', 'yes', 'sora-v3-fast / sora-v3-pro / seedance-2.0-fast-pass / seedance-2.0-pass'],
          ['<code>prompt</code>', 'yes', 'text prompt'],
          ['<code>resolution</code>', 'no', '480p / 720p / 1080p (Sora per-second: ≥1080 = HD tier; Seedance uses 720p)'],
          ['<code>aspect_ratio</code> / <code>ratio</code>', 'no', 'equivalent (16:9 / 9:16 / 1:1, etc.)'],
          ['<code>seconds</code> / <code>duration</code>', 'no', 'equivalent (Sora 5/10/15; Seedance 4/5/10/15). Per-request billing: duration does not affect cost'],
          ['<code>image_url</code>', 'no', 'Sora reference image (image-to-video)'],
          ['<code>first_image</code> / <code>last_image</code>', 'no', 'Seedance first/last frame — must be paired, not combined with reference images/videos'],
          ['<code>referenceImages</code>', 'no', 'Seedance reference images array (max 4)'],
          ['<code>referenceVideos</code>', 'no', 'Seedance reference videos array (max 3)']
        ]},
        { t: 'h4', text: 'Failure responses' },
        { t: 'code', lang: 'json', code: '// 403 insufficient balance\n{ "code": "INSUFFICIENT_BALANCE", "message": "Insufficient account balance" }\n\n// 403 video not enabled for the group\n{ "error": { "type": "permission_error", "message": "Video generation is not enabled for this group" } }\n\n// 503 no available accounts\n{ "error": { "type": "api_error", "message": "No available compatible accounts" } }' },
        { t: 'callout', variant: 'tip', html: '💡 Billing happens once on <strong>successful create</strong> (idempotent by request_id); failed creates (auth/balance/upstream errors) and polling/download are free. <strong>Per-second</strong> (Sora) = duration × per-second price (by resolution tier) × multiplier; <strong>per-request</strong> (Seedance 2.0) = flat price × multiplier, regardless of duration/resolution.' },
        { t: 'callout', variant: 'warning', html: 'Generation usually takes a few minutes. Prefer <code>GET /v1/videos/{task_id}/content</code> (auto-routes to the creating account); some relays do not allow API-key download there (return 401) — in that case download the <code>video_url</code> direct link from the completed response.' }
      ]
    },
    {
      id: 'faq',
      title: '14. FAQ',
      blocks: [
        { t: 'h3', text: 'Install issues' },
        { t: 'faq', items: [
          { q: '<code>command not found: claude</code> or \'claude\' is not recognized?', blocks: [
            { t: 'p', html: 'The most common issue — Claude Code\'s install dir is not on PATH.' },
            { t: 'p', html: '<strong>macOS / Linux:</strong>' },
            { t: 'code', lang: 'bash', code: "# Check whether the install dir is in PATH\necho $PATH | tr ':' '\\n' | grep local/bin\n\n# If empty, add it\n# zsh (macOS default)\necho 'export PATH=\"$HOME/.local/bin:$PATH\"' >> ~/.zshrc\nsource ~/.zshrc\n\n# bash (Linux default)\necho 'export PATH=\"$HOME/.local/bin:$PATH\"' >> ~/.bashrc\nsource ~/.bashrc" },
            { t: 'p', html: '<strong>Windows PowerShell:</strong>' },
            { t: 'code', lang: 'powershell', code: "# Check PATH\n$env:PATH -split ';' | Select-String 'local\\\\bin'\n\n# If empty, add it\n$currentPath = [Environment]::GetEnvironmentVariable('PATH', 'User')\n[Environment]::SetEnvironmentVariable('PATH', \"$currentPath;$env:USERPROFILE\\.local\\bin\", 'User')" },
            { t: 'p', html: 'Restart the terminal afterwards.' }
          ]},
          { q: 'Install script fails with <code>syntax error near unexpected token \'&lt;\'</code>?', blocks: [
            { t: 'p', html: 'The downloaded "script" is actually an HTML page — usually a network problem.' },
            { t: 'ul', items: [
              'Check connectivity to Google Cloud Storage: <code>curl -sI https://storage.googleapis.com</code>',
              { html: 'Try an alternative installer:', children: [
                { t: 'code', lang: 'bash', code: '# macOS\nbrew install --cask claude-code\n\n# Windows\nwinget install Anthropic.ClaudeCode' }
              ]},
              'If you see <em>App unavailable in region</em>, use a proxy.'
            ]}
          ]},
          { q: '<code>curl: (56) Failure writing output to destination</code> during install?', blocks: [
            { t: 'p', html: 'The download was cut off mid-way.' },
            { t: 'ul', items: [
              'Retry on a more stable network',
              { html: 'Or download the script first and execute it:', children: [
                { t: 'code', lang: 'bash', code: 'curl -fsSL https://claude.ai/install.sh -o install.sh\nbash install.sh' }
              ]}
            ]}
          ]},
          { q: 'TLS / SSL errors during install?', blocks: [
            { t: 'p', html: 'Common messages: <code>TLS connect error</code>, <code>SSL/TLS secure channel</code>, <code>unable to get local issuer certificate</code>.' },
            { t: 'code', lang: 'bash', code: '# Ubuntu/Debian\nsudo apt-get update && sudo apt-get install ca-certificates\n\n# macOS\nbrew install ca-certificates' },
            { t: 'p', html: 'Windows users: run this in PowerShell first:' },
            { t: 'code', lang: 'powershell', code: '[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12' },
            { t: 'p', html: 'Then re-run the installer.' }
          ]},
          { q: 'Linux install gets "Killed"?', blocks: [
            { t: 'p', html: 'Out of memory — the OOM killer ended the process. Claude Code needs ≥ 4 GB RAM. Add swap:' },
            { t: 'code', lang: 'bash', code: 'sudo fallocate -l 2G /swapfile\nsudo chmod 600 /swapfile\nsudo mkswap /swapfile\nsudo swapon /swapfile' },
            { t: 'p', html: 'Then retry the install.' }
          ]},
          { q: 'Windows: <code>Claude Code on Windows requires git-bash</code>?', blocks: [
            { t: 'ul', items: [
              'Install Git for Windows',
              'Check <em>Add to PATH</em> during install',
              'Restart your terminal'
            ]},
            { t: 'p', html: 'If installed but still erroring, set the path in <code>~/.claude/settings.json</code>:' },
            { t: 'code', lang: 'json', code: '{\n  "env": {\n    "CLAUDE_CODE_GIT_BASH_PATH": "C:\\\\Program Files\\\\Git\\\\bin\\\\bash.exe"\n  }\n}' }
          ]},
          { q: 'Windows: <code>irm</code> or <code>&amp;&amp;</code> not recognized?', blocks: [
            { t: 'ul', items: [
              '<code>irm is not recognized</code> → you are in CMD, not PowerShell. Open PowerShell and try again.',
              { html: '<code>&amp;&amp; is not valid</code> → CMD syntax inside PowerShell. Use the PowerShell variant:', children: [
                { t: 'code', lang: 'powershell', code: 'irm https://claude.ai/install.ps1 | iex' }
              ]}
            ]}
          ]}
        ]},
        { t: 'h3', text: 'Connection & auth' },
        { t: 'faq', items: [
          { q: '401 Unauthorized / Authentication failed?', blocks: [
            { t: 'p', html: 'API key authentication failed. Checklist:' },
            { t: 'ul', items: [
              'Confirm the API key is copied in full with no leading/trailing spaces',
              { html: 'Confirm the env vars are set:', children: [
                { t: 'code', lang: 'bash', code: 'echo $ANTHROPIC_API_KEY\necho $ANTHROPIC_BASE_URL' }
              ]},
              `Confirm Base URL is <code>${base}</code> (note <em>https</em>, not <em>http</em>)`,
              'Sign into the dashboard and check your balance',
              'Check whether the API key has expired or been disabled'
            ]}
          ]},
          { q: 'Connection timeout or ECONNREFUSED?', blocks: [
            { t: 'p', html: 'Can\'t reach the gateway. Checklist:' },
            { t: 'ul', items: [
              `Check connectivity: <code>curl -sI ${base}</code>`,
              'On a corporate network, set a proxy: <code>export HTTPS_PROXY=http://proxy-host:port</code>',
              'Check whether your firewall is blocking HTTPS'
            ]}
          ]},
          { q: '429 Too Many Requests / Rate limit exceeded?', blocks: [
            { t: 'ul', items: [
              'Wait a few seconds — Claude Code retries automatically',
              'Reduce the number of concurrent Claude Code instances',
              'Contact support to raise your rate limit if this is frequent'
            ]}
          ]},
          { q: 'overloaded_error or 503 Service Unavailable?', blocks: [
            { t: 'ul', items: [
              'Wait 1–2 minutes and retry',
              'This is a transient issue on Anthropic\'s side, usually short-lived',
              'If it persists, run <code>/model</code> inside Claude Code and switch'
            ]}
          ]}
        ]},
        { t: 'h3', text: 'Using Claude Code' },
        { t: 'faq', items: [
          { q: 'How do I see what the current session has cost?', blocks: [
            { t: 'p', html: 'Type <code>/cost</code> — it shows token usage and dollar cost.' }
          ]},
          { q: 'How do I switch models?', blocks: [
            { t: 'p', html: 'Type <code>/model</code> and pick one. Sonnet is recommended for everyday work; Opus for harder problems.' }
          ]},
          { q: 'Long conversation, slow or failing responses?', blocks: [
            { t: 'p', html: 'You are running out of context window.' },
            { t: 'ul', items: [
              'Type <code>/compact</code> to compress history',
              'Type <code>/clear</code> to start fresh',
              'Get in the habit of running <code>/clear</code> when switching tasks'
            ]}
          ]},
          { q: 'What can Claude Code do?', blocks: [
            { t: 'ul', items: [
              'Read and understand an entire codebase',
              'Write, edit, and delete code files',
              'Run shell commands (build, test, Git, etc.)',
              'Create Git commits, branches, and pull requests',
              'Fix bugs, write tests, refactor code',
              'Explain code and answer technical questions'
            ]}
          ]},
          { q: 'Claude Code edited my files — how do I undo?', blocks: [
            { t: 'ul', items: [
              'Press <kbd>Ctrl+Z</kbd> inside Claude Code\'s interactive UI to undo the last action',
              { html: 'If you have already exited, use Git:', children: [
                { t: 'code', lang: 'bash', code: 'git diff           # review changes\ngit checkout .     # discard uncommitted changes' }
              ]}
            ]},
            { t: 'callout', variant: 'tip', html: '💡 Run <code>git commit</code> before letting Claude Code make large changes so you can roll back easily.' }
          ]},
          { q: 'How do I stop Claude Code from auto-executing commands?', blocks: [
            { t: 'p', html: 'Add this to <code>~/.claude/settings.json</code>:' },
            { t: 'code', lang: 'json', code: '{\n  "permissions": {\n    "allow": [],\n    "deny": ["Bash(*)", "Computer(*)"]\n  }\n}' },
            { t: 'p', html: 'It will then ask before every command.' }
          ]}
        ]},
        { t: 'h3', text: 'VS Code extension' },
        { t: 'faq', items: [
          { q: 'I installed the VS Code extension but cannot find it?', blocks: [
            { t: 'ul', items: [
              'Press <kbd>Cmd+Shift+P</kbd> / <kbd>Ctrl+Shift+P</kbd>',
              'Type <code>Claude Code</code>',
              'Pick <code>Claude Code: Open in New Tab</code>'
            ]},
            { t: 'p', html: 'If you cannot find it, confirm it is installed: <kbd>Cmd+Shift+X</kbd> → search for "Claude Code".' }
          ]},
          { q: 'VS Code extension says connection failed?', blocks: [
            { t: 'ul', items: [
              'Confirm the terminal version works first (rules out API key / network)',
              'Check that env vars in VS Code <code>settings.json</code> are correct',
              'Restart VS Code',
              'Uninstall and reinstall the extension as a last resort'
            ]}
          ]}
        ]},
        { t: 'h3', text: 'OpenClaw' },
        { t: 'faq', items: [
          { q: '<code>openclaw</code> command not found?', blocks: [
            { t: 'code', lang: 'bash', code: '# Find npm global install path\nnpm prefix -g\n\n# Make sure its bin directory is in PATH\nexport PATH="$(npm prefix -g)/bin:$PATH"' },
            { t: 'p', html: 'Append to <code>~/.zshrc</code> or <code>~/.bashrc</code> for a permanent fix.' }
          ]},
          { q: 'OpenClaw fails to start?', blocks: [
            { t: 'code', lang: 'bash', code: '# Run diagnostics\nopenclaw doctor\n\n# View detailed logs\nopenclaw gateway logs\n\n# Restart the gateway\nopenclaw gateway restart' }
          ]},
          { q: 'How do I point OpenClaw at the gateway?', blocks: [
            { t: 'p', html: 'Pick Anthropic in the wizard, then enter your API key and Base URL — or edit the config directly:' },
            { t: 'code', lang: 'bash', code: 'openclaw configure' },
            { t: 'p', html: `Pick Anthropic → enter API key → set Base URL to <code>${base}</code>.` }
          ]}
        ]},
        { t: 'h3', text: 'Updates & maintenance' },
        { t: 'faq', items: [
          { q: 'How do I update Claude Code?', blocks: [
            { t: 'code', lang: 'bash', code: '# Native install updates itself; manual trigger:\nclaude update' },
            { t: 'p', html: 'For Homebrew installs:' },
            { t: 'code', lang: 'bash', code: 'brew upgrade claude-code' }
          ]},
          { q: 'How do I update OpenClaw?', blocks: [
            { t: 'code', lang: 'bash', code: 'openclaw update' }
          ]},
          { q: 'How do I completely uninstall Claude Code?', blocks: [
            { t: 'code', lang: 'bash', code: '# Native\nclaude uninstall\n\n# npm\nnpm uninstall -g @anthropic-ai/claude-code\n\n# Homebrew\nbrew uninstall --cask claude-code' }
          ]},
          { q: 'How do I completely uninstall OpenClaw?', blocks: [
            { t: 'code', lang: 'bash', code: 'npm uninstall -g openclaw\nrm -rf ~/.openclaw' }
          ]}
        ]}
      ]
    },
    {
      id: 'reference',
      title: 'Quick reference',
      blocks: [
        { t: 'table', head: ['Item', 'Value'], rows: [
          ['Gateway URL', `<code>${base}</code>`],
          ['Env var - Base URL', `<code>ANTHROPIC_BASE_URL=${base}</code>`],
          ['Env var - API Key', '<code>ANTHROPIC_API_KEY=sk-your-key</code>'],
          ['Recommended model', '<code>claude-sonnet-4-20250514</code>'],
          ['OpenClaw docs', '<a href="https://docs.openclaw.ai" target="_blank" rel="noopener noreferrer">docs.openclaw.ai</a>'],
          ['Claude Code docs', '<a href="https://code.claude.com/docs" target="_blank" rel="noopener noreferrer">code.claude.com/docs</a>']
        ]}
      ]
    }
  ]
})
