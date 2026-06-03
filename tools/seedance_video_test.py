#!/usr/bin/env python3
"""
Seedance 2.0 视频生成 - 异步任务调用脚本（**按次计费**，无第三方依赖，仅标准库）

适用模型（按次计费，时长不影响费用）：
  - seedance-2.0-fast-pass（720p，快速版）
  - seedance-2.0-pass（720p，标准版）

流程（异步任务）：
  1. POST /v1/videos              创建任务，拿到 task id
  2. GET  /v1/videos/{id}         轮询状态，直到 completed / failed
  3. 下载 mp4：优先走网关 /v1/videos/{id}/content；该端点不支持 API Key 下载时，
     回退直连完成响应里的 video_url（OSS/CDN 直链）。

用法：
  export SEEDANCE_API_KEY="sk-你的key"               # 必填：本系统某个「视频分组」下的 API Key
  export SEEDANCE_BASE_URL="https://www.cc-vibe.com"  # 可选，默认公网网关；本地 dev 用 http://127.0.0.1:8080
  python3 tools/seedance_video_test.py "一只橘猫在阳光草地上奔跑，低角度跟拍，电影感"

  # 参考图（最多 4 张）
  export SEEDANCE_REFERENCE_IMAGE_URLS='["https://example.com/a.jpg","https://example.com/b.jpg"]'
  # 首尾帧（须成对，且不能与参考图/参考视频同用）
  export SEEDANCE_FIRST_IMAGE="https://example.com/start.jpg"
  export SEEDANCE_LAST_IMAGE="https://example.com/end.jpg"

参数（环境变量；每个都可用 SEEDANCE_ 前缀，缺省时回退同名 SORA_ 前缀，方便沿用旧习惯）：
  SEEDANCE_API_KEY     必填，Bearer 鉴权用
  SEEDANCE_BASE_URL    默认 https://www.cc-vibe.com（本网关，不是上游中转）
  SEEDANCE_MODEL       默认 seedance-2.0-fast-pass
  SEEDANCE_RESOLUTION  默认 720p
  SEEDANCE_RATIO       默认 16:9（同时作为 ratio 与 aspect_ratio 发送，二者等价）
  SEEDANCE_SECONDS     默认 5；同时作为 duration(整数) 与 seconds(字符串) 发送。Seedance 支持 4/5/10/15
  SEEDANCE_MAX_SECONDS 时长上限，默认 15（按次计费时长不影响费用，可放开测全长）
  SEEDANCE_FIRST_IMAGE 首帧 URL（字段 first_image，须与 last_image 成对）
  SEEDANCE_LAST_IMAGE  尾帧 URL（字段 last_image，须与 first_image 成对）
  SEEDANCE_REFERENCE_IMAGE_URLS  JSON 数组，参考图（字段 referenceImages，最多 4 张）
  SEEDANCE_REFERENCE_VIDEO_URLS  JSON 数组，参考视频（字段 referenceVideos，最多 3 个）
  SEEDANCE_POLL_SEC    轮询间隔秒，默认 10（官方建议 30-60s，避免高频查询）
  SEEDANCE_TIMEOUT     最长等待秒，默认 600
  SEEDANCE_OUT         输出文件名，默认 video_<task_id>.mp4
  SEEDANCE_EXTRA_JSON  额外 JSON 对象，合并进请求体（覆盖上面字段，方便临时测试新字段）

注：
  - Seedance 当前不支持参考音频；首尾帧模式不能与参考图/参考视频同时使用。
  - 网关对 /v1/videos 透传：以上字段会原样转发上游。
"""

import json
import os
import sys
import time
import urllib.error
import urllib.request

try:
    sys.stdout.reconfigure(line_buffering=True)
except Exception:
    pass


def env(name, default=""):
    """读取 SEEDANCE_<name>，缺省回退 SORA_<name>，再缺省用 default。"""
    v = os.environ.get("SEEDANCE_" + name)
    if v is None or v.strip() == "":
        v = os.environ.get("SORA_" + name)
    return (v if v is not None else default).strip()


API_KEY = env("API_KEY")
BASE_URL = (env("BASE_URL", "https://www.cc-vibe.com")).rstrip("/")
MODEL = env("MODEL", "seedance-2.0-fast-pass")
RESOLUTION = env("RESOLUTION", "720p")
RATIO = env("RATIO", "16:9")

# 按次计费，时长不影响费用；上限默认放开到 15（Seedance 支持 4/5/10/15）。
try:
    _MAX_SECONDS = int(float(env("MAX_SECONDS", "15")))
