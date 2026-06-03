# Sora / Seedance 2.0 视频生成 — 测试脚本使用说明

`tools/sora_video_test.py` —— 用标准库（无第三方依赖）驱动视频生成的**异步任务**全流程：

1. `POST /v1/videos` 创建任务，拿到 `task_id`（响应 `id`）
2. `GET /v1/videos/{task_id}` 轮询状态，直到 `completed` / `failed`（建议每 5 秒一次）
3. `GET /v1/videos/{task_id}/content` 下载 mp4（**走本网关**，自动落到创建任务的同一账号）

脚本默认把时长钳到 **5 秒**（成本最低，防按秒计费烧钱），上限可用 `SORA_MAX_SECONDS` 调整。**按次计费的 Seedance 2.0 时长不影响费用**,可放开到 10/15 测全长。脚本同时兼容 Sora（按秒）与 Seedance 2.0（按次）两类模型。

> **架构说明（重要）**：脚本请求的是**本网关**（`SORA_BASE_URL`，默认 `https://www.cc-vibe.com`），用的是**本系统的 API Key**（`sk-...`）。网关再把请求**透传**给你在**账号 `base_url`** 里配置的上游中转（如 `https://www.cc-vibe.com`）。所以**不要**把 `SORA_BASE_URL` 设成上游中转地址——那样会绕过本网关的鉴权/计费。

---

## 1. 前置条件（后台需先配好）

| 步骤 | 配置 |
|---|---|
| **上游账号** | 账号管理 → 新建：平台 **OpenAI**、类型 **API Key**；`base_url` 填**上游中转主机基址**（如 `https://www.cc-vibe.com`，**不要带 `/v1/videos`**）；API Key 填中转给的密钥 |
| **分组** | 分组管理 → 新建/编辑：平台 **OpenAI**；开启 **「允许视频生成」**；在「按模型定价」里给每个模型配价格,并选**计费方式**：Sora 选「按秒」填每秒价(标清/高清),Seedance 2.0 选「按次」填单次价；把上面的账号绑定到该分组；**不要开「仅 OAuth」**（视频只调度 API Key 账号） |
| **API Key** | 在该视频分组下新建 `sk-...`，给脚本用（即 `SORA_API_KEY`） |
| **余额** | 该 key 所属用户需有余额，否则报 `INSUFFICIENT_BALANCE` |

---

## 2. 模型 ↔ 分辨率 ↔ 计费方式

| 模型 | 分辨率 | 计费方式 | 时长 |
|---|---|---|---|
| `sora-v3-fast` | `480p` | **按秒**，标清（<1080） | 5/10/15 |
| `sora-v3-pro` | `720p` | **按秒**，标清（<1080） | 5/10/15 |
| `sora-vip3-pro-720p` | `480p` / `720p` | **按秒**，标清（<1080） | 5/10/15 |
| `sora-vip3-pro-1080p` | `480p` / `720p` / `1080p` | **按秒**，`1080p` 高清其余标清 | 5/10/15 |
| `seedance-2.0-fast-pass` | `720p` | **按次**（固定单价，时长不影响费用） | 4/5/10/15 |
| `seedance-2.0-pass` | `720p` | **按次**（固定单价，时长不影响费用） | 4/5/10/15 |

> **按秒**：计费档由请求里的 `resolution` 决定（≥1080 高清每秒价，否则标清每秒价），费用 = 时长 × 每秒价 × 倍率。
> **按次**：与时长/分辨率无关，按固定单价计费（Seedance 2.0）。两种都需在分组「按模型定价」里配置该模型的价格与计费方式。

时长：脚本默认 `SORA_SECONDS=5` 且 `SORA_MAX_SECONDS=5`（防止按秒计费烧钱）。
**按次计费模型（Seedance）时长不加钱**，可 `SORA_MAX_SECONDS=10 SORA_SECONDS=10`（或 15）测全长。脚本会同时发送 `seconds` 与 `duration`、`aspect_ratio` 与 `ratio`（各为等价字段），同时兼容两类上游。

---

## 3. 基础用法

