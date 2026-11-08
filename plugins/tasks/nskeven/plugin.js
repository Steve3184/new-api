export const meta = {
  apiVersion: 1,
  key: "nskeven",
  name: "Video Provider 2",
  icon: "text:VP",
  description: {
    en: "Video generation through a unified video API",
    zh: "通过统一视频接口生成视频",
  },
  version: "1.0.0",
  author: { name: "Video Provider 2" },
  baseUrl: "https://newapi.nskeven.uk",
  models: [
    "doubao-seedance-2-0-260128",
    "doubao-seedance-2-0-fast-260128",
    "doubao-seedance-2-0-mini-260615",
    "doubao-seedance-2-5-260628",
    "video-ultra-720p",
    "video-ultra-720p-fast",
    "video-ultra-720p-mini",
    "video-ultra2.5-720p",
    "even-sd2.0-A",
    "even-sd2.5-A",
    "even-sd2.5-B",
    "even--seedance-2.0-480p",
    "even--seedance-2.0-720p",
    "even--seedance-2.0-1080p",
    "even--seedance-2.0-4k",
    "sd-2-E4",
    "sd-2-E5",
    "sd-2-E6",
    "sd-2-E8",
    "sd-2.5-E1",
    "wan-3.0-1080",
    "even-sd-2.5-30s",
  ],
  fetchMode: "per_task",
  // The vendor keeps the vendor's own stable query path; the OpenAI-compatible
  // /v1/videos path answers with a per-model shape the vendor itself advises
  // against parsing, so every query goes through /v1/video/generations.
  routes: [
    { method: "POST", path: "/v1/video/generations", type: "submit", decode: "createVideo", render: "videoCreated" },
    { method: "GET", path: "/v1/video/generations/:task_id", type: "query", render: "videoStatus" },
  ],
  protocols: [{ name: "openai_responses", supports: ["stream", "sync", "background"] }, "openai_video"],
  usageSchema: {
    seconds: {
      type: "number",
      unit: "second",
      description: { en: "Video generation unit price", zh: "视频生成单价" },
    },
    resolution: {
      enum: ["480p", "720p", "1080p", "4k"],
      enumLabels: {
        "480p": { en: "480p", zh: "480p" },
        "720p": { en: "720p", zh: "720p" },
        "1080p": { en: "1080p", zh: "1080p" },
        "4k": { en: "4K", zh: "4K" },
      },
      description: { en: "Output video resolution", zh: "输出视频分辨率" },
    },
  },
  usageExamples: [
    { label: "720p · 8s", facts: { seconds: 8, resolution: "720p" } },
    { label: "1080p · 5s", facts: { seconds: 5, resolution: "1080p" } },
  ],
};

const RESOLUTIONS = ["480p", "720p", "1080p", "4k"];

// The vendor takes `seconds` as a string; a bare number is rejected with 400.
const DEFAULT_SECONDS = 4;
const MAX_SECONDS = 60;

function trimmed(value) {
  return String(value === undefined || value === null ? "" : value).trim();
}

// Doubao Seedance reads resolution, ratio and materials from `metadata`.
function isDoubao(model) {
  return model.indexOf("doubao-seedance-") === 0;
}

// The express line keeps images at the top level but moves reference video and
// audio into `metadata`.
function isExpressLine(model) {
  return model.indexOf("sd-2") === 0 || model.indexOf("wan-3") === 0;
}

function secondsOf(value) {
  const seconds = Number(value);
  if (!Number.isFinite(seconds) || seconds <= 0) return undefined;
  return Math.min(Math.round(seconds), MAX_SECONDS);
}

function stringList(value, field) {
  if (value === undefined || value === null) return [];
  if (!Array.isArray(value)) throw new Error(field + " must be an array");
  const out = [];
  for (const item of value) {
    const url = trimmed(item);
    if (!url) throw new Error(field + " entries must be non-empty strings");
    out.push(url);
  }
  return out;
}

