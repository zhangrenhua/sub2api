# Seedance 2.0 视频生成（**按次计费**）— 测试脚本使用说明

> 本文档/脚本对应**按次计费**的 Seedance 2.0 模型。**按秒计费**（Sora 系列）请用 `tools/sora_video_test.py` + `tools/sora_video_test.md`。

`tools/seedance_video_test.py` —— 用标准库（无第三方依赖）驱动视频生成的**异步任务**全流程：

1. `POST /v1/videos` 创建任务，拿到 `task_id`（响应 `id`/`task_id`）
2. `GET /v1/videos/{task_id}` 轮询状态，直到 `completed` / `failed`
3. 下载 mp4：**先试网关 `/content`，失败则直连完成响应的 `video_url`**（实测 Seedance 中转走直链）

> **架构说明**：脚本请求的是**本网关**（`SEEDANCE_BASE_URL`，默认 `https://www.cc-vibe.com`），用**本系统的 API Key**（`sk-...`）。网关再透传给账号 `base_url` 配置的上游中转。**不要**把 `SEEDANCE_BASE_URL` 设成上游中转地址（会绕过本网关鉴权/计费）。

---

## 1. 模型

| 模型 | 分辨率 | 计费方式 | 时长 |
|---|---|---|---|
| `seedance-2.0-fast-pass` | `720p` | **按次**（固定单价，时长不影响费用） | 4/5/10/15 |
| `seedance-2.0-pass` | `720p` | **按次**（固定单价，时长不影响费用） | 4/5/10/15 |

**按次**：费用 = 该模型固定单价 × 分组视频倍率，与时长/分辨率无关。

---

## 2. 前置条件（后台需先配好）

| 步骤 | 配置 |
|---|---|
| **上游账号** | 账号管理 → 新建：平台 **OpenAI**、类型 **API Key**；`base_url` 填上游中转主机基址（**不要带 `/v1/videos`**）；API Key 填中转密钥 |
| **分组** | 分组管理 → 开启 **「允许视频生成」**；在「按模型定价」里给 `seedance-2.0-*` 选**计费方式 = 按次**并填**单次价**；绑定上面的账号；**不要开「仅 OAuth」** |
| **API Key** | 在该视频分组下新建 `sk-...`（即 `SEEDANCE_API_KEY`） |
| **余额** | 该 key 所属用户需有余额，否则 `INSUFFICIENT_BALANCE` |

> ⚠️ 若没把模型设为「按次」并配单价，会按 0 计费（不收费）。

---

## 3. 基础用法

```bash
export SEEDANCE_API_KEY="sk-你的key"
# 本地 dev：export SEEDANCE_BASE_URL="http://127.0.0.1:8080"

# 文生视频
SEEDANCE_MODEL="seedance-2.0-fast-pass" SEEDANCE_RESOLUTION="720p" SEEDANCE_SECONDS=5 \
  python3 tools/seedance_video_test.py "一只橘猫在阳光草地上奔跑，低角度跟拍，电影感"

# 标准版 + 10 秒（按次计费，时长不加钱）
SEEDANCE_MODEL="seedance-2.0-pass" SEEDANCE_SECONDS=10 \
  python3 tools/seedance_video_test.py "保持参考图人物外貌一致，自然走动，柔和镜头推进"
```

输出保存为 `video_<task_id>.mp4`（可用 `SEEDANCE_OUT` 指定）。第一个命令行参数是 **prompt**。

**参考图（最多 4 张，字段 `referenceImages`）**：
```bash
export SEEDANCE_REFERENCE_IMAGE_URLS='["https://example.com/character-a.jpg","https://example.com/character-b.jpg"]'
SEEDANCE_MODEL="seedance-2.0-pass" python3 tools/seedance_video_test.py "保持参考图人物外貌和服装一致，自然走动"
```

**首尾帧（`first_image`/`last_image`，须成对，且不能与参考图/视频同用）**：
```bash
SEEDANCE_FIRST_IMAGE="https://example.com/sea-morning.jpg" \
SEEDANCE_LAST_IMAGE="https://example.com/sea-evening.jpg" \
  python3 tools/seedance_video_test.py "从清晨过渡到黄昏的海边延时镜头，画面稳定"
```

---

## 4. 报文示例

### 4.1 创建任务（文生视频）
```http
POST /v1/videos HTTP/1.1
Host: www.cc-vibe.com
Authorization: Bearer sk-xxxx
Content-Type: application/json

{
  "model": "seedance-2.0-fast-pass",
  "prompt": "一只橘猫在阳光草地上奔跑，低角度跟拍，电影感",
  "duration": 5,
  "ratio": "16:9",
  "resolution": "720p"
}
```
返回（`200`，入队）：
```json
{ "id": "task_xxx", "task_id": "task_xxx", "object": "video", "model": "seedance-2.0-fast-pass", "status": "queued", "progress": 0 }
```