```bash
# 必填：视频分组下的本系统 API Key
export SORA_API_KEY="sk-你的key"
# 可选：本网关地址（默认 https://www.cc-vibe.com；本地 dev 用 http://127.0.0.1:8080）
export SORA_BASE_URL="https://www.cc-vibe.com"

# sora-v3-fast → 480p
SORA_MODEL="sora-v3-fast" SORA_RESOLUTION="480p" \
  python3 tools/sora_video_test.py "雨夜霓虹街道，镜头缓慢推进，电影感光影"

# sora-v3-pro → 720p
SORA_MODEL="sora-v3-pro" SORA_RESOLUTION="720p" \
  python3 tools/sora_video_test.py "a calm ocean wave at sunset, cinematic"
```

输出保存为 `video_<task_id>.mp4`（可用 `SORA_OUT` 指定文件名）。第一个命令行参数是 **prompt**。

**Seedance 2.0（按次计费，时长不加钱）**：
```bash
# 文生视频
SORA_MODEL="seedance-2.0-fast-pass" SORA_RESOLUTION="720p" SORA_ASPECT="16:9" \
SORA_MAX_SECONDS=10 SORA_SECONDS=10 \
  python3 tools/sora_video_test.py "一只橘猫在阳光草地上奔跑，低角度跟拍，电影感"

# 参考图生成（Seedance 最多 4 张；字段 reference_image_urls，等价 referenceImages）
export SORA_REFERENCE_IMAGE_URLS='["https://example.com/character-a.jpg","https://example.com/character-b.jpg"]'
SORA_MODEL="seedance-2.0-pass" SORA_RESOLUTION="720p" \
  python3 tools/sora_video_test.py "保持参考图人物外貌和服装一致，自然走动"

# 首尾帧（Seedance 用 first_image/last_image，必须成对，且不能与参考图/参考视频同用）
SORA_MODEL="seedance-2.0-pass" SORA_RESOLUTION="720p" \
SORA_FIRST_IMAGE="https://example.com/sea-morning.jpg" \
SORA_LAST_IMAGE="https://example.com/sea-evening.jpg" \
  python3 tools/sora_video_test.py "从清晨过渡到黄昏的海边延时镜头，画面稳定"
```

**Sora 图生视频**：
```bash
export SORA_IMAGE_URL="https://example.com/input.jpg"
python3 tools/sora_video_test.py "保持图片主体一致，生成自然运动镜头"
```

---

## 4. 报文示例（请求 / 返回）

> 以下都是打**本网关** `https://www.cc-vibe.com`、用**本系统 key** `sk-...` 的报文；网关再透传给上游。

### 4.1 创建任务（文生视频）

请求：
```http
POST /v1/videos HTTP/1.1
Host: www.cc-vibe.com
Authorization: Bearer sk-xxxx
Content-Type: application/json

{
  "model": "sora-v3-fast",
  "prompt": "雨夜霓虹街道，镜头缓慢推进，电影感光影",
  "aspect_ratio": "16:9",
  "resolution": "480p",
  "seconds": "5"
}
```

返回（`200`，任务已入队）：
```json
{
  "id": "task_E4GhW5UTnIz4ZtYbv1riAohsDdFyO2hb",
  "object": "video",
  "model": "sora-v3-fast",
  "status": "queued",
  "progress": 0,
  "created_at": 1779560000
}
```

### 4.2 查询状态

请求：
```http
GET /v1/videos/task_E4GhW5UTnIz4ZtYbv1riAohsDdFyO2hb HTTP/1.1
Host: www.cc-vibe.com
Authorization: Bearer sk-xxxx
```

生成中返回：
```json
{
  "id": "task_E4GhW5UTnIz4ZtYbv1riAohsDdFyO2hb",
  "status": "in_progress",
  "progress": 44
}
```

完成返回：
```json
{
  "id": "task_E4GhW5UTnIz4ZtYbv1riAohsDdFyO2hb",
  "task_id": "task_E4GhW5UTnIz4ZtYbv1riAohsDdFyO2hb",
  "object": "video",
  "model": "sora-v3-fast",
  "status": "completed",
  "progress": 100,
  "created_at": 1779560000,
  "completed_at": 1779560150,
  "seconds": "5",
  "size": "854x480",
  "video_url": "https://www.cc-vibe.com/v1/videos/task_E4GhW5UTnIz4ZtYbv1riAohsDdFyO2hb/content",
  "url": "https://www.cc-vibe.com/v1/videos/task_E4GhW5UTnIz4ZtYbv1riAohsDdFyO2hb/content"
}
```

> 状态值：`queued` / `in_progress` / `completed` / `failed`。务必用本网关的 `/content` 端点下载，别直连返回里的 `video_url`。