// OpenAI video clients describe the output as `size`. The vendor wants a
// resolution bucket plus an aspect ratio, so translate once here.
function outputFromSize(size) {
  const match = /^(\d+)\s*x\s*(\d+)$/i.exec(trimmed(size));
  if (!match) return undefined;
  const width = Number(match[1]);
  const height = Number(match[2]);
  if (!width || !height) return undefined;
  const gcd = (a, b) => (b ? gcd(b, a % b) : a);
  const divisor = gcd(width, height);
  const shorter = Math.min(width, height);
  let resolution = "480p";
  if (shorter > 1080) resolution = "4k";
  else if (shorter > 720) resolution = "1080p";
  else if (shorter > 480) resolution = "720p";
  return { resolution, aspect_ratio: width / divisor + ":" + height / divisor };
}

function normalizeResolution(value, size) {
  const explicit = trimmed(value);
  if (explicit) {
    const lowered = explicit.toLowerCase();
    if (RESOLUTIONS.indexOf(lowered) === -1) throw new Error("resolution must be one of " + RESOLUTIONS.join(", "));
    return lowered;
  }
  return outputFromSize(size) === undefined ? undefined : outputFromSize(size).resolution;
}

function normalizeAspectRatio(value, size) {
  const explicit = trimmed(value);
  if (explicit) return explicit;
  const fromSize = outputFromSize(size);
  return fromSize === undefined ? undefined : fromSize.aspect_ratio;
}

// One canonical request body feeds buildSubmitRequest, so the OpenAI video
// surface and the vendor's native surface produce the same upstream call.
function canonicalBody(input, model) {
  const prompt = trimmed(input.prompt);
  if (!prompt) throw new Error("field prompt is required");
  const seconds = secondsOf(input.seconds === undefined ? input.duration : input.seconds) || DEFAULT_SECONDS;
  const body = { model: model, prompt: prompt, seconds: seconds };
  const resolution = normalizeResolution(input.resolution, input.size);
  if (resolution) body.resolution = resolution;
  const aspectRatio = normalizeAspectRatio(input.aspect_ratio || input.ratio, input.size);
  if (aspectRatio) body.aspect_ratio = aspectRatio;
  const images = stringList(input.images, "images");
  if (!images.length && trimmed(input.input_reference)) images.push(trimmed(input.input_reference));
  const videos = stringList(input.videos, "videos");
  const audios = stringList(input.audios, "audios");
  if (images.length) body.images = images;
  if (videos.length) body.videos = videos;
  if (audios.length) body.audios = audios;
  if (trimmed(input.first_frame_url)) body.first_frame_url = trimmed(input.first_frame_url);
  if (trimmed(input.last_frame_url)) body.last_frame_url = trimmed(input.last_frame_url);
  if (input.generate_audio !== undefined) body.generate_audio = Boolean(input.generate_audio);
  if (input.metadata !== undefined) {
    if (!input.metadata || typeof input.metadata !== "object" || Array.isArray(input.metadata)) throw new Error("metadata must be an object");
    body.metadata = input.metadata;
  }
  return body;
}

