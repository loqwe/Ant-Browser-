# bat

> 脚本入口运行于 Windows。`publish.bat` 支持 Windows 打包，也可通过 Docker Desktop 调用 Linux 发布脚本。

## 用途

- `dev.bat`：本地开发启动
- `build.bat`：本地构建可执行文件
- `publish.bat`：发布打包入口（Windows / Linux / 两者）

## 用法

### `dev.bat`

适合日常开发。

```bat
bat\dev.bat
```

说明：

- 默认优先使用 `5218` 作为前端开发端口
- 如果发现同项目残留的 `dev-watcher / vite` 进程，会先自动清理
- 如果 `5218` 被其他程序占用，会自动切换到下一个可用端口，并把该端口同步传给 Vite 和 Wails

### `build.bat`

构建 `build\bin\ant-chrome.exe`。

```bat
bat\build.bat
```

说明：

- 开发分支默认按完整源码构建
- 若缺少 `go.mod`、`main.go`、`wails.json` 等核心入口文件，脚本会直接失败，避免复用旧产物掩盖问题

### `publish.bat`

发布打包入口，启动后会提示选择：

- `W`：仅 Windows
- `L`：仅 Linux（通过 Docker Desktop）
- `B`：Windows + Linux

```bat
bat\publish.bat
```

也支持无交互参数（适合脚本调用）：

```bat
bat\publish.bat W
bat\publish.bat L
bat\publish.bat B
bat\publish.bat W -Version 1.1.0
bat\publish.bat B -Version 1.1.0
```

说明：

- `-Version 1.1.0` 会覆盖本次发布使用的版本号。
- Windows / Linux 包名、NSIS 安装包版本号，以及本次构建期间读取到的 `wails.json productVersion` 会统一使用该值。

Windows 打包依赖 NSIS，默认查找顺序：

```text
MAKENSIS_PATH -> 直接指向 makensis.exe
NSIS_PATH     -> NSIS 目录或 makensis.exe
NSIS_HOME     -> NSIS 安装目录
PATH          -> where makensis.exe
```

默认兜底目录：

```text
C:\Program Files (x86)\NSIS\makensis.exe
C:\Program Files\NSIS\makensis.exe
```

Windows 分支使用的项目路径：

```text
输入：
- build\bin\ant-chrome.exe
- publish\config.init.yaml
- bin\xray.exe
- bin\sing-box.exe

临时目录：
- publish\staging\

输出：
- publish\output\AntBrowser-Setup-<version>.exe
```

说明：

- Windows 安装包包含应用本体、默认配置和代理运行时。
- 如果 `chrome\` 根目录或其一级子目录中检测到有效的 Windows `chrome.exe`，会自动一起打进 EXE 安装包。
- 如果未检测到 Windows 内核，安装包仍会保留 `chrome\README.md` 说明文件。

Linux 分支会通过 Docker Desktop 调用：

```text
docker build -f publish/linux/linux-builder.Dockerfile -t ant-browser-linux-builder:local publish/linux
docker run --rm -v <repo>:/workspace -w /workspace ant-browser-linux-builder:local ^
  bash -c "bash publish/linux/publish-linux.sh --arch <Docker当前架构>"
```

要求：Docker Desktop 已安装并启动，且 Linux 容器引擎可用。

Linux 产物输出目录：

```text
publish\output\
```

常用环境变量：

```text
NO_PAUSE=1  -> 运行结束不 pause（适合 CI 或脚本调用）
CI=1        -> 同样不 pause
```

Windows 产物：

```text
publish\output\AntBrowser-Setup-<version>.exe
```

## 备注

- `generate-bindings.bat` 是辅助脚本，通常由 `build.bat` 调用。
- `generate-bindings.bat`、`build.bat`、`dev.bat` 都假定当前分支是完整源码仓库。
- 如果这些脚本报告缺少 `go.mod`、`main.go`、`wails.json`，应先恢复源码入口，而不是继续复用旧二进制。