### 4.3 下载视频内容

请求：
```bash
curl -L "https://www.cc-vibe.com/v1/videos/task_xxx/content" \
  -H "Authorization: Bearer sk-xxxx" \
  -o sora_task_xxx.mp4
```
返回：`200`，`Content-Type: video/mp4` 的二进制内容（mp4）。

### 4.4 图生视频 / 多模态请求体

图生视频：
```json
{
  "model": "sora-v3-pro",
  "prompt": "保持人物外貌和服装一致，自然缓慢走动",
  "image_url": "https://example.com/input.jpg",
  "aspect_ratio": "16:9",
  "resolution": "720p",
  "seconds": "5"
}
```

多参考图 + 首尾帧（网关增强字段，透传上游）：
```json
{
  "model": "sora-vip3-pro-1080p",
  "prompt": "以首帧为开始、尾帧为结束，参考图保持角色与场景一致",
  "first_frame_url": "https://example.com/frames/start.jpg",
  "last_frame_url": "https://example.com/frames/end.jpg",
  "reference_image_urls": [
    "https://example.com/ref/character.jpg",
    "https://example.com/ref/outfit.jpg"
  ],
  "aspect_ratio": "16:9",
  "resolution": "1080p",
  "seconds": "5"
}
```

### 4.5 失败返回示例

余额不足（`403`）：
```json
{ "code": "INSUFFICIENT_BALANCE", "message": "Insufficient account balance" }
```

分组未开视频（`403`）：
```json
{ "error": { "type": "permission_error", "message": "Video generation is not enabled for this group" } }
```

无可用账号（`503`）：
```json
{ "error": { "type": "api_error", "message": "No available compatible accounts" } }
```

连不上上游（`502`）：
```json
{ "error": { "type": "upstream_error", "message": "Upstream request failed" } }
```

---

## 5. 环境变量

| 变量 | 必填 | 默认 | 说明 |
|---|---|---|---|
| `SORA_API_KEY` | ✅ | — | 视频分组下的本系统 API Key（Bearer 鉴权） |
| `SORA_BASE_URL` | | `https://www.cc-vibe.com` | **本网关**地址（不是上游中转） |
| `SORA_MODEL` | | `sora-v3-fast` | 视频模型名 |
| `SORA_RESOLUTION` | | `480p` | 按秒计费时 ≥1080 走高清档；Seedance 常用 `720p` |
| `SORA_ASPECT` | | `16:9` | 同时作为 `aspect_ratio` 与 `ratio` 发送（等价）。`16:9`/`9:16`/`4:3`/`3:4`/`1:1`/`21:9` |
| `SORA_SECONDS` | | `5` | 时长；同时作为 `seconds`(字符串) 与 `duration`(整数) 发送。受 `SORA_MAX_SECONDS` 钳制 |
| `SORA_MAX_SECONDS` | | `5` | 时长上限。按秒计费防烧钱；**按次计费(Seedance)时长不加钱**，可设 10/15 测全长 |
| `SORA_IMAGE_URL` | | — | 主参考图（HTTPS URL 或图片 data URL）；传了即图生视频 |
| `SORA_POLL_SEC` | | `5` | 轮询间隔秒 |
| `SORA_TIMEOUT` | | `600` | 最长等待秒 |
| `SORA_OUT` | | `video_<id>.mp4` | 输出文件名 |

**可选多模态 / 参考 / 首尾帧 / v2v 字段**（脚本会原样合入请求体并透传给上游，**能否生效取决于上游是否支持**）：

