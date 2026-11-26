---
name: fanear
description: 饭耳（FanEar）手表全天录音的记录。用户提到饭耳或手表录音，或问自己哪天说过、听到、答应过、跟谁聊过什么，那天去过哪、睡得怎样，或问小耳记的某个人时，用 fanear 命令查原话、待办、总结、足迹、健康和人物档案。
---

# fanear

饭耳把每天的总结、逐句转写、待办、健康和足迹写进用户选的保存位置；`fanear` 只读这些文件，不联网。

## 前提

先跑 `fanear version` 确认装了。没装就运行 `GOPROXY=direct go install github.com/Dikevin/fanear-cli/cmd/fanear@latest`，再 `fanear skill install --claude`。数据找不到、skill 版本不对时先跑 `fanear doctor`。

## 选命令

| 想知道 | 命令 |
|---|---|
| 某天大致做了什么、有哪些重点 | `fanear brief <日期>` |
| 谁说过什么、什么时候提到过某事 | `fanear search 词1 词2 …`，再 `fanear transcript <日期> --from HH:MM --to HH:MM` 看前后文 |
| 答应过谁、谁答应过用户、做没做完 | `fanear todos`（`--open` 只看没做完的，可跟关键词） |
| 某个人是谁、小耳对用户的了解 | `fanear memory <名字>`、`fanear memory 我` |
| 去过哪、待了多久 | `fanear places <日期>`（要放到高德、腾讯地图上加 `--gcj02`） |
| 心率、睡眠、步数 | `fanear health <日期>`，单项明细加 `--type 心率` |
| 收藏的对话 | `fanear favorites` |
| 有哪些天、数据在哪 | `fanear days`、`fanear where` |

参数以 `fanear <命令> -h` 为准；要自己算就加 `--json`。日期可写 今天、昨天、11-24、2026-11-24。

## 查法

1. 由粗到细：先用 brief 或 search 定位到日期和时刻，再用 transcript 截那一小段看上下文。一整天的转写有上万句，transcript 总是带 `--from`/`--to`。
2. search 默认命中任意一个关键词，一次给几个同义说法。转写有错字，搜不到就换近音词、换说法再搜，并说明搜了哪几天。
3. 日期不明确时先 `fanear days` 看有哪几天。

## 回答

- 先给结论，再引原话，带日期、时刻和说话人，如「11月25日 14:03:16 我：……」。
- 说话人带「（推测）」的是饭耳推测的，引用时照留；「说话人 1」「声音 14」是没认出名字的人；「杂音」是饭耳认出的背景声（视频、电视），不算任何人说的话。
- 待办状态照抄 fanear 的写法：「已做完（自己点的）」是用户自己点的；「已做完（小耳听到原话）」附的依据原话不一定是本人说的；「已做完（小耳推测）」不一定真做完了。
- 总结、时段卡、待办、记忆页都是饭耳的 AI（小耳）写的；和原话对不上时以原话为准，并指出来。

## 数据目录只读

`fanear where` 显示的数据目录只用 fanear 或只读命令访问。里面的 `录音/` 是饭耳存录音的唯一位置，删掉、挪走或改名后饭耳里就再也听不了；`记录/` 会被手机下次同步覆盖。

## 出错时

- 「没找到饭耳的数据目录」：问用户饭耳存在哪（饭耳 › 设置 › 存储），再 `fanear where --set <目录>`，指到它上面任意一层都行。NAS、对象存储要先挂载或同步到本机。
- memory、favorites 说没有备份：Google Drive、NAS、对象存储这些保存位置没有饭耳的资料备份，改用其他命令。
- 第一次读某天很慢：iCloud、网盘上的文件是读的时候才下载。
- 「解析失败（饭耳可能改了格式）」：请用户升级 fanear：`GOPROXY=direct go install github.com/Dikevin/fanear-cli/cmd/fanear@latest`。