function vendorPayload(body) {
  const model = body.model;
  const payload = { model: model, prompt: body.prompt, seconds: String(body.seconds) };
  if (isDoubao(model)) {
    // Resolution is mandatory here, so fall back to the documented default
    // rather than letting the vendor reject a request the client could not
    // express in its own vocabulary.
    const metadata = Object.assign({}, body.metadata || {});
    metadata.resolution = body.resolution || "720p";
    if (body.aspect_ratio) metadata.ratio = body.aspect_ratio;
    const content = Array.isArray(metadata.content) ? metadata.content.slice() : [];
    for (const url of body.images || []) content.push({ type: "image_url", image_url: { url: url }, role: "reference_image" });
    if (body.first_frame_url) content.push({ type: "image_url", image_url: { url: body.first_frame_url }, role: "first_frame" });
    if (body.last_frame_url) content.push({ type: "image_url", image_url: { url: body.last_frame_url }, role: "last_frame" });
    for (const url of body.videos || []) content.push({ type: "video_url", video_url: { url: url }, role: "reference_video" });
    for (const url of body.audios || []) content.push({ type: "audio_url", audio_url: { url: url }, role: "reference_audio" });
    if (content.length) metadata.content = content;
    if (body.generate_audio !== undefined) metadata.generate_audio = body.generate_audio;
    payload.metadata = metadata;
    return payload;
  }
  if (body.resolution) payload.resolution = body.resolution;
  if (body.aspect_ratio) payload.aspect_ratio = body.aspect_ratio;
  if (body.images) payload.images = body.images;
  if (body.first_frame_url) payload.first_frame_url = body.first_frame_url;
  if (body.last_frame_url) payload.last_frame_url = body.last_frame_url;
  if (body.generate_audio !== undefined) payload.generate_audio = body.generate_audio;
  const metadata = Object.assign({}, body.metadata || {});
  if (isExpressLine(model)) {
    // Reference video and audio live inside metadata on this line.
    if (body.videos && !metadata.videos) metadata.videos = body.videos;
    if (body.audios && !metadata.audios) metadata.audios = body.audios;
  } else {
    if (body.videos) payload.videos = body.videos;
    if (body.audios) payload.audios = body.audios;
  }
  if (Object.keys(metadata).length) payload.metadata = metadata;
  return payload;
}

// Responses input: text parts become the prompt, image parts become references.
function responsesInput(req) {
  const texts = [];
  const images = [];
  const input = req.input;
  if (typeof input === "string") texts.push(input);
  else if (Array.isArray(input)) {
    for (const item of input) {
      if (typeof item === "string") {
        texts.push(item);
        continue;
      }
      if (!item || typeof item !== "object" || Array.isArray(item)) continue;
      const content = item.content === undefined ? [item] : Array.isArray(item.content) ? item.content : [item.content];
      for (const part of content) {
        if (typeof part === "string") {
          texts.push(part);
          continue;
        }
        if (!part || typeof part !== "object" || Array.isArray(part)) continue;
        if ((part.type === "input_text" || part.type === "text") && typeof part.text === "string") texts.push(part.text);
        if (part.type === "input_image" || part.type === "image_url") {
          let image = part.image_url;
          if (image && typeof image === "object") image = image.url;
          if (trimmed(image)) images.push(trimmed(image));
        }
      }
    }
  }
  return { prompt: texts.filter((text) => trimmed(text)).join("\n"), images: images };
}

function jsonObject(kind) {
  if (!kind || kind.kind !== "json") throw new Error("JSON body required");
  const value = kind.value;
  if (!value || typeof value !== "object" || Array.isArray(value)) throw new Error("request body must be an object");
  return value;
}

export function buildSubmitRequest(ctx) {
  const body = vendorPayload(ctx.requestBody || {});
  return {
    url: ctx.baseUrl + "/v1/video/generations",
    method: "POST",
    headers: { Authorization: "Bearer " + ctx.apiKey, "Content-Type": "application/json" },
    body: body,
  };
}

export function parseSubmitResponse(ctx, response) {
  const body = response.body || {};
  const taskId = trimmed(body.task_id) || trimmed(body.id);
  if (!taskId) throw new Error("task_id is empty");
  return { taskId: taskId, taskData: body };
}

export function extractUsage(ctx) {
  const body = ctx.requestBody || {};
  const facts = {};
  const seconds = secondsOf(body.seconds);
  if (seconds) facts.seconds = seconds;
  if (RESOLUTIONS.indexOf(trimmed(body.resolution)) !== -1) facts.resolution = trimmed(body.resolution);
  return facts;
}

// The query response does not repeat the requested duration or resolution, so
// the submission facts stay in force and only a reported change is applied.
export function extractUsageOnComplete(task, result, data) {
  const payload = data && typeof data === "object" ? (data.data && typeof data.data === "object" ? data.data : data) : null;
  if (!payload) return {};
  const facts = {};
  const seconds = secondsOf(payload.seconds || payload.duration);
  if (seconds) facts.seconds = seconds;
  const resolution = trimmed(payload.resolution || (payload.metadata && payload.metadata.resolution));
  if (RESOLUTIONS.indexOf(resolution) !== -1) facts.resolution = resolution;
  return facts;
}

