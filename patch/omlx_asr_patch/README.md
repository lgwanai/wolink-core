# oMLX ASR Char-Level Info Patch

## 📌 说明
这个补丁用于修改 macOS 本地安装的 `oMLX.app` 应用程序包内部的 Python 源码，以支持在 ASR (语音转文本) 请求时，返回基于模型对齐的逐字时间戳（`char_level_info`）。

默认情况下，`oMLX.app` 运行的 `omlx.server` 的 `/v1/audio/transcriptions` 接口不支持接收 `forced_aligner` 参数，并且返回的 JSON 结构中不包含字级别的时间戳信息。此补丁通过修改它的路由和推理引擎，在文本生成完成后无缝追加调用强制对齐模型 (Forced Aligner)，从而补全精准的时间戳数据。

## 📂 目标修改文件
oMLX 默认安装在 macOS 的 Applications 目录下，其 Python 源码资源位于：
`/Applications/oMLX.app/Contents/Resources/omlx`

本补丁对应并替换了上述目录下的以下三个核心文件：
1. `api/audio_models.py` - 增加了 `char_level_info` 响应字段。
2. `api/audio_routes.py` - 修改了 FastAPI 的路由接口，以支持接收并透传前端传来的 `forced_aligner` 参数。
3. `engine/stt.py` - 在 `_transcribe_sync` 生成结束后，追加了调用 Aligner 模型的逻辑，并将生成的 items 映射并追加到响应字典中。

## 🚀 如何使用 (oMLX 更新后如何重新打补丁)
当你更新了 `oMLX.app` 客户端版本后，这些底层的 Python 源码会被官方新版本重置覆盖。此时你只需运行此目录下的 shell 脚本，即可轻松重新打上补丁：

```bash
cd patch/omlx_asr_patch
./apply_patch.sh
```

脚本执行完毕后，**请务必彻底退出 oMLX 客户端并重新启动**（或者重启你终端里的 omlx.server 进程），以使被修改后的 Python 代码生效。

> **安全提示**：`apply_patch.sh` 在执行替换前，会自动为你原始的系统文件创建 `.bak` 备份。