except ValueError:
    _MAX_SECONDS = 15
if _MAX_SECONDS < 1:
    _MAX_SECONDS = 1
try:
    _req_seconds = int(float(env("SECONDS", "5")))
except ValueError:
    _req_seconds = 5
if _req_seconds < 1:
    _req_seconds = 1
if _req_seconds > _MAX_SECONDS:
    print(f"⚠ 时长上限 {_MAX_SECONDS}s，已将 {_req_seconds} 钳到 {_MAX_SECONDS}（可调 SEEDANCE_MAX_SECONDS）")
    _req_seconds = _MAX_SECONDS
SECONDS = str(_req_seconds)
DURATION = _req_seconds
POLL_SEC = float(env("POLL_SEC", "10"))
TIMEOUT = float(env("TIMEOUT", "600"))
OUT = env("OUT")

PROMPT = sys.argv[1] if len(sys.argv) > 1 else "一只橘猫在阳光草地上奔跑，低角度跟拍，电影感"


def _req(method, path, body=None, raw=False):
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
    raw = env(name)
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
    """参考图/参考视频/首尾帧字段（Seedance 命名），透传上游。"""
    fields = {}
    first_image = env("FIRST_IMAGE")
    last_image = env("LAST_IMAGE")
    if first_image:
        fields["first_image"] = first_image
    if last_image:
        fields["last_image"] = last_image
    ref_images = _json_array_env("REFERENCE_IMAGE_URLS")
    if ref_images:
        if len(ref_images) > 4:
            print(f"⚠ Seedance 参考图最多 4 张，已传 {len(ref_images)} 张（上游可能截断/报错）")
        fields["referenceImages"] = ref_images
    ref_videos = _json_array_env("REFERENCE_VIDEO_URLS")
    if ref_videos:
        if len(ref_videos) > 3:
            print(f"⚠ Seedance 参考视频最多 3 个，已传 {len(ref_videos)} 个")
        fields["referenceVideos"] = ref_videos
    if (first_image or last_image) and (ref_images or ref_videos):
        print("⚠ 首尾帧模式不能与参考图/参考视频同用，上游可能报错")
    extra = env("EXTRA_JSON")
    if extra:
        try:
            obj = json.loads(extra)
        except Exception:
            sys.exit("✗ SEEDANCE_EXTRA_JSON 不是合法 JSON")
        if not isinstance(obj, dict):
            sys.exit("✗ SEEDANCE_EXTRA_JSON 必须是 JSON 对象")
        fields.update(obj)
    return fields


def main():
    if not API_KEY:
        sys.exit("✗ 请先设置 SEEDANCE_API_KEY 环境变量")

    # 1) 创建任务。duration/seconds、ratio/aspect_ratio 各发等价两写法兼容上游。
    create_body = {
        "model": MODEL,
        "prompt": PROMPT,
        "ratio": RATIO,
        "aspect_ratio": RATIO,
        "resolution": RESOLUTION,
        "duration": DURATION,
        "seconds": SECONDS,
    }
    extra_fields = optional_create_fields()
    create_body.update(extra_fields)

    extras_note = f" +{list(extra_fields)}" if extra_fields else ""
    print(f"→ 创建视频任务 model={MODEL} resolution={RESOLUTION} duration={DURATION} ratio={RATIO}{extras_note}")
    status, resp = _req("POST", "/v1/videos", create_body)
    if status >= 400:
        sys.exit(f"✗ 创建失败 HTTP {status}: {json.dumps(resp, ensure_ascii=False)}")

    task_id = (resp.get("id") or resp.get("task_id") or "").strip()
    if not task_id:
        sys.exit(f"✗ 创建响应缺少 id/task_id: {json.dumps(resp, ensure_ascii=False)}")
    print(f"✓ 任务已创建 id={task_id} status={resp.get('status')}（按次计费，已扣一次费）")

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
        print(f"  · status={st} progress={resp.get('progress', '')}")
        if st in ("completed", "succeeded", "success"):
            final = resp
            break
        if st in ("failed", "error", "canceled", "cancelled"):
            sys.exit(f"✗ 任务失败: {json.dumps(resp, ensure_ascii=False)}")
    if final is None:
        sys.exit(f"✗ 超时（{TIMEOUT}s）未完成，task_id={task_id}")

    # 3) 下载内容：先试网关 /content，失败则回退直链 video_url。
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
