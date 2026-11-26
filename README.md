# DBX VCS Manager

Git / SVN workbench plugin for DBX. 初始开发版本 0.1.0。

第一阶段：只读本地工作区导入、状态、Diff、历史和 Git 分支浏览。需要系统 PATH 中的 Git / SVN CLI。仅导入可信仓库；原生 Sidecar 不受浏览器沙箱保护。

目前尚未通过 DBX 桌面宿主安装验收，不是正式发布版本。

## 开发

```sh
cd backend
go test ./...
```

DBX 官方插件文档：https://dbxio.com/cn/docs/plugin-development
