#!/usr/bin/env python3
"""
Sora 视频生成 - 异步任务调用脚本（无第三方依赖，仅用标准库）

流程（异步任务）：
  1. POST /v1/videos              创建任务，拿到 task id
  2. GET  /v1/videos/{id}         轮询状态，直到 completed / failed
  3. GET  /v1/videos/{id}/content 下载 mp4（走本网关，自动落到创建任务的同一账号）

用法：
  export SORA_API_KEY="sk-你的key"          # 必填：本系统某个「视频分组」下的 API Key
  export SORA_BASE_URL="https://www.cc-vibe.com"   # 可选，默认公网网关；本地 dev 用 http://127.0.0.1:8080
  export SORA_MODEL="sora-v3-fast"    # 可选，默认 720p 模型
  python3 scripts/sora_video_test.py "雨夜霓虹街道，镜头缓慢推进，电影感光影"

  # 图生视频：再加一个图片 URL 环境变量
  export SORA_IMAGE_URL="https://example.com/input.jpg"

参数（环境变量，均可选，除 SORA_API_KEY）：
  SORA_API_KEY    必填，Bearer 鉴权用
  SORA_BASE_URL   默认 https://www.cc-vibe.com
  SORA_MODEL      默认 sora-v3-fast
  SORA_RESOLUTION 默认 480p（480p/720p/1080p；>=1080 走高清计费档）
  SORA_ASPECT     默认 16:9（16:9 / 9:16 / 4:3 / 3:4 / 1:1 / 21:9）
  SORA_SECONDS    默认 5；脚本硬性上限 5 秒（设更大会被钳回 5，省钱）
  SORA_IMAGE_URL  可选，主参考图（HTTPS URL 或图片 data URL）；传了即图生视频
  SORA_POLL_SEC   轮询间隔秒，默认 5
  SORA_TIMEOUT    最长等待秒，默认 600
  SORA_OUT        输出文件名，默认 sora_<task_id>.mp4

可选多模态 / 参考 / 首尾帧 / v2v 字段（透传给上游，能否生效取决于上游是否支持）：
  SORA_REFERENCE_IMAGE_URLS  JSON 数组，多张参考图（最多 9 张）
  SORA_REFERENCE_VIDEO_URLS  JSON 数组，多个参考视频（最多 3 个）
  SORA_REFERENCE_AUDIO_URLS  JSON 数组，多个参考音频（最多 3 个）
  SORA_REFERENCE_TEXT        文本参考（角色设定/分镜/品牌规范等）
  SORA_FIRST_FRAME_URL       首帧图片 URL 或图片 data URL
  SORA_LAST_FRAME_URL        尾帧图片 URL 或图片 data URL
  SORA_SOURCE_VIDEO_URL      v2v 源视频 URL
  SORA_SOURCE_VIDEO_ID       v2v 源视频任务 ID
  SORA_EXTRA_JSON            额外 JSON 对象，合并进请求体（方便临时测试新字段）

注：本网关仅暴露 /v1/videos、/v1/videos/{id}、/v1/videos/{id}/content，
不提供 /v1/videos/edits；v2v 通过请求体的 source_video_url / source_video_id 走 /v1/videos。
"""

import json
import os
import sys
import time
import urllib.error
import urllib.request

# 实时输出进度（非 TTY 下 Python 默认块缓冲，会让轮询进度看不到）。
try:
    sys.stdout.reconfigure(line_buffering=True)
except Exception:
    pass

API_KEY = os.environ.get("SORA_API_KEY", "").strip()
BASE_URL = os.environ.get("SORA_BASE_URL", "https://www.cc-vibe.com").rstrip("/")
MODEL = os.environ.get("SORA_MODEL", "sora-v3-fast").strip()
RESOLUTION = os.environ.get("SORA_RESOLUTION", "480p").strip()
ASPECT = os.environ.get("SORA_ASPECT", "16:9").strip()

# 硬性限制为 5 秒（最短/最便宜档）。测试用脚本不允许生成更长视频以免浪费费用：
# 即使 SORA_SECONDS 设成 10/15，也会被钳到 5。
_MAX_SECONDS = 5
try:
    _req_seconds = int(float(os.environ.get("SORA_SECONDS", "5").strip()))
except ValueError:
    _req_seconds = _MAX_SECONDS
if _req_seconds < 1:
    _req_seconds = 1
if _req_seconds > _MAX_SECONDS:
    print(f"⚠ 测试脚本限制最长 {_MAX_SECONDS}s，已将 seconds={_req_seconds} 钳到 {_MAX_SECONDS}（省钱）")
    _req_seconds = _MAX_SECONDS
SECONDS = str(_req_seconds)
POLL_SEC = float(os.environ.get("SORA_POLL_SEC", "5"))
TIMEOUT = float(os.environ.get("SORA_TIMEOUT", "600"))
OUT = os.environ.get("SORA_OUT", "").strip()

PROMPT = sys.argv[1] if len(sys.argv) > 1 else "雨夜霓虹街道，镜头缓慢推进，电影感光影"


