package main

import (
 "bufio"
 "bytes"
 "context"
 "encoding/json"
 "encoding/xml"
 "errors"
 "fmt"
 "io"
 "os"
 "os/exec"
 "path/filepath"
 "strings"
 "time"
)

const pluginID = "io.github.code-new-bie.vcs"
const version = "0.2.2"
const maxOutput = 2 * 1024 * 1024

type repo struct {
 Kind string `json:"kind"`
 Path string `json:"path"`
}
type params struct {
 Kind string `json:"kind"`
 Path string `json:"path"`
 File string `json:"file"`
 Action string `json:"action"`
 Message string `json:"message"`
 Confirmed bool `json:"confirmed"`
}
type change struct {
 Path string `json:"path"`
 Status string `json:"status"`
 Original string `json:"original,omitempty"`
}
type capped struct {
 bytes.Buffer
 truncated bool
}
func (b *capped) Write(p []byte) (int,error) {
 n:=len(p)
 left:=maxOutput-b.Len()
 if left<n { b.truncated=true; p=p[:left] }
 _,_=b.Buffer.Write(p)
 return n,nil
}
func run(ctx context.Context, dir,name string,args ...string)(string,error){
 exe,err:=exec.LookPath(name)
 if err!=nil{return "",fmt.Errorf("%s CLI not found in PATH",name)}
 ctx,cancel:=context.WithTimeout(ctx,25*time.Second)
 defer cancel()
 cmd:=exec.CommandContext(ctx,exe,args...)
 cmd.Dir=dir
 env:=[]string{}
 for _,v:=range os.Environ(){
  k,_,_:=strings.Cut(v,"=")
  k=strings.ToUpper(k)
  if strings.HasPrefix(k,"GIT_")||k=="LC_ALL"||k=="LANG"{continue}
  env=append(env,v)
 }
 cmd.Env=append(env,"GIT_TERMINAL_PROMPT=0","GIT_OPTIONAL_LOCKS=0","LC_ALL=C.UTF-8")
 var out,stderr capped
 cmd.Stdout=&out;cmd.Stderr=&stderr;cmd.WaitDelay=2*time.Second
 err=cmd.Run()
 if ctx.Err()!=nil{return "",errors.New("operation timed out")}
 if err!=nil{return "",fmt.Errorf("%s command failed: %s",name,strings.TrimSpace(stderr.String()))}
 if out.truncated{return "",errors.New("output exceeds 2 MiB; select a single file or smaller history")}
 return out.String(),nil
}
func git(ctx context.Context,dir string,args ...string)(string,error){
 return run(ctx,dir,"git",append([]string{"--no-pager","--literal-pathspecs","-c","core.fsmonitor=false","-c","core.quotepath=false","-c","log.showSignature=false"},args...)...)
}
func svn(ctx context.Context,dir string,args ...string)(string,error){
 return run(ctx,dir,"svn",append([]string{"--non-interactive"},args...)...)
}
func inspect(ctx context.Context,p params)(repo,error){
 if !filepath.IsAbs(p.Path){return repo{},errors.New("absolute local directory required")}
 dir,err:=filepath.EvalSymlinks(filepath.Clean(p.Path))
 if err!=nil{return repo{},err}
 s,err:=os.Stat(dir)
 if err!=nil||!s.IsDir(){return repo{},errors.New("directory not found")}
 var root string
 switch p.Kind{
 case "git":root,err=git(ctx,dir,"rev-parse","--show-toplevel")
 case "svn":root,err=svn(ctx,dir,"info","--show-item","wc-root",".")
 default:return repo{},errors.New("kind must be git or svn")
 }
 if err!=nil{return repo{},err}
 root=strings.TrimRight(root,"\r\n")
 if !filepath.IsAbs(root){return repo{},errors.New("invalid repository root")}
 return repo{p.Kind,filepath.Clean(root)},nil
}
func gitChanges(raw string)([]change,error){
 result:=[]change{}
 parts:=strings.Split(raw,"\x00")
 for i:=0;i<len(parts);i++{
  s:=parts[i]
  if s==""{continue}
  if len(s)<4||s[2]!=' '{return nil,errors.New("invalid git status record")}
  c:=change{Path:s[3:],Status:s[:2]}
  if strings.ContainsAny(c.Status,"RC"){
   i++
   if i>=len(parts)||parts[i]==""{return nil,errors.New("missing rename source")}
   c.Original=parts[i]
  }
  result=append(result,c)
 }
 return result,nil
}
func svnChanges(raw string)([]change,error){
 var doc struct{
 Targets []struct{
 Entries []struct{
 Path string `xml:"path,attr"`
 WC struct{
 Item string `xml:"item,attr"`
 Props string `xml:"props,attr"`
 Conflict string `xml:"tree-conflicted,attr"`
 } `xml:"wc-status"`
 } `xml:"entry"`
 } `xml:"target"`
 }
 if err:=xml.Unmarshal([]byte(raw),&doc);err!=nil{return nil,err}
 out:=[]change{}
 for _,t:=range doc.Targets{for _,e:=range t.Entries{
  status:=e.WC.Item
  if e.WC.Props=="modified"||e.WC.Props=="conflicted"{status+=" / props "+e.WC.Props}
  if e.WC.Conflict=="true"{status+=" / tree conflict"}
  out=append(out,change{Path:e.Path,Status:status})
 }}
 return out,nil
}
func fileArg(root,file string)(string,error){
 if file==""{return "",nil}
 // Reject rooted and drive-relative paths on every platform.
 if filepath.IsAbs(file)||filepath.VolumeName(file)!=""||strings.HasPrefix(file,"/")||strings.HasPrefix(file,"\\")||
 (len(file)>=2&&file[1]==':'){return "",errors.New("relative file required")}
 if strings.ContainsAny(file,"\x00\r\n"){return "",errors.New("unsupported file name")}
 // Detect traversal with either separator, without rewriting legitimate
 // backslashes in Unix filenames.
 for _,part:=range strings.Split(strings.ReplaceAll(file,"\\","/"),"/"){
  if part==".."{return "",errors.New("file outside repository")}
 }
 file=filepath.Clean(file)
 if file=="."||!filepath.IsLocal(file){return "",errors.New("file outside repository")}
 // Git and SVN handle symlinks as versioned entries; do not resolve the
 // final path to its target. Reject symlinked ancestor directories instead.
 for parent:=filepath.Dir(file);parent!=".";parent=filepath.Dir(parent){
  info,err:=os.Lstat(filepath.Join(root,parent))
  if err!=nil{if os.IsNotExist(err){continue};return "",err}
  if info.Mode()&os.ModeSymlink!=0{return "",errors.New("symlinked ancestor directory")}
 }
 return file,nil
}
var errMethod=errors.New("method not found")
func handle(ctx context.Context,method string,p params)(any,error){
 switch method{
 case "vcs/discover":return discoverSqlTarget(ctx,p)
 case "sqlFile/decorations/"+pluginID+".sql-folder":return sqlDecorations(ctx,p)
 case "vcs/write":return writeOperation(ctx,p)
 case "vcs/tools":
  result:=map[string]bool{}
  for _,k:=range []string{"git","svn"}{_,e:=exec.LookPath(k);result[k]=e==nil}
  return result,nil
 case "vcs/inspect","vcs/status","vcs/diff","vcs/log","vcs/branches":
 default:return nil,errMethod
 }
 r,e:=inspect(ctx,p)
 if e!=nil{return nil,e}
 if method=="vcs/inspect"{return r,nil}
 f,e:=fileArg(r.Path,p.File)
 if e!=nil{return nil,e}
 if r.Kind=="git"{
 switch method{
 case "vcs/status":
  s,e:=git(ctx,r.Path,"status","--porcelain=v1","-z","--untracked-files=all")
  if e!=nil{return nil,e}
  return gitChanges(s)
 case "vcs/diff":
  a:=[]string{"diff","--no-ext-diff","--no-textconv","--no-color"}
  if f!=""{a=append(a,"--",f)}
  work,e:=git(ctx,r.Path,a...)
  if e!=nil{return nil,e}
  a=append([]string{"diff","--cached","--no-ext-diff","--no-textconv","--no-color"},a[4:]...)
  staged,e:=git(ctx,r.Path,a...)
  if e!=nil{return nil,e}
  return map[string]string{"working":work,"staged":staged},nil
 case "vcs/log":return git(ctx,r.Path,"log","-n","50","--format=%h %ad %an %s","--date=iso-strict","--no-decorate")
 case "vcs/branches":return git(ctx,r.Path,"branch","--all","--no-color","--no-column")
 }}
 switch method{
 case "vcs/status":
  s,e:=svn(ctx,r.Path,"status","--xml","--ignore-externals",".")
  if e!=nil{return nil,e}
  return svnChanges(s)
 case "vcs/diff":
  a:=[]string{"diff","--internal-diff"}
  // Working-copy diff without explicit revisions takes literal paths, not peg syntax.
  if f!=""{a=append(a,"--",f)}else{a=append(a,".")}
  s,e:=svn(ctx,r.Path,a...)
  return map[string]string{"working":s},e
 case "vcs/log":return svn(ctx,r.Path,"log","--limit","50",".")
 case "vcs/branches":return nil,errors.New("SVN branch layout is repository-specific; not implemented")
 }
 return nil,errMethod
}
type request struct{
 JSONRPC string `json:"jsonrpc"`
 ID json.RawMessage `json:"id"`
 Method string `json:"method"`
 Params json.RawMessage `json:"params"`
}
func respond(line []byte)map[string]any{
 var req request
 res:=map[string]any{"jsonrpc":"2.0","id":nil}
 fail:=func(code int,msg string)map[string]any{res["error"]=map[string]any{"code":code,"message":msg};return res}
 if json.Unmarshal(line,&req)!=nil{return fail(-32700,"parse error")}
 if len(req.ID)>0{res["id"]=req.ID}
 if req.JSONRPC!="2.0"||req.Method==""{return fail(-32600,"invalid request")}
 if len(req.ID)==0{return nil}
 switch req.Method{
 case "plugin/initialize":
  var p struct{Host struct{Versions []int `json:"protocolVersions"`} `json:"host"`}
  if json.Unmarshal(req.Params,&p)!=nil{return fail(-32602,"invalid initialization")}
  ok:=false
  for _,v:=range p.Host.Versions{if v==1{ok=true}}
  if !ok{return fail(-32602,"protocol 1 required")}
  res["result"]=map[string]any{"protocolVersion":1,"capabilities":[]string{},"plugin":map[string]string{"id":pluginID,"version":version}}
  return res
 case "plugin/shutdown":res["result"]=map[string]any{};return res
 }
 var p params
 if len(req.Params)>0&&json.Unmarshal(req.Params,&p)!=nil{return fail(-32602,"invalid params")}
 v,e:=handle(context.Background(),req.Method,p)
 if e!=nil{if errors.Is(e,errMethod){return fail(-32601,e.Error())};return fail(-32000,e.Error())}
 res["result"]=v
 return res
}
func serve(in io.Reader,out io.Writer)error{
 scanner:=bufio.NewScanner(in)
 scanner.Buffer(make([]byte,65536),1024*1024)
 enc:=json.NewEncoder(out)
 for scanner.Scan(){if r:=respond(scanner.Bytes());r!=nil{if err:=enc.Encode(r);err!=nil{return err}}}
 return scanner.Err()
}
func main(){if e:=serve(os.Stdin,os.Stdout);e!=nil{fmt.Fprintln(os.Stderr,e);os.Exit(1)}}
