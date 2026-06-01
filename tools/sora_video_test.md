# Sora 视频生成 — 测试脚本使用说明

`tools/sora_video_test.py` —— 用标准库（无第三方依赖）驱动 Sora 视频生成的**异步任务**全流程：

1. `POST /v1/videos` 创建任务，拿到 `task_id`
2. `GET /v1/videos/{task_id}` 轮询状态，直到 `completed` / `failed`
3. `GET /v1/videos/{task_id}/content` 下载 mp4（**走本网关**，自动落到创建任务的同一账号）

脚本硬性限制 **5 秒**，单次成本最低（即使把 `SORA_SECONDS` 设大也会被钳回 5）。

---

## 1. 前置条件（后台需先配好）

| 步骤 | 配置 |
|---|---|
| **上游账号** | 账号管理 → 新建：平台 **OpenAI**、类型 **API Key**；`base_url` 填上游中转**主机基址**（如 `https://www.zitxitongxue.com`，**不要带 `/v1/videos`**）；API Key 填中转给的密钥 |
| **分组** | 分组管理 → 新建/编辑：平台 **OpenAI**；开启 **「允许视频生成」**；在「按模型定价」里给每个模型配每秒价；把上面的账号绑定到该分组；**不要开「仅 OAuth」**（视频只调度 API Key 账号） |
| **API Key** | 在该视频分组下新建 `sk-...`，给脚本用 |
| **余额** | 该 key 所属用户需有余额（否则报 `INSUFFICIENT_BALANCE`） |

---

## 2. 模型 ↔ 分辨率对照

| 模型 | 分辨率 | 计费档 |
|---|---|---|
| `sora-v3-fast` | `480p` | 标清（<1080） |
| `sora-v3-pro` | `720p` | 标清（<1080） |

> 计费档由请求里的 `resolution` 决定：**≥1080 走高清每秒价，否则标清每秒价**。

---

## 3. 用法

```bash
# 必填：视频分组下的 API Key
export SORA_API_KEY="sk-你的key"

# sora-v3-fast → 480p
SORA_MODEL="sora-v3-fast" SORA_RESOLUTION="480p" \
  python3 tools/sora_video_test.py "雨夜霓虹街道，镜头缓慢推进，电影感光影"

# sora-v3-pro → 720p
SORA_MODEL="sora-v3-pro" SORA_RESOLUTION="720p" \
  python3 tools/sora_video_test.py "a calm ocean wave at sunset, cinematic"
```

输出保存为 `sora_<task_id>.mp4`（可用 `SORA_OUT` 指定文件名）。

**图生视频**：再加 `export SORA_IMAGE_URL="https://example.com/input.jpg"`。

---

## 4. 环境变量

| 变量 | 必填 | 默认 | 说明 |
|---|---|---|---|
| `SORA_API_KEY` | ✅ | — | 视频分组下的 API Key（Bearer 鉴权） |
| `SORA_BASE_URL` | | `http://127.0.0.1:8080` | **本网关**地址（不是上游中转） |
| `SORA_MODEL` | | `sora-vip3-pro-720p` | 视频模型名（如 `sora-v3-fast`/`sora-v3-pro`） |
| `SORA_RESOLUTION` | | `720p` | `480p`/`720p`/`1080p`；≥1080 走高清计费档 |
| `SORA_ASPECT` | | `16:9` | `16:9`/`9:16`/`4:3`/`3:4`/`1:1`/`21:9` |
| `SORA_SECONDS` | | `5` | 硬性上限 5（设更大会被钳回 5，省钱） |
| `SORA_IMAGE_URL` | | — | 传了即图生视频 |
| `SORA_POLL_SEC` | | `5` | 轮询间隔秒 |
| `SORA_TIMEOUT` | | `600` | 最长等待秒 |
| `SORA_OUT` | | `sora_<id>.mp4` | 输出文件名 |

第一个命令行参数是 **prompt**（提示词）。

---

## 5. 常见报错对照

| HTTP | 含义 | 排查 |
|---|---|---|
| `401` | key 无效 | 检查 `SORA_API_KEY` |
| `403 INSUFFICIENT_BALANCE` | 余额不足 | 给该用户充值 |
| `403 Video generation is not enabled` | 分组没开视频 | 分组里打开「允许视频生成」 |
| `503 No available compatible accounts` | 没有可用账号 | 分组要绑定 **API Key** 类型的 OpenAI 账号、且 schedulable；模型名要和上游一致 |
| `502 Upstream request failed` | 连不上上游 | 账号 `base_url` 是否正确/可达；上游被墙/reset 时考虑给账号配代理 |

排障时可查网关侧真实上游错误：

```sql
SELECT created_at, requested_model, upstream_error_message
FROM ops_error_logs
WHERE request_path LIKE '%/videos%' AND error_phase='upstream'
ORDER BY created_at DESC LIMIT 5;
```

---

## 6. 计费说明（重要）

- **只在 create 成功时计费一次**，按 `request_id` 幂等；轮询/下载不计费；失败的 create（4xx/5xx/EOF/余额/权限）**不计费**。
- 计费公式：`时长(秒) × 该模型每秒价(按 resolution 区分标清/高清) × 分组视频倍率`，`billing_mode=video`。
- ⚠️ **在「创建成功」时扣费，不是「生成完成」时**：若 create 成功但后续生成 `failed`，用户**已扣费且不自动退**。是否合适取决于上游对"提交后失败"是否退款。
- ⚠️ 记账为**异步尽力**：记账时 DB 瞬时报错会漏记一笔（平台少收，不多扣用户），与图片/对话计费一致。

校验某次计费：

```sql
SELECT created_at, requested_model, billing_mode, total_cost
FROM usage_logs WHERE inbound_endpoint LIKE '%videos%'
ORDER BY created_at DESC LIMIT 5;
```