export function buildQueryRequest(ctx) {
  return {
    url: ctx.baseUrl + "/v1/video/generations/" + encodeURIComponent(ctx.taskId),
    method: "GET",
    headers: { Authorization: "Bearer " + ctx.apiKey },
  };
}

// The stable envelope: {code, message, data:{task_id, status, progress, fail_reason, ...}}.
// Only `data.status` decides the outcome; `progress` and `fail_reason` are
// display and diagnostics.
export function parseTaskResult(ctx, body) {
  const envelope = body && typeof body === "object" ? body : {};
  const payload = envelope.data && typeof envelope.data === "object" ? envelope.data : null;
  if (!payload) return { status: "UNKNOWN", reason: "response has no data object" };
  if (envelope.code !== undefined && envelope.code !== "success") {
    return { status: "UNKNOWN", reason: trimmed(envelope.message) || "unrecognized response code" };
  }
  const status = trimmed(payload.status).toUpperCase();
  const known = ["NOT_START", "SUBMITTED", "QUEUED", "IN_PROGRESS", "SUCCESS", "FAILURE", "UNKNOWN"];
  if (known.indexOf(status) === -1) return { status: "UNKNOWN", reason: "unrecognized status: " + status };
  const result = { status: status === "UNKNOWN" ? "UNKNOWN" : status };
  const progress = trimmed(payload.progress);
  if (progress) result.progress = progress;
  if (status === "FAILURE") result.reason = trimmed(payload.fail_reason) || "task failed";
  return result;
}

export function listArtifacts(task) {
  if (task.status !== "SUCCESS") return [];
  return [{ key: "video", type: "video" }];
}

export function buildContentRequest(ctx) {
  if (ctx.artifactKey !== "video") throw new Error("artifact_not_found");
  return {
    url: ctx.baseUrl + "/v1/videos/" + encodeURIComponent(ctx.upstreamTaskId) + "/content",
    method: ctx.clientRequest.method,
    headers: { Authorization: "Bearer " + ctx.apiKey },
  };
}

const VIDEO_STATUS = {
  NOT_START: "queued",
  SUBMITTED: "queued",
  QUEUED: "queued",
  IN_PROGRESS: "in_progress",
  SUCCESS: "completed",
  FAILURE: "failed",
};

function progressPercent(value) {
  const number = Number(trimmed(value).replace("%", ""));
  if (!Number.isFinite(number) || number < 0 || number > 100) return 0;
  return Math.round(number);
}

// task.data holds the last upstream snapshot; the vendor envelope is nested
// under `data`, while the submit receipt is flat.
function taskPayload(task) {
  const data = task.data;
  if (data && typeof data === "object") {
    if (data.data && typeof data.data === "object") return data.data;
    return data;
  }
  return {};
}

export const native = {
  createVideo(ctx) {
    const body = jsonObject(ctx.body);
    const model = trimmed(body.model);
    if (!model) throw new Error("model is required");
    return { kind: "submit", model: model, requestBody: canonicalBody(body, model) };
  },
  videoCreated(ctx, task) {
    return {
      id: task.task_id,
      task_id: task.task_id,
      object: "video",
      status: "queued",
      created_at: Number(task.created_at || 0),
    };
  },
  // Client-facing fields never carry a vendor address: the query snapshot holds
  // the vendor's signed result_url, which is useless to a client and would leak
  // the upstream, so a finished task only answers with the gateway artifact URL.
  videoStatus(ctx, task) {
    const payload = taskPayload(task);
    const status = trimmed(task.status).toUpperCase();
    const artifact = ctx && ctx.artifacts && ctx.artifacts.video;
    return {
      code: "success",
      message: "",
      data: {
        id: payload.id === undefined ? null : payload.id,
        task_id: task.task_id,
        status: status,
        progress: trimmed(payload.progress) || (status === "SUCCESS" ? "100%" : ""),
        fail_reason: trimmed(payload.fail_reason) || trimmed(task.fail_reason),
        result_url: trimmed(artifact && artifact.url),
        created_at: Number(task.created_at || 0),
        finish_time: status === "SUCCESS" || status === "FAILURE" ? Number(task.updated_at || 0) : 0,
      },
    };
  },
};

