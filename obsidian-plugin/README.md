# Quinfi Obsidian Plugin

Quinfi 的 Obsidian 插件子项目，用于在 Obsidian Vault 中集成基金投研工作流。

## Stack

- TypeScript
- esbuild
- Obsidian API

## Development

```bash
cd obsidian-plugin
npm ci
npm run dev
```

## Build

```bash
npm run build
```

构建产物：`obsidian-plugin/main.js`

## Key Files

- `src/main.ts`: 插件入口
- `src/view.ts`: 视图逻辑
- `src/api.ts`: 后端接口调用
- `manifest.json`: Obsidian 插件清单