### 4.2 查询状态
```http
GET /v1/videos/task_xxx HTTP/1.1
Authorization: Bearer sk-xxxx
```
完成返回（`video_url` 可能是直链，如 OSS）：
```json
{
  "id": "task_xxx",
  "task_id": "task_xxx",
  "status": "completed",
  "video_url": "https://opcbucket.oss-cn-beijing.aliyuncs.com/.../xxx.mp4",
  "url": "https://opcbucket.oss-cn-beijing.aliyuncs.com/.../xxx.mp4"
}
```
> 状态值：`queued` / `in_progress`(或 `processing`) / `completed` / `failed`。

### 4.3 下载
脚本**先试网关** `GET /v1/videos/{id}/content`；若该端点返回 `401 login required`（部分中转不支持 API Key 下载），自动回退**直连** `video_url`。手动下载直链：
```bash
curl -L "https://opcbucket.oss-cn-beijing.aliyuncs.com/.../xxx.mp4" -o video.mp4
```

---

## 5. 环境变量

> 每个变量都可用 `SEEDANCE_` 前缀；缺省时回退同名 `SORA_` 前缀（沿用旧脚本习惯）。

| 变量 | 必填 | 默认 | 说明 |
|---|---|---|---|
| `SEEDANCE_API_KEY` | ✅ | — | 视频分组下的本系统 API Key |
| `SEEDANCE_BASE_URL` | | `https://www.cc-vibe.com` | **本网关**地址（不是上游中转） |
| `SEEDANCE_MODEL` | | `seedance-2.0-fast-pass` | `seedance-2.0-fast-pass` / `seedance-2.0-pass` |
| `SEEDANCE_RESOLUTION` | | `720p` | 输出清晰度 |
| `SEEDANCE_RATIO` | | `16:9` | 同时作为 `ratio` 与 `aspect_ratio` 发送（等价）。`16:9`/`9:16`/`1:1` 等 |
| `SEEDANCE_SECONDS` | | `5` | 时长；同时作为 `duration`(整数) 与 `seconds`(字符串) 发送。支持 4/5/10/15 |
| `SEEDANCE_MAX_SECONDS` | | `15` | 时长上限（按次计费时长不加钱，默认放开到 15） |
| `SEEDANCE_FIRST_IMAGE` | | — | 首帧 URL（`first_image`，须与 `last_image` 成对） |
| `SEEDANCE_LAST_IMAGE` | | — | 尾帧 URL（`last_image`） |
| `SEEDANCE_REFERENCE_IMAGE_URLS` | | — | JSON 数组，参考图（`referenceImages`，**最多 4**） |
| `SEEDANCE_REFERENCE_VIDEO_URLS` | | — | JSON 数组，参考视频（`referenceVideos`，**最多 3**） |
| `SEEDANCE_POLL_SEC` | | `10` | 轮询间隔秒（官方建议 30-60s） |
| `SEEDANCE_TIMEOUT` | | `600` | 最长等待秒 |
| `SEEDANCE_OUT` | | `video_<id>.mp4` | 输出文件名 |
| `SEEDANCE_EXTRA_JSON` | | — | 额外 JSON 对象，合并进请求体（可覆盖上面字段） |

**字段规则**：纯文生视频只需 `model`+`prompt`；首尾帧须 `first_image`+`last_image` 成对且不与参考图/视频同用；素材须是服务端可访问的 URL（不支持 base64/`data:`）；当前不支持参考音频。

---

## 6. 计费说明（重要）

- **只在 create 成功时计费一次**，按 `request_id` 幂等；轮询/下载不计费；失败的 create（4xx/5xx/余额/权限）**不计费**。
- 计费 = 该模型「按次」单价 × 分组视频倍率（`usage_logs.billing_mode=video`）。**时长/分辨率不影响费用**。
- ⚠️ 必须在分组「按模型定价」里把 `seedance-2.0-*` 设为「按次」并配单价，否则按 0 计费。
- ⚠️ **create 成功即扣费，不是生成完成时**；若 create 成功但生成 `failed`，已扣费且不自动退。

校验某次计费：
```sql
SELECT created_at, requested_model, billing_mode, total_cost
FROM usage_logs WHERE requested_model LIKE 'seedance%'
ORDER BY created_at DESC LIMIT 5;
```

---

## 7. 常见报错

| HTTP | 含义 | 排查 |
|---|---|---|
| `401` | key 无效 | 检查 `SEEDANCE_API_KEY` |
| `403 INSUFFICIENT_BALANCE` | 余额不足 | 给该用户充值 |
| `403 Video generation is not enabled` | 分组没开视频 | 分组里打开「允许视频生成」 |
| `503 No available compatible accounts` | 没有可用账号 | 分组要绑定 **API Key** 类型 OpenAI 账号且 schedulable；模型名与上游一致 |
| `502 Upstream request failed` | 连不上上游 | 账号 `base_url` 是否正确/可达；必要时给账号配代理 |
| 下载 `401 login required` | 中转 `/content` 不支持 API Key 下载 | 脚本已自动回退直连 `video_url`（无需处理） |
