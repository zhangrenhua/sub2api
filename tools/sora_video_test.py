#!/usr/bin/env python3
"""
视频生成 - 异步任务调用脚本（Sora / Seedance 2.0 通用，无第三方依赖，仅标准库）

流程（异步任务）：
  1. POST /v1/videos              创建任务，拿到 task id
  2. GET  /v1/videos/{id}         轮询状态，直到 completed / failed
  3. GET  /v1/videos/{id}/content 下载 mp4（走本网关，自动落到创建任务的同一账号）

适用模型：
  - Sora：sora-v3-fast(480p) / sora-v3-pro(720p) 等，按秒计费。
  - Seedance 2.0：seedance-2.0-fast-pass / seedance-2.0-pass，按次计费（时长不影响费用）。
  网关对 /v1/videos 是「透传」，下面所有字段会原样转发上游，能否生效取决于上游。

用法：
  export SORA_API_KEY="sk-你的key"               # 必填：本系统某个「视频分组」下的 API Key
  export SORA_BASE_URL="https://www.cc-vibe.com"  # 可选，默认公网网关；本地 dev 用 http://127.0.0.1:8080
  export SORA_MODEL="sora-v3-fast"                # 可选
  python3 tools/sora_video_test.py "雨夜霓虹街道，镜头缓慢推进，电影感光影"

  # Seedance 2.0 示例（按次计费，时长不加钱）：
  SORA_MODEL="seedance-2.0-fast-pass" SORA_RESOLUTION="720p" SORA_ASPECT="16:9" \
  SORA_MAX_SECONDS=10 SORA_SECONDS=10 \
    python3 tools/sora_video_test.py "一只橘猫在阳光草地上奔跑，低角度跟拍，电影感"

参数（环境变量，除 SORA_API_KEY 外均可选）：
  SORA_API_KEY    必填，Bearer 鉴权用
  SORA_BASE_URL   默认 https://www.cc-vibe.com（本网关，不是上游中转）
  SORA_MODEL      默认 sora-v3-fast
  SORA_RESOLUTION 默认 480p（如 480p/720p/1080p；按秒计费时 >=1080 走高清档）
  SORA_ASPECT     默认 16:9；同时作为 aspect_ratio 与 ratio 发送（两者等价）
  SORA_SECONDS    默认 5；同时作为 seconds(字符串) 与 duration(整数) 发送（两者等价）
  SORA_MAX_SECONDS 时长上限，默认 5（省钱）。按秒计费模型务必小心；
                  Seedance 等「按次」模型时长不影响费用，可设为 10/15 测全长。
  SORA_IMAGE_URL  可选，主参考图（HTTPS URL 或图片 data URL）；传了即图生视频
  SORA_POLL_SEC   轮询间隔秒，默认 5
  SORA_TIMEOUT    最长等待秒，默认 600
  SORA_OUT        输出文件名，默认 video_<task_id>.mp4

可选 参考 / 首尾帧 / v2v 字段（透传给上游，能否生效取决于上游是否支持）：
  SORA_FIRST_IMAGE           Seedance 首帧图片 URL（字段 first_image，需与 last_image 成对）
  SORA_LAST_IMAGE            Seedance 尾帧图片 URL（字段 last_image，需与 first_image 成对）
  SORA_FIRST_FRAME_URL       Sora 首帧（字段 first_frame_url）
  SORA_LAST_FRAME_URL        Sora 尾帧（字段 last_frame_url）
  SORA_REFERENCE_IMAGE_URLS  JSON 数组，多张参考图（字段 reference_image_urls，
                             Seedance 也接受该字段名，等价 referenceImages，最多 4 张）
  SORA_REFERENCE_VIDEO_URLS  JSON 数组，多个参考视频（字段 reference_video_urls，
                             Seedance 等价 referenceVideos，最多 3 个）
  SORA_REFERENCE_AUDIO_URLS  JSON 数组，多个参考音频（字段 reference_audio_urls；Seedance 暂不支持音频）
  SORA_REFERENCE_TEXT        文本参考（角色设定/分镜/品牌规范等）
  SORA_SOURCE_VIDEO_URL      v2v 源视频 URL
  SORA_SOURCE_VIDEO_ID       v2v 源视频任务 ID
  SORA_EXTRA_JSON            额外 JSON 对象，合并进请求体（覆盖上面字段，方便临时测试新字段）

注：
  - 网关仅暴露 /v1/videos、/v1/videos/{id}、/v1/videos/{id}/content，不提供 /v1/videos/edits；
    v2v 通过请求体的 source_video_url / source_video_id 走 /v1/videos。
  - Seedance 首尾帧模式（first_image+last_image）不能与参考图/参考视频同时使用。
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

# 时长上限默认 5 秒（最短/最便宜档），避免按秒计费模型浪费费用。
# 按次计费模型（Seedance 2.0 等）时长不影响费用，可用 SORA_MAX_SECONDS=10/15 测全长。
try:
    _MAX_SECONDS = int(float(os.environ.get("SORA_MAX_SECONDS", "5").strip()))
except ValueError:
    _MAX_SECONDS = 5
if _MAX_SECONDS < 1:
    _MAX_SECONDS = 1
try:
    _req_seconds = int(float(os.environ.get("SORA_SECONDS", "5").strip()))
except ValueError:
    _req_seconds = _MAX_SECONDS
if _req_seconds < 1:
    _req_seconds = 1
if _req_seconds > _MAX_SECONDS:
    print(f"⚠ 时长上限 {_MAX_SECONDS}s，已将 seconds={_req_seconds} 钳到 {_MAX_SECONDS}"
          f"（如为按次计费模型可设 SORA_MAX_SECONDS 放开）")
    _req_seconds = _MAX_SECONDS
SECONDS = str(_req_seconds)          # seconds 字段（字符串，Sora 习惯）
DURATION = _req_seconds              # duration 字段（整数，Seedance 习惯）
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
    """把可选的参考/首尾帧/v2v 字段从环境变量合入请求体。

    网关对 /v1/videos 是透传：这些字段会原样转发给上游，能否生效取决于上游是否支持。
    Sora（first_frame_url/last_frame_url）与 Seedance（first_image/last_image）首尾帧
    字段名不同，分别提供独立环境变量，按目标上游选用。
    """
    fields = {}
    str_map = {
        "SORA_IMAGE_URL": "image_url",
        "SORA_REFERENCE_TEXT": "reference_text",
        "SORA_FIRST_FRAME_URL": "first_frame_url",  # Sora 首帧
        "SORA_LAST_FRAME_URL": "last_frame_url",    # Sora 尾帧
        "SORA_FIRST_IMAGE": "first_image",          # Seedance 2.0 首帧
        "SORA_LAST_IMAGE": "last_image",            # Seedance 2.0 尾帧
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

    # 1) 创建任务。aspect_ratio/ratio 与 seconds/duration 各发两种等价写法，
    #    兼容 Sora 与 Seedance 2.0 两种上游字段约定。
    create_body = {
        "model": MODEL,
        "prompt": PROMPT,
        "aspect_ratio": ASPECT,
        "ratio": ASPECT,
        "resolution": RESOLUTION,
        "seconds": SECONDS,
        "duration": DURATION,
    }
    extra_fields = optional_create_fields()
    create_body.update(extra_fields)

    extras_note = f" +{list(extra_fields)}" if extra_fields else ""
    print(f"→ 创建视频任务 model={MODEL} resolution={RESOLUTION} "
          f"duration={DURATION} ratio={ASPECT}{extras_note}")
    status, resp = _req("POST", "/v1/videos", create_body)
    if status >= 400:
        sys.exit(f"✗ 创建失败 HTTP {status}: {json.dumps(resp, ensure_ascii=False)}")

    # Seedance 返回 id 与 task_id（同值）；Sora 返回 id。两者都兼容。
    task_id = (resp.get("id") or resp.get("task_id") or "").strip()
    if not task_id:
        sys.exit(f"✗ 创建响应缺少 id/task_id: {json.dumps(resp, ensure_ascii=False)}")
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

    # 3) 下载内容。优先走本网关 /content（落到原账号、带鉴权）；
    #    部分上游中转（如 Seedance 中转）的 /content 不支持 API Key 下载（401 login required），
    #    但完成响应里给了直链 video_url（OSS/CDN），此时回退直连该直链。
    out = OUT or f"video_{task_id}.mp4"
    video_url = (final.get("video_url") or final.get("url") or "").strip()
    print(f"→ 下载视频到 {out}（先试网关 /content）")
    status, body = _req("GET", f"/v1/videos/{task_id}/content", raw=True)
    data = None
    if status < 400 and body:
        data = body
    else:
        snippet = body[:200].decode("utf-8", "replace") if body else ""
        print(f"  · 网关 /content HTTP {status}（{snippet}），回退直链 video_url")
        if video_url.startswith(("http://", "https://")):
            try:
                with urllib.request.urlopen(video_url, timeout=120) as r:
                    data = r.read()
            except Exception as e:
                sys.exit(f"✗ 直链下载失败: {e}")
        else:
            sys.exit(f"✗ 下载失败：网关 /content HTTP {status}，且完成响应无可用 video_url 直链")
    with open(out, "wb") as f:
        f.write(data)
    print(f"✓ 完成，已保存 {out}（{len(data)} bytes）")
    if video_url:
        print(f"  video_url: {video_url}")


if __name__ == "__main__":
    main()
