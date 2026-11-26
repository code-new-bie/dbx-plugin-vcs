"use strict";
const $=id=>document.getElementById(id);
let host,repos=[],active=null,selected=null,pending=null,busy=false,ready=false,queuedContext=null;
const invoke=(method,p={})=>host.invoke("vcs/"+method,p,{timeoutMs:60000});
const labels={add:"添加 / 暂存",unstage:"取消暂存",commit:"提交",fetch:"Fetch",pull:"Pull（仅快进）",push:"Push",update:"SVN Update"};
function notify(s){$("notice").textContent=s;}
function output(s){$("output").textContent=s||"（无内容）";}
function showDiff(d){const parts=[["工作区",d.working],["暂存区",d.staged]].filter(x=>x[1]!==undefined);const pre=$("output");pre.replaceChildren();for(const [name,text] of parts){const h=document.createElement("strong");h.textContent=name+"\n";pre.append(h);for(const line of (text||"（无差异）").split("\n")){const span=document.createElement("span");span.textContent=line+"\n";if(line.startsWith("+")&&!line.startsWith("+++"))span.className="added";if(line.startsWith("-")&&!line.startsWith("---"))span.className="removed";pre.append(span);}}}
function sync(){document.querySelectorAll("button").forEach(b=>b.disabled=busy||!ready);$("branches").disabled=busy||!ready||active?.kind!=="git";$("execute").disabled=busy||!pending||!$("ack").checked||(pending?.action==="commit"&&!$("message").value.trim());}
async function task(fn){if(busy||!ready)return;busy=true;sync();notify("处理中…");try{await fn();notify("");}catch(e){notify(String(e?.message||e));}finally{busy=false;sync();if(queuedContext){const next=queuedContext;queuedContext=null;applySqlContext(next);}}}
async function save(){if(host.capabilities?.storage)await host.storage.set("repositories",repos);}
function closeConfirm(){pending=null;$("confirmation").hidden=true;$("ack").checked=false;$("message").value="";sync();}
function chooseAction(action){
 if(busy||!active)return;
 const needsFile=action==="add"||action==="unstage"||(active.kind==="svn"&&action==="commit");
 if(needsFile&&!selected){notify("请先选择工作区变更文件");return;}
 pending={...active,action,file:needsFile?selected.path:""};
 $("summary").textContent=[labels[action],active.path,pending.file||"整个工作区 / 当前分支",action==="commit"&&active.kind==="git"?"提交当前全部已暂存变更（包括其他客户端暂存的文件）。":"",["fetch","pull","push"].includes(action)?"使用当前分支配置的 upstream；不强制推送，不自动合并。":"",action==="update"?"更新整个 SVN 工作区，冲突将保留待处理。":""].filter(Boolean).join("\n");
 $("confirmation").hidden=false;$("messageLabel").hidden=action!=="commit";$("message").value="";$("ack").checked=false;sync();
}
function actions(){
 $("operations").hidden=!active;
 $("selection").textContent=selected?"已选择："+selected.path:"未选择文件";
 const root=$("actions");root.replaceChildren();if(!active)return;
 const names=active.kind==="git"?["add","unstage","commit","fetch","pull","push"]:["add","commit","update"];
 for(const a of names){const b=document.createElement("button");b.textContent=labels[a];b.onclick=()=>chooseAction(a);root.append(b);}
 sync();
}
function renderRepos(){const root=$("repos");root.replaceChildren();for(const r of repos){const row=document.createElement("div"),b=document.createElement("button"),del=document.createElement("button");b.textContent=r.kind.toUpperCase()+" · "+r.path;b.className=r===active?"active":"";b.onclick=()=>task(async()=>{active=r;renderRepos();await status();});del.textContent="×";del.title="仅移除收藏，不删除文件";del.onclick=()=>task(async()=>{repos=repos.filter(x=>x!==r);if(active===r){active=null;selected=null;closeConfirm();$("title").textContent="请选择仓库";$("changes").replaceChildren();output("");actions();}renderRepos();await save();});row.append(b,del);root.append(row);}}
async function status(){if(!active)throw Error("请选择仓库");selected=null;closeConfirm();actions();$("title").textContent=active.path;$("changes").replaceChildren();output("");const rows=await invoke("status",active);for(const c of rows){const b=document.createElement("button");b.textContent=c.status+"  "+c.path+(c.original?" ← "+c.original:"");b.onclick=()=>task(async()=>{selected=c;closeConfirm();actions();for(const n of $("changes").children)n.classList.toggle("active",n===b);showDiff(await invoke("diff",{...active,file:c.path}));});$("changes").append(b);}if(!rows.length)output("工作区无变更");}
$("add").onsubmit=e=>{e.preventDefault();task(async()=>{const r=await invoke("inspect",{kind:$("kind").value,path:$("path").value.trim()});active=repos.find(x=>x.kind===r.kind&&x.path===r.path)||r;if(!repos.includes(active))repos.push(active);renderRepos();await save();await status();});};
$("refresh").onclick=()=>task(status);
function applySqlContext(context){
 const target=context?.sqlFile;
 if(!target||!["file","folder"].includes(target.kind)||typeof target.path!=="string"||!target.path)return;
 if(busy){queuedContext=context;return;}
 task(async()=>{
  // Reset selection before discovery, including on error, so a failed context
  // cannot leave a previous repository armed for a write.
  active=null;selected=null;closeConfirm();renderRepos();actions();$("changes").replaceChildren();$("title").textContent=target.path;output("正在识别当前 SQL 文件所在仓库…");
  const result=await invoke("discover",{kind:target.kind,path:target.path});
  const r=result.repository;
  active=repos.find(x=>x.kind===r.kind&&x.path===r.path)||r;
  if(!repos.includes(active))repos.push(active);
  renderRepos();await status();
  if(result.file){
   const file=result.file;
   selected={path:file};
   actions();
   $("title").textContent=active.path+" · "+file;
   showDiff(await invoke("diff",{...active,file}));
  }
  // Do not persist a host-selected path unless the user explicitly imports it.
 });
}
function leaveSelection(){selected=null;closeConfirm();actions();$("changes").replaceChildren();}
$("history").onclick=()=>task(async()=>{if(!active)throw Error("请选择仓库");leaveSelection();output(await invoke("log",active));});
$("branches").onclick=()=>task(async()=>{if(active?.kind!=="git")throw Error("仅支持 Git 分支");leaveSelection();output(await invoke("branches",active));});
$("allDiff").onclick=()=>task(async()=>{if(!active)throw Error("请选择仓库");closeConfirm();showDiff(await invoke("diff",active));});
$("ack").onchange=sync;$("message").oninput=sync;$("cancel").onclick=closeConfirm;
$("execute").onclick=()=>{if(!pending||!$("ack").checked)return;const p={...pending,message:$("message").value,confirmed:true};task(async()=>{closeConfirm();const result=await invoke("write",p);try{await status();}catch(e){output("操作已返回成功，刷新失败："+String(e?.message||e)+"\n"+result);return;}output("操作已返回成功。\n"+(result||""));});};
sync();
(async()=>{host=window.dbxPlugin;if(!host){notify("请通过 DBX 工作台或官方开发宿主打开。");return;}await host.ready;const t=await invoke("tools");$("tools").textContent="Git "+(t.git?"✓":"未安装")+" · SVN "+(t.svn?"✓":"未安装");if(host.capabilities?.storage){const data=await host.storage.get("repositories");if(Array.isArray(data))repos=data.filter(r=>["git","svn"].includes(r.kind)&&typeof r.path==="string");}ready=true;renderRepos();sync();if(typeof host.onContext==="function")host.onContext(applySqlContext);applySqlContext(host.context);})().catch(e=>notify(String(e?.message||e)));