// The Responses surface renders the finished video as an HTML video tag, the
// same shape the other video plugins answer with.
function videoTag(ctx) {
  const artifact = ctx && ctx.artifacts && ctx.artifacts.video;
  const url = trimmed(artifact && artifact.url);
  if (!url) throw new Error("video artifact is unavailable");
  const escaped = url.replace(/&/g, "&amp;").replace(/"/g, "&quot;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
  return '<video controls src="' + escaped + '"></video>';
}

export const protocols = {
  openai_responses: {
    decodeRequest(ctx) {
      const input = jsonObject(ctx.body);
      if (input.input !== undefined && typeof input.input !== "string" && !Array.isArray(input.input)) {
        throw new Error("input must be a string or array");
      }
      const parsed = responsesInput(input);
      const prompt = parsed.prompt || trimmed(input.prompt);
      if (!prompt) throw new Error("input is required");
      const requestBody = canonicalBody(
        {
          prompt: prompt,
          images: parsed.images,
          seconds: input.seconds === undefined ? input.duration : input.seconds,
          size: input.size,
          resolution: input.resolution,
          aspect_ratio: input.aspect_ratio,
        },
        ctx.upstreamModel || ctx.model
      );
      return {
        kind: "submit",
        model: ctx.model,
        action: requestBody.images && requestBody.images.length ? "image_to_video" : "text_to_video",
        requestBody: requestBody,
      };
    },
    renderEvents(ctx, task, previousState) {
      const status = trimmed(task.status).toUpperCase();
      const progress = progressPercent(task.progress);
      const state = { status: status, progress: progress };
      if (status === "SUCCESS") {
        const events = previousState && previousState.status === status ? [] : [{ type: "output", data: videoTag(ctx) }];
        return { events: events, state: state, done: true };
      }
      if (status === "FAILURE") {
        return {
          events: [{ type: "error", code: "task_failed", message: trimmed(task.fail_reason) || "task failed" }],
          state: state,
          done: true,
        };
      }
      if (previousState && previousState.status === status && previousState.progress === progress) {
        return { events: [], state: state, done: false };
      }
      return { events: [{ type: "progress", message: status.toLowerCase(), progress: progress }], state: state, done: false };
    },
    renderFinal(ctx) {
      return {
        output: [
          {
            type: "message",
            status: "completed",
            role: "assistant",
            content: [{ type: "output_text", text: videoTag(ctx), annotations: [], logprobs: [] }],
          },
        ],
        metadata: { vendor: "video_provider_2" },
      };
    },
  },
  openai_video: {
    decodeRequest(ctx) {
      const input = jsonObject(ctx.body);
      return {
        kind: "submit",
        model: ctx.model,
        action: input.input_reference || (Array.isArray(input.images) && input.images.length) ? "image_to_video" : "text_to_video",
        requestBody: canonicalBody(input, ctx.upstreamModel || ctx.model),
      };
    },
    // No URL is rendered: a client fetches the video from the gateway content
    // route, so the vendor's address never appears in the response.
    render(ctx, task) {
      const status = trimmed(task.status).toUpperCase();
      const output = {
        id: task.task_id,
        object: "video",
        model: (task.properties || {}).origin_model_name || "",
        status: VIDEO_STATUS[status] || "unknown",
        progress: progressPercent(task.progress),
        created_at: Number(task.created_at || 0),
      };
      const completedAt = Number(task.finished_at || task.updated_at || 0);
      if (completedAt > 0 && (status === "SUCCESS" || status === "FAILURE")) output.completed_at = completedAt;
      if (status === "FAILURE") {
        output.error = { code: "video_generation_failed", message: trimmed(task.fail_reason) || "The video generation task failed." };
      }
      return output;
    },
  },
};