| 变量 | 对应字段 | 数量/格式 | 说明 |
|---|---|---|---|
| `SORA_REFERENCE_IMAGE_URLS` | `reference_image_urls` | JSON 数组（Sora≤9 / **Seedance≤4**） | 多张参考图；Seedance 等价 `referenceImages` |
| `SORA_REFERENCE_VIDEO_URLS` | `reference_video_urls` | JSON 数组，最多 3 | 多个参考视频；Seedance 等价 `referenceVideos` |
| `SORA_REFERENCE_AUDIO_URLS` | `reference_audio_urls` | JSON 数组，最多 3 | 多个参考音频；**Seedance 暂不支持音频** |
| `SORA_REFERENCE_TEXT` | `reference_text` | 文本 | 角色设定/分镜/品牌规范/台词 |
| `SORA_FIRST_IMAGE` | `first_image` | URL 或 data URL | **Seedance 首帧**，须与 `last_image` 成对，且不能与参考图/视频同用 |
| `SORA_LAST_IMAGE` | `last_image` | URL 或 data URL | **Seedance 尾帧**，须与 `first_image` 成对 |
| `SORA_FIRST_FRAME_URL` | `first_frame_url` | URL 或 data URL | **Sora 首帧**（优先级高于 `image_url`） |
| `SORA_LAST_FRAME_URL` | `last_frame_url` | URL 或 data URL | **Sora 尾帧** |
| `SORA_SOURCE_VIDEO_URL` | `source_video_url` | URL | v2v 源视频 |
| `SORA_SOURCE_VIDEO_ID` | `source_video_id` | 任务 ID | v2v 基于已生成视频继续编辑/重混 |
| `SORA_EXTRA_JSON` | （合并）| JSON 对象 | 任意额外字段，合并进请求体；可覆盖上面字段，方便临时测试新字段 |

---

## 6. 多模态 / 参考资产示例

9 张参考图：
```bash
export SORA_REFERENCE_IMAGE_URLS='[
  "https://example.com/ref/character_front.jpg",
  "https://example.com/ref/character_side.jpg",
  "https://example.com/ref/outfit_detail.jpg",
  "https://example.com/ref/environment.jpg"
]'
python3 tools/sora_video_test.py "参考多张图片生成角色一致的镜头，保持外貌/服装/道具/光影一致"
```

3 个参考视频 / 3 个参考音频：
```bash
export SORA_REFERENCE_VIDEO_URLS='["https://example.com/v1.mp4","https://example.com/v2.mp4"]'
export SORA_REFERENCE_AUDIO_URLS='["https://example.com/bgm.mp3","https://example.com/ambient.wav"]'
python3 tools/sora_video_test.py "参考视频的运镜与动作节奏、参考音频的音乐与环境声，生成电影感短片"
```

文本参考：
```bash
export SORA_REFERENCE_TEXT='品牌关键词：高级、克制、未来感。角色不换脸，米色风衣+银色耳机。镜头：街道建立 -> 侧脸 -> 道具特写 -> 结尾 logo 留白。'
python3 tools/sora_video_test.py "按文本参考里的品牌规范和分镜结构生成短片"
```

首尾帧：
```bash
export SORA_FIRST_FRAME_URL="https://example.com/frames/start.jpg"
export SORA_LAST_FRAME_URL="https://example.com/frames/end.jpg"
python3 tools/sora_video_test.py "从首帧自然开始，人物缓慢前行，最终过渡到尾帧构图"
```

v2v（源视频 URL 或已生成任务 ID，走 `/v1/videos` 的请求体字段）：
```bash
export SORA_SOURCE_VIDEO_URL="https://example.com/input/source.mp4"
python3 tools/sora_video_test.py "保留源视频动作与运镜，把场景改为赛博朋克雨夜街道"
# 或基于已生成视频：
export SORA_SOURCE_VIDEO_ID="task_xxx"
```

临时测试任意新字段：
```bash
export SORA_EXTRA_JSON='{"reference_text":"胶片感、低饱和","seed":12345}'
python3 tools/sora_video_test.py "按参考文本生成视频"
```

### 参考资产建议
- 图片：`jpg`/`jpeg`/`png`/`webp`；视频：`mp4`/`mov`/`webm`；音频：`mp3`/`wav`/`m4a`/`aac`。
- 生产建议用**可公网访问的 HTTPS URL**或上传后引用 `file_id`。
- 字段优先级：首帧 `first_frame_url` > `image_url` > `reference_image_urls[0]`；尾帧仅由 `last_frame_url` 指定。

---

## 7. Base64 说明
- **参考图片支持完整 base64 data URL**（必须带前缀，如 `data:image/jpeg;base64,...`，**不要传裸 base64**）。`image_url` / `first_frame_url` / `last_frame_url` / `reference_image_urls` 均可用 data URL。
- base64 会显著放大 JSON 体积，生产更推荐 HTTPS URL。注意网关/Nginx/CDN 的 body size 限制（过大报 `413`）。
- **参考视频不要用 base64**：体积过大、上游也无公开口径。v2v 请用 `source_video_url` / `source_video_id`。

---

