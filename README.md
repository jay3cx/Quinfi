# Quinfi

Quinfi 是一个面向个人投资者的 AI 基金投研助手项目。

## 项目结构

| 路径 | 说明 |
| --- | --- |
| `cmd/` | Go 服务入口 |
| `internal/` | 后端业务模块 |
| `pkg/` | 通用组件 |
| `web/` | React Web 前端 |
| `obsidian-plugin/` | Obsidian 插件（TypeScript） |

## 快速开始

### 1) 启动后端

```bash
cp config.example.yaml config.yaml
make deps
make run
```

后端地址：`http://localhost:8080`

### 2) 启动前端

```bash
cd web
npm ci
npm run dev
```

前端地址：`http://localhost:5173`

### 3) 构建 Obsidian 插件

```bash
cd obsidian-plugin
npm ci
npm run build
```

## 文档

- `web/README.md`
- `obsidian-plugin/README.md`

## 说明

`openspec/`、Obsidian 笔记（`notes/`、`.obsidian/`）以及非 README Markdown 文档为本地内部内容，不上传 GitHub。
