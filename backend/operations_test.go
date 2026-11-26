package main

import (
 "context"
 "os"
 "os/exec"
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

func TestWriteSVN(t *testing.T){
 if _,e:=exec.LookPath("svn");e!=nil{t.Skip(e)};if _,e:=exec.LookPath("svnadmin");e!=nil{t.Skip(e)}
 ctx:=context.Background();base:=t.TempDir();r:=filepath.Join(base,"repo");wc:=filepath.Join(base,"wc")
 if _,e:=run(ctx,base,"svnadmin","create",r);e!=nil{t.Fatal(e)}
 url:="file://"+filepath.ToSlash(r);if !strings.HasPrefix(filepath.ToSlash(r),"/"){url="file:///"+filepath.ToSlash(r)}
 if _,e:=svn(ctx,base,"checkout",url,wc);e!=nil{t.Fatal(e)}
 file:="test.txt";if e:=os.WriteFile(filepath.Join(wc,file),[]byte("test\n"),0600);e!=nil{t.Fatal(e)}
 p:=params{Kind:"svn",Path:wc,File:file,Action:"add",Confirmed:true}
 if _,e:=writeOperation(ctx,p);e!=nil{t.Fatal(e)};p.Action="commit";p.Message="initial";if _,e:=writeOperation(ctx,p);e!=nil{t.Fatal(e)}
 p.Action="update";p.File="";if _,e:=writeOperation(ctx,p);e!=nil{t.Fatal(e)}
}
