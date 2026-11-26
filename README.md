# DBX VCS Manager

Git / SVN workbench plugin for DBX. 初始开发版本 0.1.0（只读原型，未正式发布）。

## 功能

- 导入可信本地工作区、Git/SVN CLI 检测、收藏仓库（Host Storage 支持时）。
- Git 状态、工作区与暂存区 Diff、最近 50 条历史、分支列表。
- SVN XML 状态、Diff、最近 50 条历史。SVN log 可能访问远程服务器。
- 暂不支持提交、推送、更新、分支切换、冲突处理或 MCP。未跟踪文件的内容不显示在 Git Diff 中。

## Windows x64 开发

要求：Go 1.22+、Git、SVN（使用对应功能时）、Node.js、DBX CLI。

```powershell
cd backend
go test ./...
cd ..
New-Item -ItemType Directory -Force bin/windows-x64 | Out-Null
go -C backend build -o ../bin/windows-x64/backend.exe .
npm install -g @dbx-app/plugin-cli
dbx-plugin dev --path . --port 5190
# 停止开发宿主后打包候选包
dbx-plugin package . --target windows-x64
```

本地候选包需要在 DBX 中启用允许未签名开发包。原生 Sidecar 不受 iframe 沙箱保护；仅导入可信仓库，本版不保存认证信息。实际 DBX 安装、Windows 运行和 SVN 联调仍待验证。

官方文档：https://dbxio.com/cn/docs/plugin-development
