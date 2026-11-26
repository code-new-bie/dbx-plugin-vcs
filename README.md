# DBX VCS Manager

Git / SVN workbench plugin for DBX. 初始开发版本 0.2.1（原生 SQL 集成实验版本）。

## 原生 SQL 文件集成（实验）

需要包含 [DBX 宿主扩展 PR #1](https://github.com/code-new-bie/dbx/pull/1) 的定制 DBX。官方未扩展的 DBX 不支持此 Manifest 的 `sql-file` / `sql-folder` 贡献点；请使用 v0.2.0。

在 DBX 原生 SQL 文件树打开工作目录，右键文件或目录选择 **Git / SVN — SQL 文件版本控制** / **Git / SVN — SQL 目录版本控制**。插件读取宿主传入的绝对路径，自动识别最近的工作副本并定位 Diff；目录操作仍以实际仓库根为范围。它不复制 SQL 文件，不替换原生编辑器，不直接执行 SQL。写操作仍须确认。工具栏的「版本控制」命令打开底部面板，目前需手动选择仓库；原生右键则打开带上下文的工作台。

本阶段不含 SQL 文件树状态标记、原生编辑器行级标记和内部 SQL Library 集成。尚未完成定制 DBX 与插件联合安装验收。

## 功能

- 导入可信本地工作区、Git/SVN CLI 检测、收藏仓库（Host Storage 支持时）。
- Git 状态、工作区与暂存区 Diff、最近 50 条历史、分支列表。
- SVN XML 状态、Diff、最近 50 条历史。SVN log 可能访问远程服务器。
- Git：单路径暂存/取消暂存、提交整个暂存区、Fetch、仅快进 Pull、Push 至当前分支 upstream。Git 需事先配置身份、远程和 upstream；取消暂存在无 HEAD 的新仓库中暂不支持。
- SVN：单路径 Add、Commit（不递归）、Update（保留冲突，忽略 externals）。操作须在工作台明确确认；RPC 确认标记不是宿主级权限边界。
- 暂不支持 Clone/Checkout、分支切换、冲突处理或 MCP。未跟踪文件内容不显示在 Git Diff 中。CLI 单次命令限制 25 秒，超时后请检查实际状态，不应盲目重试。

## Windows x64 开发

要求：Go 1.22+、Git、SVN（使用对应功能时）、Node.js、DBX CLI。

```powershell
cd backend
go test ./...
cd ..
npm install -g @dbx-app/plugin-cli
dbx-plugin dev --path . --port 5190
# 停止开发宿主后打包候选包
dbx-plugin package .
```

CLI 根据 dbx-plugin.toml 自动编译 Go 后端，生成当前平台的安装包；无需手动构建 bin 目录。

本地候选包需要在 DBX 中启用允许未签名开发包。原生 Sidecar 不受 iframe 沙箱保护；仅导入可信仓库，本版不保存认证信息。实际 DBX 安装、Windows 运行和 SVN 联调仍待验证。

官方文档：https://dbxio.com/cn/docs/plugin-development
