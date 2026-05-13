# DirMap

实时目录结构监控工具，利用 LLM 为每个文件夹生成简洁描述。专为 AI Agent 设计，帮助大模型快速理解项目结构。

## 工作原理

```
文件系统事件 → 防抖处理 → LLM 分析 → Markdown 输出
```

1. 实时监控指定目录的创建/删除/重命名事件
2. 将目录元数据发送给兼容 Claude 的 API
3. 为每个文件夹生成一句话描述
4. 将结果以 Markdown 表格形式写入配置的输出目录

## 快速开始

### 本地运行

```bash
# 设置 API Key
export ANTHROPIC_API_KEY=sk-xxx

# 编译并运行
make build
./bin/system-agent-rag -config config.yaml
```

### Docker 运行

```bash
# 构建镜像
make docker-build

# 启动服务（后台运行，自动重启）
make docker-up

# 查看日志
make docker-logs

# 停止服务
make docker-down
```

> **macOS 用户注意**：Docker Desktop 在 macOS 上无法可靠地传递文件系统事件（inotify）。请在 `config-docker.yaml` 中启用轮询模式：
> ```yaml
> polling:
>   enabled: true
>   interval: 10s
> ```

## 配置说明

复制 `config.yaml.example` 为 `config.yaml` 并编辑：

```yaml
watch_paths:
  - /path/to/watch          # 要监控的目录

output_dir: /path/to/output # 输出目录

llm:
  base_url: ""              # 自定义 API 端点（可选）
  api_key: ""               # 或使用 ANTHROPIC_API_KEY 环境变量
  model: "claude-haiku-4-5"
  max_tokens: 4096
  temperature: 0.3
  max_batch_size: 10        # 每次 LLM 调用的最大目录数

debounce:
  interval: 3s              # 事件防抖间隔
  max_wait: 30s             # 最大等待时间

initial_scan: true          # 启动时全量扫描
ignore_patterns:
  - ".git"
  - "node_modules"
```

## 输出示例

```markdown
# Directory Descriptions: /project
Generated: 2026-05-13 12:00:00

| Path | Modified | Description |
|------|----------|-------------|
| ./ | 2026-05-12 | Go 项目根目录 |
| src/ | 2026-05-10 | 主应用源代码 |
| internal/ | 2026-05-11 | 不对外暴露的私有包 |
```

## 架构

```
main.go → config.Load() → agent.New() → agent.Run(ctx)

数据流：
  fsnotify 事件 → watcher → debouncer（3s/30s 防抖）
    → scanner 读取目录元数据
    → summarizer 调用 LLM API（批量、流式、带重试）
    → writer 原子写入 .md 文件

内存缓存：map[watchPath]map[dirPath]FileInfo
  - 增量更新：只重新总结变化的目录
  - LLM 失败时保留旧描述
```

## License

MIT