def _req(method, path, body=None, raw=False):
    """发起请求；raw=True 返回 (status, bytes)，否则返回 (status, dict)。"""
    url = BASE_URL + path
    data = json.dumps(body).encode("utf-8") if body is not None else None
    req = urllib.request.Request(url, data=data, method=method)
    req.add_header("Authorization", "Bearer " + API_KEY)
    if data is not None:
        req.add_header("Content-Type", "application/json")
    try:
        with urllib.request.urlopen(req, timeout=60) as resp:
            payload = resp.read()
            if raw:
                return resp.status, payload
            return resp.status, (json.loads(payload) if payload else {})
    except urllib.error.HTTPError as e:
        payload = e.read()
        if raw:
            return e.code, payload
        try:
            return e.code, json.loads(payload)
        except Exception:
            return e.code, {"_raw": payload.decode("utf-8", "replace")}


def _json_array_env(name):
    raw = os.environ.get(name, "").strip()
    if not raw:
        return None
    try:
        val = json.loads(raw)
    except Exception:
        sys.exit(f"✗ {name} 不是合法 JSON 数组")
    if not isinstance(val, list):
        sys.exit(f"✗ {name} 必须是 JSON 数组，例如 '[\"https://a.jpg\",\"https://b.jpg\"]'")
    return val


def optional_create_fields():
    """把可选的多模态/参考/首尾帧/v2v 字段从环境变量合入请求体。

    网关对 /v1/videos 是透传：这些字段会原样转发给上游，能否生效取决于上游是否支持。
    """
    fields = {}
    str_map = {
        "SORA_IMAGE_URL": "image_url",
        "SORA_REFERENCE_TEXT": "reference_text",
        "SORA_FIRST_FRAME_URL": "first_frame_url",
        "SORA_LAST_FRAME_URL": "last_frame_url",
        "SORA_SOURCE_VIDEO_URL": "source_video_url",
        "SORA_SOURCE_VIDEO_ID": "source_video_id",
    }
    for env, key in str_map.items():
        v = os.environ.get(env, "").strip()
        if v:
            fields[key] = v
    arr_map = {
        "SORA_REFERENCE_IMAGE_URLS": "reference_image_urls",
        "SORA_REFERENCE_VIDEO_URLS": "reference_video_urls",
        "SORA_REFERENCE_AUDIO_URLS": "reference_audio_urls",
    }
    for env, key in arr_map.items():
        arr = _json_array_env(env)
        if arr:
            fields[key] = arr
    extra = os.environ.get("SORA_EXTRA_JSON", "").strip()
    if extra:
        try:
            obj = json.loads(extra)
        except Exception:
            sys.exit("✗ SORA_EXTRA_JSON 不是合法 JSON")
        if not isinstance(obj, dict):
            sys.exit("✗ SORA_EXTRA_JSON 必须是 JSON 对象")
        fields.update(obj)  # 最后合并，可覆盖上面的字段
    return fields


def main():
    if not API_KEY:
        sys.exit("✗ 请先设置 SORA_API_KEY 环境变量")

    # 1) 创建任务
    create_body = {
        "model": MODEL,
        "prompt": PROMPT,
        "aspect_ratio": ASPECT,
        "resolution": RESOLUTION,
        "seconds": SECONDS,
    }
    extra_fields = optional_create_fields()
    create_body.update(extra_fields)

    extras_note = f" +{list(extra_fields)}" if extra_fields else ""
    print(f"→ 创建视频任务 model={MODEL} resolution={RESOLUTION} seconds={SECONDS}{extras_note}")
    status, resp = _req("POST", "/v1/videos", create_body)
    if status >= 400:
        sys.exit(f"✗ 创建失败 HTTP {status}: {json.dumps(resp, ensure_ascii=False)}")

    task_id = (resp.get("id") or "").strip()
    if not task_id:
        sys.exit(f"✗ 创建响应缺少 id: {json.dumps(resp, ensure_ascii=False)}")
    print(f"✓ 任务已创建 id={task_id} status={resp.get('status')}")

    # 2) 轮询状态
    deadline = time.time() + TIMEOUT
    final = None
    while time.time() < deadline:
        time.sleep(POLL_SEC)
        status, resp = _req("GET", f"/v1/videos/{task_id}")
        if status >= 400:
            print(f"  · 查询 HTTP {status}: {json.dumps(resp, ensure_ascii=False)}")
            continue
        st = (resp.get("status") or "").lower()
        progress = resp.get("progress", "")
        print(f"  · status={st} progress={progress}")
        if st in ("completed", "succeeded", "success"):
            final = resp
            break
        if st in ("failed", "error", "canceled", "cancelled"):
            sys.exit(f"✗ 任务失败: {json.dumps(resp, ensure_ascii=False)}")
    if final is None:
        sys.exit(f"✗ 超时（{TIMEOUT}s）未完成，task_id={task_id}")

    # 3) 下载内容（走本网关，自动路由到创建任务的账号）
    out = OUT or f"sora_{task_id}.mp4"
    print(f"→ 下载视频到 {out}")
    status, data = _req("GET", f"/v1/videos/{task_id}/content", raw=True)
    if status >= 400:
        sys.exit(f"✗ 下载失败 HTTP {status}: {data[:500].decode('utf-8', 'replace')}")
    with open(out, "wb") as f:
        f.write(data)
    print(f"✓ 完成，已保存 {out}（{len(data)} bytes）")
    if final.get("video_url"):
        print(f"  上游 video_url: {final['video_url']}")


if __name__ == "__main__":
    main()
