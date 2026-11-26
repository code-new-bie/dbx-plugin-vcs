package main

import (
 "context"
 "os"
 "os/exec"
 "net/url"
 "runtime"
 "path/filepath"
 "strings"
 "testing"
)

func TestWriteGit(t *testing.T) {
 if _,err:=exec.LookPath("git");err!=nil{t.Skip(err)}
 ctx:=context.Background(); dir:=t.TempDir(); remote:=t.TempDir()
 g:=func(d string,args ...string)string{t.Helper();s,e:=git(ctx,d,args...);if e!=nil{t.Fatal(e)};return s}
 g(remote,"init","--bare");g(dir,"init","-b","main");g(dir,"config","user.name","DBX Test");g(dir,"config","user.email","test@example.invalid")
 file:="hello world.txt";if e:=os.WriteFile(filepath.Join(dir,file),[]byte("first\n"),0600);e!=nil{t.Fatal(e)}
 p:=params{Kind:"git",Path:dir,File:file,Action:"add"}
 if _,e:=writeOperation(ctx,p);e==nil{t.Fatal("unconfirmed operation accepted")}
 p.Confirmed=true
 w:=func(action,file,msg string){t.Helper();p.Action=action;p.File=file;p.Message=msg;if _,e:=writeOperation(ctx,p);e!=nil{t.Fatal(e)}}
 w("add",file,"");w("commit","","initial")
 g(dir,"remote","add","origin",remote);g(dir,"push","-u","origin","main")
 if e:=os.WriteFile(filepath.Join(dir,file),[]byte("second\n"),0600);e!=nil{t.Fatal(e)}
 w("add",file,"");w("unstage",file,"");if s:=g(dir,"diff","--cached","--name-only");s!=""{t.Fatal(s)}
 w("add",file,"");w("commit","","second");w("fetch","","");w("pull","","");w("push","","")
 if s:=g(remote,"log","-1","--format=%s","main");strings.TrimSpace(s)!="second"{t.Fatal(s)}
 p.Action="reset";if _,e:=writeOperation(ctx,p);e==nil{t.Fatal("unsupported action accepted")}
}

// CI must never silently skip SVN validation.
func requireSVN(t *testing.T) {
 t.Helper()
 for _, tool := range []string{"svn", "svnadmin"} {
  if _, err := exec.LookPath(tool); err != nil {
   if os.Getenv("DBX_REQUIRE_SVN") == "1" { t.Fatalf("%s required: %v", tool, err) }
   t.Skipf("%s unavailable: %v", tool, err)
  }
 }
}

func TestWriteSVN(t *testing.T) {
 requireSVN(t)
 ctx := context.Background()
 base := t.TempDir()
 root := filepath.Join(base, "repo with spaces")
 wc := filepath.Join(base, "wc")
 other := filepath.Join(base, "other")
 if _, err := run(ctx, base, "svnadmin", "create", root); err != nil { t.Fatal(err) }
 path := filepath.ToSlash(root)
 if runtime.GOOS == "windows" { path = "/" + path }
 remote := (&url.URL{Scheme:"file", Path:path}).String()
 s := func(dir string, args ...string) string {
  t.Helper()
  out, err := svn(ctx, dir, args...)
  if err != nil { t.Fatal(err) }
  return out
 }
 s(base, "checkout", remote, wc)
 s(base, "checkout", remote, other)
 file := "hello @ world.txt"
 write := func(dir, body string) {
  t.Helper()
  if err := os.WriteFile(filepath.Join(dir, file), []byte(body), 0600); err != nil { t.Fatal(err) }
 }
 read := func(dir string) string {
  t.Helper()
  b, err := os.ReadFile(filepath.Join(dir,file))
  if err != nil { t.Fatal(err) }
  return string(b)
 }
 p := params{Kind:"svn", Path:wc}
 query := func(method, file string) any {
  t.Helper()
  q := p; q.File = file
  result, err := handle(ctx, "vcs/"+method, q)
  if err != nil { t.Fatal(err) }
  return result
 }
 op := func(action, file, message string) {
  t.Helper()
  q := p; q.Action = action; q.File = file; q.Message = message; q.Confirmed = true
  if _, err := writeOperation(ctx, q); err != nil { t.Fatal(err) }
 }
 assertStatus := func(want string) {
  t.Helper()
  rows := query("status", "").([]change)
  for _, row := range rows {
   if filepath.ToSlash(row.Path) == file && strings.Contains(row.Status, want) { return }
  }
  t.Fatalf("expected %s for %q, got %+v",want,file,rows)
 }
 assertClean := func() {
  t.Helper()
  if rows := query("status", "").([]change); len(rows) != 0 { t.Fatalf("expected clean working copy, got %+v",rows) }
 }
 t.Log("inspect_and_unversioned")
 {
  got := query("inspect","").(repo)
  want, err := filepath.EvalSymlinks(wc)
  if err != nil { t.Fatal(err) }
  if got.Kind != "svn" || filepath.Clean(got.Path) != filepath.Clean(want) { t.Fatalf("unexpected repository: %+v",got) }
  assertClean()
  write(wc,"initial\n")
  assertStatus("unversioned")
 }
 t.Log("confirmation_and_add")
 {
  q := p; q.Action = "add"; q.File = file
  if _, err := writeOperation(ctx,q); err == nil { t.Fatal("unconfirmed write accepted") }
  assertStatus("unversioned")
  op("add",file,"")
  assertStatus("added")
 }
 t.Log("commit_and_log")
 {
  op("commit",file,"initial revision")
  assertClean()
  // Update the root so SVN log from '.' includes the file commit.
  op("update","","")
  if log := query("log","").(string); !strings.Contains(log,"initial revision") { t.Fatal(log) }
  s(other,"update")
  if got := read(other); got != "initial\n" { t.Fatal(got) }
 }
 t.Log("diff_and_commit")
 {
  write(wc,"changed locally\n")
  assertStatus("modified")
  diff := query("diff",file).(map[string]string)["working"]
  if !strings.Contains(diff,"-initial") || !strings.Contains(diff,"+changed locally") { t.Fatal(diff) }
  op("commit",file,"second revision")
  assertClean()
 }
 t.Log("update_from_second_working_copy")
 {
  s(other,"update")
  write(other,"changed remotely\n")
  s(other,"commit","-m","remote revision","--",file+"@")
  op("update","","")
  if got := read(wc); got != "changed remotely\n" { t.Fatal(got) }
  assertClean()
  if log := query("log","").(string); !strings.Contains(log,"remote revision") { t.Fatal(log) }
 }
 t.Log("conflict_is_preserved")
 {
  write(wc,"local conflict\n")
  write(other,"remote conflict\n")
  s(other,"commit","-m","conflicting revision","--",file+"@")
  op("update","","")
  assertStatus("conflicted")
  if got := read(wc); !strings.Contains(got,"local conflict") || !strings.Contains(got,"remote conflict") { t.Fatal(got) }
 }
}