## 8. 字段与端点说明
- **必填**：`model`、`prompt`。其余按上游可选。脚本默认还会带 `aspect_ratio`+`ratio`、`resolution`、`seconds`+`duration`。
- **字段命名差异（Sora vs Seedance 2.0）**：脚本对核心维度发送两种等价写法以兼容两类上游：
  - 时长：`seconds`(字符串) + `duration`(整数) —— 两者等价。
  - 比例：`aspect_ratio` + `ratio` —— 两者等价。
  - 首尾帧：Sora 用 `first_frame_url`/`last_frame_url`，**Seedance 用 `first_image`/`last_image`**（分别有独立环境变量）。
  - 参考图/视频：`reference_image_urls`/`reference_video_urls`（Seedance 等价 `referenceImages`/`referenceVideos`，Seedance 上限分别为 4/3，且不支持参考音频）。
- `reference_*` / 首尾帧 / `source_video_*` 是**透传字段**：网关对 `/v1/videos` 原样转发上游，能否生效取决于上游。**上游若只接受其官方字段**，多余字段可能被忽略或报错。
- 本网关**只提供** `/v1/videos`、`/v1/videos/{id}`、`/v1/videos/{id}/content`，**不提供** `/v1/videos/edits`。v2v 通过请求体的 `source_video_url` / `source_video_id` 走 `/v1/videos`。
- 响应里任务 ID：Sora 返回 `id`，Seedance 同时返回 `id` 与 `task_id`（同值）；脚本两者都兼容。
- **下载差异（实测）**：部分中转的 `/v1/videos/{id}/content` 不支持 API Key 下载（返回 `401 login required`），而是在完成响应的 `video_url` 里给**直链**（如阿里云 OSS）。脚本已做回退：**先试网关 `/content`，失败则直连 `video_url`**。Seedance 2.0 中转实测走的是直链。

---

## 9. 常见报错对照

| HTTP | 含义 | 排查 |
|---|---|---|
| `400` | 请求体过大 / JSON 非法 / base64 格式错 | 用完整 data URL；大图改 URL；数组字段须为合法 JSON |
| `401` | key 无效 | 检查 `SORA_API_KEY` |
| `403 INSUFFICIENT_BALANCE` | 余额不足 | 给该用户充值 |
| `403 Video generation is not enabled` | 分组没开视频 | 分组里打开「允许视频生成」 |
| `413 Payload Too Large` | 请求体过大 | 别把大视频/大量高清图转 base64；改用 URL / file_id |
| `503 No available compatible accounts` | 没有可用账号 | 分组要绑定 **API Key** 类型 OpenAI 账号且 schedulable；模型名与上游一致 |
| `502 Upstream request failed` | 连不上上游 | 账号 `base_url` 是否正确/可达；上游被墙/reset 时给账号配代理 |

查网关侧真实上游错误：
```sql
SELECT created_at, requested_model, upstream_error_message
FROM ops_error_logs
WHERE request_path LIKE '%/videos%' AND error_phase='upstream'
ORDER BY created_at DESC LIMIT 5;
```

---

## 10. 计费说明（重要）

- **只在 create 成功时计费一次**，按 `request_id` 幂等；轮询/下载不计费；失败的 create（4xx/5xx/EOF/余额/权限）**不计费**。
- 计费方式由分组「按模型定价」里该模型的 `billing_mode` 决定（`usage_logs.billing_mode=video`）：
  - **按秒（per_second，默认，如 Sora）**：`时长(秒) × 每秒价(按 resolution 区分标清/高清) × 分组视频倍率`。
  - **按次（per_request，如 Seedance 2.0）**：`固定单价 × 分组视频倍率`，与时长/分辨率无关。
- ⚠️ 按次计费下时长不影响费用，但**按模型定价里必须把该模型设为「按次」并配单价**，否则按 0 计费（不收费）。
- ⚠️ **在「创建成功」时扣费，不是「生成完成」时**：若 create 成功但后续生成 `failed`，用户**已扣费且不自动退**。是否合适取决于上游对"提交后失败"是否退款。
- ⚠️ 记账为**异步尽力**：记账时 DB 瞬时报错会漏记一笔（平台少收，不多扣用户），与图片/对话计费一致。

校验某次计费：
```sql
SELECT created_at, requested_model, billing_mode, total_cost
FROM usage_logs WHERE inbound_endpoint LIKE '%videos%'
ORDER BY created_at DESC LIMIT 5;
```
