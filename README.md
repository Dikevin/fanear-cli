# fanear

读[饭耳（FanEar）](https://fanear.fandao.dev)写进保存位置的记录：每天的总结、逐句转写、待办、健康和足迹，以及小耳的记忆和收藏。给人在终端里查，也给 Claude Code、Codex、Antigravity 这类 agent 当工具用。

A read-only CLI (and agent skill) for the daily records the FanEar Apple Watch recorder writes to its save location.

> 非官方工具，和饭耳开发者无关。只读本机文件，不联网，不改、不删、不挪任何文件。

## 安装

需要 Go 1.23 以上：

```bash
GOPROXY=direct go install github.com/Dikevin/fanear-cli/cmd/fanear@latest
```

`GOPROXY=direct` 直接从 GitHub 取最新的 tag；走公司或镜像代理时，`@latest` 可能拿到代理缓存的旧版本。

给 agent 装 skill（写到 `~/.agents/skills/fanear`，Codex 和 Antigravity 读这里；`--claude` 再给 Claude Code 建个链接）：

```bash
fanear skill install --claude
```

装好的 skill 旁边有个 `.installed.json`，记着是哪个版本写的。以后升级 fanear，第一次运行任何命令时会把旧的 skill 换成新版，并在 stderr 说一声；`--dir` 装到别处的也会记下来一起更新。从源码直接构建的开发版不会动已装的 skill。

装完或者出问题时跑一下 `fanear doctor`：它检查数据目录、最近的记录、记录格式、skill 版本和 Claude Code 的链接，有问题时退出码是 1，并给出修法。

## 数据在哪

饭耳可以同时存到好几个地方（饭耳 › 设置 › 存储）。fanear 读本机上的目录，按内容认饭耳的记录（`记录/年/月/日/`），所以指到保存位置上面任意一层都行：

| 饭耳的保存位置 | fanear 怎么读 |
|---|---|
| iCloud（默认） | 自动找到 |
| 「文件」App 里选的文件夹 | 在 iCloud 云盘里的自动找到；其他的用 `--root` 指过去 |
| Google Drive | 装 Google Drive 桌面版后自动找到（`~/Library/CloudStorage`） |
| NAS（WebDAV、群晖、飞牛） | 在 Finder「连接服务器」挂上后自动找到（`/Volumes`），或用 rclone 挂载后 `--root` |
| 对象存储（OSS、COS、TOS、S3、R2） | 用 rclone 等工具挂载或同步到本机后 `--root` |
| Webhook | 不支持：饭耳是往外推，没有可读的文件 |

找数据的顺序：`--root` 参数 → 环境变量 `FANEAR_ROOT` → `fanear where --set <目录>` 记住的目录 → 自动查找（只在 macOS 上）。`fanear where` 显示最后用的是哪个。

`memory` 和 `favorites` 读的是饭耳“换手机找回资料”的备份，只有 iCloud 和「文件」App 文件夹这两种保存位置有。

## 命令

| 命令 | 做什么 |
|---|---|
| `fanear where` | 数据目录在哪、怎么找到的、有哪些天 |
| `fanear days` | 有记录的日期和各自的文件 |
| `fanear brief [日期]` | 某天的总结（饭耳总结.md 原文） |
| `fanear transcript [日期] --from 14:00 --to 15:30` | 某段时间的逐句转写 |
| `fanear search 词1 词2 …` | 跨天搜原句，最近的在前 |
| `fanear todos [--open\|--done\|--all]` | 待办，以及做完的依据 |
| `fanear health [日期] [--type 心率]` | 健康数据汇总或明细 |
| `fanear places [日期] [--gcj02]` | 去过哪、停了多久，可换算成高德、腾讯地图用的坐标 |
| `fanear memory [名字]` | 小耳的记忆：关于你、人物和话题档案 |
| `fanear favorites` | 收藏的对话 |
| `fanear skill [install]` | 打印或安装 agent skill |
| `fanear doctor` | 检查数据目录、记录、skill 装得对不对 |
| `fanear version` | 版本 |

所有命令都能加 `--json`。日期可以写 今天、昨天、前天、`2026-11-24`、`11-24`、`11月24日`。每个命令的参数见 `fanear <命令> -h`。

退出码：0 成功，1 运行出错（比如找不到数据），2 命令或参数写错。写错时 fanear 会提示最接近的写法，比如把 todos 打成 todo，会问“你是不是想输入 todos？”；加了 `--json` 时，错误以 `{"ok":false,"error":"…","suggestion":"…"}` 写到 stderr。

## 不做的事

- 不写、不删、不挪数据目录里的任何文件。`录音/` 是饭耳存录音的地方，在这里删了饭耳里就听不了。
- 不处理音频，不调用模型，不联网（远程存储请先挂载或同步到本机）。
- 不读声纹、任务队列这些饭耳内部数据。
- 只有关键词搜索，没有语义搜索和索引。

## 格式

字段的意思以饭耳写在 `记录/README.md` 里的说明为准。`memory`、`favorites` 读的是没有公开文档的内部格式，饭耳改了格式时会报“解析失败”，而不是读出错的内容。

## License

MIT
