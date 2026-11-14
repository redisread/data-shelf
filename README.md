# data-shelf

使用 [mygo](https://mygo.run/zh) 开发的 macOS 原生桌面应用，用文件夹整理 JSON 日志和 Markdown 笔记，为选中的原文添加高亮和评论。所有资料保存在本机，不需要账户、服务端或同步。

## 使用

打开 `build/darwin-arm64/data-shelf.app`，或通过同目录的 DMG 安装。

- 左侧「＋ 文件」「＋ 文件夹」直接输入名称，Enter 创建、Escape 取消。
- 支持嵌套文件夹、文件名筛选、多标签；右键可改名或移到废纸篓，拖动条目到文件夹移动。
- 将文件或文件夹拖入窗口，或点击导入按钮，复制到当前分类；同名文件不会被覆盖。
- 粘贴 JSON 自动识别。没有扩展名的合法 JSON 在自动保存后补 `.json`；显式 `.md`、`.json` 扩展名优先。工具栏可手动选择格式。
- JSON 格式化保留数字精度、字段顺序及重复键；非法 JSON 原文仍可保存，并显示错误。
- 选中文字后点击「＋ 评论」。评论可编辑、删除，点击「定位原文」跳回选区；修改覆盖原文后转为「待重新定位」，可关联新的选区。
- Markdown 支持源码、预览、分栏。评论从源码创建；预览支持标题、列表、表格、代码块、链接。远程图片不会自动加载。
- 内容约每 1.2 秒自动保存；⌘S 立即保存，正常关闭/退出前保存。
- Finder 修改文件后自动刷新；有未保存编辑时暂停保存，提供本地副本和外部原文，两份均保留。

快捷键：⌘N 新建文件、⌘⇧N 新建文件夹、⌘S 保存、⌘F 文件内查找、⌘⇧F 格式化 JSON、⌘⇧M 添加评论。编辑器支持撤销/重做、行号和折叠。

## 数据与备份

默认路径 `~/Documents/data-shelf`，可在左下角更换。正文是普通 UTF-8 文件，评论存放在资料目录的 `.data-shelf/metadata.json`。备份时复制整个目录，**包含隐藏的 `.data-shelf` 文件夹**。

应用内改名/移动会迁移评论。Finder 移走或删除文件后，原评论可在「未关联评论」中查看；可打开目标文件，在「未关联评论」中将评论复制到当前文件，再选择原文重新定位。无法可靠定位的评论不会猜测绑定。外部修改期间未打开的文件，其评论在下次打开时保守地转为待定位。普通编辑遇到重复原文也会保守地转为待定位；格式化采用专门的偏移映射。路径被新文件复用时，旧评论归档到「未关联评论」，不会覆盖。

资料目录同时只允许一个应用实例写入。损坏的评论文件会阻止打开资料目录，避免静默覆盖。仅修改评论不重写正文；当前编辑器编辑混合 LF/CRLF 的正文时会统一为原有的 CRLF 样式。写入使用同目录临时文件、同步和原子替换；这不能替代备份，也不提供跨任意外部编辑器的事务锁。

首版单个文件上限为 10 MiB，目标资料规模为数千份文件。文件树使用虚拟列表，只渲染可见行。导入仅支持 UTF-8 普通文本，跳过隐藏条目，拒绝符号链接及二进制文件；单次导入最多 10,000 个条目。移到废纸篓保留评论元数据，便于恢复。

## 开发

需要 Go 1.27.3+；当前固定 mygo v0.9.0。无 Node、Bun 或 WebView 依赖。

```sh
go mod download
go test ./...
go test -race ./...
go vet ./...
go tool mygo dev
go tool mygo build
```

macOS Apple Silicon 的构建产物位于 `build/darwin-arm64/`。本地构建使用临时签名，尚未进行 Developer ID 公证，不是已公证的对外发行版本。

使用独立测试资料目录启动：

```sh
DATA_SHELF_ROOT=/tmp/data-shelf-demo go tool mygo dev
```

图标可通过 `swift scripts/icon.swift` 重建。`DATA_SHELF_SCREENSHOTS=1 go test -run TestCreateEditFormatAnnotateAndReload` 会输出 `/tmp/data-shelf-light.png` 和 `/tmp/data-shelf-dark.png`。

## 代码结构

- `internal/shelf`：本地文件、导入、冲突检测、格式识别与评论锚点，不依赖 UI。
- `app.go`：应用状态、编辑操作、保存与外部刷新、文件树。
- `views.go`：编辑、Markdown 预览、评论界面。
- `main.go`：原生窗口、菜单、设置和退出生命周期。
- `GLOSSARY.md`、`docs/`：已确认的产品术语与决策。

框架参考：[原生 UI](https://mygo.run/zh/docs/ui)、[代码编辑器](https://mygo.run/zh/docs/ui/code-editor)、[Markdown](https://mygo.run/zh/docs/ui/markdown)。
